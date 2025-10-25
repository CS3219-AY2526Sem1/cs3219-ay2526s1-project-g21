export interface InterviewHistoryItem {
  matchId: string;
  questionTitle: string;
  category: string;
  difficulty: string;
  language: string;
  endedAt: string;
  durationSeconds: number;
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
  const response = await fetch(`http://localhost:8080/api/history/${userId}`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch history: ${response.statusText}`);
  }

  const data = await response.json();

  // Transform backend data to frontend format
  return data.map((item: any) => ({
    matchId: item.matchId,
    questionTitle: item.questionTitle,
    category: item.category,
    difficulty: item.difficulty,
    language: item.language,
    endedAt: item.endedAt,
    durationSeconds: item.durationSeconds,
  }));
}

/**
 * Check if user has an active room
 */
export async function checkActiveRoom(userId: string): Promise<ActiveRoomResponse> {
  const response = await fetch(`http://localhost:8084/api/v1/room/active/${userId}`, {
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
