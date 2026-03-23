import { useEffect, useState, useCallback, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useFortSocket } from "../contexts/FortSocketContext";
import { fetchChatHistory, fetchLLMStatus } from "../utils/api";
import type { AgentInfo, ChatMessage, WSMessage } from "../types";

export default function ChatPage() {
  const { agentId } = useParams<{ agentId?: string }>();
  const navigate = useNavigate();
  const { send, subscribe } = useFortSocket();
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [chatMessages, setChatMessages] = useState<Record<string, ChatMessage[]>>({});
  const [input, setInput] = useState("");
  const [modelTier, setModelTier] = useState<"auto" | "fast" | "standard" | "powerful">("auto");
  const [hasGreeted, setHasGreeted] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const historyLoadedRef = useRef(false);
  const handledTaskIds = useRef(new Set<string>());

  const selectedAgent = agentId || null;

  // Load agents
  useEffect(() => {
    const unsub = subscribe("agents.response", (msg: WSMessage) => {
      setAgents(msg.payload as AgentInfo[]);
    });
    send("agents");
    return unsub;
  }, [send, subscribe]);

  // Auto-select default agent if none specified
  useEffect(() => {
    if (!agentId && agents.length > 0) {
      const running = agents.find((a) => a.status === "running");
      if (running) {
        navigate(`/chat/${running.config.id}`, { replace: true });
      }
    }
  }, [agentId, agents, navigate]);

  // Load chat history
  useEffect(() => {
    if (historyLoadedRef.current) return;
    historyLoadedRef.current = true;
    fetchChatHistory()
      .then((tasks) => {
        const msgs: Record<string, ChatMessage[]> = {};
        for (const t of tasks) {
          const aid = t.assignedAgent;
          if (!aid) continue;
          if (!msgs[aid]) msgs[aid] = [];
          const isGreeting =
            t.source === "background" &&
            (t.description || "").includes("Please greet me");
          if (!isGreeting) {
            msgs[aid].push({
              role: "user",
              text: t.description,
              ts: new Date(t.createdAt).getTime(),
            });
          }
          if (t.result) {
            msgs[aid].push({
              role: "agent",
              text: t.result,
              ts: new Date(t.createdAt).getTime() + 1,
              task: isGreeting
                ? null
                : { shortId: t.shortId, title: t.title, status: t.status },
            });
          }
        }
        setChatMessages(msgs);
      })
      .catch(() => {});
  }, []);

  // Auto-greet (only if LLM is valid)
  useEffect(() => {
    if (!selectedAgent || hasGreeted) return;
    const msgs = chatMessages[selectedAgent];
    if (msgs && msgs.length > 0) {
      setHasGreeted(true);
      return;
    }
    if (!historyLoadedRef.current) return;
    setHasGreeted(true);
    // Validate LLM before auto-greeting
    fetchLLMStatus()
      .then((status) => {
        if (status?.valid) {
          send("chat", { text: "__greeting__", agentId: selectedAgent, hidden: true });
        }
      })
      .catch(() => {});
  }, [selectedAgent, chatMessages, hasGreeted, send]);

  // Handle incoming messages
  const addMessage = useCallback(
    (aid: string, role: "user" | "agent", text: string, task?: ChatMessage["task"]) => {
      setChatMessages((prev) => ({
        ...prev,
        [aid]: [...(prev[aid] || []), { role, text, ts: Date.now(), task }],
      }));
    },
    [],
  );

  useEffect(() => {
    const unsubs = [
      subscribe("chat.response", (msg: WSMessage) => {
        const p = msg.payload as {
          hidden?: boolean;
          taskId?: string;
          task?: { id?: string; result?: string; assignedAgent?: string };
        };
        // Track all tasks from chat.response so task.status_changed can skip them
        const taskId = p?.taskId || p?.task?.id;
        if (taskId) handledTaskIds.current.add(taskId);
        // Only show hidden (greeting) messages here — normal messages come via task.status_changed
        if (p?.hidden && p.task?.result) {
          const aid = p.task.assignedAgent || selectedAgent;
          if (aid) addMessage(aid, "agent", p.task.result);
        }
      }),
      subscribe("task.status_changed", (msg: WSMessage) => {
        const t = msg.payload as {
          id?: string;
          result?: string;
          source?: string;
          status?: string;
          assignedAgent?: string;
          title?: string;
          shortId?: string;
        };
        if (!t?.result) return;
        if (t.status !== "completed" && t.status !== "failed") return;
        if (t.source !== "user_chat" && t.source !== "background") return;
        if (t.id && handledTaskIds.current.has(t.id)) return;
        const aid = t.assignedAgent;
        if (!aid) return;
        const isGreeting =
          t.source === "background" && (t.title || "").includes("Please greet me");
        // Skip greeting responses here — they're handled by chat.response
        if (isGreeting) return;
        addMessage(aid, "agent", t.result, {
          shortId: t.shortId || "",
          title: t.title || "",
          status: t.status || "completed",
        });
      }),
    ];
    return () => unsubs.forEach((u) => u());
  }, [subscribe, addMessage, selectedAgent]);

  // Scroll to bottom on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [chatMessages, selectedAgent]);

  const sendChat = () => {
    const text = input.trim();
    if (!text || !selectedAgent) return;
    setInput("");
    addMessage(selectedAgent, "user", text);
    const payload: Record<string, unknown> = { text, agentId: selectedAgent };
    if (modelTier !== "auto") payload.modelTier = modelTier;
    send("chat", payload);
  };

  const currentAgent = agents.find((a) => a.config.id === selectedAgent);
  const messages = selectedAgent ? chatMessages[selectedAgent] || [] : [];

  const getEmoji = (aid: string) => {
    const a = agents.find((x) => x.config.id === aid);
    return a?.emoji || "🤖";
  };

  return (
    <div className="chat-layout">
      <div className="chat-sidebar">
        <div className="chat-sidebar-header">Agents</div>
        <div className="chat-agent-list">
          {agents.map((a) => (
            <div
              key={a.config.id}
              className={`chat-agent-item${selectedAgent === a.config.id ? " active" : ""}`}
              onClick={() => navigate(`/chat/${a.config.id}`)}
            >
              <span className="chat-agent-item-emoji">{a.emoji || "🤖"}</span>
              <span className="chat-agent-item-name">{a.config.name}</span>
            </div>
          ))}
          {agents.length === 0 && (
            <div style={{ padding: 16, color: "var(--text-secondary)", fontSize: 12 }}>
              No agents available
            </div>
          )}
        </div>
      </div>
      <div className="chat-main">
        <div className="chat-header">
          <span className="chat-header-emoji">
            {currentAgent ? getEmoji(currentAgent.config.id) : "💬"}
          </span>
          <span className="chat-header-name">
            {currentAgent?.config.name || "Select an agent"}
          </span>
        </div>
        <div className="chat-messages">
          {messages.map((m, i) => (
            <div key={i} className={`chat-msg ${m.role}`}>
              <div className="chat-msg-avatar">
                {m.role === "user" ? "You" : getEmoji(selectedAgent!)}
              </div>
              <div className="chat-msg-body">
                <div className="chat-msg-content">
                  {m.text}
                  <button
                    className="copy-btn"
                    title="Copy"
                    onClick={() => navigator.clipboard.writeText(m.text)}
                  >
                    Copy
                  </button>
                </div>
                {m.task && (
                  <div className={`chat-msg-task ${m.task.status}`}>
                    <span className="task-id">{m.task.shortId}</span>
                    <span className="task-title">{m.task.title}</span>
                    <span className={`task-status-badge ${m.task.status}`}>
                      {m.task.status}
                    </span>
                  </div>
                )}
              </div>
            </div>
          ))}
          <div ref={messagesEndRef} />
        </div>
        <div className="chat-input-bar">
          <select
            className="model-select"
            value={modelTier}
            onChange={(e) => setModelTier(e.target.value as typeof modelTier)}
            disabled={!selectedAgent}
          >
            <option value="auto">Auto</option>
            <option value="fast">Fast</option>
            <option value="standard">Standard</option>
            <option value="powerful">Powerful</option>
          </select>
          <input
            className="chat-input"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                sendChat();
              }
            }}
            placeholder={selectedAgent ? "Type a message..." : "Select an agent first"}
            disabled={!selectedAgent}
          />
          <button className="chat-send-btn" onClick={sendChat} disabled={!selectedAgent}>
            Send
          </button>
        </div>
      </div>
    </div>
  );
}
