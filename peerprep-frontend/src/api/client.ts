const USER_API_BASE = (import.meta as any).env?.VITE_USER_API_BASE || "http://localhost:8081";

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const url = `${USER_API_BASE}${path.startsWith("/") ? path : `/${path}`}`;
  const res = await fetch(url, init);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  const contentType = res.headers.get("content-type") || "";
  if (contentType.includes("application/json")) {
    return (await res.json()) as T;
  }
  // @ts-expect-error allow void return when no JSON
  return undefined;
} 