import { useEffect, useState } from "react";
import { verifyAccount } from "@/api/auth";
import { useSearchParams, Link } from "react-router-dom";

export default function VerifyAccount() {
  const [params] = useSearchParams();
  const [status, setStatus] = useState<"pending" | "ok" | "error">("pending");
  const [message, setMessage] = useState<string>("Verifying your account...");

  useEffect(() => {
    const token = params.get("token");
    const statusParam = params.get("status");

    // If redirected from backend after successful verification
    if (!token && statusParam === "ok") {
      setStatus("ok");
      setMessage("Your email is verified. You can now log in.");
      return;
    }

    if (!token) {
      setStatus("error");
      setMessage("Missing verification token.");
      return;
    }

    (async () => {
      try {
        await verifyAccount(token);
        setStatus("ok");
        setMessage("Your email is verified. You can now log in.");
      } catch (e: any) {
        setStatus("error");
        setMessage(e?.message || "Verification failed or token expired.");
      }
    })();
  }, [params]);

  return (
    <section className="mx-auto max-w-xl px-6 py-20 text-center">
      <h1 className="text-2xl font-semibold text-black mb-4">Email Verification</h1>
      <p className={status === "ok" ? "text-green-700" : status === "error" ? "text-red-700" : "text-slate-700"}>{message}</p>
      <div className="mt-6">
        <Link to="/login" className="rounded-md bg-[#2F6FED] px-4 py-2 text-sm font-medium text-white hover:brightness-95">Go to Login</Link>
      </div>
    </section>
  );
}


