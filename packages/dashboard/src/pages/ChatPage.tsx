import { useEffect, useState, useCallback, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useFortSocket } from "../contexts/FortSocketContext";
import { fetchLLMStatus } from "../utils/api";
import ToolCallBlock from "../components/ToolCallBlock";
import type { AgentInfo, ChatMessage, PlanSubtask, ToolCallEvent, WSMessage, ThreadMessage } from "../types";

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
  const [planningStatus, setPlanningStatus] = useState<Record<string, string>>({});
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

  // Auto-select default agent if none specified. Prefer the primary
  // (isDefault) agent — otherwise the portal can land on Triager (which loads
  // first) and the hatch greeting gets routed to the wrong agent.
  useEffect(() => {
    if (!agentId && agents.length > 0) {
      const isRunning = (a: AgentInfo) => a.status === "running";
      const target = agents.find((a) => isRunning(a) && a.isDefault) ?? agents.find(isRunning);
      if (target) {
        navigate(`/chat/${target.config.id}`, { replace: true });
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
    // Only check `configured` (cheap), not `validateAuth` (which probes the
    // global default provider and can return invalid even when the agent's
    // own provider works fine — gating on it suppresses hatch onboarding).
    fetchLLMStatus()
      .then((status) => {
        if (status?.configured) {
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
      subscribe("agent.classifying", (msg: WSMessage) => {
        const p = msg.payload as { agentId?: string };
        if (p?.agentId) setPlanningStatus((prev) => ({ ...prev, [p.agentId!]: "Reading…" }));
      }),
      subscribe("agent.classified", (msg: WSMessage) => {
        const p = msg.payload as {
          agentId?: string;
          taskId?: string;
          isTask?: boolean;
          confidence?: number;
          summary?: string;
        };
        if (!p?.agentId || !p.taskId) return;

        // Drop the "Reading…" pill once classified. If classifier said it's a
        // task, the decomposer is about to fire and will set its own status.
        // If it's a question, leave thinkingAgents on so the user sees the
        // agent is still composing a reply.
        if (!p.isTask) {
          setPlanningStatus((prev) => { const n = { ...prev }; delete n[p.agentId!]; return n; });
          setThinkingAgents((prev) => new Set(prev).add(p.agentId!));
        }

        // Push an inline classification card with yes/no training buttons.
        // The card sits in the chat stream right before the agent's reply,
        // so users see the classifier's reasoning and can correct it without
        // leaving the conversation.
        setChatMessages((prev) => ({
          ...prev,
          [p.agentId!]: [
            ...(prev[p.agentId!] || []),
            {
              role: "classification" as const,
              text: "",
              ts: Date.now(),
              classification: {
                taskId: p.taskId!,
                classifiedAs: p.isTask ? "task" : "question",
                confidence: typeof p.confidence === "number" ? p.confidence : 0,
                summary: typeof p.summary === "string" ? p.summary : "",
                feedback: "pending",
              },
            },
          ],
        }));
      }),
      subscribe("agent.decomposing", (msg: WSMessage) => {
        const p = msg.payload as { agentId?: string };
        if (p?.agentId) setPlanningStatus((prev) => ({ ...prev, [p.agentId!]: "Breaking this down…" }));
      }),
      subscribe("agent.decomposed", (msg: WSMessage) => {
        const p = msg.payload as {
          agentId?: string;
          taskId?: string;
          subtasks?: Array<{ id: string; shortId: string; title: string }>;
        };
        if (!p?.agentId || !p.taskId || !p.subtasks) return;
        // Append a plan message to the chat for this agent.
        setChatMessages((prev) => ({
          ...prev,
          [p.agentId!]: [
            ...(prev[p.agentId!] || []),
            {
              role: "plan" as const,
              text: "",
              ts: Date.now(),
              plan: {
                parentTaskId: p.taskId!,
                subtasks: p.subtasks!.map((s) => ({ ...s, status: "created" as const })),
              },
            },
          ],
        }));
        // Clear the planning pill — the plan card replaces it visually.
        setPlanningStatus((prev) => { const n = { ...prev }; delete n[p.agentId!]; return n; });
      }),
      subscribe("agent.decomposed_failed", (msg: WSMessage) => {
        const p = msg.payload as { agentId?: string };
        if (p?.agentId) setPlanningStatus((prev) => { const n = { ...prev }; delete n[p.agentId!]; return n; });
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
          parentId?: string | null;
          assignedAgent?: string;
          title?: string;
          shortId?: string;
        };

        // Update any plan-card subtasks that match this task id, regardless of
        // status (so live ⏳/✅/❌ indicators reflect in_progress + completed).
        if (t?.id && t.status) {
          setChatMessages((prev) => {
            const next: typeof prev = {};
            let changed = false;
            for (const [aid, msgs] of Object.entries(prev)) {
              let agentChanged = false;
              const newMsgs = msgs.map((m) => {
                if (m.role !== "plan" || !m.plan) return m;
                const matchIdx = m.plan.subtasks.findIndex((s) => s.id === t.id);
                if (matchIdx < 0) return m;
                const newSubs = m.plan.subtasks.slice();
                newSubs[matchIdx] = { ...newSubs[matchIdx], status: t.status as PlanSubtask["status"] };
                agentChanged = true;
                return { ...m, plan: { ...m.plan, subtasks: newSubs } };
              });
              if (agentChanged) { next[aid] = newMsgs; changed = true; }
              else next[aid] = msgs;
            }
            return changed ? next : prev;
          });
        }

        if (!t?.result) return;
        if (t.status !== "completed" && t.status !== "failed" && t.status !== "needs_review") return;
        if (t.source !== "user_chat" && t.source !== "background") return;
        // Skip subtasks — their results render via the plan card, not as standalone messages.
        if (t.parentId) return;
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
            if (m.role === "plan" && m.plan) {
              return (
                <div key={i} className="chat-plan-card">
                  <div className="chat-plan-header">
                    <span className="chat-plan-icon">📋</span>
                    <span className="chat-plan-title">Plan</span>
                  </div>
                  <ol className="chat-plan-list">
                    {m.plan.subtasks.map((s) => {
                      const icon =
                        s.status === "completed" ? "✅" :
                        s.status === "failed" ? "❌" :
                        s.status === "in_progress" ? "⏳" :
                        s.status === "blocked" || s.status === "needs_review" ? "⚠️" :
                        "▢";
                      return (
                        <li key={s.id} className={`chat-plan-step chat-plan-step--${s.status}`}>
                          <span className="chat-plan-step-icon">{icon}</span>
                          <span className="chat-plan-step-title">{s.title}</span>
                          <span className="chat-plan-step-id">{s.shortId}</span>
                        </li>
                      );
                    })}
                  </ol>
                </div>
              );
            }
            if (m.role === "classification" && m.classification) {
              const c = m.classification;
              const pct = Math.round(c.confidence * 100);
              const handleFeedback = (correctedTo: "task" | "question" | null) => {
                if (correctedTo) {
                  send("triager.reclassify", { taskId: c.taskId, corrected: correctedTo });
                }
                setChatMessages((prev) => {
                  const aid = selectedAgent!;
                  const msgs = prev[aid] || [];
                  const newMsgs = msgs.map((mm, idx) =>
                    idx === i && mm.role === "classification" && mm.classification
                      ? { ...mm, classification: { ...mm.classification, feedback: correctedTo ? "corrected" as const : "confirmed" as const } }
                      : mm,
                  );
                  return { ...prev, [aid]: newMsgs };
                });
              };
              return (
                <div key={i} className="chat-classification-card">
                  <div className="chat-classification-icon">🧭</div>
                  <div className="chat-classification-body">
                    <div className="chat-classification-line">
                      <span className="chat-classification-label">Triager</span>
                      <span className="chat-classification-verdict">
                        classified this as <strong>{c.classifiedAs === "task" ? "a task" : "a question"}</strong>
                      </span>
                      <span className="chat-classification-confidence">{pct}%</span>
                    </div>
                    {c.summary && (
                      <div className="chat-classification-summary">"{c.summary}"</div>
                    )}
                    {c.feedback === "pending" && (
                      <div className="chat-classification-feedback">
                        <span className="chat-classification-prompt">Right call?</span>
                        <button
                          className="chat-classification-btn yes"
                          onClick={() => handleFeedback(null)}
                          title="Confirm — store as a positive example"
                        >👍 Yes</button>
                        <button
                          className="chat-classification-btn no"
                          onClick={() => handleFeedback(c.classifiedAs === "task" ? "question" : "task")}
                          title={`Move to ${c.classifiedAs === "task" ? "Questions" : "Tasks"} board and remember this correction`}
                        >👎 No — should be a {c.classifiedAs === "task" ? "question" : "task"}</button>
                      </div>
                    )}
                    {c.feedback === "confirmed" && (
                      <div className="chat-classification-feedback chat-classification-feedback--done">
                        ✓ Thanks — added as a positive example.
                      </div>
                    )}
                    {c.feedback === "corrected" && (
                      <div className="chat-classification-feedback chat-classification-feedback--done">
                        ✓ Moved to {c.classifiedAs === "task" ? "Questions" : "Tasks"} board. Triager will learn from this.
                      </div>
                    )}
                  </div>
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
          {selectedAgent && planningStatus[selectedAgent] && (
            <div className="chat-msg agent">
              <div className="chat-msg-avatar">{getEmoji(selectedAgent)}</div>
              <div className="chat-msg-body">
                <div className="chat-msg-content chat-planning-pill">
                  {planningStatus[selectedAgent]}
                </div>
              </div>
            </div>
          )}
          {selectedAgent && !planningStatus[selectedAgent] && thinkingAgents.has(selectedAgent) && (
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
