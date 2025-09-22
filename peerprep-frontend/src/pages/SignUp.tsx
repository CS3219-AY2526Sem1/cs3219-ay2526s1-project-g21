import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import Field from "@/components/Field";
import { handleFormChange } from "@/utils/form";

export default function SignUp() {
  const nav = useNavigate();
  const [form, setForm] = useState({
    username: "",
    email: "",
    password: "",
    repeat: "",
  });

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    nav("/login"); // Redirect to login page after sign up
  };

  return (
    <div className="mx-auto max-w-md px-6 py-14">
      <h1 className="mb-8 text-3xl font-semibold text-black text-center">
        Let’s Start Practicing!
      </h1>

      <form className="space-y-5" onSubmit={onSubmit}>
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

        <button className="w-full rounded-md bg-[#2F6FED] px-4 py-2.5 text-white font-medium">
          Sign Up
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
