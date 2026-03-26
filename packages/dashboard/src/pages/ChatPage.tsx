import { useEffect, useState, useCallback, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useFortSocket } from "../contexts/FortSocketContext";
import { fetchLLMStatus } from "../utils/api";
import ToolCallBlock from "../components/ToolCallBlock";
import type { AgentInfo, ChatMessage, ToolCallEvent, WSMessage, ThreadMessage } from "../types";

export default function ChatPage() {
  const { agentId } = useParams<{ agentId?: string }>();
  const navigate = useNavigate();
  const { send, subscribe, connected } = useFortSocket();
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [chatMessages, setChatMessages] = useState<Record<string, ChatMessage[]>>({});
  const [input, setInput] = useState("");
  const [modelTier, setModelTier] = useState<"auto" | "fast" | "standard" | "powerful">("auto");
  const [hasGreeted, setHasGreeted] = useState(false);
  const [thinkingAgents, setThinkingAgents] = useState<Set<string>>(new Set());
  const messagesEndRef = useRef<HTMLDivElement>(null);
  // threadIdByAgent: persists the thread ID per agent so refreshes restore context
  const threadIdByAgent = useRef<Record<string, string>>(
    (() => {
      try {
        return JSON.parse(localStorage.getItem("fort.threadIdByAgent") ?? "{}") as Record<string, string>;
      } catch {
        return {};
      }
    })(),
  );
  const historyRequestedRef = useRef<Record<string, boolean>>({});
  const [historyFetched, setHistoryFetched] = useState(false);
  const shownTaskIds = useRef(new Set<string>());

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

  // Load thread history when agent is selected or connection is restored
  useEffect(() => {
    if (!selectedAgent || !connected) return;
    if (historyRequestedRef.current[selectedAgent]) return;
    historyRequestedRef.current[selectedAgent] = true;
    send("thread.history", { agentId: selectedAgent });
  }, [selectedAgent, connected, send]);

  // Handle thread.history.response — convert ThreadMessages to ChatMessages
  useEffect(() => {
    const unsub = subscribe("thread.history.response", (msg: WSMessage) => {
      const p = msg.payload as {
        agentId?: string;
        threadId?: string | null;
        messages: ThreadMessage[];
      };
      const aid = p?.agentId;
      if (!aid) return;

      // Persist threadId for future fork support
      if (p.threadId) {
        threadIdByAgent.current[aid] = p.threadId;
        try {
          localStorage.setItem("fort.threadIdByAgent", JSON.stringify(threadIdByAgent.current));
        } catch {
          /* localStorage may be unavailable */
        }
      }

      const converted: ChatMessage[] = (p.messages ?? [])
        .filter((m) => m.role === "user" || m.role === "agent")
        .map((m) => ({
          role: m.role as "user" | "agent",
          text: m.content,
          ts: new Date(m.createdAt).getTime(),
        }));

      setChatMessages((prev) => ({ ...prev, [aid]: converted }));
      setHistoryFetched(true);
    });
    return unsub;
  }, [subscribe]);

  // Auto-greet: only on first-ever conversation with an agent (no history at all)
  useEffect(() => {
    if (!selectedAgent || hasGreeted) return;
    // Wait until history fetch has actually completed
    if (!historyFetched) return;
    const msgs = chatMessages[selectedAgent];
    // If the agent already has ANY chat history, skip greeting entirely
    if (msgs && msgs.length > 0) {
      setHasGreeted(true);
      return;
    }
    setHasGreeted(true);
    // Only greet if this is a brand-new agent with zero history
    fetchLLMStatus()
      .then((status) => {
        if (status?.valid) {
          send("chat", { text: "__greeting__", agentId: selectedAgent, hidden: true });
        }
      })
      .catch(() => {});
  }, [selectedAgent, chatMessages, hasGreeted, historyFetched, send]);

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

  const addToolMessage = useCallback(
    (
      agentId: string,
      event: ToolCallEvent,
      eventType: "tool.executed" | "tool.denied" | "tool.error",
    ) => {
      setChatMessages((prev) => ({
        ...prev,
        [agentId]: [
          ...(prev[agentId] || []),
          {
            role: "tool" as const,
            text: event.toolName,
            ts: event.calledAt ? new Date(event.calledAt).getTime() : Date.now(),
            toolCall: event,
            toolEventType: eventType,
          },
        ],
      }));
    },
    [],
  );

  useEffect(() => {
    const unsubs = [
      subscribe("agent.acknowledged", (msg: WSMessage) => {
        const p = msg.payload as { agentId?: string };
        if (p?.agentId) {
          setThinkingAgents((prev) => new Set(prev).add(p.agentId!));
        }
      }),
      subscribe("tool.executed", (msg: WSMessage) => {
        const event = msg.payload as ToolCallEvent;
        const aid = event.agentId || selectedAgent;
        if (aid) addToolMessage(aid, event, "tool.executed");
      }),
      subscribe("tool.denied", (msg: WSMessage) => {
        const event = msg.payload as ToolCallEvent;
        const aid = event.agentId || selectedAgent;
        if (aid) addToolMessage(aid, event, "tool.denied");
      }),
      subscribe("tool.error", (msg: WSMessage) => {
        const event = msg.payload as ToolCallEvent;
        const aid = event.agentId || selectedAgent;
        if (aid) addToolMessage(aid, event, "tool.error");
      }),
      subscribe("chat.response", (msg: WSMessage) => {
        const p = msg.payload as {
          hidden?: boolean;
          taskId?: string;
          task?: { id?: string; result?: string; assignedAgent?: string };
        };
        // Track task ID to prevent duplicates
        const taskId = p?.taskId || p?.task?.id;
        if (taskId) shownTaskIds.current.add(taskId);
        // Only show hidden (greeting) messages here — normal messages come via task.status_changed
        if (p?.hidden && p.task?.result) {
          const aid = p.task.assignedAgent || selectedAgent;
          if (aid) {
            addMessage(aid, "agent", p.task.result);
            setThinkingAgents((prev) => {
              const next = new Set(prev);
              next.delete(aid);
              return next;
            });
          }
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
        if (t.status !== "completed" && t.status !== "failed" && t.status !== "needs_review") return;
        if (t.source !== "user_chat" && t.source !== "background") return;
        // Skip tasks already shown (dedup across chat.response and task.status_changed)
        if (t.id && shownTaskIds.current.has(t.id)) return;
        if (t.id) shownTaskIds.current.add(t.id);
        const aid = t.assignedAgent;
        if (!aid) return;
        const isGreeting =
          t.source === "background" && (t.title || "").includes("Please greet me");
        // Skip greeting responses — handled by chat.response
        if (isGreeting) return;
        setThinkingAgents((prev) => {
          const next = new Set(prev);
          next.delete(aid);
          return next;
        });
        addMessage(aid, "agent", t.result, {
          shortId: t.shortId || "",
          title: t.title || "",
          status: t.status || "completed",
        });
      }),
    ];
    return () => unsubs.forEach((u) => u());
  }, [subscribe, addMessage, addToolMessage, selectedAgent]);

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

  const formatTime = (ts: number) => {
    const d = new Date(ts);
    const now = new Date();
    const time = d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
    if (d.toDateString() === now.toDateString()) return time;
    const yesterday = new Date(now);
    yesterday.setDate(yesterday.getDate() - 1);
    if (d.toDateString() === yesterday.toDateString()) return `Yesterday ${time}`;
    return `${d.toLocaleDateString([], { month: "short", day: "numeric" })} ${time}`;
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
          {messages.map((m, i) => {
            if (m.role === "tool" && m.toolCall && m.toolEventType) {
              return (
                <div key={i} className="chat-tool-row">
                  <ToolCallBlock event={m.toolCall} eventType={m.toolEventType} />
                </div>
              );
            }
            return (
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
                  <div className="chat-msg-meta">
                    <span className="chat-msg-time">{formatTime(m.ts)}</span>
                    {m.task && (
                      <>
                        <span className="task-id">{m.task.shortId}</span>
                        <span className={`task-status-badge ${m.task.status}`}>
                          {m.task.status}
                        </span>
                      </>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
          {selectedAgent && thinkingAgents.has(selectedAgent) && (
            <div className="chat-msg agent">
              <div className="chat-msg-avatar">{getEmoji(selectedAgent)}</div>
              <div className="chat-msg-body">
                <div className="chat-msg-content chat-thinking">
                  <span className="thinking-dot" />
                  <span className="thinking-dot" />
                  <span className="thinking-dot" />
                </div>
              </div>
            </div>
          )}
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
