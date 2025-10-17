import { useCallback, useEffect, useRef, useState } from 'react';
import type {
  VoiceChatConfig,
  VoiceChatState,
  SignalingMessage,
  RoomStatus,
  WebRTCConfig,
} from '@/types/voiceChat';

const VOICE_SERVICE_URL = 'http://localhost:8085';
const VOICE_SERVICE_WS = 'ws://localhost:8085';

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
  const remoteStreamsRef = useRef<Map<string, MediaStream>>(new Map());
  const webrtcConfigRef = useRef<RTCConfiguration | null>(null);
  const configRef = useRef(config);

  // Keep config ref up to date
  useEffect(() => {
    configRef.current = config;
  }, [config]);

  // Fetch WebRTC configuration from backend
  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const res = await fetch(`${VOICE_SERVICE_URL}/api/v1/webrtc/config`);
        if (res.ok) {
          const config: WebRTCConfig = await res.json();
          webrtcConfigRef.current = config;
          console.log('Loaded WebRTC config:', config);
        } else {
          // Fallback to default STUN servers
          webrtcConfigRef.current = {
            iceServers: [
              { urls: ['stun:stun.l.google.com:19302'] },
            ],
          };
        }
      } catch (error) {
        console.error('Failed to fetch WebRTC config, using defaults:', error);
        webrtcConfigRef.current = {
          iceServers: [
            { urls: ['stun:stun.l.google.com:19302'] },
          ],
        };
      }
    };
    fetchConfig();
  }, []);

  const createPeerConnection = useCallback((remoteUserId: string) => {
    if (!webrtcConfigRef.current) {
      console.error('WebRTC config not loaded yet');
      return null;
    }

    const peerConnection = new RTCPeerConnection(webrtcConfigRef.current);

    // Add local stream tracks
    if (localStreamRef.current) {
      localStreamRef.current.getTracks().forEach(track => {
        peerConnection.addTrack(track, localStreamRef.current!);
      });
    }

    // Handle incoming tracks
    peerConnection.ontrack = (event) => {
      console.log('Received remote track from', remoteUserId);
      const remoteStream = event.streams[0];
      remoteStreamsRef.current.set(remoteUserId, remoteStream);

      // Create audio element for this remote user
      const audio = new Audio();
      audio.srcObject = remoteStream;
      audio.autoplay = true;
      audio.play().catch(err => console.error('Error playing audio:', err));
    };

    // Handle ICE candidates
    peerConnection.onicecandidate = (event) => {
      if (event.candidate && wsRef.current) {
        const currentConfig = configRef.current;
        const message: SignalingMessage = {
          type: 'ice-candidate',
          from: currentConfig.userId,
          to: remoteUserId,
          roomId: currentConfig.roomId,
          data: event.candidate,
          timestamp: new Date().toISOString(),
        };
        wsRef.current.send(JSON.stringify(message));
      }
    };

    // Handle connection state changes
    peerConnection.onconnectionstatechange = () => {
      console.log(`Peer connection with ${remoteUserId}:`, peerConnection.connectionState);
      if (peerConnection.connectionState === 'failed' ||
          peerConnection.connectionState === 'disconnected') {
        // Clean up this peer connection
        peerConnectionsRef.current.delete(remoteUserId);
        remoteStreamsRef.current.delete(remoteUserId);
      }
    };

    peerConnectionsRef.current.set(remoteUserId, peerConnection);
    return peerConnection;
  }, []);

  const handleSignalingMessage = useCallback(async (message: SignalingMessage | RoomStatus) => {
    const currentConfig = configRef.current;

    switch (message.type) {
      case 'room-status': {
        const roomStatus = message as RoomStatus;
        setState(prev => ({
          ...prev,
          participants: roomStatus.users || [],
        }));

        // When we get room status, check if there are other users to connect to
        if (roomStatus.users) {
          const otherUsers = roomStatus.users.filter((u: { id: string }) => u.id !== currentConfig.userId);
          for (const user of otherUsers) {
            // Only initiate call if we don't already have a connection
            if (!peerConnectionsRef.current.has(user.id)) {
              console.log('Found user to connect to:', user.username);

              // Create peer connection and initiate call inline to avoid dependency issues
              const peerConnection = createPeerConnection(user.id);
              if (peerConnection) {
                try {
                  const offer = await peerConnection.createOffer();
                  await peerConnection.setLocalDescription(offer);

                  if (wsRef.current) {
                    const message: SignalingMessage = {
                      type: 'offer',
                      from: currentConfig.userId,
                      to: user.id,
                      roomId: currentConfig.roomId,
                      data: offer,
                      timestamp: new Date().toISOString(),
                    };
                    wsRef.current.send(JSON.stringify(message));
                  }
                } catch (error) {
                  console.error('Error creating offer:', error);
                }
              }
            }
          }
        }
        break;
      }

      case 'offer': {
        const offerMsg = message as SignalingMessage;
        console.log('Received offer from', offerMsg.from);

        let peerConnection = peerConnectionsRef.current.get(offerMsg.from);
        if (!peerConnection) {
          const newConnection = createPeerConnection(offerMsg.from);
          if (newConnection) {
            peerConnection = newConnection;
          }
        }

        if (peerConnection) {
          try {
            await peerConnection.setRemoteDescription(new RTCSessionDescription(offerMsg.data));
            const answer = await peerConnection.createAnswer();
            await peerConnection.setLocalDescription(answer);

            // Send answer
            if (wsRef.current) {
              const answerMessage: SignalingMessage = {
                type: 'answer',
                from: currentConfig.userId,
                to: offerMsg.from,
                roomId: currentConfig.roomId,
                data: answer,
                timestamp: new Date().toISOString(),
              };
              wsRef.current.send(JSON.stringify(answerMessage));
            }
          } catch (error) {
            console.error('Error handling offer:', error);
          }
        }
        break;
      }

      case 'answer': {
        const answerMsg = message as SignalingMessage;
        console.log('Received answer from', answerMsg.from);
        const peerConnection = peerConnectionsRef.current.get(answerMsg.from);
        if (peerConnection) {
          try {
            await peerConnection.setRemoteDescription(new RTCSessionDescription(answerMsg.data));
          } catch (error) {
            console.error('Error handling answer:', error);
          }
        }
        break;
      }

      case 'ice-candidate': {
        const candidateMsg = message as SignalingMessage;
        const peerConnection = peerConnectionsRef.current.get(candidateMsg.from);
        if (peerConnection) {
          try {
            await peerConnection.addIceCandidate(new RTCIceCandidate(candidateMsg.data));
          } catch (error) {
            console.error('Error adding ICE candidate:', error);
          }
        }
        break;
      }

      default:
        console.log('Unknown message type:', message.type);
    }
  }, [createPeerConnection]);

  const connect = useCallback(async () => {
    if (!webrtcConfigRef.current) {
      setState(prev => ({ ...prev, error: 'WebRTC configuration not loaded' }));
      return;
    }

    const currentConfig = configRef.current;

    try {
      // Get user media
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
        video: false,
      });
      localStreamRef.current = stream;

      // Connect to WebSocket with token
      const wsUrl = `${VOICE_SERVICE_WS}/api/v1/room/${currentConfig.roomId}/voice?token=${encodeURIComponent(currentConfig.token)}`;
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        console.log('Connected to voice chat WebSocket');
        setState(prev => ({ ...prev, isConnected: true, error: null }));

        // Send join message
        const joinMessage: SignalingMessage = {
          type: 'join',
          from: currentConfig.userId,
          to: '',
          roomId: currentConfig.roomId,
          data: {
            userId: currentConfig.userId,
            username: currentConfig.username,
          },
          timestamp: new Date().toISOString(),
        };
        ws.send(JSON.stringify(joinMessage));
      };

      ws.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data);
          handleSignalingMessage(message);
        } catch (error) {
          console.error('Error parsing message:', error);
        }
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
      if (error instanceof Error) {
        if (error.name === 'NotAllowedError') {
          setState(prev => ({ ...prev, error: 'Microphone access denied' }));
        } else if (error.name === 'NotFoundError') {
          setState(prev => ({ ...prev, error: 'No microphone found' }));
        } else {
          setState(prev => ({ ...prev, error: 'Failed to access microphone' }));
        }
      }
    }
  }, [handleSignalingMessage]);

  const disconnect = useCallback(() => {
    const currentConfig = configRef.current;

    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      const leaveMessage: SignalingMessage = {
        type: 'leave',
        from: currentConfig.userId,
        to: '',
        roomId: currentConfig.roomId,
        data: {},
        timestamp: new Date().toISOString(),
      };
      wsRef.current.send(JSON.stringify(leaveMessage));
      wsRef.current.close();
    }

    // Stop local stream
    if (localStreamRef.current) {
      localStreamRef.current.getTracks().forEach(track => track.stop());
      localStreamRef.current = null;
    }

    // Close all peer connections
    peerConnectionsRef.current.forEach(pc => pc.close());
    peerConnectionsRef.current.clear();

    // Clear remote streams
    remoteStreamsRef.current.clear();

    setState(prev => ({ ...prev, isConnected: false, participants: [] }));
  }, []);

  const toggleMute = useCallback(() => {
    const currentConfig = configRef.current;

    if (localStreamRef.current) {
      const audioTrack = localStreamRef.current.getAudioTracks()[0];
      if (audioTrack) {
        audioTrack.enabled = !audioTrack.enabled;
        const newMutedState = !audioTrack.enabled;
        setState(prev => ({ ...prev, isMuted: newMutedState }));

        // Send mute/unmute message
        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
          const message: SignalingMessage = {
            type: newMutedState ? 'mute' : 'unmute',
            from: currentConfig.userId,
            to: '',
            roomId: currentConfig.roomId,
            data: {},
            timestamp: new Date().toISOString(),
          };
          wsRef.current.send(JSON.stringify(message));
        }
      }
    }
  }, []);

  const toggleDeaf = useCallback(() => {
    setState(prev => {
      const newDeafState = !prev.isDeaf;

      // Mute/unmute all remote streams
      remoteStreamsRef.current.forEach(stream => {
        stream.getAudioTracks().forEach(track => {
          track.enabled = !newDeafState;
        });
      });

      return { ...prev, isDeaf: newDeafState };
    });
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
