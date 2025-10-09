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
