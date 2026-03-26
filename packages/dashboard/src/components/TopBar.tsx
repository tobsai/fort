import { useEffect, useState } from "react";
import { NavLink } from "react-router-dom";
import { useFortSocket } from "../contexts/FortSocketContext";

export default function TopBar() {
  const { connected, send, subscribe } = useFortSocket();
  const [pendingCount, setPendingCount] = useState(0);

  useEffect(() => {
    // Fetch initial pending count
    send("approvals.list");

    const unsubList = subscribe("approvals.list.response", (msg) => {
      const payload = msg.payload as { pending: unknown[] };
      setPendingCount(payload.pending?.length ?? 0);
    });

    // Update count when a new approval arrives
    const unsubNew = subscribe("approval.new", () => {
      send("approvals.list");
    });

    // Update count when an approval is resolved
    const unsubResolved = subscribe("approval.respond.response", () => {
      send("approvals.list");
    });

    return () => {
      unsubList();
      unsubNew();
      unsubResolved();
    };
  }, [send, subscribe]);

  return (
    <header className="topbar">
      <span className="logo">FORT</span>
      <nav className="nav-tabs">
        <NavLink to="/chat" className={({ isActive }) => `nav-tab${isActive ? " active" : ""}`}>
          Chat
        </NavLink>
        <NavLink to="/dashboard" className={({ isActive }) => `nav-tab${isActive ? " active" : ""}`}>
          Dashboard
        </NavLink>
        <NavLink to="/agents" className={({ isActive }) => `nav-tab${isActive ? " active" : ""}`}>
          Agents
        </NavLink>
        <NavLink to="/settings" className={({ isActive }) => `nav-tab${isActive ? " active" : ""}`}>
          Settings
        </NavLink>
        <NavLink to="/usage" className={({ isActive }) => `nav-tab${isActive ? " active" : ""}`}>
          Usage
        </NavLink>
        <NavLink to="/schedules" className={({ isActive }) => `nav-tab${isActive ? " active" : ""}`}>
          Schedules
        </NavLink>
        <NavLink to="/diagnostics" className={({ isActive }) => `nav-tab${isActive ? " active" : ""}`}>
          Diagnostics
        </NavLink>
      </nav>
      <div className="topbar-right">
        <NavLink
          to="/approvals"
          title="Tool Approvals"
          style={{ position: "relative", textDecoration: "none", marginRight: "0.75rem" }}
        >
          <span style={{ fontSize: 18 }}>🛡️</span>
          {pendingCount > 0 && (
            <span style={{
              position: "absolute",
              top: -6,
              right: -8,
              background: "#ef4444",
              color: "#fff",
              borderRadius: "50%",
              fontSize: 10,
              fontWeight: 700,
              minWidth: 16,
              height: 16,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              padding: "0 3px",
            }}>
              {pendingCount}
            </span>
          )}
        </NavLink>
        <span className={`ws-dot${connected ? " connected" : ""}`} />
        <span className="ws-label">{connected ? "Connected" : "Disconnected"}</span>
      </div>
    </header>
  );
}
