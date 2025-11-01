export type DetailLevel = 'beginner' | 'intermediate' | 'advanced';
export type Language = 'python' | 'java' | 'cpp' | 'javascript' | 'typescript';

export interface ExplainRequest {
  code: string;
  language: Language;
  detail_level: DetailLevel;
  request_id?: string;
}

export interface ExplainResponse {
  explanation: string;
  request_id: string;
  metadata: {
    processing_time_ms: number;
    detail_level: DetailLevel;
    provider?: string;
    model?: string;
  };
}

// Can point this to a proxy path in frontend dev server later if needed.
const AI_BASE = import.meta.env.VITE_AI_BASE_URL ?? 'http://localhost:8086';

export async function explainCode(payload: ExplainRequest): Promise<ExplainResponse> {
  const res = await fetch(`${AI_BASE}/ai/explain`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err?.message ?? `AI explain failed with ${res.status}`);
  }
  return res.json();
}

export async function getHint(payload: {
    code: string;
    language: "python" | "java" | "cpp" | "javascript" | "typescript";
    question: {
        prompt_markdown: string;
        title?: string;
        difficulty?: string;
        topic_tags?: string[];
        constraints?: string;
    };
    request_id?: string;
    }) {
    const res = await fetch(`${AI_BASE}/ai/hint`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
    });
    if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err?.message ?? `AI explain failed with ${res.status}`);
    }
    return (await res.json()) as { hint: string; request_id: string; metadata: any };
}
