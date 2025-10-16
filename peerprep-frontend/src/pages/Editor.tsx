import { useEffect, useMemo, useRef, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import toast from "react-hot-toast";
import CodeEditor from "@uiw/react-textarea-code-editor";
import { getRoomStatus } from "@/api/match";
import { RoomInfo, Question } from "@/types/question";

type WSFrame =
  | { type: "init"; data: { sessionId: string; doc: { text: string; version: number }; language: string } }
  | { type: "doc"; data: { text: string; version: number } }
  | { type: "cursor"; data: { userId: string; pos: number } }
  | { type: "chat"; data: { userId: string; message: string } }
  | { type: "stdout"; data: string }
  | { type: "stderr"; data: string }
  | { type: "exit"; data: { code: number; timedOut: boolean } }
  | { type: "language"; data: string }
  | { type: "error"; data: string };

const CODE_TEMPLATES: Record<string, string> = {
  javascript: 'console.log("Hello from JavaScript!");\n',
  typescript: 'function main(): void {\n  console.log("Hello from TypeScript!");\n}\n\nmain();\n',
  python: 'print("Hello from Python!")\n',
  cpp: '#include <iostream>\n\nint main() {\n    std::cout << "Hello from C++!" << std::endl;\n    return 0;\n}\n',
  java: 'public class Main {\n    public static void main(String[] args) {\n        System.out.println("Hello from Java!");\n    }\n}\n',
};

export default function Editor() {
  const { roomId } = useParams<{ roomId: string }>();

  const [question, setQuestion] = useState<Question | null>(null);
  const [roomInfo, setRoomInfo] = useState<RoomInfo | null>(null);
  const [roomLoading, setRoomLoading] = useState<boolean>(true);
  const [language, setLanguage] = useState<string>("python");
  const [code, setCode] = useState<string>(CODE_TEMPLATES["python"] ?? "");
  const [docVersion, setDocVersion] = useState<number>(0);
  const [stdout, setStdout] = useState<string>("");
  const [stderr, setStderr] = useState<string>("");
  const [exitInfo, setExitInfo] = useState<{ code: number | null; timedOut: boolean } | null>(null);
  const [runError, setRunError] = useState<string | null>(null);
  const [isRunning, setIsRunning] = useState<boolean>(false);

  const wsRef = useRef<WebSocket | null>(null);

  const nav = useNavigate();

  const resetRunOutputs = () => {
    setStdout("");
    setStderr("");
    setExitInfo(null);
    setRunError(null);
  };

  const applyTemplate = (lang: string) => {
    const template = CODE_TEMPLATES[lang];
    if (!template) return;

    const currentCode = code;
    const currentVersion = docVersion;
    setCode(template);
    resetRunOutputs();
    setIsRunning(false);

    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({
          type: "edit",
          data: {
            baseVersion: currentVersion,
            rangeStart: 0,
            rangeEnd: currentCode.length,
            text: template,
          },
        })
      );
      setDocVersion(currentVersion + 1);
    }
  };

  // Fetch room status and question
  useEffect(() => {
    if (!roomId) return;

    const fetchRoomInfo = async () => {
      try {
        setRoomLoading(true);
        
        // Get token from sessionStorage
        const token = sessionStorage.getItem(`room_token_${roomId}`);
        if (!token) {
          toast.error("No access token found. Please join a room first.", {
            position: "bottom-center",
            duration: 5000,
          });
          nav("/lobby");
          return;
        }

        const room = await getRoomStatus(roomId, token);
        setRoomInfo(room);

        if (room.status === "ready" && room.question) {
          setQuestion(room.question);
        } else if (room.status === "error") {
          toast.error("Failed to load room. Please try again.", {
            position: "bottom-center",
            duration: 5000,
          });
          nav("/lobby");
        } else {
          // Room is still processing, wait and retry
          setTimeout(fetchRoomInfo, 1000);
        }
      } catch (error) {
        console.error("Failed to fetch room info:", error);
        toast.error("Failed to load room. Please try again.", {
          position: "bottom-center",
          duration: 5000,
        });
        nav("/lobby");
      } finally {
        setRoomLoading(false);
      }
    };

    fetchRoomInfo();
  }, [roomId, nav]);

  // WebSocket collab setup
  useEffect(() => {
    if (!roomInfo) return;

    // Get token from sessionStorage
    const token = sessionStorage.getItem(`room_token_${roomInfo.matchId}`);
    if (!token) {
      toast.error("No access token found. Please join a room first.", {
        position: "bottom-center",
        duration: 5000,
      });
      nav("/lobby");
      return;
    }

    const ws = new WebSocket(`http://localhost:8084/ws/session/${roomInfo.matchId}?token=${encodeURIComponent(token)}`);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("Connected to collab service", roomInfo.matchId);
      ws.send(
        JSON.stringify({
          type: "init",
          data: { sessionId: roomInfo.matchId, language },
        })
      );
    };

    ws.onmessage = (msg) => {
      const frame: WSFrame = JSON.parse(msg.data);

      switch (frame.type) {
        case "init":
          setLanguage(frame.data.language ?? language);
          setCode(frame.data.doc.text);
          setDocVersion(frame.data.doc.version);
          break;
        case "doc":
          setCode(frame.data.text);
          setDocVersion(frame.data.version);
          break;
        case "language":
          setLanguage(frame.data);
          break;
        case "stdout":
          setStdout((prev) => prev + frame.data);
          break;
        case "stderr":
          setStderr((prev) => prev + frame.data);
          break;
        case "exit":
          setExitInfo({ code: frame.data.code, timedOut: frame.data.timedOut });
          setIsRunning(false);
          break;
        case "error":
          console.log("Error!");
          if (typeof frame.data === "string") {
            if (frame.data === "room_full") {
              toast.error("Invalid Room!", {
                position: "bottom-center",
                duration: 5000,
              });

              nav(`/interview`);
            }

            if (frame.data === "version_mismatch") {
              console.error("WS version mismatch:", frame.data);
              break;
            }
          }
          setRunError(typeof frame.data === "string" ? frame.data : "Unexpected error");
          setIsRunning(false);
          console.error("WS error:", frame.data);
          break;
        default:
          // ignore cursor/chat/stdout/stderr for now
          break;
      }
    };

    ws.onclose = () => console.log("Collab WS closed");

    return () => ws.close();
  }, [roomInfo, language]);

  const handleChange = (evn: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newCode = evn.target.value;
    setCode(newCode);

    wsRef.current?.send(
      JSON.stringify({
        type: "edit",
        data: {
          baseVersion: docVersion,
          rangeStart: 0,
          rangeEnd: code.length,
          text: newCode,
        },
      })
    );
  };

  const handleLanguageChange = (nextLang: string) => {
    const nextTemplate = CODE_TEMPLATES[nextLang];
    const previousTemplate = CODE_TEMPLATES[language];
    const trimmed = code.trim();
    const shouldReplace =
      trimmed.length === 0 || (previousTemplate && trimmed === previousTemplate.trim());

    setLanguage(nextLang);
    resetRunOutputs();
    setIsRunning(false);

    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({
          type: "language",
          data: {
            language: nextLang,
          },
        })
      );
    }

    if (shouldReplace && nextTemplate) {
      applyTemplate(nextLang);
    }
  };

  const handleRun = () => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      setRunError("Not connected to collaboration service");
      return;
    }

    const executableLanguages = new Set(["python", "java", "cpp"]);
    if (!executableLanguages.has(language)) {
      setRunError("Execution is not available for the selected language");
      return;
    }

    resetRunOutputs();
    setIsRunning(true);

    wsRef.current.send(
      JSON.stringify({
        type: "run",
        data: {
          language,
          code,
        },
      })
    );
  };

  // Show loading state while room is being set up
  if (roomLoading || !roomInfo) {
    return (
      <div className="mx-auto w-full px-0 md:px-2">
        <div className="mb-4 flex items-center justify-between px-6">
          <div>
            <h1 className="text-2xl font-semibold text-black">Collaborative Editor</h1>
            <p className="text-sm text-gray-500">Room: {roomId ?? "new"}</p>
          </div>
        </div>
        <div className="flex items-center justify-center h-64">
          <div className="text-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto mb-4"></div>
            <p className="text-gray-600">Setting up room and fetching question...</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full px-0 md:px-2">
      <div className="mb-4 flex items-center justify-between px-6">
        <div>
          <h1 className="text-2xl font-semibold text-black">Collaborative Editor</h1>
          <p className="text-sm text-gray-500">Room: {roomId ?? "new"}</p>
        </div>
        <div className="flex items-center gap-2">
          <select
            value={language}
            onChange={(e) => handleLanguageChange(e.target.value)}
            className="rounded-md border border-gray-300 bg-white px-3 py-2 text-sm"
          >
            <option value="javascript">JavaScript</option>
            <option value="typescript">TypeScript</option>
            <option value="python">Python</option>
            <option value="cpp">C++</option>
            <option value="java">Java</option>
          </select>
          <button
            type="button"
            onClick={handleRun}
            disabled={isRunning}
            className="rounded-md bg-[#2F6FED] px-3 py-2 text-white text-sm hover:brightness-95 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isRunning ? "Running..." : "Run"}
          </button>
        </div>
      </div>

      <div className="flex flex-col lg:flex-row gap-4 lg:gap-6 px-6">
        {/* Question panel */}
        <div className="order-2 lg:order-1 space-y-4 lg:w-1/2">
          <div className="rounded-lg border border-gray-200 bg-white p-5">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold text-black">{question?.title ?? "Loading..."}</h2>
              {question?.difficulty && (
                <span className="text-xs rounded-full bg-gray-100 px-2 py-1 text-gray-600">
                  {question.difficulty}
                </span>
              )}
            </div>
            <p className="mt-3 text-sm leading-6 text-gray-700 whitespace-pre-wrap">
              {question?.prompt_markdown}
            </p>
            {question?.topic_tags?.length ? (
              <div className="mt-4 flex flex-wrap gap-2">
                {question.topic_tags.map((t) => (
                  <span key={t} className="text-xs rounded-md bg-gray-100 px-2 py-1 text-gray-600">
                    {t}
                  </span>
                ))}
              </div>
            ) : null}
          </div>
        </div>

        {/* Code Editor */}
        <div className="order-1 lg:order-2 lg:w-1/2">
          <div className="rounded-lg border border-gray-200 bg-white">
            <div className="border-b border-gray-200 px-4 py-2 text-sm text-gray-600">
              Editor — {language}
            </div>
            <div className="p-3">
              <CodeEditor
                value={code}
                language={language}
                placeholder={`Write ${language} code...`}
                onChange={handleChange}
                data-color-mode="dark"
                padding={15}
                style={{
                  backgroundColor: "#1E1E1E ",
                  height: "60vh",
                  borderRadius: "5px",
                  fontFamily:
                    "ui-monospace,SFMono-Regular,SF Mono,Consolas,Liberation Mono,Menlo,monospace",
                }}
              />
            </div>
            <div className="border-t border-gray-200 px-4 py-3 text-sm">
              <div className="mb-2 font-medium text-gray-700">Run Output</div>
              {runError ? (
                <div className="rounded-md bg-red-50 px-3 py-2 text-red-700">{runError}</div>
              ) : (
                <div className="space-y-3">
                  <div>
                    <div className="text-xs uppercase tracking-wide text-gray-500">stdout</div>
                    <pre className="mt-1 max-h-40 overflow-y-auto rounded bg-gray-900 p-3 text-xs text-green-200">
                      {stdout || (isRunning ? "Waiting for output..." : "No output")}
                    </pre>
                  </div>
                  <div>
                    <div className="text-xs uppercase tracking-wide text-gray-500">stderr</div>
                    <pre className="mt-1 max-h-40 overflow-y-auto rounded bg-gray-900 p-3 text-xs text-red-200">
                      {stderr || (isRunning ? "" : "No errors")}
                    </pre>
                  </div>
                  {exitInfo && (
                    <div className="text-xs text-gray-500">
                      Exit code: {exitInfo.code} {exitInfo.timedOut ? "(timed out)" : ""}
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
