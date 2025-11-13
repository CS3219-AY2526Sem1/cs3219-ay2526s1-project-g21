import { useCallback, useEffect, useRef, useState } from 'react';
import type {
  VoiceChatConfig,
  VoiceChatState,
  SignalingMessage,
  RoomStatus,
} from '@/types/voiceChat';

const VOICE_API_BASE = (import.meta as any).env?.VITE_VOICE_API_BASE || "http://localhost:8085";
const VOICE_WEBSOCKET_BASE = (import.meta as any).env?.VITE_VOICE_WEBSOCKET_BASE || "ws://localhost:8085";

export const useVoiceChat = (config: VoiceChatConfig) => {
  const [state, setState] = useState<VoiceChatState>({
    isConnected: false,
    isMuted: false,
    isDeaf: false,
    participants: [],
    error: null,
  });

  const wsRef = useRef<WebSocket | null>(null);
  const peerConnectionsRef = useRef<Map<string, RTCPeerConnection>>(new Map());
  const localStreamRef = useRef<MediaStream | null>(null);
  const audioElementsRef = useRef<Map<string, HTMLAudioElement>>(new Map());

  const createPeerConnection = useCallback((remoteUserId: string, rtcConfig: RTCConfiguration) => {
    const pc = new RTCPeerConnection(rtcConfig);

    // Add local tracks
    localStreamRef.current?.getTracks().forEach(track => {
      pc.addTrack(track, localStreamRef.current!);
    });

    // Handle remote tracks
    pc.ontrack = (event) => {
      const [remoteStream] = event.streams;

      // Clean up old audio element if exists
      const oldAudio = audioElementsRef.current.get(remoteUserId);
      if (oldAudio) {
        oldAudio.pause();
        oldAudio.srcObject = null;
      }

      // Create new audio element
      const audio = new Audio();
      audio.srcObject = remoteStream;
      audio.autoplay = true;
      audio.muted = state.isDeaf;
      audioElementsRef.current.set(remoteUserId, audio);

      audio.play().catch(err => console.error('Audio play failed:', err));
    };

    // Handle ICE candidates
    pc.onicecandidate = (event) => {
      if (event.candidate && wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({
          type: 'ice-candidate',
          from: config.userId,
          to: remoteUserId,
          roomId: config.roomId,
          data: event.candidate,
          timestamp: new Date().toISOString(),
        }));
      }
    };

    // Auto cleanup on failure or disconnect
    pc.onconnectionstatechange = () => {
      if (pc.connectionState === 'failed' ||
          pc.connectionState === 'disconnected' ||
          pc.connectionState === 'closed') {
        peerConnectionsRef.current.delete(remoteUserId);
        const audio = audioElementsRef.current.get(remoteUserId);
        if (audio) {
          audio.pause();
          audio.srcObject = null;
          audioElementsRef.current.delete(remoteUserId);
        }
      }
    };

    peerConnectionsRef.current.set(remoteUserId, pc);
    return pc;
  }, [config.userId, config.roomId, state.isDeaf]);

  const handleSignaling = useCallback(async (msg: SignalingMessage | RoomStatus) => {
    // Get fresh config
    const rtcConfig: RTCConfiguration = await fetch(`${VOICE_API_BASE}/api/v1/voice/webrtc/config`)
      .then(r => r.ok ? r.json() : { iceServers: [{ urls: ['stun:stun.l.google.com:19302'] }] })
      .catch(() => ({ iceServers: [{ urls: ['stun:stun.l.google.com:19302'] }] }));

    switch (msg.type) {
      case 'room-status': {
        const { users = [] } = msg as RoomStatus;
        setState(prev => ({ ...prev, participants: users }));

        const userIds = new Set(users.map((u: { id: string }) => u.id));

        // Clean up peer connections for users who left
        peerConnectionsRef.current.forEach((pc, userId) => {
          if (!userIds.has(userId)) {
            pc.close();
            peerConnectionsRef.current.delete(userId);
            const audio = audioElementsRef.current.get(userId);
            if (audio) {
              audio.pause();
              audio.srcObject = null;
              audioElementsRef.current.delete(userId);
            }
          }
        });

        // Connect to new users or reconnect to users with stale connections
        users
          .filter((u: { id: string }) => u.id !== config.userId)
          .forEach(async (user: { id: string }) => {
            const existingPc = peerConnectionsRef.current.get(user.id);

            // Skip if we have a good connection
            if (existingPc && existingPc.connectionState === 'connected') {
              return;
            }

            // Clean up stale connection if exists
            if (existingPc) {
              existingPc.close();
              peerConnectionsRef.current.delete(user.id);
            }

            const pc = createPeerConnection(user.id, rtcConfig);

            // Only create offer if we have lower userId
            if (config.userId < user.id) {
              const offer = await pc.createOffer();
              await pc.setLocalDescription(offer);
              wsRef.current?.send(JSON.stringify({
                type: 'offer',
                from: config.userId,
                to: user.id,
                roomId: config.roomId,
                data: offer,
                timestamp: new Date().toISOString(),
              }));
            }
          });
        break;
      }

      case 'offer': {
        const { from, data } = msg as SignalingMessage;
        let pc = peerConnectionsRef.current.get(from);

        // Recreate if in bad state or closed
        if (pc && (pc.signalingState !== 'stable' || pc.connectionState === 'closed')) {
          pc.close();
          peerConnectionsRef.current.delete(from);
          pc = createPeerConnection(from, rtcConfig);
        } else if (!pc) {
          pc = createPeerConnection(from, rtcConfig);
        }

        await pc.setRemoteDescription(new RTCSessionDescription(data));
        const answer = await pc.createAnswer();
        await pc.setLocalDescription(answer);

        wsRef.current?.send(JSON.stringify({
          type: 'answer',
          from: config.userId,
          to: from,
          roomId: config.roomId,
          data: answer,
          timestamp: new Date().toISOString(),
        }));
        break;
      }

      case 'answer': {
        const { from, data } = msg as SignalingMessage;
        const pc = peerConnectionsRef.current.get(from);
        if (pc?.signalingState === 'have-local-offer') {
          await pc.setRemoteDescription(new RTCSessionDescription(data));
        }
        break;
      }

      case 'ice-candidate': {
        const { from, data } = msg as SignalingMessage;
        const pc = peerConnectionsRef.current.get(from);
        if (pc?.remoteDescription) {
          await pc.addIceCandidate(new RTCIceCandidate(data));
        }
        break;
      }
    }
  }, [config.userId, config.roomId, createPeerConnection]);

  const connect = useCallback(async () => {
    try {
      // Get microphone
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
      });
      localStreamRef.current = stream;

      // Connect WebSocket
      const ws = new WebSocket(
        `${VOICE_WEBSOCKET_BASE}/api/v1/voice/room/${config.roomId}/voice?token=${encodeURIComponent(config.token)}`
      );
      wsRef.current = ws;

      ws.onopen = () => {
        setState(prev => ({ ...prev, isConnected: true, error: null }));
        ws.send(JSON.stringify({
          type: 'join',
          from: config.userId,
          roomId: config.roomId,
          data: { userId: config.userId, username: config.username },
          timestamp: new Date().toISOString(),
        }));
      };

      ws.onmessage = (event) => {
        handleSignaling(JSON.parse(event.data)).catch(console.error);
      };

      ws.onclose = () => setState(prev => ({ ...prev, isConnected: false }));
      ws.onerror = () => setState(prev => ({ ...prev, error: 'Connection error' }));

    } catch (error: any) {
      const errorMap: Record<string, string> = {
        NotAllowedError: 'Microphone access denied',
        NotFoundError: 'No microphone found',
      };
      setState(prev => ({ ...prev, error: errorMap[error?.name] || 'Failed to access microphone' }));
    }
  }, [config, handleSignaling]);

  const disconnect = useCallback(() => {
    // Send leave message
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: 'leave',
        from: config.userId,
        roomId: config.roomId,
        data: {},
        timestamp: new Date().toISOString(),
      }));
      wsRef.current.close();
    }
    wsRef.current = null;

    // Stop all tracks
    localStreamRef.current?.getTracks().forEach(track => track.stop());
    localStreamRef.current = null;

    // Close all peer connections
    peerConnectionsRef.current.forEach(pc => pc.close());
    peerConnectionsRef.current.clear();

    // Clean up all audio elements
    audioElementsRef.current.forEach(audio => {
      audio.pause();
      audio.srcObject = null;
    });
    audioElementsRef.current.clear();

    setState({ isConnected: false, isMuted: false, isDeaf: false, participants: [], error: null });
  }, [config.userId, config.roomId]);

  const toggleMute = useCallback(() => {
    const track = localStreamRef.current?.getAudioTracks()[0];
    if (track) {
      track.enabled = !track.enabled;
      setState(prev => ({ ...prev, isMuted: !track.enabled }));

      // Notify server
      wsRef.current?.send(JSON.stringify({
        type: track.enabled ? 'unmute' : 'mute',
        from: config.userId,
        roomId: config.roomId,
        data: {},
        timestamp: new Date().toISOString(),
      }));
    }
  }, [config.userId, config.roomId]);

  const toggleDeaf = useCallback(() => {
    const newDeaf = !state.isDeaf;

    // Mute/unmute all audio elements
    audioElementsRef.current.forEach(audio => {
      audio.muted = newDeaf;
    });

    setState(prev => ({ ...prev, isDeaf: newDeaf }));
  }, [state.isDeaf]);

  // Disconnect when component unmounts (e.g., navigating away)
  useEffect(() => {
    return () => {
      disconnect();
    };
  }, [disconnect]);

  return {
    ...state,
    connect,
    disconnect,
    toggleMute,
    toggleDeaf,
  };
};
