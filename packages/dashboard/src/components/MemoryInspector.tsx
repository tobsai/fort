interface MemoryNode {
  id: string;
  label: string;
  type: "context" | "fact" | "relation";
}

const MOCK_NODES: MemoryNode[] = [
  { id: "n-1", label: "Project Architecture", type: "context" },
  { id: "n-2", label: "Auth Flow", type: "context" },
  { id: "n-3", label: "uses-bcrypt", type: "fact" },
  { id: "n-4", label: "API rate limit: 100/min", type: "fact" },
  { id: "n-5", label: "auth -> database", type: "relation" },
  { id: "n-6", label: "worker -> queue", type: "relation" },
  { id: "n-7", label: "User Preferences", type: "context" },
  { id: "n-8", label: "deploy-target: k8s", type: "fact" },
];

const TYPE_ICONS: Record<MemoryNode["type"], string> = {
  context: "\u25cb",
  fact: "\u25a1",
  relation: "\u25c7",
};

function MemoryInspector() {
  const grouped = {
    context: MOCK_NODES.filter((n) => n.type === "context"),
    fact: MOCK_NODES.filter((n) => n.type === "fact"),
    relation: MOCK_NODES.filter((n) => n.type === "relation"),
  };

  return (
    <div className="panel">
      <div className="panel-header">
        <h2>Memory Inspector</h2>
        <span style={{ fontSize: 12, color: "var(--text-muted)" }}>
          {MOCK_NODES.length} nodes
        </span>
      </div>

      <div className="panel-body">
        <div className="memory-layout">
          <div className="memory-sidebar">
            {(["context", "fact", "relation"] as const).map((type) => (
              <div key={type}>
                <h3>{type}s</h3>
                {grouped[type].map((node) => (
                  <div key={node.id} className="memory-node">
                    <span>{TYPE_ICONS[node.type]}</span>
                    <span>{node.label}</span>
                  </div>
                ))}
              </div>
            ))}
          </div>

          <div className="memory-canvas">
            Graph visualization placeholder
          </div>
        </div>
      </div>
    </div>
  );
}

export default MemoryInspector;
