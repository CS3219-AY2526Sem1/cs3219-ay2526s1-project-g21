import { useCallback, useEffect, useRef, useState } from 'react';

interface VoiceChatConfig {
  roomId: string;
  userId: string;
  username: string;
}

interface VoiceChatState {
  isConnected: boolean;
  isMuted: boolean;
  isDeaf: boolean;
  participants: Participant[];
  error: string | null;
}

interface Participant {
  id: string;
  username: string;
  isMuted: boolean;
  isDeaf: boolean;
}

interface SignalingMessage {
  type: 'offer' | 'answer' | 'ice-candidate' | 'join' | 'leave' | 'mute' | 'unmute' | 'room-status';
  from: string;
  to: string;
  roomId: string;
  data: any;
  timestamp: string;
}

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
  const audioRef = useRef<HTMLAudioElement | null>(null);

  // WebRTC configuration
  const webrtcConfig: RTCConfiguration = {
    iceServers: [
      { urls: 'stun:stun.l.google.com:19302' },
      { urls: 'stun:stun1.l.google.com:19302' },
    ],
  };

  const connect = useCallback(async () => {
    try {
      // Get user media
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: true,
        video: false,
      });
      localStreamRef.current = stream;

      // Create audio element for remote audio
      if (!audioRef.current) {
        audioRef.current = new Audio();
        audioRef.current.autoplay = true;
      }

      // Connect to WebSocket
      const ws = new WebSocket(`ws://localhost:8084/api/v1/room/${config.roomId}/voice`);
      wsRef.current = ws;

      ws.onopen = () => {
        console.log('Connected to voice chat');
        setState(prev => ({ ...prev, isConnected: true, error: null }));

        // Send join message
        const joinMessage: SignalingMessage = {
          type: 'join',
          from: config.userId,
          to: '',
          roomId: config.roomId,
          data: {
            userId: config.userId,
            username: config.username,
          },
          timestamp: new Date().toISOString(),
        };
        ws.send(JSON.stringify(joinMessage));
      };

      ws.onmessage = (event) => {
        const message: SignalingMessage = JSON.parse(event.data);
        handleSignalingMessage(message);
      };

      ws.onclose = () => {
        console.log('Disconnected from voice chat');
        setState(prev => ({ ...prev, isConnected: false }));
      };

      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        setState(prev => ({ ...prev, error: 'Connection error' }));
      };

    } catch (error) {
      console.error('Failed to connect to voice chat:', error);
      setState(prev => ({ ...prev, error: 'Failed to access microphone' }));
    }
  }, [config]);

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      const leaveMessage: SignalingMessage = {
        type: 'leave',
        from: config.userId,
        to: '',
        roomId: config.roomId,
        data: {},
        timestamp: new Date().toISOString(),
      };
      wsRef.current.send(JSON.stringify(leaveMessage));
      wsRef.current.close();
    }

    // Stop local stream
    if (localStreamRef.current) {
      localStreamRef.current.getTracks().forEach(track => track.stop());
    }

    // Close all peer connections
    peerConnectionsRef.current.forEach(pc => pc.close());
    peerConnectionsRef.current.clear();

    setState(prev => ({ ...prev, isConnected: false, participants: [] }));
  }, [config]);

  const toggleMute = useCallback(() => {
    if (localStreamRef.current) {
      const audioTrack = localStreamRef.current.getAudioTracks()[0];
      if (audioTrack) {
        audioTrack.enabled = !audioTrack.enabled;
        setState(prev => ({ ...prev, isMuted: !audioTrack.enabled }));

        // Send mute/unmute message
        if (wsRef.current) {
          const message: SignalingMessage = {
            type: audioTrack.enabled ? 'unmute' : 'mute',
            from: config.userId,
            to: '',
            roomId: config.roomId,
            data: {},
            timestamp: new Date().toISOString(),
          };
          wsRef.current.send(JSON.stringify(message));
        }
      }
    }
  }, [config]);

  const toggleDeaf = useCallback(() => {
    setState(prev => ({ ...prev, isDeaf: !prev.isDeaf }));
  }, []);

  const handleSignalingMessage = useCallback((message: SignalingMessage) => {
    switch (message.type) {
      case 'room-status':
        setState(prev => ({
          ...prev,
          participants: message.data.users || [],
        }));
        break;
      case 'offer':
        handleOffer(message);
        break;
      case 'answer':
        handleAnswer(message);
        break;
      case 'ice-candidate':
        handleICECandidate(message);
        break;
      default:
        console.log('Unknown message type:', message.type);
    }
  }, []);

  const handleOffer = useCallback(async (message: SignalingMessage) => {
    try {
      const peerConnection = new RTCPeerConnection(webrtcConfig);
      peerConnectionsRef.current.set(message.from, peerConnection);

      // Add local stream
      if (localStreamRef.current) {
        localStreamRef.current.getTracks().forEach(track => {
          peerConnection.addTrack(track, localStreamRef.current!);
        });
      }

      // Handle remote stream
      peerConnection.ontrack = (event) => {
        if (audioRef.current && event.streams[0]) {
          audioRef.current.srcObject = event.streams[0];
        }
      };

      // Set remote description
      await peerConnection.setRemoteDescription(message.data);

      // Create answer
      const answer = await peerConnection.createAnswer();
      await peerConnection.setLocalDescription(answer);

      // Send answer
      if (wsRef.current) {
        const answerMessage: SignalingMessage = {
          type: 'answer',
          from: config.userId,
          to: message.from,
          roomId: config.roomId,
          data: answer,
          timestamp: new Date().toISOString(),
        };
        wsRef.current.send(JSON.stringify(answerMessage));
      }
    } catch (error) {
      console.error('Error handling offer:', error);
    }
  }, [config, webrtcConfig]);

  const handleAnswer = useCallback(async (message: SignalingMessage) => {
    const peerConnection = peerConnectionsRef.current.get(message.from);
    if (peerConnection) {
      await peerConnection.setRemoteDescription(message.data);
    }
  }, []);

  const handleICECandidate = useCallback(async (message: SignalingMessage) => {
    const peerConnection = peerConnectionsRef.current.get(message.from);
    if (peerConnection) {
      await peerConnection.addIceCandidate(message.data);
    }
  }, []);

  // Cleanup on unmount
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
