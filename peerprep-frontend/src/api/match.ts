import { RoomInfo } from "@/types/question";

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

export async function getRoomStatus(matchId: string): Promise<RoomInfo> {
    const res = await fetch(`http://localhost:8082/api/v1/room/${matchId}`);
    
    if (!res.ok) {
        throw new Error(`Failed to get room status: ${res.status}`);
    }

    return res.json();
}
