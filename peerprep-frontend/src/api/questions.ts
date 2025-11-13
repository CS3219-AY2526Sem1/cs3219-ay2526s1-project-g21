import { Question, RandomQuestionFilters } from "@/types/question";

// TODO: remove localhost call in prod
const baseUrl = (import.meta as any).env?.VITE_QUESTION_API_BASE ?? "http://localhost:8082";
const QUESTION_API_BASE = baseUrl.includes('/api/v1')
  ? `${baseUrl.replace(/\/+$/, '')}/questions`
  : `${baseUrl}/api/v1/questions`;

export async function questionApiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  // building url
  let normalizedPath = path;
  if (!path.startsWith("/")) {
    normalizedPath = "/" + path;
  }

  const url = QUESTION_API_BASE + normalizedPath;

  // preserve user headers and add sensible defaults
  const requestHeaders = new Headers(init?.headers as HeadersInit | undefined);

  if (!requestHeaders.has("Accept")) {
    requestHeaders.set("Accept", "application/json");
  }

  const hasBody = init?.body != null;
  if (hasBody && !requestHeaders.has("Content-Type")) {
    requestHeaders.set("Content-Type", "application/json");
  }

  const fetchOptions: RequestInit = {
    ...init,
    headers: requestHeaders,
  };

  const res = await fetch(url, fetchOptions);

  // try to parse error body as JSON, fallback to text
  if (!res.ok) {
    let errBody: string | undefined;
    try {
      const json = await res.json();
      errBody = typeof json === "string" ? json : JSON.stringify(json);
    } catch {
      try {
        errBody = await res.text();
      } catch {
        /* ignore */
      }
    }
    throw new Error(errBody || `Request failed: ${res.status}`);
  }

  const contentType = res.headers.get("content-type") || "";
  if (contentType.includes("application/json")) {
    return (await res.json()) as T;
  }

  throw new Error("Response is not JSON");
}

export async function getRandomQuestion(filters?: RandomQuestionFilters): Promise<Question> {
  let path = "/random";

  if (filters) {
    const params = new URLSearchParams();

    if (filters.difficulty?.value) {
      params.append("difficulty", filters.difficulty.value);
    }

    if (filters.topic_tags && filters.topic_tags.length > 0) {
      params.append("topic", filters.topic_tags.join(","));
    }

    if (params.toString()) {
      path += "?" + params.toString();
    }
  }

  return questionApiFetch<Question>(path);
}

export async function getAllQuestions(page?: number, limit?: number, search?: string): Promise<{
  total: number;
  items: Question[];
  page: number;
  limit: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
}> {
  let path = "/";

  if (page !== undefined || limit !== undefined || search !== undefined) {
    const params = new URLSearchParams();

    if (page !== undefined) {
      params.append("page", page.toString());
    }

    if (limit !== undefined) {
      params.append("limit", limit.toString());
    }

    if (search !== undefined && search !== "") {
      params.append("search", search);
    }

    path += "?" + params.toString();
  }

  return questionApiFetch<{
    total: number;
    items: Question[];
    page: number;
    limit: number;
    totalPages: number;
    hasNext: boolean;
    hasPrev: boolean;
  }>(path);
}

export async function getQuestionById(id: string): Promise<Question> {
  const safeId = encodeURIComponent(id);
  return questionApiFetch<Question>(`/questions/${safeId}`);
}