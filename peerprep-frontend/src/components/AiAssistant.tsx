import { useMemo, useState } from "react";
import { useExplain } from "@/hooks/useAi";
import type { DetailLevel, Language } from "@/api/ai";

type Mode = "Explain" | "Hint" | "Tests" | "Refactor" | "Summary";

interface Props {
  getCode: () => string;
  language: Language;
  className?: string;
}

export default function AIAssistant({ getCode, language, className }: Props) {
  const [detail, setDetail] = useState<DetailLevel>("intermediate");
  const [activeMode, setActiveMode] = useState<Mode>("Explain");
  const { run, loading, text, error, setText, setError } = useExplain();

  const modes: Mode[] = ["Explain", "Hint", "Tests", "Refactor", "Summary"];
  const canRunExplain = useMemo(() => activeMode === "Explain", [activeMode]);

    const handleModeClick = async (m: Mode) => {
        setActiveMode(m);
        if (m === "Explain") {
            setError('');  
            await run({ code: getCode(), language, detail });
        } else { 
            setError('');   // Clears error
            setText(`${m} is not implemented yet — coming soon.`); //Stub for now
        }
    };

  return (
    <div className={`flex flex-col gap-3 ${className ?? ""}`}>
      {/* Controls row (Detail Level only) */}
      <div className="flex items-center justify-between">
        <label className="text-sm">
          Detail Level
          <select
            value={detail}
            onChange={(e) => setDetail(e.target.value as DetailLevel)}
            className="ml-2 border rounded px-2 py-1 text-sm"
          >
            <option value="beginner">Beginner</option>
            <option value="intermediate">Intermediate</option>
            <option value="advanced">Advanced</option>
          </select>
        </label>
        <div className="text-xs text-gray-500">
          Language: <span className="font-mono">{language}</span>
        </div>
      </div>

      {/* Mode bubbles (icons + labels) */}
    <div className="grid grid-cols-2 gap-2">
    {modes.map((m) => {
        const active = activeMode === m;
        return (
        <button
            key={m}
            onClick={() => handleModeClick(m)}
            className={`flex items-center gap-3 rounded-md border px-3 py-3 text-left transition
            ${active ? "ring-2 ring-blue-500 bg-blue-50" : "hover:bg-black/5"}`}
        >
            {/* Icons */}
            <div className="flex items-center justify-center w-8 h-8 text-gray-700">
            {m === "Explain" && (
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                <path
                    d="M12 3l1.8 4.2L18 9l-4.2 1.8L12 15l-1.8-4.2L6 9l4.2-1.8L12 3z"
                    stroke="currentColor"
                    strokeWidth="1.6"
                />
                </svg>
            )}
            {m === "Hint" && (
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                <path
                    d="M12 3a7 7 0 00-7 7c0 3.3 2.1 5 3.5 6.2 1.1.9 1.5 1.4 1.5 2.3V20h4v-1.5c0-.9.4-1.4 1.5-2.3C16 15 19 13 19 10a7 7 0 00-7-7z"
                    stroke="currentColor"
                    strokeWidth="1.6"
                />
                </svg>
            )}
            {m === "Tests" && (
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                <rect
                    x="5"
                    y="4"
                    width="14"
                    height="16"
                    rx="2"
                    stroke="currentColor"
                    strokeWidth="1.6"
                />
                <path d="M8 9h8M8 12h8M8 15h8" stroke="currentColor" strokeWidth="1.6" />
                </svg>
            )}
            {m === "Refactor" && (
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                <path
                    d="M4 7h7l-2-2m2 2l-2 2M20 17h-7l2 2m-2-2l2-2"
                    stroke="currentColor"
                    strokeWidth="1.6"
                />
                </svg>
            )}
            {m === "Summary" && (
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                <path
                    d="M6 7h12M6 12h12M6 17h8"
                    stroke="currentColor"
                    strokeWidth="1.6"
                />
                </svg>
            )}
            </div>

            {/* TEXT */}
            <div className="flex flex-col justify-center">
            <div className="font-medium">{m}</div>
            <div className="text-xs text-gray-500 leading-tight">
                {m === "Explain" && "Get plain-language explanations"}
                {m === "Hint" && "Contextual hints (peer-aware)"}
                {m === "Tests" && "Generate test cases"}
                {m === "Refactor" && "Code improvement suggestions"}
                {m === "Summary" && "Session learning recap"}
            </div>
            </div>
        </button>
        );
    })}
    </div>


      {/* Response box */}
      <div className="min-h-32 max-h-64 overflow-auto border rounded p-2 bg-neutral-50">
        {loading && <div className="animate-pulse">Thinking…</div>}
        {!loading && error && <div className="text-red-600">{error}</div>}
        {!loading && !error && text && <pre className="whitespace-pre-wrap">{text}</pre>}
        {!loading && !error && !text && (
          <div className="text-gray-400">Run a mode to see the output here.</div>
        )}
      </div>
    </div>
  );
}
