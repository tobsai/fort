import { useState } from "react";
import TaskPortal from "./components/TaskPortal";
import AgentMonitor from "./components/AgentMonitor";
import MemoryInspector from "./components/MemoryInspector";

type Tab = "tasks" | "agents" | "memory";

function App() {
  const [activeTab, setActiveTab] = useState<Tab>("tasks");

  const tabs: { key: Tab; label: string }[] = [
    { key: "tasks", label: "Tasks" },
    { key: "agents", label: "Agents" },
    { key: "memory", label: "Memory" },
  ];

  return (
    <>
      <header className="app-header">
        <span className="logo">FORT</span>
        <nav className="tab-nav">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              className={activeTab === tab.key ? "active" : ""}
              onClick={() => setActiveTab(tab.key)}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </header>

      <main className="app-content">
        {activeTab === "tasks" && <TaskPortal />}
        {activeTab === "agents" && <AgentMonitor />}
        {activeTab === "memory" && <MemoryInspector />}
      </main>
    </>
  );
}

export default App;
