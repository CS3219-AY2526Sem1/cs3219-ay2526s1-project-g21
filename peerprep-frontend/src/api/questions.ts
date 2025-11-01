import { Question, RandomQuestionFilters } from "@/types/question";

// TODO: remove localhost call in prod
const QUESTION_API_BASE = (import.meta as any).env?.VITE_QUESTION_API_BASE || "http://localhost:8082/api/v1";

export async function questionApiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  // building url
  let normalizedPath = path;
  if (!path.startsWith("/")) {
    normalizedPath = "/" + path;
  }

  // const url = new URL(normalizedPath, QUESTION_API_BASE).toString();
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
  let path = "/questions/random";

  if (filters) {
    const params = new URLSearchParams();

    if (filters.difficulty) {
      params.append("difficulty", filters.difficulty);
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

export async function getAllQuestions(): Promise<{ total: number; items: Question[] }> {
  return questionApiFetch<{ total: number; items: Question[] }>("/questions");
}

export async function getQuestionById(id: string): Promise<Question> {
  const safeId = encodeURIComponent(id);
  return questionApiFetch<Question>(`/questions/${safeId}`);
}