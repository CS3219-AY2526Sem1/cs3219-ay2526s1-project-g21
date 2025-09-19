import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import Field from "@/components/Field";
import { useAuth } from "@/context/AuthContext";

export default function Login() {
  const { login } = useAuth();
  const nav = useNavigate();
  const [u, setU] = useState("");
  const [p, setP] = useState("");

  return (
    <div className="mx-auto max-w-md px-6 py-14">
      <h1 className="mb-8 text-3xl font-semibold text-black text-center">
        Let’s Start Practicing!
      </h1>

      <form
        className="space-y-5"
        onSubmit={(e) => {
          e.preventDefault();
          login(u, p);           // TODO: IMPLEMENT AUTHENTICATION
          nav("/");
        }}
      >
        <Field label="Username" value={u} onChange={(e) => setU(e.target.value)} />
        <Field label="Password" type="password" value={p} onChange={(e) => setP(e.target.value)} />

        <button type="submit" className="w-full rounded-md bg-[#2F6FED] px-4 py-2.5 text-white font-medium">
          Log In
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
