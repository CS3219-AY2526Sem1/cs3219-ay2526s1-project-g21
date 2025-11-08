import { useMemo, useState } from "react";
import { TriangleAlert } from "lucide-react";
import { useExplain } from "@/hooks/useAi";
import type { DetailLevel, Language } from "@/api/ai";
import { getHint, generateTests, generateRefactorTips } from "@/api/ai";

type Mode = "Explain" | "Hint" | "Tests" | "Refactor" | "Summary";

interface Props {
  getCode: () => string;
  language: Language;
  getQuestion: () => {
    prompt_markdown: string;
    title?: string;
    difficulty?: string;
    constraints?: string;
    topic_tags?: string[];
  };
  className?: string;
}

const ACTION_LABEL: Record<Mode, string> = {
  Explain: "Explain Code",
  Hint: "Get Hint",
  Tests: "Generate Test Cases",
  Refactor: "Refactor Code",
  Summary: "Give Summary",
};

export default function AIAssistant({ getCode, language, getQuestion, className }: Props) {
  const [detail, setDetail] = useState<DetailLevel>("intermediate");
  const [activeMode, setActiveMode] = useState<Mode>("Explain");
  const { run, loading, text, error, setText, setError } = useExplain();
  const [hintLevel, setHintLevel] = useState<DetailLevel>("beginner");


  const modes: Mode[] = ["Explain", "Hint", "Tests", "Refactor"];
  const showDetail = activeMode === "Explain";

  // Primary action handler
  const onPrimaryAction = async () => {
    setError("");
    setText("");
    try {
        if (activeMode === "Explain") {
        await run({ code: getCode(), language, detail });
        return;
        }

        if (activeMode === "Hint") {
        setText("Generating hint...");
        const q = getQuestion();
        const resp = await getHint({
            code: getCode(),
            language,
            hint_level: hintLevel,
            question: q,
        });
        setText(resp.hint);
        return;
        }


        if (activeMode === "Tests") {
        setText("Generating test cases...");
        const q = getQuestion(); // grab the latest question context
        if (!q?.prompt_markdown || q.prompt_markdown.trim().length === 0) {
          setError("Missing question context. Please open a question or ensure its description is loaded.");
          return;
        }
        const resp = await generateTests({
            code: getCode(),
            language,
            question: q,
          });
          setText(resp.tests_code);
          return;
        }

        if (activeMode === "Refactor") {
          setText("Reviewing your code for refactor opportunities...");
          try {
            const q = getQuestion();
            const resp = await generateRefactorTips({
              code: getCode(),
              language,
              question: q,
            });
            setText(resp.tips_text || "No significant refactor tips found.");
          } catch (e: any) {
            setError(e?.message ?? "Failed to generate refactor tips");
          }
          return;
        }


        // Default stub for other modes
        setText(`${activeMode} is not implemented yet — coming soon.`);
    } catch (e: any) {
        setError(e?.message ?? "Failed to run AI action");
    }
    };


  return (
    <div className={`flex flex-col gap-3 pt-2 ${className ?? ""}`}>
      {/* Gen AI Warning */}
      <div className="flex items-start gap-2 rounded-md bg-yellow-50 border border-yellow-200 px-3 py-2 text-sm">
        <TriangleAlert className="w-5 h-5 text-yellow-600 flex-shrink-0 mt-0.5" />
        <span className="text-yellow-800">
          AI-generated content may contain errors. Please verify important information.
        </span>
      </div>

      {/* Mode bubbles */}
      <div className="grid grid-cols-2 gap-2">
        {modes.map((m) => {
          const active = activeMode === m;
          return (
            <button
              key={m}
              onClick={() => {
                setActiveMode(m);
                setError("");
                setText(""); // clear panel until action is clicked
              }}
              className={`flex items-center gap-3 rounded-md border px-3 py-3 text-left transition
                ${active ? "ring-2 ring-blue-500 bg-blue-50" : "hover:bg-black/5"}`}
            >
              {/* Icon (left, centered, slightly bigger) */}
              <div className="flex items-center justify-center w-8 h-8 text-gray-700">
                {m === "Explain" && (
                  <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                    <path d="M12 3l1.8 4.2L18 9l-4.2 1.8L12 15l-1.8-4.2L6 9l4.2-1.8L12 3z" stroke="currentColor" strokeWidth="1.6"/>
                  </svg>
                )}
                {m === "Hint" && (
                  <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                    <path d="M12 3a7 7 0 00-7 7c0 3.3 2.1 5 3.5 6.2 1.1.9 1.5 1.4 1.5 2.3V20h4v-1.5c0-.9.4-1.4 1.5-2.3C16 15 19 13 19 10a7 7 0 00-7-7z" stroke="currentColor" strokeWidth="1.6"/>
                  </svg>
                )}
                {m === "Tests" && (
                  <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                    <rect x="5" y="4" width="14" height="16" rx="2" stroke="currentColor" strokeWidth="1.6"/>
                    <path d="M8 9h8M8 12h8M8 15h8" stroke="currentColor" strokeWidth="1.6"/>
                  </svg>
                )}
                {m === "Refactor" && (
                  <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                    <path d="M4 7h7l-2-2m2 2l-2 2M20 17h-7l2 2m-2-2l2-2" stroke="currentColor" strokeWidth="1.6"/>
                  </svg>
                )}
              </div>

              <div className="flex flex-col justify-center">
                <div className="font-medium">{m}</div>
                <div className="text-xs text-gray-500 leading-tight">
                  {m === "Explain" && "Get plain-language explanations"}
                  {m === "Hint" && "Contextual hints"}
                  {m === "Tests" && "Generate test cases"}
                  {m === "Refactor" && "Code improvement suggestions"}
                  {m === "Summary" && "Session learning recap"}
                </div>
              </div>
            </button>
          );
        })}
      </div>

      {/* Detail level: only for Explain */}
      {showDetail && (
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
      )}


      {activeMode === "Hint" && (
        <div className="flex items-center justify-between">
            <label className="text-sm">
            Hint Level
            <select
                value={hintLevel}
                onChange={(e) => setHintLevel(e.target.value as DetailLevel)}
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
        )}


      {/* Primary action button */}
      <button
        onClick={onPrimaryAction}
        disabled={loading}
        className="mt-1 rounded-md px-4 py-2 text-sm font-medium border bg-blue-600 text-white disabled:opacity-60"
      >
        {ACTION_LABEL[activeMode]}
      </button>

      {/* Response box */}
      <div className="min-h-32 max-h-64 overflow-auto border rounded p-2 bg-neutral-50">
        {loading && <div className="animate-pulse">Thinking…</div>}
        {!loading && error && <div className="text-red-600">{error}</div>}
        {!loading && !error && text && <pre className="whitespace-pre-wrap">{text}</pre>}
        {!loading && !error && !text && (
          <div className="text-gray-400">Select a mode, then click the button above.</div>
        )}
      </div>
    </div>
  );
}
