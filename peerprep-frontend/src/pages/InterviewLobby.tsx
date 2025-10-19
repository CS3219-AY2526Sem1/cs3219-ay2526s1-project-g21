import { useEffect, useState } from "react";
import { getMe } from "@/api/auth";
import { joinQueue, getRoomStatus } from "@/api/match";
import { RoomInfo } from "@/types/question";
import { useAuth } from "@/context/AuthContext";
import { handleFormChange } from "@/utils/form";
import { startCase } from "lodash";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";
import InterviewFieldSelector from "@/components/InterviewFieldSelector";

// --- Types ---
interface User {
    id: number;
    username: string;
    email: string;
}

type Difficulty = "easy" | "medium" | "hard";

type Category = "array" | "graphs" | "dynamic programming" | "greedy" | "linked list";

interface InterviewHistoryItem {
    question: string;
    category: string;
    difficulty: string;
    date: string;
}


export default function InterviewLobby() {
    const { token } = useAuth();
    const [user, setUser] = useState<User | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const nav = useNavigate();

    const difficulties: Difficulty[] = ["easy", "medium", "hard"];

    const categories: Category[] = [
        "array",
        "graphs",
        "dynamic programming",
        "greedy",
        "linked list",
    ];

    const [form, setForm] = useState<{ category: Category; difficulty: Difficulty }>({
        category: categories[0],
        difficulty: difficulties[0],
    });

    // Function to wait for room to be ready
    const waitForRoomReady = async (matchId: string, userToken: string) => {
        const maxAttempts = 30; // 30 seconds max wait time
        let attempts = 0;

        const checkRoomStatus = async (): Promise<RoomInfo | null> => {
            try {
                return await getRoomStatus(matchId, userToken);
            } catch (error) {
                console.error("Failed to get room status:", error);
                return null;
            }
        };

        const pollRoom = async (): Promise<void> => {
            if (attempts >= maxAttempts) {
                toast.error("Room setup timed out. Please try again.", {
                    position: "bottom-center",
                    duration: 5000,
                });
                return;
            }

            attempts++;
            const roomInfo = await checkRoomStatus();

            if (!roomInfo) {
                // Room not found yet, wait and retry
                setTimeout(pollRoom, 1000);
                return;
            }

            if (roomInfo.status === "ready") {
                toast.success("Room is ready! Redirecting...", {
                    position: "bottom-center",
                    duration: 3000,
                });
                nav(`/room/${matchId}`);
            } else if (roomInfo.status === "error") {
                toast.error("Failed to set up room. Please try again.", {
                    position: "bottom-center",
                    duration: 5000,
                });
            } else {
                // Still processing, wait and retry
                setTimeout(pollRoom, 1000);
            }
        };

        // Start polling
        setTimeout(pollRoom, 1000);
    };

    const interviewHistoryHeaders: (keyof InterviewHistoryItem)[] = [
        "question",
        "category",
        "difficulty",
        "date",
    ];

    // Placeholder interview history items
    const interviewHistoryItems: InterviewHistoryItem[] = [
        {
            question: "Reverse a linked list",
            category: "Linked List",
            difficulty: "Hard",
            date: "31/08/2025",
        },
        {
            question: "Find value in 2d array",
            category: "Binary Search",
            difficulty: "Medium",
            date: "15/09/2025",
        },
        {
            question: "Best time to buy and sell stock",
            category: "Two Pointers",
            difficulty: "Easy",
            date: "08/10/2025",
        },
    ];

    useEffect(() => {
        let cancelled = false;

        async function load() {
            if (!token) {
                setLoading(false);
                return;
            }
            try {
                const me = await getMe(token);
                if (!cancelled) setUser(me);
            } catch (e) {
                if (!cancelled)
                    setError(e instanceof Error ? e.message : "Failed to load account");
            } finally {
                if (!cancelled) setLoading(false);
            }
        }

        load();
        setLoading(true);

        // WebSocket setup
        const ws = new WebSocket("ws://localhost:8083/ws");

        ws.onopen = () => console.log("Connected to matchmaking WebSocket");

        ws.onmessage = async (event) => {
            try {
                const data: RoomInfo = JSON.parse(event.data);
                console.log("Received match event:", data);
                if (data.user1 == user?.id?.toString() || data.user2 == user?.id?.toString()) {
                    const matchId = data.matchId;
                    const otherUserId = data.user1 == user?.id?.toString() ? data.user2 : data.user1;
                    const userToken = data.user1 == user?.id?.toString() ? data.token1 : data.token2;

                    const matchMessage = `Matched with user ${otherUserId} for ${data.category} (${data.difficulty}). Setting up room...`;

                    toast.success(matchMessage, {
                        position: "bottom-center",
                        duration: 5000,
                    });

                    // Store token in sessionStorage for later use
                    if (userToken) {
                        sessionStorage.setItem(`room_token_${matchId}`, userToken);
                    }

                    // Wait for room to be ready
                    if (userToken) {
                        await waitForRoomReady(matchId, userToken);
                    }
                }
            } catch (err) {
                console.error("Failed to parse WebSocket message", err);
            }
        };

        ws.onclose = () => console.log("Disconnected from WebSocket");

        return () => {
            cancelled = true;
            ws.close();
        };
    }, [token, user?.id]);

    return (
        <section className="mx-auto flex flex-col gap-16 px-4 py-12 sm:px-6 lg:max-w-5xl">
            {/* New Interview Section */}
            <section>
                <h1 className="text-3xl font-semibold text-black">Start a New Interview</h1>
                <section className="mt-8 flex flex-col gap-10 rounded-xl border border-gray-200 bg-white px-5 py-6 shadow-sm sm:px-8">
                    <section className="grid gap-6 md:grid-cols-2">
                        <InterviewFieldSelector
                            name="category"
                            fieldOptions={categories}
                            onChange={(e) => handleFormChange(e, setForm)}
                        />
                        <InterviewFieldSelector
                            name="difficulty"
                            fieldOptions={difficulties}
                            onChange={(e) => handleFormChange(e, setForm)}
                        />
                    </section>
                    <section className="flex flex-col items-stretch gap-3 sm:flex-row sm:justify-center sm:gap-4">
                        <button
                            onClick={() =>
                                joinQueue(user?.id, form.category, form.difficulty)
                            }
                            className="w-full rounded-md bg-[#2F6FED] px-7 py-2.5 text-lg font-medium text-white hover:brightness-95 sm:w-auto"
                            disabled={!user}
                        >
                            Start Interviewing!
                        </button>
                    </section>
                </section>
            </section>

            {/* Past Interviews Section */}
            <section>
                <h1 className="text-3xl font-semibold text-black">Past Interviews</h1>
                <section className="mt-8 rounded-xl border border-gray-200 bg-white shadow-sm">
                    <div className="w-full overflow-x-auto">
                        <table className="w-full min-w-[600px]">
                            <thead className="bg-gray-50">
                                <tr>
                                    {interviewHistoryHeaders.map((x) => (
                                        <th key={x} className="px-4 py-3 text-left text-sm font-semibold text-slate-700">
                                            {startCase(x)}
                                        </th>
                                    ))}
                                </tr>
                            </thead>
                            <tbody>
                                {interviewHistoryItems.map((interviewItem) => (
                                    <tr
                                        key={interviewItem.question + interviewItem.date}
                                        className="border-t border-gray-200"
                                    >
                                        {interviewHistoryHeaders.map((header) => (
                                            <td key={header} className="px-4 py-3 text-sm text-slate-700">
                                                {interviewItem[header]}
                                            </td>
                                        ))}
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </section>
            </section>
        </section>
    );
}
