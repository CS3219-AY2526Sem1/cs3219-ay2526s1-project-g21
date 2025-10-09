import { InputHTMLAttributes } from "react";

type Props = InputHTMLAttributes<HTMLInputElement> & { label: string };

export default function Field({ label, ...props }: Props) {
  return (
    <label className="block text-left">
      <div className="text-sm font-medium text-black">{label}</div>
      <input
        {...props}
        className="mt-2 w-full rounded-md border border-[#D1D5DB] bg-white px-3 py-2
                   text-[15px] outline-none focus:border-[#2F6FED]"
      />
    </label>
  );
}
