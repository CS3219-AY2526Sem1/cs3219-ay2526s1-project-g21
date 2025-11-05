import { apiFetch } from "@/api/client";

export async function changeUsername(token: string, userId: number | string, username: string): Promise<{ id: number; username: string }> {
  return apiFetch<{ id: number; username: string }>(`/api/v1/users/${userId}/username`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify({ username }),
  });
}

export async function changePassword(token: string, userId: number | string, newPassword: string, confirmPassword: string): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>(`/api/v1/users/${userId}/password`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify({ newPassword, confirmPassword }),
  });
}

export async function initiateEmailChange(token: string, userId: number | string, email: string): Promise<{ ok: boolean }> {
  return apiFetch<{ ok: boolean }>(`/api/v1/users/${userId}/email-change`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify({ email }),
  });
}


