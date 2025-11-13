import { useEffect, useState } from "react";
import { confirmEmailChange } from "@/api/auth";
import { useSearchParams, Link } from "react-router-dom";

export default function ConfirmEmail() {
  const [params] = useSearchParams();
  const [status, setStatus] = useState<"pending" | "ok" | "error">("pending");
  const [message, setMessage] = useState<string>("Confirming your new email...");

  useEffect(() => {
    const token = params.get("token");
    const statusParam = params.get("status");

    // If redirected from backend after successful confirmation
    if (!token && statusParam === "ok") {
      setStatus("ok");
      setMessage("Your email has been updated.");
      return;
    }

    if (!token) {
      setStatus("error");
      setMessage("Missing token.");
      return;
    }

    (async () => {
      try {
        await confirmEmailChange(token);
        setStatus("ok");
        setMessage("Your email has been updated.");
      } catch (e: any) {
        setStatus("error");
        setMessage(e?.message || "Token invalid or expired.");
      }
    })();
  }, [params]);

  return (
    <section className="mx-auto max-w-xl px-6 py-20 text-center">
      <h1 className="text-2xl font-semibold text-black mb-4">Confirm Email Change</h1>
      <p className={status === "ok" ? "text-green-700" : status === "error" ? "text-red-700" : "text-slate-700"}>{message}</p>
      <div className="mt-6">
        <Link to="/account" className="rounded-md bg-[#2F6FED] px-4 py-2 text-sm font-medium text-white hover:brightness-95">Back to Account</Link>
      </div>
    </section>
  );
}


