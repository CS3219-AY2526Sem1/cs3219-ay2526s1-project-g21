import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import Field from "@/components/Field";
import { handleFormChange } from "@/utils/form";
import { register } from "@/api/auth";

export default function SignUp() {
  const nav = useNavigate();
  const [form, setForm] = useState({
    username: "",
    email: "",
    password: "",
    repeat: "",
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!form.username || !form.email || !form.password || !form.repeat) {
      setError("All fields are required.");
      return;
    }
    if (form.password !== form.repeat) {
      setError("Passwords do not match.");
      return;
    }

    setLoading(true);
    try {
      await register(form.username, form.email, form.password);
      nav("/login");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Registration failed";
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
          label="Email"
          type="email"
          name="email"
          value={form.email}
          onChange={(e) => handleFormChange(e, setForm)}
        />
        <Field
          label="Password"
          type="password"
          name="password"
          value={form.password}
          onChange={(e) => handleFormChange(e, setForm)}
        />
        <Field
          label="Repeat Password"
          type="password"
          name="repeat"
          value={form.repeat}
          onChange={(e) => handleFormChange(e, setForm)}
        />

        <button className="w-full rounded-md bg-[#2F6FED] px-4 py-2.5 text-white font-medium disabled:opacity-60" disabled={loading}>
          {loading ? "Creating account..." : "Sign Up"}
        </button>

        <Link
          to="/login"
          className="block w-full rounded-md border border-[#D1D5DB] px-4 py-2.5 text-center"
        >
          Log Into an Existing Account
        </Link>
      </form>
    </div>
  );
}
