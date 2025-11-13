import { RoomInfo } from "@/types/question";

const MATCH_API_BASE = (import.meta as any).env?.VITE_MATCH_API_BASE || "http://localhost:8083";

const COLLAB_API_BASE = (import.meta as any).env?.VITE_COLLAB_API_BASE || "http://localhost:8084";

export async function checkUserPreExistingMatch(userId: number | undefined) {
    const res = await fetch(`${MATCH_API_BASE}/api/v1/match/check?userId=${userId?.toString()}`, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json'
        },
    })

    const data = await res.json();
    return data;
}

export async function joinQueue(userId: number | undefined, category: string, difficulty: string): Promise<void> {
    if (!userId) {
        console.error("User ID not provided for matchmaking");
        return;
    }

    const res = await fetch(`${MATCH_API_BASE}/api/v1/match/join`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ userId: userId.toString(), category, difficulty }),
    });

    if (!res.ok) {
        console.error("Failed to join queue:", res.status, await res.text());
        return;
    }

    const data = await res.json();
    console.log(data);
}

export async function cancelQueue(userId: number | undefined) {
    const res = await fetch(`${MATCH_API_BASE}/api/v1/match/cancel`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ userId: userId?.toString() }),
    })

    const data = await res.json();
    return data;
}

export async function acceptMatch(userId: number | undefined, matchId: string | null) {
    const res = await fetch(`${MATCH_API_BASE}/api/v1/match/handshake`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ userId: userId?.toString(), matchId, accept: true })
    });

    console.log(res);
}

export async function exitRoom(userId: number | undefined) {
    try {
        const res = await fetch(`${MATCH_API_BASE}/api/v1/match/done`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ userId: userId?.toString() })
        });

        console.log(res);
    }
    catch (e) {
        console.log(e);
    }
}

export async function getRoomStatus(matchId: string, token: string): Promise<RoomInfo> {
    const res = await fetch(`${COLLAB_API_BASE}/api/v1/collab/room/${matchId}`, {
        headers: {
            "Authorization": `Bearer ${token}`,
        },
    });

    if (!res.ok) {
        throw new Error(`Failed to get room status: ${res.status}`);
    }

    return res.json();
}

export async function rerollQuestion(matchId: string, token: string): Promise<RoomInfo> {
    const res = await fetch(`${COLLAB_API_BASE}/api/v1/collab/room/${matchId}/reroll`, {
        method: "POST",
        headers: {
            "Authorization": `Bearer ${token}`,
        },
    });

    if (!res.ok) {
        const message = await res.text();
        throw new Error(message || `Failed to reroll question: ${res.status}`);
    }

    return res.json();
}

export interface SessionMetrics {
    voiceUsed: boolean;
    voiceDuration: number;
    codeChanges: number;
}

export interface SessionFeedbackPayload {
    sessionId: string;
    matchId: string;
    user1Id: string;
    user2Id: string;
    difficulty: string;
    sessionDuration: number;
    user1Metrics: SessionMetrics;
    user2Metrics: SessionMetrics;
}

export async function submitSessionFeedback(feedback: SessionFeedbackPayload): Promise<void> {
    try {
        const res = await fetch(`${MATCH_API_BASE}/api/v1/match/session/feedback`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify(feedback),
        });

        if (!res.ok) {
            console.error("Failed to submit session feedback:", res.status, await res.text());
            return;
        }

        const data = await res.json();
        console.log("Session feedback submitted successfully:", data);
    } catch (error) {
        console.error("Error submitting session feedback:", error);
    }
}
