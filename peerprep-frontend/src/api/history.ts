const USER_API_BASE = (import.meta as any).env?.VITE_USER_API_BASE || "http://localhost:8081";
const COLLAB_API_BASE = (import.meta as any).env?.VITE_COLLAB_API_BASE || "http://localhost:8084";

export interface InterviewHistoryItem {
  matchId: string;
  user1Id: string;
  user1Name: string;
  user2Id: string;
  user2Name: string;
  questionId: number;
  questionTitle: string;
  category: string;
  difficulty: string;
  language: string;
  finalCode: string;
  startedAt: string;
  endedAt: string;
  durationSeconds: number;
  rerollsUsed: number;
}

export interface ActiveRoomResponse {
  active: boolean;
  matchId?: string;
  status?: string;
  token?: string;
}

/**
 * Get user's interview history
 */
export async function getUserHistory(userId: string): Promise<InterviewHistoryItem[]> {
  const response = await fetch(`${USER_API_BASE}/api/v1/users/history/${userId}`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch history: ${response.statusText}`);
  }

  const data = await response.json();

  // Return all fields from backend
  return data;
}

/**
 * Check if user has an active room
 */
export async function checkActiveRoom(userId: string): Promise<ActiveRoomResponse> {
  const response = await fetch(`${COLLAB_API_BASE}/api/v1/collab/room/active/${userId}`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });

  if (!response.ok) {
    return { active: false };
  }

  return response.json();
}
