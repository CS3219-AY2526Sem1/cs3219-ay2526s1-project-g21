import { Route, Routes, Navigate } from "react-router-dom";
import NavBar from "@/components/Nav";
import Home from "@/pages/Home";
import Login from "@/pages/Login";
import InterviewLobby from "@/pages/InterviewLobby";
import SignUp from "@/pages/SignUp";
import Forgot from "@/pages/Forgot";
import Account from "@/pages/Account";
import Questions from "@/pages/Questions";
import Editor from "@/pages/Editor";
import { useAuth } from "@/context/AuthContext";
import { Toaster } from 'react-hot-toast'

function Protected({ children }: { children: JSX.Element }) {
  const { isLoggedIn } = useAuth();
  return isLoggedIn ? children : <Navigate to="/login" replace />;
}

export default function App() {
  return (
    <div className="min-h-screen bg-white">
      <NavBar />
      <Toaster />
      <main className="mx-auto max-w-7xl px-6 py-12">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/login" element={<Login />} />
          <Route path="/signup" element={<SignUp />} />
          <Route path="/forgot" element={<Forgot />} />

          <Route path="/account" element={<Protected><Account /></Protected>} />
          <Route path="/interview" element={<Protected><InterviewLobby /></Protected>} />
          <Route path="/questions" element={<Protected><Questions /></Protected>} />
          <Route path="/room/:roomId" element={<Protected><Editor /></Protected>} />

          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  );
}
