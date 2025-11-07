import { CSSProperties, useEffect, useRef, useState } from "react";
import { getMe } from "@/api/auth";
import { getUserHistory, InterviewHistoryItem } from "@/api/history";
import { joinQueue, getRoomStatus, checkUserPreExistingMatch, cancelQueue, acceptMatch, exitRoom } from "@/api/match";
import { RoomInfo, Category, Difficulty, MatchEvent } from "@/types/question";
import { useAuth } from "@/context/AuthContext";
import { handleFormChange } from "@/utils/form";
import { startCase } from "lodash";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";
import InterviewFieldSelector from "@/components/InterviewFieldSelector";
import { InterviewDetailsModal } from "@/components/InterviewDetailsModal";
import { BarLoader, } from "react-spinners";
import useBeforeClose from "@/hooks/useBeforeClose";

const MATCH_WEBSOCKET_BASE = (import.meta as any).env?.VITE_MATCH_WEBSOCKET_BASE || "ws://localhost:8083";

const override: CSSProperties = {
  display: "block",
  margin: "0 auto",
};

// --- Types ---
interface User {
  id: number;
  username: string;
  email: string;
}

export default function InterviewLobby() {
  const { token } = useAuth();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [inQueue, setInQueue] = useState(false);
  const [matchFound, setMatchFound] = useState(false);
  const [roomId, setRoomId] = useState<string | null>(null);

  const criteria2MessageTimer = useRef<NodeJS.Timeout>();
  const criteria3MessageTimer = useRef<NodeJS.Timeout>();
  const exitMessageTimer = useRef<NodeJS.Timeout>();

  const criteria1LoadingText = "Searching for users with the same preferences..."
  const criteria2LoadingText = "Searching for other users with different preferred difficulty..."
  const criteria3LoadingText = "Searching for other users with different preferred category..."

  const criteria1Timeout = 100000; // in miliseconds - 100 seconds
  const criteria2Timeout = 200000; // in miliseconds - 200 seconds
  const exitTimeout = 300000; // in miliseconds - 300 seconds  

  const [loadingText, setLoadingText] = useState(criteria1LoadingText)

  const nav = useNavigate();

  useBeforeClose(() => {
    cancelQueue(user?.id);
  })

  const difficulties: readonly Difficulty[] = ["Easy", "Medium", "Hard"];

  const categories: readonly Category[] = [
    "Array",
    "Graphs",
    "Dynamic Programming",
    "Greedy",
    "Linked List",
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
        exitRoom(user?.id);
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
        exitRoom(user?.id);
      } else {
        // Still processing, wait and retry
        setTimeout(pollRoom, 1000);
      }
    };

    // Start polling
    setTimeout(pollRoom, 1000);
  };

  const startSearching = () => {
    joinQueue(user?.id, form.category, form.difficulty)
    setInQueue(true);
    criteria2MessageTimer.current = setTimeout(() => {
      if (!roomId) {
        setLoadingText(criteria2LoadingText)
      }
    }, criteria1Timeout)

    criteria3MessageTimer.current = setTimeout(() => {
      setLoadingText(criteria3LoadingText)
    }, criteria2Timeout)

    exitMessageTimer.current = setTimeout(() => {
      setInQueue(false);
      cancelQueue(user?.id);
      toast.error("Unable to find match at this time :( Please try again later", {
        position: "bottom-center",
        duration: 5000,
      });
    }, exitTimeout)
  }

  const clearSearchMessageTimeouts = () => {
    clearTimeout(criteria2MessageTimer.current);
    clearTimeout(criteria3MessageTimer.current);
    clearTimeout(exitMessageTimer.current);
  }

  const cancelSearching = () => {
    cancelQueue(user?.id);
    setInQueue(false);
    setLoadingText(criteria1LoadingText)
  }

  const handshake = () => {
    acceptMatch(user?.id, roomId)
    setLoadingText(criteria1LoadingText)
  }

  const [interviewHistory, setInterviewHistory] = useState<InterviewHistoryItem[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [selectedInterview, setSelectedInterview] = useState<InterviewHistoryItem | null>(null);

  const interviewHistoryHeaders = [
    "Question",
    "Category",
    "Difficulty",
    "Language",
    "Duration",
    "Date",
    "Actions",
  ] as const;

  const loadHistory = async (userId: number) => {
    setHistoryLoading(true);
    try {
      const history = await getUserHistory(String(userId));
      setInterviewHistory(history);
    } catch (error) {
      console.error("Failed to load interview history:", error);
    } finally {
      setHistoryLoading(false);
    }
  };

  const formatDate = (isoDate: string) => {
    try {
      const date = new Date(isoDate);
      return date.toLocaleDateString();
    } catch {
      return isoDate;
    }
  };

  const formatDuration = (seconds: number) => {
    const minutes = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${minutes}m ${secs}s`;
  };

  useEffect(() => {
    let cancelled = false;

    async function load() {
      if (!token) {
        setLoading(false);
        return;
      }
      try {
        const me = await getMe(token);
        if (!cancelled) {
          setUser(me);
          // Load interview history
          loadHistory(me.id);
        }
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
    if (!user) {
      return () => {
        cancelled = true;
      };
    }

    async function redirectToPreExistingMatch() {
      const data = await checkUserPreExistingMatch(user?.id)

      if (data.inRoom) {
        const userToken = data.token;
        const matchId = data.roomId;

        nav(`/room/${matchId}`);

        // Store token in sessionStorage for later use
        if (userToken) {
          sessionStorage.setItem(`room_token_${matchId}`, userToken);
        }

        // Wait for room to be ready
        if (userToken) {
          await waitForRoomReady(matchId, userToken);
        }
      }
    }

    redirectToPreExistingMatch();

    const ws = new WebSocket(`${MATCH_WEBSOCKET_BASE}/api/v1/match/ws?userId=${user?.id}`);

    ws.onopen = () => console.log("Connected to matchmaking WebSocket");

    ws.onmessage = async (event) => {
      try {
        const data: MatchEvent = JSON.parse(event.data);
        console.log("Received match event:", data);

        switch (data.type) {
          case "match_confirmed":
            setInQueue(false);
            const matchId = data.matchId;
            const userToken = data.token;

            const matchMessage = `Matched for ${data.category} (${data.difficulty}). Setting up room...`;

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
              setMatchFound(false);
            }
            break;

          case "match_pending":
            clearSearchMessageTimeouts();
            setRoomId(data.matchId);
            setInQueue(false);
            setMatchFound(true);
            break;

          case "requeued":
            setInQueue(true);
            setMatchFound(false);
            setLoadingText(criteria1LoadingText);
            toast.error("Your partner didn't join in time :( Putting you back into the matchmaking queue", {
              position: "bottom-center",
              duration: 5000,
            });
            break;

          case "timeout":
            cancelQueue(user?.id);
            setInQueue(false);
            setMatchFound(false);
            setLoadingText(criteria1LoadingText);
            break;
        }
      } catch (err) {
        console.error("Failed to parse WebSocket message", err);
      }
    };
    ws.onclose = () => console.log("Disconnected from WebSocket");

    return () => {
      cancelled = true;
      ws.close();
      cancelQueue(user?.id);
    };
  }, [token, user?.id]);


  return (
    <>
      <InterviewDetailsModal
        interview={selectedInterview}
        currentUserId={String(user?.id || "")}
        onClose={() => setSelectedInterview(null)}
      />
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
                onClick={startSearching}
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
                {historyLoading ? (
                  <tr>
                    <td colSpan={7} className="text-center py-8 text-gray-500">
                      Loading history...
                    </td>
                  </tr>
                ) : interviewHistory.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="text-center py-8 text-gray-500">
                      No past interviews yet
                    </td>
                  </tr>
                ) : (
                  interviewHistory.map((interviewItem) => (
                    <tr
                      key={interviewItem.matchId}
                      className="border-t border-black"
                    >
                      <td className="text-left pl-4 py-4">
                        {interviewItem.questionTitle}
                      </td>
                      <td className="text-left pl-4 py-4">
                        {startCase(interviewItem.category)}
                      </td>
                      <td className="text-left pl-4 py-4">
                        {startCase(interviewItem.difficulty)}
                      </td>
                      <td className="text-left pl-4 py-4">
                        {startCase(interviewItem.language)}
                      </td>
                      <td className="text-left pl-4 py-4">
                        {formatDuration(interviewItem.durationSeconds)}
                      </td>
                      <td className="text-left pl-4 py-4">
                        {formatDate(interviewItem.endedAt)}
                      </td>
                      <td className="text-left pl-4 py-4">
                        <button
                          onClick={() => setSelectedInterview(interviewItem)}
                          className="text-blue-600 hover:text-blue-800 underline"
                        >
                          View
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </section>
        </section>
      </section>
      {inQueue && (
        <div className="top-0 left-0 absolute w-dvw h-dvh bg-[rgba(0,0,0,0.5)] flex items-center justify-center">
          <div className="bg-white flex flex-col items-center w-[600px] py-6 gap-4 rounded-md">
            <BarLoader
              color={"#2F6FED"}
              loading={inQueue}
              cssOverride={override}
              height={12}
              width={200}
              speedMultiplier={0.5}
            />
            {loadingText}
            <button
              onClick={cancelSearching}
              className="rounded-md bg-red-500 px-7 py-2 text-lg font-medium text-white hover:brightness-95"
              disabled={!user}
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {matchFound && (
        <div className="top-0 left-0 absolute w-dvw h-dvh bg-[rgba(0,0,0,0.5)] flex items-center justify-center">
          <div className="bg-white flex flex-col items-center w-[600px] py-6 gap-4 rounded-md">
            Match Found!
            <button
              onClick={handshake}
              className="rounded-md bg-[#2F6FED] px-7 py-2 text-lg font-medium text-white hover:brightness-95"
              disabled={!user}
            >
              Join Interview!
            </button>
          </div>
        </div >
      )
      }
    </>
  );
}
