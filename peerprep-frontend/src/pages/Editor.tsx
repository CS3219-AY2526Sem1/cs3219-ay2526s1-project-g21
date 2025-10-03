import { useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "react-router-dom";
import CodeEditor from "@uiw/react-textarea-code-editor";

type Question = {
  id: string;
  title: string;
  description: string;
  difficulty?: "Easy" | "Medium" | "Hard";
  tags?: string[];
};

type WSFrame =
  | { type: "init"; data: { sessionId: string; doc: { text: string; version: number }; language: string } }
  | { type: "doc"; data: { text: string; version: number } }
  | { type: "cursor"; data: { userId: string; pos: number } }
  | { type: "chat"; data: { userId: string; message: string } }
  | { type: "stdout"; data: string }
  | { type: "stderr"; data: string }
  | { type: "exit"; data: { code: number; timedOut: boolean } }
  | { type: "error"; data: string };

export default function Editor() {
  const { roomId } = useParams<{ roomId: string }>();

  const [question, setQuestion] = useState<Question | null>(null);
  const [language, setLanguage] = useState<string>("python");
  const [code, setCode] = useState<string>("// Start coding here\n");
  const [docVersion, setDocVersion] = useState<number>(0);

  const wsRef = useRef<WebSocket | null>(null);
  const isRoomValid = useMemo(() => Boolean(roomId && roomId.trim().length > 0), [roomId]);


  useEffect(() => {
    // Placeholder: In the future, fetch the question based on room/session
    // For now, hydrate with a sample prompt to validate layout and UX
    const mock: Question = {
      id: "sample-1",
      title: "Two Sum",
      description:
        "Given an array of integers nums and an integer target, return indices of the two numbers such that they add up to target.",
      difficulty: "Easy",
      tags: ["Array", "Hash Table"],
    };
    setQuestion(mock);
  }, []);

  // WebSocket collab setup
  useEffect(() => {
    if (!isRoomValid) return;

    const ws = new WebSocket(`${import.meta.env.VITE_COLLAB_SERVICE_WS}/ws/session/${roomId}`);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("Connected to collab service", roomId);
      ws.send(
        JSON.stringify({
          type: "init",
          data: { sessionId: roomId, language },
        })
      );
    };

    ws.onmessage = (msg) => {
      const frame: WSFrame = JSON.parse(msg.data);

      switch (frame.type) {
        case "init":
          setCode(frame.data.doc.text);
          setDocVersion(frame.data.doc.version);
          break;
        case "doc":
          setCode(frame.data.text);
          setDocVersion(frame.data.version);
          break;
        case "error":
          console.error("WS error:", frame.data);
          break;
        default:
          // ignore cursor/chat/stdout/stderr for now
          break;
      }
    };

    ws.onclose = () => console.log("Collab WS closed");

    return () => ws.close();
  }, [roomId, language, isRoomValid]);

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
            onChange={(e) => setLanguage(e.target.value)}
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
            className="rounded-md bg-[#2F6FED] px-3 py-2 text-white text-sm hover:brightness-95"
          >
            Run
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
              {question?.description}
            </p>
            {question?.tags?.length ? (
              <div className="mt-4 flex flex-wrap gap-2">
                {question.tags.map((t) => (
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
          </div>
        </div>
      </div>
    </div>
  );
}
