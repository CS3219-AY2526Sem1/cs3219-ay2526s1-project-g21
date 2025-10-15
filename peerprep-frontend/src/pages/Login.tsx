import { useState } from "react";
import { Link } from "react-router-dom";
import Field from "@/components/Field";
import { useAuth } from "@/context/AuthContext";
import { handleFormChange } from "@/utils/form";
import toast from "react-hot-toast";

export default function Login() {
  const { login } = useAuth();

  const [form, setForm] = useState({ username: "", password: "" });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await login(form.username, form.password);
      toast.success("Successfully logged in", {
        position: "bottom-center",
      });
    } catch (err) {
      let message = "Login failed";
      if (err instanceof Error) {
        const errMessageJson = JSON.parse(err.message)
        message = errMessageJson.error;
      }
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mx-auto max-w-md px-6 py-14">
      <h1 className="mb-8 text-3xl font-semibold text-black text-center">
        Let’s Start Practicing!
      </h1>

      <form className="space-y-5" onSubmit={onSubmit}>
        {error && (
          <div className="rounded-md border border-red-300 bg-red-50 px-4 py-2 text-sm text-red-700">
            {error}
          </div>
        )}
        <Field
          label="Username"
          name="username"
          value={form.username}
          onChange={(e) => handleFormChange(e, setForm)}
        />
        <Field
          label="Password"
          type="password"
          name="password"
          value={form.password}
          onChange={(e) => handleFormChange(e, setForm)}
        />

        <button
          type="submit"
          className="w-full rounded-md bg-[#2F6FED] px-4 py-2.5 text-white font-medium disabled:opacity-60"
          disabled={loading}
        >
          {loading ? "Logging in..." : "Log In"}
        </button>

        <Link to="/forgot" className="block w-full rounded-md border border-[#D1D5DB] px-4 py-2.5 text-center">
          Forgot Username/Password?
        </Link>

        <Link to="/signup" className="block w-full rounded-md bg-[#F3F4F6] px-4 py-2.5 text-center">
          Create a New Account
        </Link>
      </form>
    </div>
  );
}
