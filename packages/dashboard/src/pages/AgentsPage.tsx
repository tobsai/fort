import { useEffect, useState } from "react";
import { useFortSocket } from "../contexts/FortSocketContext";
import type { AgentInfo, WSMessage } from "../types";

export default function AgentsPage() {
  const { send, subscribe } = useFortSocket();
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [selected, setSelected] = useState<AgentInfo | null>(null);

  useEffect(() => {
    const unsub = subscribe("agents.response", (msg: WSMessage) => {
      setAgents((msg.payload as AgentInfo[]) || []);
    });
    send("agents");
    return unsub;
  }, [send, subscribe]);

  return (
    <div className="agents-page">
      <div className="agents-grid">
        {agents.map((a) => {
          const cfg = a.config;
          return (
            <div
              key={cfg.id}
              className="agent-card"
              onClick={() => setSelected(a)}
            >
              <div className="agent-card-emoji">{a.emoji || "🤖"}</div>
              <div className="agent-card-info">
                <div className="agent-card-name">{cfg.name}</div>
                <div className="agent-card-desc">{cfg.description}</div>
              </div>
              <span className={`agent-status-badge ${a.status}`}>
                {a.status}
              </span>
            </div>
          );
        })}
        {agents.length === 0 && (
          <div className="empty-state">No agents created yet</div>
        )}
      </div>

      {selected && (
        <div className="modal-overlay" onClick={(e) => {
          if (e.target === e.currentTarget) setSelected(null);
        }}>
          <div className="modal">
            <div className="modal-header">
              <span className="modal-emoji">{selected.emoji || "🤖"}</span>
              <span className="modal-title">{selected.config.name}</span>
              <button className="modal-close" onClick={() => setSelected(null)}>
                &times;
              </button>
            </div>
            <div className="modal-body">
              {selected.soul || "No SOUL.md content available for this agent."}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
