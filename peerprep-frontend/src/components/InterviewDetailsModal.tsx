import { InterviewHistoryItem } from "@/api/history";
import { startCase } from "lodash";

interface InterviewDetailsModalProps {
  interview: InterviewHistoryItem | null;
  currentUserId: string;
  onClose: () => void;
}

export function InterviewDetailsModal({
  interview,
  currentUserId,
  onClose,
}: InterviewDetailsModalProps) {
  if (!interview) return null;

  const formatDuration = (seconds: number) => {
    const minutes = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${minutes}m ${secs}s`;
  };

  const formatDateTime = (isoDate: string) => {
    try {
      const date = new Date(isoDate);
      return date.toLocaleString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
        hour: "numeric",
        minute: "2-digit",
        hour12: true,
      });
    } catch {
      return isoDate;
    }
  };

  const partnerName =
    interview.user1Id === currentUserId ? interview.user2Name : interview.user1Name;

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "rgba(0, 0, 0, 0.5)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 10000,
      }}
      onClick={onClose}
    >
      <div
        style={{
          backgroundColor: "white",
          borderRadius: "8px",
          padding: "24px",
          maxWidth: "800px",
          maxHeight: "90vh",
          width: "90%",
          overflow: "auto",
          boxShadow: "0 4px 16px rgba(0,0,0,0.2)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "24px",
          }}
        >
          <h2 style={{ fontSize: "24px", fontWeight: "bold", margin: 0 }}>
            Interview Session Details
          </h2>
          <button
            onClick={onClose}
            style={{
              fontSize: "24px",
              background: "none",
              border: "none",
              cursor: "pointer",
              padding: "4px 8px",
            }}
          >
            ×
          </button>
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
          {/* Session Info Grid */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "140px 1fr",
              gap: "12px",
              fontSize: "15px",
            }}
          >
            <div style={{ fontWeight: "600" }}>Question:</div>
            <div>{interview.questionTitle}</div>

            <div style={{ fontWeight: "600" }}>Category:</div>
            <div>{startCase(interview.category)}</div>

            <div style={{ fontWeight: "600" }}>Difficulty:</div>
            <div>
              <span
                style={{
                  padding: "2px 8px",
                  borderRadius: "4px",
                  backgroundColor:
                    interview.difficulty === "easy"
                      ? "#d4edda"
                      : interview.difficulty === "medium"
                      ? "#fff3cd"
                      : "#f8d7da",
                  color:
                    interview.difficulty === "easy"
                      ? "#155724"
                      : interview.difficulty === "medium"
                      ? "#856404"
                      : "#721c24",
                  fontWeight: "500",
                }}
              >
                {startCase(interview.difficulty)}
              </span>
            </div>

            <div style={{ fontWeight: "600" }}>Language:</div>
            <div>{startCase(interview.language)}</div>

            <div style={{ fontWeight: "600" }}>Duration:</div>
            <div>{formatDuration(interview.durationSeconds)}</div>

            <div style={{ fontWeight: "600" }}>Partner:</div>
            <div>{partnerName}</div>

            <div style={{ fontWeight: "600" }}>Rerolls Used:</div>
            <div>{interview.rerollsUsed}</div>

            <div style={{ fontWeight: "600" }}>Started:</div>
            <div>{formatDateTime(interview.startedAt)}</div>

            <div style={{ fontWeight: "600" }}>Ended:</div>
            <div>{formatDateTime(interview.endedAt)}</div>
          </div>

          {/* Code Section */}
          <div style={{ marginTop: "8px" }}>
            <div style={{ fontWeight: "600", marginBottom: "8px", fontSize: "15px" }}>
              Final Code:
            </div>
            <div
              style={{
                backgroundColor: "#f5f5f5",
                border: "1px solid #ddd",
                borderRadius: "4px",
                padding: "16px",
                fontFamily: "monospace",
                fontSize: "13px",
                whiteSpace: "pre-wrap",
                overflowX: "auto",
                maxHeight: "400px",
                overflowY: "auto",
              }}
            >
              {interview.finalCode || "(No code saved)"}
            </div>
          </div>

          {/* Close Button */}
          <div style={{ display: "flex", justifyContent: "flex-end", marginTop: "8px" }}>
            <button
              onClick={onClose}
              style={{
                backgroundColor: "#2F6FED",
                color: "white",
                border: "none",
                padding: "10px 24px",
                borderRadius: "4px",
                cursor: "pointer",
                fontSize: "14px",
                fontWeight: "500",
              }}
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
