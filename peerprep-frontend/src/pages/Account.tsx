import { useEffect, useState } from "react";
import { getMe } from "@/api/auth";
import { useAuth } from "@/context/AuthContext";

export default function Account() {
  const { token } = useAuth();
  const [user, setUser] = useState<{ id: number; username: string; email: string } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
            <div className="text-slate-500">Recent Activity</div>
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
                  <button className="rounded-md border border-[#D1D5DB] px-3 py-2 text-sm hover:bg-gray-50">
                    Change Username
                  </button>
                </div>
              </section>

              <hr className="my-2 border-[#E5E7EB]" />

              <section className="py-8">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h2 className="text-lg font-semibold text-black">Email Address</h2>
                    <p className="mt-2 text-black">{user.email}</p>
                  </div>
                  <button className="rounded-md border border-[#D1D5DB] px-3 py-2 text-sm hover:bg-gray-50">
                    Change Email
                  </button>
                </div>
              </section>

              <hr className="my-2 border-[#E5E7EB]" />

              <section className="pt-8">
                <button className="rounded-md bg-[#2F6FED] px-4 py-2 text-sm font-medium text-white hover:brightness-95">
                  Reset Password
                </button>
              </section>
            </>
          ) : null}
        </div>
      </div>
    </section>
  );
}
