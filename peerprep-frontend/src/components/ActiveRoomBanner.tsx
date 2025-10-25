import { useEffect, useState } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { checkActiveRoom } from "../api/history";

interface ActiveRoomBannerProps {
  userId: string;
}

export function ActiveRoomBanner({ userId }: ActiveRoomBannerProps) {
  const [activeRoom, setActiveRoom] = useState<{ matchId: string; status: string } | null>(null);
  const [isVisible, setIsVisible] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    const checkForActiveRoom = async () => {
      // Don't show banner if user is already in a room page
      const isInRoom = location.pathname.startsWith('/room/');
      if (isInRoom) {
        setIsVisible(false);
        return;
      }

      try {
        const response = await checkActiveRoom(userId);
        if (response.active && response.matchId) {
          setActiveRoom({ matchId: response.matchId, status: response.status || "ready" });
          setIsVisible(true);

          // Store token in sessionStorage for rejoining
          if (response.token) {
            sessionStorage.setItem(`room_token_${response.matchId}`, response.token);
          }
        } else {
          setActiveRoom(null);
          setIsVisible(false);
        }
      } catch (error) {
        console.error("Failed to check for active room:", error);
      }
    };

    // Check immediately
    checkForActiveRoom();

    // Check periodically (every 10 seconds)
    const interval = setInterval(checkForActiveRoom, 10000);

    return () => clearInterval(interval);
  }, [userId, location.pathname]);

  const handleRejoin = () => {
    if (activeRoom) {
      navigate(`/room/${activeRoom.matchId}`);
    }
  };

  const handleDismiss = () => {
    setIsVisible(false);
  };

  if (!isVisible || !activeRoom) {
    return null;
  }

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        backgroundColor: "#4CAF50",
        color: "white",
        padding: "12px 24px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        zIndex: 9999,
        boxShadow: "0 2px 8px rgba(0,0,0,0.2)",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: "16px" }}>
        <span style={{ fontWeight: "bold", fontSize: "16px" }}>
          📝 You have an ongoing interview session
        </span>
      </div>
      <div style={{ display: "flex", gap: "12px" }}>
        <button
          onClick={handleRejoin}
          style={{
            backgroundColor: "white",
            color: "#4CAF50",
            border: "none",
            padding: "8px 16px",
            borderRadius: "4px",
            fontWeight: "bold",
            cursor: "pointer",
            fontSize: "14px",
          }}
        >
          Rejoin Session
        </button>
        <button
          onClick={handleDismiss}
          style={{
            backgroundColor: "transparent",
            color: "white",
            border: "1px solid white",
            padding: "8px 16px",
            borderRadius: "4px",
            cursor: "pointer",
            fontSize: "14px",
          }}
        >
          Dismiss
        </button>
      </div>
    </div>
  );
}
