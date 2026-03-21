interface Agent {
  id: string;
  name: string;
  status: "running" | "idle";
  currentTask?: string;
  tokensUsed: number;
  uptime: string;
}

const MOCK_AGENTS: Agent[] = [
  {
    id: "agent-alpha",
    name: "Agent Alpha",
    status: "running",
    currentTask: "Refactor authentication module",
    tokensUsed: 24500,
    uptime: "1h 23m",
  },
  {
    id: "agent-beta",
    name: "Agent Beta",
    status: "running",
    currentTask: "Write integration tests for API",
    tokensUsed: 18200,
    uptime: "47m",
  },
  {
    id: "agent-gamma",
    name: "Agent Gamma",
    status: "idle",
    tokensUsed: 0,
    uptime: "2h 10m",
  },
];

function AgentMonitor() {
  return (
    <div className="panel">
      <div className="panel-header">
        <h2>Agent Monitor</h2>
        <span style={{ fontSize: 12, color: "var(--text-muted)" }}>
          {MOCK_AGENTS.filter((a) => a.status === "running").length} running
          {" / "}
          {MOCK_AGENTS.length} total
        </span>
      </div>

      <div className="panel-body">
        <div className="agent-grid">
          {MOCK_AGENTS.map((agent) => (
            <div key={agent.id} className="agent-card">
              <div className="agent-card-header">
                <span className="agent-name">{agent.name}</span>
                <span className={`agent-status ${agent.status}`}>
                  {agent.status}
                </span>
              </div>

              {agent.currentTask && (
                <div className="agent-detail">
                  Task: {agent.currentTask}
                </div>
              )}

              <div className="agent-detail">
                Tokens: {agent.tokensUsed.toLocaleString()}
              </div>
              <div className="agent-detail">
                Uptime: {agent.uptime}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export default AgentMonitor;
