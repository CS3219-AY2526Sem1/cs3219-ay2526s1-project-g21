import React, { createContext, useContext, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { login as apiLogin } from "@/api/auth";
import toast from "react-hot-toast";

const SESSION_OWNER_KEY = "sessionOwner";
const TOKEN_KEY = "token";

type AuthContextType = {
  isLoggedIn: boolean;
  token: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
};

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY));
  const [tabId] = useState(() => {
    const existing = sessionStorage.getItem("tabId");
    if (existing) return existing;
    const newId = crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`;
    sessionStorage.setItem("tabId", newId);
    return newId;
  });
  const isLoggedIn = useMemo(() => !!token, [token]);
  const navigate = useNavigate();

  // Persist token, claim ownership, and check for other tab sessions
  useEffect(() => {
    if (!token) {
      localStorage.removeItem(TOKEN_KEY);
      if (localStorage.getItem(SESSION_OWNER_KEY) === tabId) {
        localStorage.removeItem(SESSION_OWNER_KEY);
      }
      return;
    }

    localStorage.setItem(TOKEN_KEY, token);

    const owner = localStorage.getItem(SESSION_OWNER_KEY);
    if (!owner || owner === tabId) {
      localStorage.setItem(SESSION_OWNER_KEY, tabId);
    } else {
      // Another tab owns the session → logout this tab
      setToken(null);
      navigate("/login");
      toast.error("You have been logged out by another tab", {
        position: "bottom-center",
      });
      return;
    }

    // Listen for ownership changes from other tabs
    const handleStorage = (e: StorageEvent) => {
      if (e.key === SESSION_OWNER_KEY && e.newValue !== tabId) {
        setToken(null);
        navigate("/login");
        toast.error("You have been logged out by another tab", {
          position: "bottom-center",
        });
      }
    };
    window.addEventListener("storage", handleStorage);
    return () => window.removeEventListener("storage", handleStorage);
  }, [token, tabId, navigate]);

  const login = async (username: string, password: string) => {
    const data = await apiLogin(username, password);
    setToken(data["token"]);
    localStorage.setItem(SESSION_OWNER_KEY, tabId);
    navigate("/");
  };

  const logout = () => {
    setToken(null);
    navigate("/login");
  };

  return (
    <AuthContext.Provider value={{ isLoggedIn, token, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
};
