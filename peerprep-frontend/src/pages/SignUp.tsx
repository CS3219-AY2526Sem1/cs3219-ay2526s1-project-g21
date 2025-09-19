import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import Field from "@/components/Field";
import { useAuth } from "@/context/AuthContext";

export default function SignUp() {
  const { login } = useAuth();
  const nav = useNavigate();
  const [form, set] = useState({ username: "", email: "", password: "", repeat: "" });

  return (
    <div className="mx-auto max-w-md px-6 py-14">
      <h1 className="mb-8 text-3xl font-semibold text-black text-center">
        Let’s Start Practicing!
      </h1>

      <form
        className="space-y-5"
        onSubmit={(e) => {
          e.preventDefault();
          login(form.email, form.password); // TODO: IMPLEMENT AUTHENTICATION
          nav("/");
        }}
      >
        <Field label="Username" value={form.username} onChange={(e) => set({ ...form, username: e.target.value })} />
        <Field label="Email" type="email" value={form.email} onChange={(e) => set({ ...form, email: e.target.value })} />
        <Field label="Password" type="password" value={form.password} onChange={(e) => set({ ...form, password: e.target.value })} />
        <Field label="Repeat Password" type="password" value={form.repeat} onChange={(e) => set({ ...form, repeat: e.target.value })} />

        <button className="w-full rounded-md bg-[#2F6FED] px-4 py-2.5 text-white font-medium">
          Sign Up
        </button>
        <Link to="/login" className="block w-full rounded-md border border-[#D1D5DB] px-4 py-2.5 text-center">
          Log Into an Existing Account
        </Link>
      </form>
    </div>
  );
}
