import { useState } from "react";
import { Link, NavLink, NavLinkProps } from "react-router-dom";
import { Menu, X } from "lucide-react";
import { useAuth } from "@/context/AuthContext";
import AccountMenu from "@/components/AccountMenu";

export default function NavBar() {
  const { isLoggedIn, logout } = useAuth();
  const [mobileOpen, setMobileOpen] = useState(false);
  
  const link: NavLinkProps["className"] = ({ isActive }) =>
    "text-[15px] font-medium transition px-2 py-1 rounded-md " +
    (isActive ? "text-ink-900" : "text-slate-600 hover:text-ink-900");

  const closeMobileMenu = () => setMobileOpen(false);

  return (
    <header className="sticky top-0 z-20 border-b border-slate-200 bg-white">
      <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6">
        <Link to="/" className="text-2xl font-semibold tracking-tight text-ink-900">
          PeerPrep
        </Link>

        <nav className="hidden items-center gap-6 md:flex">
          <NavLink to="/interview" className={link} onClick={closeMobileMenu}>Interview</NavLink>
          <NavLink to="/questions" className={link} onClick={closeMobileMenu}>Questions</NavLink>
          {isLoggedIn ? (
            <AccountMenu />
          ) : (
            <NavLink
              to="/login"
              className="inline-flex items-center justify-center rounded-md bg-[#2F6FED] px-4 py-2 text-white text-sm font-medium hover:brightness-95"
            >
              Login
            </NavLink>
          )}
        </nav>

        <button
          type="button"
          className="inline-flex items-center justify-center rounded-md border border-slate-300 p-2 text-slate-600 hover:bg-slate-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-[#2F6FED] md:hidden"
          aria-label="Toggle navigation menu"
          aria-expanded={mobileOpen}
          aria-controls="mobile-nav"
          onClick={() => setMobileOpen((prev) => !prev)}
        >
          {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      </div>

      {mobileOpen ? (
        <div id="mobile-nav" className="border-t border-slate-200 bg-white md:hidden">
          <nav className="mx-auto flex max-w-7xl flex-col gap-2 px-4 py-4">
            <NavLink to="/interview" className={link} onClick={closeMobileMenu}>
              Interview
            </NavLink>
            <NavLink to="/questions" className={link} onClick={closeMobileMenu}>
              Questions
            </NavLink>
            {isLoggedIn ? (
              <>
                <NavLink
                  to="/account"
                  className="text-[15px] font-medium px-2 py-1 rounded-md text-slate-600 hover:text-ink-900"
                  onClick={closeMobileMenu}
                >
                  Account
                </NavLink>
                <button
                  type="button"
                  onClick={() => {
                    logout();
                    closeMobileMenu();
                  }}
                  className="text-left text-[15px] font-medium px-2 py-1 rounded-md text-red-600 hover:bg-red-50"
                >
                  Log out
                </button>
              </>
            ) : (
              <NavLink
                to="/login"
                className="inline-flex items-center justify-center rounded-md bg-[#2F6FED] px-4 py-2 text-white text-sm font-medium hover:brightness-95"
                onClick={closeMobileMenu}
              >
                Login
              </NavLink>
            )}
          </nav>
        </div>
      ) : null}
    </header>
  );
}
