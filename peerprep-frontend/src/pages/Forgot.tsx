import { useState } from "react";
import Field from "@/components/Field";
import { forgotPassword } from "@/api/auth";
import toast from "react-hot-toast";

export default function Forgot() {
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!email) {
      setError("Email is required");
      return;
    }
    setLoading(true);
    try {
      await forgotPassword(email);
      toast.success("If the email exists, recovery details have been sent.", {
        position: "bottom-center",
      });
    } catch (err) {
      // Silently succeed to avoid enumeration; show generic message
      toast.success("If the email exists, recovery details have been sent.", {
        position: "bottom-center",
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mx-auto max-w-md px-6 py-14">
      <h1 className="mb-8 text-3xl font-semibold text-black text-center">Reset your password</h1>
      <form className="space-y-5" onSubmit={onSubmit}>
        {error && (
          <div className="rounded-md border border-red-300 bg-red-50 px-4 py-2 text-sm text-red-700">{error}</div>
        )}
        <Field
          label="Email"
          type="email"
          placeholder="you@example.com"
          name="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
        <button type="submit" className="w-full rounded-md bg-[#2F6FED] px-4 py-2.5 text-white font-medium disabled:opacity-60" disabled={loading}>
          {loading ? "Sending..." : "Send Recovery Email"}
        </button>
      </form>
    </div>
  );
}
