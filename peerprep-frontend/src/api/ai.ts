const AI_BASE = import.meta.env.VITE_AI_BASE_URL ?? 'http://localhost:8086';

// ============================================================================
// Types
// ============================================================================

export type DetailLevel = 'beginner' | 'intermediate' | 'advanced';
export type Language = 'python' | 'java' | 'cpp' | 'javascript' | 'typescript';

// ============================================================================
// Constants
// ============================================================================

export const API_ENDPOINTS = {
  EXPLAIN: 'explain',
  HINT: 'hint',
  TESTS: 'tests',
  REFACTOR_TIPS: 'refactor-tips',
} as const;

export const DETAIL_LEVELS: readonly DetailLevel[] = ['beginner', 'intermediate', 'advanced'];
export const LANGUAGES: readonly Language[] = ['python', 'java', 'cpp', 'javascript', 'typescript'];

// ============================================================================
// Shared Interfaces
// ============================================================================

export interface Metadata {
  processing_time_ms: number;
  provider?: string;
  model?: string;
  detail_level?: DetailLevel;
}

export interface QuestionContext {
  prompt_markdown: string;
  title?: string;
  difficulty?: string;
  topic_tags?: string[];
  constraints?: string;
}

// ============================================================================
// Request/Response Interfaces
// ============================================================================

export interface ExplainRequest {
  code: string;
  language: Language;
  detail_level: DetailLevel;
  request_id?: string;
}

export interface ExplainResponse {
  content: string;
  request_id: string;
  metadata: Metadata;
}

export interface HintRequest {
  code: string;
  language: Language;
  hint_level: DetailLevel;
  question: QuestionContext;
  request_id?: string;
}

export interface HintResponse {
  hint: string;
  request_id: string;
  metadata: Metadata;
}

export interface TestsRequest {
  code: string;
  language: Language;
  question: QuestionContext;
  framework?: string;
  request_id?: string;
}

export interface TestsResponse {
  tests_code: string;
  request_id: string;
  metadata: Metadata;
}

export interface RefactorRequest {
  code: string;
  language: Language;
  question: QuestionContext;
  request_id?: string;
}

export interface RefactorResponse {
  tips_text: string;
  request_id: string;
  metadata: Metadata;
}

// ============================================================================
// Helper Functions
// ============================================================================


function aiUrl(endpoint: string): string {
  let base = AI_BASE.replace(/\/+$/, '');
  const path = endpoint.replace(/^\/+/, '');

  // If base already includes /api/v1/ai, just append endpoint
  if (base.endsWith('/api/v1/ai')) {
    return `${base}/${path}`;
  }

  // If base includes /api/v1 but not /ai, append /ai
  if (base.endsWith('/api/v1')) {
    return `${base}/ai/${path}`;
  }

  // Otherwise, append full path /api/v1/ai
  return `${base}/api/v1/ai/${path}`;
}

function handleApiError(res: Response, errorJson: any, defaultMessage: string): Error {
  if (res.status === 429) {
    return new Error('AI resources exhausted. Please try again later');
  }
  return new Error(errorJson?.message ?? defaultMessage);
}

// ============================================================================
// API Functions
// ============================================================================

export async function explainCode(payload: ExplainRequest): Promise<ExplainResponse> {
  const res = await fetch(aiUrl(API_ENDPOINTS.EXPLAIN), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw handleApiError(res, err, `AI explain failed with ${res.status}`);
  }
  return res.json();
}

export async function getHint(payload: HintRequest): Promise<HintResponse> {
  const res = await fetch(aiUrl(API_ENDPOINTS.HINT), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw handleApiError(res, err, `AI hint failed with ${res.status}`);
  }
  return res.json();
}

export async function generateTests(payload: TestsRequest): Promise<TestsResponse> {
  const url = aiUrl(API_ENDPOINTS.TESTS);
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  const bodyText = await res.text();
  let json: any = null;
  try {
    json = JSON.parse(bodyText);
  } catch {
    /* text body */
  }

  if (!res.ok) {
    throw handleApiError(res, json, 'Failed to generate test cases');
  }
  return json as TestsResponse;
}

export async function generateRefactorTips(payload: RefactorRequest): Promise<RefactorResponse> {
  const url = aiUrl(API_ENDPOINTS.REFACTOR_TIPS);
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  const text = await res.text();
  let json: any = null;
  try {
    json = JSON.parse(text);
  } catch {
    /* ignore */
  }

  if (!res.ok) {
    throw handleApiError(res, json, 'Failed to generate refactor tips');
  }
  return json as RefactorResponse;
}
