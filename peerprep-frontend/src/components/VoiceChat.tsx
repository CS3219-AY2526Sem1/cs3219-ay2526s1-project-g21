import React, { useImperativeHandle, forwardRef } from "react";
import { useVoiceChat } from "@/hooks/useVoiceChat";
import type { VoiceChatConfig } from "@/types/voiceChat";

type VoiceChatProps = VoiceChatConfig & {
  onConnectionChange?: (isConnected: boolean) => void;
};

const VoiceChat: React.FC<VoiceChatProps> = ({
  roomId,
  userId,
  username,
  token,
  onConnectionChange,
}) => {
  const {
    isConnected,
    isMuted,
    isDeaf,
    participants,
    error,
    connect,
    disconnect,
    toggleMute,
    toggleDeaf,
  } = useVoiceChat({ roomId, userId, username, token });

  // Notify parent when connection state changes
  React.useEffect(() => {
    onConnectionChange?.(isConnected);
  }, [isConnected, onConnectionChange]);

  const handleConnect = () => {
    if (!isConnected) {
      connect();
    } else {
      disconnect();
    }
  };

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-gray-900">Voice Chat</h3>
        <div className="flex items-center space-x-2">
          <div
            className={`w-3 h-3 rounded-full ${
              isConnected ? "bg-green-500" : "bg-red-500"
            }`}
          />
          <span className="text-sm text-gray-600">
            {isConnected ? "Connected" : "Disconnected"}
          </span>
        </div>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-md">
          <p className="text-sm text-red-600">{error}</p>
        </div>
      )}

      <div className="space-y-4">
        {/* Connection Controls */}
        <div className="flex space-x-2">
          <button
            onClick={handleConnect}
            className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
              isConnected
                ? "bg-red-600 text-white hover:bg-red-700"
                : "bg-blue-600 text-white hover:bg-blue-700"
            }`}
          >
            {isConnected ? "Disconnect" : "Connect"}
          </button>

          {isConnected && (
            <>
              <button
                onClick={toggleMute}
                className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                  isMuted
                    ? "bg-red-600 text-white hover:bg-red-700"
                    : "bg-gray-600 text-white hover:bg-gray-700"
                }`}
              >
                {isMuted ? "Unmute" : "Mute"}
              </button>

              <button
                onClick={toggleDeaf}
                className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                  isDeaf
                    ? "bg-red-600 text-white hover:bg-red-700"
                    : "bg-gray-600 text-white hover:bg-gray-700"
                }`}
              >
                {isDeaf ? "Undeaf" : "Deaf"}
              </button>
            </>
          )}
        </div>

        {/* Participants */}
        {isConnected && participants.length > 0 && (
          <div>
            <h4 className="text-sm font-medium text-gray-700 mb-2">
              Participants
            </h4>
            <div className="space-y-2">
              {participants.map((participant) => (
                <div
                  key={participant.id}
                  className="flex items-center justify-between p-2 bg-gray-50 rounded-md"
                >
                  <div className="flex items-center space-x-2">
                    <div className="w-2 h-2 bg-green-500 rounded-full" />
                    <span className="text-sm text-gray-900">
                      {participant.username}
                    </span>
                    {participant.id === userId && (
                      <span className="text-xs text-blue-600">(You)</span>
                    )}
                  </div>
                  <div className="flex items-center space-x-2">
                    {participant.isMuted && (
                      <span className="text-xs text-red-600">Muted</span>
                    )}
                    {participant.isDeaf && (
                      <span className="text-xs text-red-600">Deaf</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Status Messages */}
        {!isConnected && (
          <div className="text-center py-4">
            <p className="text-sm text-gray-500">
              Click "Connect" to join the voice chat
            </p>
          </div>
        )}

        {isConnected && participants.length === 0 && (
          <div className="text-center py-4">
            <p className="text-sm text-gray-500">
              Waiting for other participants...
            </p>
          </div>
        )}
      </div>
    </div>
  );
};

VoiceChat.displayName = "VoiceChat";

export default VoiceChat;
