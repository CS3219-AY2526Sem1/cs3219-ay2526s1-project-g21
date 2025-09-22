import Field from "@/components/Field";

export default function Forgot() {
  return (
    <div className="mx-auto max-w-md px-6 py-14">
      <h1 className="mb-8 text-3xl font-semibold text-black text-center">Reset your password</h1>
      <div className="space-y-5">
        <Field label="Email" type="email" placeholder="you@example.com" />
        <button className="w-full rounded-md bg-[#2F6FED] px-4 py-2.5 text-white font-medium">
          Send Reset Link
        </button>
      </div>
    </div>
  );
}
