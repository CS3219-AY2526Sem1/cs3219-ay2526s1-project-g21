import { RoomInfo } from "@/types/question";

export async function checkUserPreExistingMatch(userId: number | undefined) {
    const res = await fetch(`http://localhost:8083/check?userId=${userId?.toString()}`, {
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

    const res = await fetch("http://localhost:8083/join", {
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
    const res = await fetch(`http://localhost:8083/cancel`, {
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
    const res = await fetch('http://localhost:8083/handshake', {
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
        const res = await fetch('http://localhost:8083/done', {
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
    const res = await fetch(`http://localhost:8084/api/v1/room/${matchId}`, {
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
    const res = await fetch(`http://localhost:8084/api/v1/room/${matchId}/reroll`, {
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
