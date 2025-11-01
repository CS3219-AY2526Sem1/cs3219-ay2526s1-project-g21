import { useEffect, useState } from "react";
import AiAssistant from "@/components/AiAssistant";
import type { Language } from "@/api/ai";
import "./AIAssistantDropdown.css";

type Props = {
  getCode: () => string;
  language: Language;
  defaultOpen?: boolean;
  className?: string;
};

const LS_KEY = "peerprep.ai.assistant.open";

export default function AIAssistantDropdown({
  getCode,
  language,
  defaultOpen = true,
  className,
}: Props) {
  const [open, setOpen] = useState<boolean>(() => {
    const saved = localStorage.getItem(LS_KEY);
    return saved === null ? defaultOpen : saved === "1";
  });

  useEffect(() => {
    localStorage.setItem(LS_KEY, open ? "1" : "0");
  }, [open]);

  return (
    <div className={`ai-dropdown ${className ?? ""}`}>
      {/* Header */}
      <button
        type="button"
        className="w-full flex items-center justify-between px-4 py-3 text-left select-none"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls="ai-assistant-panel"
      >
        <div className="flex items-center gap-2">
          <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-blue-600 text-white text-xs">✦</span>
          <span className="font-semibold">AI Assistant</span>
        </div>
        <svg
          className="ai-chevron h-4 w-4"
          data-open={open || undefined}
          viewBox="0 0 20 20"
          fill="currentColor"
          aria-hidden="true"
        >
          <path
            fillRule="evenodd"
            d="M5.23 7.21a.75.75 0 011.06.02L10 10.94l3.71-3.71a.75.75 0 111.06 1.06l-4.24 4.24a.75.75 0 01-1.06 0L5.21 8.29a.75.75 0 01.02-1.08z"
            clipRule="evenodd"
          />
        </svg>
      </button>

      {/* Collapsible body */}
      <div className="ai-collapse" data-open={open || undefined} id="ai-assistant-panel">
        <div className="ai-content">
          <div className="p-4 pt-0">
            <AiAssistant getCode={getCode} language={language} />
          </div>
        </div>
      </div>
    </div>
  );
}
