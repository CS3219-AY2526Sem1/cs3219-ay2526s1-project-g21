import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import toast from "react-hot-toast";
import MonacoEditor, { type OnMount } from "@monaco-editor/react";;
import ReactMarkdown from "react-markdown";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { oneDark } from "react-syntax-highlighter/dist/esm/styles/prism";
import VoiceChat from "@/components/VoiceChat";
import { useAuth } from "@/context/AuthContext";
import { getMe } from "@/api/auth";
import { getRoomStatus, rerollQuestion, submitSessionFeedback } from "@/api/match";
import { RoomInfo, Question } from "@/types/question";
import AiAssistantDropdown from "@/components/AiAssistantDropdown";
import { Language } from "@/api/ai";
import { useSessionMetrics } from "@/hooks/useSessionMetrics";
import type { editor as MonacoEditorNS } from "monaco-editor";

type MonacoType = typeof import("monaco-editor");

type WSFrame =
  | { type: "init"; data: { sessionId: string; doc: { text: string; version: number }; language: string } }
  | { type: "doc"; data: { text: string; version: number } }
  | { type: "cursor"; data: { userId: string; pos: number } }
  | { type: "chat"; data: { userId: string; message: string } }
  | { type: "stdout"; data: string }
  | { type: "stderr"; data: string }
  | { type: "exit"; data: { code: number; timedOut: boolean } }
  | { type: "language"; data: string }
  | { type: "run_reset"; data?: null }
  | { type: "question"; data: { question: Question | null; rerollsRemaining: number } }
  | { type: "error"; data: string }
  | { type: "session_ended"; data: { reason?: string } };

type EditChange = {
  rangeStart: number;
  rangeEnd: number;
  text: string;
};

const CODE_TEMPLATES: Record<string, string> = {
  python: 'print("Hello from Python!")\n',
  cpp: '#include <iostream>\n\nint main() {\n    std::cout << "Hello from C++!" << std::endl;\n    return 0;\n}\n',
  java: 'public class Main {\n    public static void main(String[] args) {\n        System.out.println("Hello from Java!");\n    }\n}\n',
};

const COLLAB_WEBSOCKET_BASE = (import.meta as any).env?.VITE_COLLAB_WEBSOCKET_BASE || "ws://localhost:8084";

function computeEditChange(prev: string, next: string): EditChange | null {
  if (prev === next) {
    return null;
  }

  let start = 0;
  const prevLen = prev.length;
  const nextLen = next.length;

  while (start < prevLen && start < nextLen && prev[start] === next[start]) {
    start++;
  }

  let endPrev = prevLen - 1;
  let endNext = nextLen - 1;

  while (endPrev >= start && endNext >= start && prev[endPrev] === next[endNext]) {
    endPrev--;
    endNext--;
  }

  return {
    rangeStart: start,
    rangeEnd: endPrev + 1,
    text: next.slice(start, endNext + 1),
  };
}

