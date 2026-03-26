import { useEffect, useState, useCallback } from "react";
import { useFortSocket } from "../contexts/FortSocketContext";
import { timeAgo } from "../utils/time";

interface ApprovalRequest {
  id: string;
  taskId: string;
  agentId: string;
  toolName: string;
  toolTier: number;
  parameters: unknown;
  status: "pending" | "approved" | "rejected";
  rejectionReason: string | null;
  createdAt: string;
  resolvedAt: string | null;
}

const tierBadgeStyle = (tier: number): React.CSSProperties => ({
  display: "inline-block",
  padding: "2px 8px",
  borderRadius: 4,
  fontSize: 11,
  fontWeight: 700,
  background: tier === 3 ? "#ff000033" : "#f59e0b33",
  color: tier === 3 ? "#ff6b6b" : "#f59e0b",
  border: `1px solid ${tier === 3 ? "#ff6b6b55" : "#f59e0b55"}`,
});

const cardStyle: React.CSSProperties = {
  background: "#1e1e28",
  border: "1px solid #333",
  borderRadius: 8,
  padding: "1rem 1.25rem",
  marginBottom: "1rem",
};

const btnStyle = (variant: "approve" | "reject" | "cancel"): React.CSSProperties => ({
  padding: "0.35rem 1rem",
  borderRadius: 6,
  border: "none",
  cursor: "pointer",
  fontWeight: 600,
  fontSize: 13,
  background:
    variant === "approve" ? "#22c55e" : variant === "reject" ? "#ef4444" : "#444",
  color: "#fff",
});

export default function ApprovalsPage() {
  const { send, subscribe } = useFortSocket();
  const [approvals, setApprovals] = useState<ApprovalRequest[]>([]);
  const [rejectingId, setRejectingId] = useState<string | null>(null);
  const [rejectReason, setRejectReason] = useState("");

  const fetchPending = useCallback(() => {
    send("approvals.list");
  }, [send]);

  useEffect(() => {
    fetchPending();

    const unsub = subscribe("approvals.list.response", (msg) => {
      const payload = msg.payload as { pending: ApprovalRequest[] };
      setApprovals(payload.pending ?? []);
    });

    // Real-time: new approval pushed from server
    const unsubNew = subscribe("approval.new", () => {
      fetchPending();
    });

    return () => {
      unsub();
      unsubNew();
    };
  }, [fetchPending, subscribe]);

  const handleApprove = (id: string) => {
    send("approval.respond", { id, approved: true });
    setApprovals((prev) => prev.filter((a) => a.id !== id));
  };

  const handleReject = (id: string) => {
    send("approval.respond", { id, approved: false, rejectionReason: rejectReason || undefined });
    setApprovals((prev) => prev.filter((a) => a.id !== id));
    setRejectingId(null);
    setRejectReason("");
  };

  return (
    <div style={{ padding: "1.5rem", maxWidth: 800, margin: "0 auto" }}>
      <h2 style={{ marginBottom: "0.25rem" }}>Tool Approvals</h2>
      <p style={{ color: "#888", fontSize: 13, marginBottom: "1.5rem" }}>
        Agents are waiting for your approval before executing these tools.
      </p>

      {approvals.length === 0 && (
        <div style={{ color: "#666", textAlign: "center", padding: "3rem 0" }}>
          No pending approvals
        </div>
      )}

      {approvals.map((approval) => (
        <div key={approval.id} style={cardStyle}>
          <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "0.5rem" }}>
            <span style={tierBadgeStyle(approval.toolTier)}>
              {approval.toolTier === 3 ? "🔴 TIER 3 — DESTRUCTIVE" : "⚠️ TIER 2 — Requires Approval"}
            </span>
            <span style={{ fontWeight: 700, fontSize: 15 }}>{approval.toolName}</span>
            <span style={{ color: "#888", fontSize: 12, marginLeft: "auto" }}>
              {timeAgo(new Date(approval.createdAt))}
            </span>
          </div>

          {approval.toolTier === 3 && (
            <div style={{
              background: "#ff000011",
              border: "1px solid #ff6b6b44",
              borderRadius: 6,
              padding: "0.5rem 0.75rem",
              marginBottom: "0.75rem",
              fontSize: 12,
              color: "#ff9999",
            }}>
              This is a destructive / irreversible action. Review carefully before approving.
            </div>
          )}

          <div style={{ display: "flex", gap: "1.5rem", marginBottom: "0.75rem", fontSize: 12, color: "#aaa" }}>
            <span>Agent: <strong style={{ color: "#ddd" }}>{approval.agentId}</strong></span>
            <span>Task: <strong style={{ color: "#ddd" }}>{approval.taskId}</strong></span>
          </div>

          <div style={{ marginBottom: "0.75rem" }}>
            <div style={{ fontSize: 11, color: "#888", marginBottom: "0.25rem" }}>Parameters</div>
            <pre style={{
              background: "#13131a",
              borderRadius: 6,
              padding: "0.75rem",
              fontSize: 12,
              color: "#c8e6c9",
              overflow: "auto",
              maxHeight: 200,
              margin: 0,
            }}>
              {JSON.stringify(approval.parameters, null, 2)}
            </pre>
          </div>

          {rejectingId === approval.id ? (
            <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
              <input
                type="text"
                placeholder="Rejection reason (optional)"
                value={rejectReason}
                onChange={(e) => { setRejectReason(e.target.value); }}
                style={{
                  flex: 1,
                  padding: "0.35rem 0.6rem",
                  borderRadius: 6,
                  border: "1px solid #444",
                  background: "#13131a",
                  color: "#eee",
                  fontSize: 13,
                }}
              />
              <button style={btnStyle("reject")} onClick={() => { handleReject(approval.id); }}>
                Confirm Reject
              </button>
              <button style={btnStyle("cancel")} onClick={() => { setRejectingId(null); setRejectReason(""); }}>
                Cancel
              </button>
            </div>
          ) : (
            <div style={{ display: "flex", gap: "0.5rem" }}>
              <button style={btnStyle("approve")} onClick={() => { handleApprove(approval.id); }}>
                Approve
              </button>
              <button style={btnStyle("reject")} onClick={() => { setRejectingId(approval.id); }}>
                Reject
              </button>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
