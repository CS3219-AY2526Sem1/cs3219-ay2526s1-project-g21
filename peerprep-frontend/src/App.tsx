import { Route, Routes, Navigate } from "react-router-dom";
import { useEffect, useState } from "react";
import NavBar from "@/components/Nav";
import { ActiveRoomBanner } from "@/components/ActiveRoomBanner";
import Home from "@/pages/Home";
import Login from "@/pages/Login";
import InterviewLobby from "@/pages/InterviewLobby";
import SignUp from "@/pages/SignUp";
import Forgot from "@/pages/Forgot";
import Account from "@/pages/Account";
import Questions from "@/pages/Questions";
import Editor from "@/pages/Editor";
import PageNotFound from "@/pages/PageNotFound";
import VerifyAccount from "@/pages/VerifyAccount";
import ConfirmEmail from "@/pages/ConfirmEmail";
import AdminModels from "@/pages/AdminModels";
import { useAuth } from "@/context/AuthContext";
import { getMe } from "@/api/auth";
import { Toaster } from 'react-hot-toast'

function Protected({ children }: { children: JSX.Element }) {
  const { isLoggedIn } = useAuth();
  return isLoggedIn ? children : <Navigate to="/login" replace />;
}

export default function App() {
  const { isLoggedIn, token } = useAuth();
  const [userId, setUserId] = useState<string | null>(null);

  useEffect(() => {
    const fetchUserId = async () => {
      if (isLoggedIn && token) {
        try {
          const user = await getMe(token);
          setUserId(String(user.id));
        } catch (error) {
          console.error("Failed to fetch user info:", error);
        }
      } else {
        setUserId(null);
      }
    };

    fetchUserId();
  }, [isLoggedIn, token]);

  return (
    <div className="min-h-screen bg-white">
      <NavBar />
      <Toaster />
      {isLoggedIn && userId && <ActiveRoomBanner userId={userId} />}
      <main className="mx-auto max-w-7xl px-6 py-12">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/login" element={<Login />} />
          <Route path="/signup" element={<SignUp />} />
          <Route path="/forgot" element={<Forgot />} />
          <Route path="/verifyaccount" element={<VerifyAccount />} />
          <Route path="/changeemail" element={<ConfirmEmail />} />

          <Route path="/account" element={<Protected><Account /></Protected>} />
          <Route path="/interview" element={<Protected><InterviewLobby /></Protected>} />
          <Route path="/questions" element={<Protected><Questions /></Protected>} />
          <Route path="/room/:roomId" element={<Protected><Editor /></Protected>} />

          <Route path="/admin" element={<AdminModels />} />

          <Route path="*" element={<PageNotFound />} />
        </Routes>
      </main>
    </div>
  );
}