export default function Editor() {
  const { roomId } = useParams<{ roomId: string }>();
  const [user, setUser] = useState<{ id: number; username: string; email: string } | null>(null);
  const { token } = useAuth();

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
  const [isRerolling, setIsRerolling] = useState<boolean>(false);
  const [rerollsRemaining, setRerollsRemaining] = useState<number>(0);
  const [voiceConnected, setVoiceConnected] = useState<boolean>(false);

  const wsRef = useRef<WebSocket | null>(null);
  const docVersionRef = useRef(docVersion);
  const editorRef = useRef<MonacoEditorNS.IStandaloneCodeEditor | null>(null);
  const monacoRef = useRef<MonacoType | null>(null);
  const suppressChangeRef = useRef(false);
  const codeRef = useRef(code);
  const sessionIdRef = useRef<string | null>(null);

  // Initialize session metrics tracking
  const metrics = useSessionMetrics(
    user?.id.toString() || "",
    voiceConnected
  );

  const getCode = useCallback(() => code, [code]);
  const getQuestion = useCallback(() => {
    return {
      prompt_markdown: question?.prompt_markdown ?? "",
      title: question?.title ?? "",
      difficulty: question?.difficulty ?? "",
      constraints: question?.constraints ?? "",
      topic_tags: question?.topic_tags ?? [],
    };
  }, [question]);

  const nav = useNavigate();
  const matchId = roomInfo?.matchId;

  const resetRunOutputs = () => {
    setStdout("");
    setStderr("");
    setExitInfo(null);
    setRunError(null);
  };

  useEffect(() => {
    docVersionRef.current = docVersion;
  }, [docVersion]);

  useEffect(() => {
    codeRef.current = code;
  }, [code]);

  const applyDocToEditor = useCallback((nextText: string) => {
    const editor = editorRef.current;
    const monaco = monacoRef.current;
    if (!editor || !monaco) {
      return;
    }
    const model = editor.getModel();
    if (!model) {
      return;
    }
    const prevValue = model.getValue();
    if (prevValue === nextText) {
      return;
    }
    const change = computeEditChange(prevValue, nextText);
    if (!change) {
      return;
    }
    const startPos = model.getPositionAt(change.rangeStart);
    const endPos = model.getPositionAt(change.rangeEnd);

    suppressChangeRef.current = true;
    model.pushEditOperations(
      [],
      [
        {
          range: new monaco.Range(
            startPos.lineNumber,
            startPos.column,
            endPos.lineNumber,
            endPos.column
          ),
          text: change.text,
          forceMoveMarkers: true,
        },
      ],
      () => null
    );
    suppressChangeRef.current = false;
  }, []);

  const sendEdit = useCallback(
    (change: EditChange | null) => {
      if (!change) {
        return;
      }
      const ws = wsRef.current;
      if (!ws || ws.readyState !== WebSocket.OPEN) {
        return;
      }
      ws.send(
        JSON.stringify({
          type: "edit",
          data: {
            baseVersion: docVersionRef.current,
            rangeStart: change.rangeStart,
            rangeEnd: change.rangeEnd,
            text: change.text,
          },
        })
      );

      // Track code change for metrics
      metrics.trackCodeChange();
    },
    [metrics]
  );

  const handleEditorMount = useCallback<OnMount>((editor, monaco) => {
    editorRef.current = editor;
    monacoRef.current = monaco as MonacoType;
    suppressChangeRef.current = true;
    editor.setValue(codeRef.current);
    suppressChangeRef.current = false;
    const model = editor.getModel();
    if (model) {
      monaco.editor.setModelLanguage(model, language);
    }
  }, [language]);

  const handleEditorChange = useCallback(
    (value?: string, ev?: MonacoEditorNS.IModelContentChangedEvent) => {
      if (suppressChangeRef.current) {
        return;
      }
      const nextValue = value ?? "";
      const prevValue = codeRef.current;

      setCode(nextValue);
      codeRef.current = nextValue;

      if (!ev || ev.changes.length === 0) {
        sendEdit(computeEditChange(prevValue, nextValue));
        return;
      }

      for (const change of ev.changes) {
        sendEdit({
          rangeStart: change.rangeOffset,
          rangeEnd: change.rangeOffset + change.rangeLength,
          text: change.text,
        })
      }
    },
    [sendEdit]
  );

  const applyServerDoc = useCallback(
    (doc: { text: string; version: number }) => {
      codeRef.current = doc.text;
      docVersionRef.current = doc.version;
      setCode(doc.text);
      setDocVersion(doc.version);
      applyDocToEditor(doc.text);
    },
    [applyDocToEditor]
  );

  useEffect(() => {
    const monaco = monacoRef.current;
    const editor = editorRef.current;
    if (monaco && editor) {
      const model = editor.getModel();
      if (model) {
        monaco.editor.setModelLanguage(model, language);
      }
    }
  }, [language]);

  const applyTemplate = (lang: string) => {
    const template = CODE_TEMPLATES[lang];
    if (!template) return;

    const currentCode = code;
    codeRef.current = template;
    setCode(template);
    resetRunOutputs();
    setIsRunning(false);

    applyDocToEditor(template);

    sendEdit({
      rangeStart: 0,
      rangeEnd: currentCode.length,
      text: template,
    });
  };


  // Map the editor's language (string) to AI Assistant's accepted union type
  const aiLanguage: 'python' | 'java' | 'cpp' =
  language === 'python' || language === 'java' || language === 'cpp'
    ? (language as Language)
    : 'python';


  useEffect(() => {
    if (!token) return;
    getMe(token).then((me) => setUser(me));
  }, [token]);

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
        setRerollsRemaining(room.rerollsRemaining ?? 0);

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
    if (!matchId) return;

    const token = sessionStorage.getItem(`room_token_${matchId}`);
    if (!token) {
      toast.error("No access token found. Please join a room first.", {
        position: "bottom-center",
        duration: 5000,
      });
      nav("/lobby");
      return;
    }

    const ws = new WebSocket(`${COLLAB_WEBSOCKET_BASE}/api/v1/collab/ws/session/${matchId}?token=${encodeURIComponent(token)}`);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("Connected to collab service", matchId);
      ws.send(
        JSON.stringify({
          type: "init",
          data: { sessionId: matchId, language },
        })
      );
    };

    ws.onmessage = (msg) => {
      const frame: WSFrame = JSON.parse(msg.data);

      switch (frame.type) {
        case "init": {
          if (frame.data.language) {
            setLanguage(frame.data.language);
          }
          applyServerDoc(frame.data.doc);

          // Store session ID for metrics
          if (frame.data.sessionId) {
            sessionIdRef.current = frame.data.sessionId;
          }
          break;
        }
        case "doc":
          applyServerDoc(frame.data);
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
        case "run_reset":
          setStdout("");
          setStderr("");
          setExitInfo(null);
          setRunError(null);
          setIsRunning(true);
          break;
        case "question": {
          const nextQuestion = frame.data.question ?? null;
          setQuestion(nextQuestion);
          setRoomInfo((prev) => {
            if (!prev) {
              return prev;
            }
            return {
              ...prev,
              question: nextQuestion === null ? undefined : nextQuestion,
              rerollsRemaining: frame.data.rerollsRemaining,
            };
          });
          setRerollsRemaining(frame.data.rerollsRemaining ?? 0);
          break;
        }
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

            if (frame.data === "sandbox_unavailable") {
              const message = "Code execution sandbox is unavailable. Please start Docker and try again.";
              toast.error(message, {
                position: "bottom-center",
                duration: 5000,
              });
              setRunError(message);
              setIsRunning(false);
              break;
            }
          }
          setRunError(typeof frame.data === "string" ? frame.data : "Unexpected error");
          setIsRunning(false);
          console.error("WS error:", frame.data);
          break;
        case "session_ended": {
          const reason = (frame.data && typeof frame.data === "object" && (frame.data as any).reason) || "session_ended";
          toast((reason === "partner_left" ? "Your partner left the session." : "Session ended."), {
            position: "bottom-center",
            duration: 3000,
          });

          if (matchId) sessionStorage.removeItem(`room_token_${matchId}`);
          try {
            wsRef.current?.close();
          } catch (e) {
            // ignore
          }
          nav("/");
          break;
        }
        default:
          break;
      }
    };

    ws.onclose = () => console.log("Collab WS closed");

    return () => ws.close();
  }, [matchId, language, nav]);

  const handleLanguageChange = (nextLang: string) => {
    const nextTemplate = CODE_TEMPLATES[nextLang];
    const currentTrimmed = code.trim();
    const nextTrimmed = nextTemplate?.trim();
    const losingCustomCode = currentTrimmed.length > 0 && nextTrimmed && currentTrimmed !== nextTrimmed;

    if (losingCustomCode) {
      toast("Switching languages replaces your current code. Only your latest run is kept in history.", {
        position: "bottom-center",
        duration: 5000,
        icon: "!",
      });
    }

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

    applyTemplate(nextLang);
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
          code: codeRef.current,
        },
      })
    );
  };

  const handleReroll = async () => {
    if (!matchId) return;

    const token = sessionStorage.getItem(`room_token_${matchId}`);
    if (!token) {
      toast.error("No access token found. Please join a room first.", {
        position: "bottom-center",
        duration: 5000,
      });
      nav("/lobby");
      return;
    }

    if (rerollsRemaining <= 0) {
      toast.error("No rerolls remaining for this room.", {
        position: "bottom-center",
        duration: 4000,
      });
      return;
    }

    try {
      setIsRerolling(true);
      const updatedRoom = await rerollQuestion(matchId, token);
      setRoomInfo(updatedRoom);
      setQuestion(updatedRoom.question ?? null);
      setRerollsRemaining(updatedRoom.rerollsRemaining ?? 0);

      toast.success("Loaded a new question!", {
        position: "bottom-center",
        duration: 4000,
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to reroll question";
      toast.error(message || "Failed to reroll question", {
        position: "bottom-center",
        duration: 5000,
      });
    } finally {
      setIsRerolling(false);
    }
  };

  const handleExit = async () => {
    // Submit session feedback before exiting
    // Note: Each user submits their own metrics independently
    // The backend will aggregate when both users have submitted
    if (matchId && roomInfo && sessionIdRef.current && user) {
      try {
        const userMetrics = metrics.getMetrics();
        const sessionDuration = metrics.getSessionDuration();

        // Determine user1 and user2 IDs (use room info for consistency)
        const userId = user.id.toString();

        // Each user only submits their own metrics
        // Backend expects both users to submit separately
        const initMetrics = { voiceUsed: false, voiceDuration: 0, codeChanges: 0 }

        await submitSessionFeedback({
          sessionId: sessionIdRef.current,
          matchId: matchId,
          user1Id: roomInfo.user1,
          user2Id: roomInfo.user2,
          difficulty: question?.difficulty?.toLowerCase() || "medium",
          sessionDuration: sessionDuration,
          // Fill in this user's metrics, leave partner's empty (backend will aggregate)
          user1Metrics: userId === roomInfo.user1 ? userMetrics : initMetrics,
          user2Metrics: userId === roomInfo.user2 ? userMetrics : initMetrics,
        });

        console.log("Session feedback submitted successfully");
      } catch (error) {
        console.error("Failed to submit session feedback:", error);
        // Continue with exit even if metrics submission fails
      }
    }

    if (matchId && wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      try {
        wsRef.current.send(JSON.stringify({ type: "end_session" }))
      } catch (e) {
        console.error("Failed to send end_session frame:", e)
      }
      wsRef.current.close()
    }
    if (matchId) sessionStorage.removeItem(`room_token_${matchId}`)
    nav("/")
  }

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
          <button
            type="button"
            onClick={handleExit}
            disabled={isRunning}
            className="rounded-md bg-red-500 px-3 py-2 text-white text-sm hover:brightness-95 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Exit Session
          </button>
        </div>
      </div>
      <div className="px-6 mb-4 text-xs text-amber-600 flex items-center gap-2">
        <span aria-hidden="true" className="font-semibold">
          Warning:
        </span>
        <span>
          Switching languages resets the editor with the selected template. Only your most recent run is kept in history.
        </span>
      </div>

      <div className="px-6 flex flex-col gap-4 lg:grid lg:grid-cols-2 xl:gap-6">
        {/* Question panel */}
        <div className="flex flex-col gap-4 lg:pr-2">
          <div className="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            {/* Header */}
            <div className="p-5 border-b border-gray-200">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <h2 className="text-xl font-semibold text-black">{question?.title ?? "Loading..."}</h2>
                </div>
                <div className="flex items-center gap-2">
                  {question?.difficulty && (
                    <span className={`text-xs rounded-full px-2 py-1 font-medium ${
                      question.difficulty === 'Easy' ? 'bg-green-100 text-green-700' :
                      question.difficulty === 'Medium' ? 'bg-yellow-100 text-yellow-700' :
                      'bg-red-100 text-red-700'
                    }`}>
                      {question.difficulty}
                    </span>
                  )}
                  <button
                    type="button"
                    onClick={handleReroll}
                    disabled={roomLoading || isRerolling || rerollsRemaining <= 0}
                    className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {isRerolling ? "Rerolling..." : `Reroll (${rerollsRemaining} left)`}
                  </button>
                </div>
              </div>
              {question?.topic_tags?.length ? (
                <div className="mt-3 flex flex-wrap gap-2">
                  {question.topic_tags.map((t) => (
                    <span key={t} className="text-xs rounded-md bg-blue-50 px-2 py-1 text-blue-700 font-medium">
                      {t}
                    </span>
                  ))}
                </div>
              ) : null}
            </div>

            {/* Description */}
            <div className="p-5 border-b border-gray-100">
              <div className="prose prose-sm max-w-none text-sm">
                <ReactMarkdown
                  components={{
                    code({ node, inline, className, children, ...props }: any) {
                      const match = /language-(\w+)/.exec(className || '');
                      const language = match ? match[1] : '';
                      return !inline && language ? (
                        <SyntaxHighlighter
                          style={oneDark}
                          language={language}
                          PreTag="div"
                          customStyle={{ fontSize: '0.875rem', marginTop: '0.5rem', marginBottom: '0.5rem' }}
                          {...props}
                        >
                          {String(children).replace(/\n$/, '')}
                        </SyntaxHighlighter>
                      ) : (
                        <code className="bg-gray-100 px-1.5 py-0.5 rounded text-sm font-mono text-pink-600" {...props}>
                          {children}
                        </code>
                      );
                    },
                    p: ({ children }) => <p className="mb-3 leading-relaxed text-gray-700">{children}</p>,
                    ul: ({ children }) => <ul className="list-disc list-inside mb-3 space-y-1 text-gray-700">{children}</ul>,
                    ol: ({ children }) => <ol className="list-decimal list-inside mb-3 space-y-1 text-gray-700">{children}</ol>,
                    li: ({ children }) => <li className="ml-4">{children}</li>,
                    h3: ({ children }) => <h3 className="text-base font-semibold mb-2 mt-4 text-gray-900">{children}</h3>,
                    strong: ({ children }) => <strong className="font-semibold text-gray-900">{children}</strong>,
                  }}
                >
                  {question?.prompt_markdown || ''}
                </ReactMarkdown>
              </div>
            </div>

            {/* Examples */}
            {question?.test_cases && question.test_cases.length > 0 && (
              <div className="p-5 border-b border-gray-100">
                <h3 className="text-sm font-semibold text-gray-900 mb-3">Examples</h3>
                <div className="space-y-4">
                  {question.test_cases.slice(0, 3).map((testCase, idx) => (
                    <div key={idx} className="bg-gray-50 rounded-lg p-4 border border-gray-200">
                      <p className="text-xs font-semibold text-gray-700 mb-2">Example {idx + 1}:</p>
                      <div className="space-y-2">
                        <div>
                          <span className="text-xs font-medium text-gray-600">Input:</span>
                          <pre className="mt-1 bg-white p-2 rounded border border-gray-200 text-xs font-mono overflow-x-auto">{testCase.input}</pre>
                        </div>
                        <div>
                          <span className="text-xs font-medium text-gray-600">Output:</span>
                          <pre className="mt-1 bg-white p-2 rounded border border-gray-200 text-xs font-mono overflow-x-auto">{testCase.output}</pre>
                        </div>
                        {testCase.description && (
                          <div>
                            <span className="text-xs font-medium text-gray-600">Explanation:</span>
                            <p className="mt-1 text-xs text-gray-700">{testCase.description}</p>
                          </div>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Constraints */}
            {question?.constraints && (
              <div className="p-5">
                <h3 className="text-sm font-semibold text-gray-900 mb-3">Constraints</h3>
                <div className="text-xs text-gray-700 space-y-1">
                  {question.constraints.split('\n').map((constraint, idx) => (
                    constraint.trim() && (
                      <div key={idx} className="flex items-start gap-2">
                        <span className="text-gray-400 mt-0.5">•</span>
                        <span className="font-mono">{constraint.trim()}</span>
                      </div>
                    )
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Voice Chat */}
          {user && roomId && (
            <VoiceChat
              roomId={roomId}
              userId={user.id.toString()}
              username={user.username}
              token={sessionStorage.getItem(`room_token_${roomId}`) || ''}
              onConnectionChange={setVoiceConnected}
            />
          )}

          {/* AI Assistant*/}
            <AiAssistantDropdown
              getCode={getCode}  
              language={aiLanguage}
              getQuestion={getQuestion}
            />
        </div>

        {/* Code Editor */}
        <div className="lg:pl-2">
          <div className="rounded-lg border border-gray-200 bg-white shadow-sm">
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 px-4 py-2 text-sm text-gray-600">
              Editor — {language}
            </div>
            <div className="p-3">
              <div className="max-h-[70vh] min-h-[320px] overflow-auto rounded-md bg-[#1E1E1E]">
                <MonacoEditor
                  height="70vh"
                  defaultLanguage={language}
                  defaultValue={code}
                  onMount={handleEditorMount}
                  onChange={handleEditorChange}
                  theme="vs-dark"
                  options={{
                    minimap: { enabled: false },
                    scrollBeyondLastLine: false,
                    automaticLayout: true,
                    fontSize: 14,
                    fontLigatures: true,
                  }}
                />
              </div>
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
