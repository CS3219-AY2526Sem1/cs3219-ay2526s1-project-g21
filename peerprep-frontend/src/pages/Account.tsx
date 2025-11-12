import { useEffect, useState } from "react";
import { getMe } from "@/api/auth";
import { useAuth } from "@/context/AuthContext";
import { changePassword, changeUsername, initiateEmailChange } from "@/api/user";
import toast from "react-hot-toast";

export default function Account() {
  const { token } = useAuth();
  const [user, setUser] = useState<{ id: number; username: string; email: string } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showUsernameInput, setShowUsernameInput] = useState(false);
  const [newUsername, setNewUsername] = useState("");
  const [showEmailInput, setShowEmailInput] = useState(false);
  const [newEmail, setNewEmail] = useState("");
  const [showPasswordInput, setShowPasswordInput] = useState(false);
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      if (!token) {
        setLoading(false);
        return;
      }
      try {
        const me = await getMe(token);
        if (!cancelled) setUser(me);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load account");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => { cancelled = true; };
  }, [token]);

  return (
    <section className="mx-auto max-w-5xl px-6 py-14">
      <h1 className="text-3xl font-semibold text-black">Your Account</h1>

      <div className="mt-10 grid grid-cols-12 gap-10">
        <aside className="col-span-12 md:col-span-3">
          <nav className="space-y-3 text-[15px]">
            <div className="font-medium text-black">Account Settings</div>
          </nav>
        </aside>

        <div className="col-span-12 md:col-span-9">
          {loading ? (
            <div className="text-slate-600">Loading...</div>
          ) : error ? (
            <div className="rounded-md border border-red-300 bg-red-50 px-4 py-2 text-sm text-red-700">{error}</div>
          ) : user ? (
            <>
              <section className="pb-8">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h2 className="text-lg font-semibold text-black">Username</h2>
                    <p className="mt-2 text-black">{user.username}</p>
                  </div>
                  <button className="rounded-md border border-[#D1D5DB] px-3 py-2 text-sm hover:bg-gray-50" onClick={() => setShowUsernameInput((v) => !v)}>
                    Change Username
                  </button>
                </div>
                {showUsernameInput && (
                  <div className="mt-4 flex items-center gap-3">
                    <input
                      className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm"
                      placeholder="New username"
                      value={newUsername}
                      onChange={(e) => setNewUsername(e.target.value)}
                    />
                    <button
                      disabled={submitting || !newUsername.trim() || !user}
                      onClick={async () => {
                        if (!token || !user) return;
                        setSubmitting(true);
                        try {
                          const res = await changeUsername(token, user.id, newUsername.trim());
                          setUser((u) => (u ? { ...u, username: res.username } : u));
                          setShowUsernameInput(false);
                          setNewUsername("");
                          toast.success("Username updated");
                        } catch (e: any) {
                          toast.error(e?.message || "Failed to change username");
                        } finally {
                          setSubmitting(false);
                        }
                      }}
                      className="rounded-md bg-[#2F6FED] px-3 py-2 text-sm font-medium text-white hover:brightness-95 disabled:opacity-50"
                    >
                      Save
                    </button>
                  </div>
                )}
              </section>

              <hr className="my-2 border-[#E5E7EB]" />

              <section className="py-8">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h2 className="text-lg font-semibold text-black">Email Address</h2>
                    <p className="mt-2 text-black">{user.email}</p>
                  </div>
                  <button className="rounded-md border border-[#D1D5DB] px-3 py-2 text-sm hover:bg-gray-50" onClick={() => setShowEmailInput((v) => !v)}>
                    Change Email
                  </button>
                </div>
                {showEmailInput && (
                  <div className="mt-4 flex items-center gap-3">
                    <input
                      className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm"
                      placeholder="New email"
                      value={newEmail}
                      onChange={(e) => setNewEmail(e.target.value)}
                      type="email"
                    />
                    <button
                      disabled={submitting || !newEmail.trim() || !user}
                      onClick={async () => {
                        if (!token || !user) return;
                        setSubmitting(true);
                        try {
                          await initiateEmailChange(token, user.id, newEmail.trim());
                          setShowEmailInput(false);
                          setNewEmail("");
                          toast.success("Confirmation sent to new email. Check your inbox.");
                        } catch (e: any) {
                          toast.error(e?.message || "Failed to initiate email change");
                        } finally {
                          setSubmitting(false);
                        }
                      }}
                      className="rounded-md bg-[#2F6FED] px-3 py-2 text-sm font-medium text-white hover:brightness-95 disabled:opacity-50"
                    >
                      Send
                    </button>
                  </div>
                )}
              </section>

              <hr className="my-2 border-[#E5E7EB]" />

              <section className="pt-8">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h2 className="text-lg font-semibold text-black">Password</h2>
                  </div>
                  <button className="rounded-md border border-[#D1D5DB] px-3 py-2 text-sm hover:bg-gray-50" onClick={() => setShowPasswordInput((v) => !v)}>
                    Change Password
                  </button>
                </div>
                {showPasswordInput && (
                  <div className="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2">
                    <input
                      className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm"
                      placeholder="New password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      type="password"
                    />
                    <input
                      className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm"
                      placeholder="Confirm password"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      type="password"
                    />
                    <div>
                      <button
                        disabled={submitting || !user}
                        onClick={async () => {
                          if (!token || !user) return;
                          setSubmitting(true);
                          try {
                            await changePassword(token, user.id, newPassword, confirmPassword);
                            setShowPasswordInput(false);
                            setNewPassword("");
                            setConfirmPassword("");
                            toast.success("Password changed");
                          } catch (e: any) {
                            toast.error(e?.message || "Failed to change password");
                          } finally {
                            setSubmitting(false);
                          }
                        }}
                        className="rounded-md bg-[#2F6FED] px-3 py-2 text-sm font-medium text-white hover:brightness-95 disabled:opacity-50"
                      >
                        Save
                      </button>
                    </div>
                  </div>
                )}
              </section>
            </>
          ) : null}
        </div>
      </div>
    </section>
  );
}
