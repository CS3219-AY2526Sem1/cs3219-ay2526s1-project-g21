import React, { createContext, useContext, useState } from "react";

type AuthContextType = {
  isLoggedIn: boolean;
  login: (email: string, password: string) => void;
  logout: () => void;
};

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [isLoggedIn, setIsLoggedIn] = useState(false);

  const login = (email: string, password: string) => {
    // TODO: IMPLEMENT AUTHENTICATION
    console.log("Logged in with:", email, password);
    setIsLoggedIn(true);
  };

  const logout = () => {
    setIsLoggedIn(false);
    window.location.href = "/login";
  };

  return (
    <AuthContext.Provider value={{ isLoggedIn, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
};
