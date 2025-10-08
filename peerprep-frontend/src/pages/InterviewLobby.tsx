import { useEffect, useState } from "react";
import { getMe } from "@/api/auth";
import { joinQueue } from "@/api/match";
import { useAuth } from "@/context/AuthContext";
import { handleFormChange } from "@/utils/form";
import { startCase } from "lodash";
import toast from "react-hot-toast";
import InterviewFieldSelector from "@/components/InterviewFieldSelector";

// --- Types ---
interface User {
    id: number;
    username: string;
    email: string;
}

type Difficulty = "easy" | "medium" | "hard";

type Category = "arrays" | "graphs" | "dynamic programming" | "greedy" | "linked list";

interface InterviewHistoryItem {
    question: string;
    category: string;
    difficulty: string;
    date: string;
}

interface MatchEvent {
    matchId: string;
    user1: number;
    user2: number;
    category: string;
    difficulty: string;
}

export default function InterviewLobby() {
    const { token } = useAuth();
    const [user, setUser] = useState<User | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const difficulties: Difficulty[] = ["easy", "medium", "hard"];

    const categories: Category[] = [
        "arrays",
        "graphs",
        "dynamic programming",
        "greedy",
        "linked list",
    ];

    const [form, setForm] = useState<{ category: Category; difficulty: Difficulty }>({
        category: categories[0],
        difficulty: difficulties[0],
    });

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

        ws.onmessage = (event) => {
            try {
                const data: MatchEvent = JSON.parse(event.data);

                if (data.user1 === user?.id || data.user2 === user?.id) {
                    const matchId = data.matchId;
                    const otherUserId = data.user1 === user?.id ? data.user2 : data.user1;

                    const matchMessage = `Matched with user ${otherUserId} for ${data.category} (${data.difficulty}). Room ID: ${matchId}`;

                    toast.success(matchMessage, {
                        position: "bottom-center",
                        duration: 5000,
                    });
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
        <section className="mx-auto max-w-5xl px-6 flex flex-col gap-20">
            {/* New Interview Section */}
            <section>
                <h1 className="text-3xl font-semibold text-black">Start a New Interview</h1>
                <section className="border border-gray-600 rounded-md flex flex-col gap-12 mt-8 px-10 py-6">
                    <section className="flex gap-28">
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
                    <section className="flex gap-4 justify-center">
                        <button
                            onClick={() =>
                                joinQueue(user?.id, form.category, form.difficulty)
                            }
                            className="rounded-md bg-[#2F6FED] px-7 py-2 text-lg font-medium text-white hover:brightness-95"
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
                <section className="border border-gray-600 rounded-md flex flex-col gap-12 mt-8">
                    <table className="rounded-md w-full">
                        <thead>
                            <tr>
                                {interviewHistoryHeaders.map((x) => (
                                    <th key={x} className="text-left pl-4 py-4">
                                        {startCase(x)}
                                    </th>
                                ))}
                            </tr>
                        </thead>
                        <tbody>
                            {interviewHistoryItems.map((interviewItem) => (
                                <tr
                                    key={interviewItem.question + interviewItem.date}
                                    className="border-t border-black"
                                >
                                    {interviewHistoryHeaders.map((header) => (
                                        <td key={header} className="text-left pl-4 py-4">
                                            {interviewItem[header]}
                                        </td>
                                    ))}
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </section>
            </section>
        </section>
    );
}
