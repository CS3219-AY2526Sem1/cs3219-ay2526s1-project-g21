import { useEffect, useRef, useState } from 'react';

export interface SessionMetrics {
  voiceUsed: boolean;
  voiceDuration: number; // seconds
  codeChanges: number;
}

export interface SessionMetricsData {
  sessionId: string;
  matchId: string;
  user1Id: string;
  user2Id: string;
  difficulty: string;
  sessionDuration: number;
  user1Metrics: SessionMetrics;
  user2Metrics: SessionMetrics;
}

export const useSessionMetrics = (userId: string, isVoiceConnected: boolean) => {
  const [codeChanges, setCodeChanges] = useState(0);
  const sessionStartRef = useRef<number>(Date.now());
  const voiceConnectedAtRef = useRef<number | null>(null);
  const totalVoiceDurationRef = useRef<number>(0);
  const hasUsedVoiceRef = useRef<boolean>(false);

  // Track voice connection time
  useEffect(() => {
    if (isVoiceConnected && !voiceConnectedAtRef.current) {
      // Voice just connected
      voiceConnectedAtRef.current = Date.now();
      hasUsedVoiceRef.current = true;
    } else if (!isVoiceConnected && voiceConnectedAtRef.current) {
      // Voice just disconnected, add to total duration
      const duration = Math.floor((Date.now() - voiceConnectedAtRef.current) / 1000);
      totalVoiceDurationRef.current += duration;
      voiceConnectedAtRef.current = null;
    }
  }, [isVoiceConnected]);

  // Increment code changes counter
  const trackCodeChange = () => {
    setCodeChanges(prev => prev + 1);
  };

  // Get current metrics for this user
  const getMetrics = (): SessionMetrics => {
    // If voice is currently connected, add the current session duration
    let voiceDuration = totalVoiceDurationRef.current;
    if (voiceConnectedAtRef.current) {
      voiceDuration += Math.floor((Date.now() - voiceConnectedAtRef.current) / 1000);
    }

    return {
      voiceUsed: hasUsedVoiceRef.current,
      voiceDuration,
      codeChanges,
    };
  };

  // Get session duration in seconds
  const getSessionDuration = (): number => {
    return Math.floor((Date.now() - sessionStartRef.current) / 1000);
  };

  // Reset metrics (useful for testing or if session restarts)
  const resetMetrics = () => {
    setCodeChanges(0);
    sessionStartRef.current = Date.now();
    voiceConnectedAtRef.current = null;
    totalVoiceDurationRef.current = 0;
    hasUsedVoiceRef.current = false;
  };

  return {
    trackCodeChange,
    trackMessage,
    getMetrics,
    getSessionDuration,
    resetMetrics,
  };
};
