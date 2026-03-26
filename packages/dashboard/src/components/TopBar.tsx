import { NavLink } from "react-router-dom";
import { useFortSocket } from "../contexts/FortSocketContext";

export default function TopBar() {
  const { connected } = useFortSocket();

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
      </nav>
      <div className="topbar-right">
        <span className={`ws-dot${connected ? " connected" : ""}`} />
        <span className="ws-label">{connected ? "Connected" : "Disconnected"}</span>
      </div>
    </header>
  );
}
