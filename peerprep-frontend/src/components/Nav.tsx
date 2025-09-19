import { Link, NavLink, NavLinkProps } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import AccountMenu from "@/components/AccountMenu";

export default function NavBar() {
  const { isLoggedIn } = useAuth();
  
  const link: NavLinkProps["className"] = ({ isActive }) =>
    "text-[15px] font-medium transition px-2 py-1 rounded-md " +
    (isActive ? "text-ink-900" : "text-slate-600 hover:text-ink-900");

  return (
    <header className="sticky top-0 z-20 border-b border-slate-200 bg-white">
      <div className="mx-auto max-w-7xl px-6 h-16 flex items-center justify-between">
        <Link to="/" className="text-2xl font-semibold tracking-tight text-ink-900">
          PeerPrep
        </Link>

        <nav className="flex items-center gap-6">
          <NavLink to="/interview" className={link}>Interview</NavLink>
          <NavLink to="/questions" className={link}>Questions</NavLink>

          {isLoggedIn ? (
            <AccountMenu />
          ) : (
            <NavLink
              to="/login"
              className="inline-flex items-center justify-center rounded-md bg-[#2F6FED] px-4 py-2 text-white text-sm font-medium hover:brightness-95"
            >
              Log In
            </NavLink>
          )}
        </nav>
      </div>
    </header>
  );
}
