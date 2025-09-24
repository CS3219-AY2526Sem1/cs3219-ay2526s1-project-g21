export async function login(username: string, password: string): Promise<{ token: string }> {
  const res = await fetch(`/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    const msg = await res.text();
    throw new Error(msg || "Login failed");
  }
  return (await res.json()) as { token: string };
} 