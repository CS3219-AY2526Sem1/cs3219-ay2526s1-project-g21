export interface VoiceChatConfig {
  roomId: string;
  userId: string;
  username: string;
  token: string; // JWT token from match service
}

export interface VoiceChatState {
  isConnected: boolean;
  isMuted: boolean;
  isDeaf: boolean;
  participants: Participant[];
  error: string | null;
}

export interface Participant {
  id: string;
  username: string;
  isMuted: boolean;
  isDeaf: boolean;
  joinedAt: string;
}

export interface SignalingMessage {
  type: 'offer' | 'answer' | 'ice-candidate' | 'join' | 'leave' | 'mute' | 'unmute' | 'room-status';
  from: string;
  to: string;
  roomId: string;
  data: any;
  timestamp: string;
}

export interface RoomStatus {
  type: 'room-status';
  roomId: string;
  users: Participant[];
  userCount: number;
  timestamp: string;
}

export interface WebRTCConfig {
  iceServers: Array<{
    urls: string[];
    username?: string;
    credential?: string;
  }>;
}
