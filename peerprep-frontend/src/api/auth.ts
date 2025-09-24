import { apiFetch } from "@/api/client";

export async function login(username: string, password: string): Promise<{ token: string }> {
  return apiFetch<{ token: string }>(`/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
}

export async function register(username: string, email: string, password: string): Promise<void> {
  await apiFetch<void>(`/api/v1/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, email, password }),
  });
} 