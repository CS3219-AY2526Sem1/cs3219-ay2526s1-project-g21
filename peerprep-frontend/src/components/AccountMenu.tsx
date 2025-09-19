import { Link } from "react-router-dom";
import { useState, useRef, useEffect } from "react";
import { useAuth } from "@/context/AuthContext";

export default function AccountMenu() {
  const { logout } = useAuth();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, []);

  return (
    <div
      ref={ref}
      className="relative"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
    >
      <Link
        to="/account"
        className="text-[15px] font-medium px-2 py-1 rounded-md text-slate-600 hover:text-ink-900"
        onFocus={() => setOpen(true)}
        onBlur={(e) => {
          if (!ref.current?.contains(e.relatedTarget as Node)) setOpen(false);
        }}
      >
        My Account
      </Link>

      <div
        className={[
          "absolute left-0 top-full z-30 w-56 rounded-2xl border border-slate-200 bg-white shadow-card",
          "transition ease-out duration-150 origin-top",
          open ? "opacity-100 scale-100 visible" : "opacity-0 scale-95 invisible",
        ].join(" ")}
      >
        <div className="p-2">
          <Link
            to="/account"
            className="block rounded-lg px-3 py-2 text-sm text-ink-900 hover:bg-slate-50 focus:bg-slate-50 focus:outline-none"
            onClick={() => setOpen(false)}
          >
            Settings
          </Link>
          <button
            onClick={() => logout()}
            className="block w-full rounded-lg px-3 py-2 text-left text-sm text-red-600 hover:bg-red-50 focus:bg-red-50 focus:outline-none"
          >
            Log out
          </button>
        </div>
      </div>
    </div>
  );
}
