import { useEffect, useState, useCallback } from "react";
import { useFortSocket } from "../contexts/FortSocketContext";
import { timeAgo } from "../utils/time";

interface UsageSummary {
  period: string;
  totalInputTokens: number;
  totalOutputTokens: number;
  totalCacheReadTokens: number;
  totalCacheWriteTokens: number;
  totalTokens: number;
  estimatedCostUsd: number;
  eventCount: number;
}

interface AgentUsage {
  agentId: string;
  totalTokens: number;
  totalInputTokens: number;
  totalOutputTokens: number;
  estimatedCostUsd: number;
  eventCount: number;
}

interface UsageTotals {
  totalInputTokens: number;
  totalOutputTokens: number;
  totalCacheReadTokens: number;
  totalCacheWriteTokens: number;
  totalTokens: number;
  estimatedCostUsd: number;
  eventCount: number;
  uniqueAgents: number;
  uniqueModels: number;
}

interface UsageRecord {
  id: string;
  taskId: string;
  agentId: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  totalTokens: number;
  estimatedCostUsd: number | null;
  createdAt: string;
}

function fmtTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(2)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
  return String(n);
}

function fmtCost(usd: number): string {
  if (usd < 0.001) return "<$0.001";
  return `$${usd.toFixed(4)}`;
}

export default function UsagePage() {
  const { send, subscribe } = useFortSocket();
  const [period, setPeriod] = useState<"day" | "week" | "month">("week");
  const [summary, setSummary] = useState<UsageSummary | null>(null);
  const [byAgent, setByAgent] = useState<AgentUsage[]>([]);
  const [totals, setTotals] = useState<UsageTotals | null>(null);
  const [recent] = useState<UsageRecord[]>([]);

  const fetchAll = useCallback(() => {
    send("usage.summary", { period });
    send("usage.by_agent", { period });
    send("usage.totals", {});
    // Recent events: query with no filters, limit 20
    send("usage.summary", { period: "day" });
  }, [send, period]);

  useEffect(() => {
    fetchAll();
  }, [fetchAll]);

  useEffect(() => {
    const unsub1 = subscribe("usage.summary.response", (msg) => {
      setSummary(msg.payload as UsageSummary);
    });
    const unsub2 = subscribe("usage.by_agent.response", (msg) => {
      setByAgent((msg.payload as AgentUsage[]) ?? []);
    });
    const unsub3 = subscribe("usage.totals.response", (msg) => {
      setTotals(msg.payload as UsageTotals);
    });
    return () => { unsub1(); unsub2(); unsub3(); };
  }, [subscribe]);

  return (
    <div style={{ padding: "1.5rem", maxWidth: 1100, margin: "0 auto" }}>
      <div style={{ display: "flex", alignItems: "center", gap: "1rem", marginBottom: "1.5rem" }}>
        <h2 style={{ margin: 0 }}>Usage & Cost</h2>
        <div style={{ marginLeft: "auto", display: "flex", gap: "0.5rem" }}>
          {(["day", "week", "month"] as const).map((p) => (
            <button
              key={p}
              onClick={() => { setPeriod(p); }}
              style={{
                padding: "0.3rem 0.8rem",
                borderRadius: 6,
                border: "1px solid #444",
                background: period === p ? "#6c5ce7" : "transparent",
                color: period === p ? "#fff" : "#aaa",
                cursor: "pointer",
                fontWeight: period === p ? 600 : 400,
              }}
            >
              {p.charAt(0).toUpperCase() + p.slice(1)}
            </button>
          ))}
        </div>
      </div>

      {/* Summary cards */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: "1rem", marginBottom: "2rem" }}>
        <SummaryCard
          label={`Tokens (${period})`}
          value={summary ? fmtTokens(summary.totalTokens) : "—"}
          sub={summary ? `${fmtTokens(summary.totalInputTokens)} in / ${fmtTokens(summary.totalOutputTokens)} out` : ""}
        />
        <SummaryCard
          label={`Est. cost (${period})`}
          value={summary ? fmtCost(summary.estimatedCostUsd) : "—"}
          sub={summary ? `${summary.eventCount} LLM calls` : ""}
        />
        <SummaryCard
          label="All-time tokens"
          value={totals ? fmtTokens(totals.totalTokens) : "—"}
          sub={totals ? `${totals.uniqueAgents} agents, ${totals.uniqueModels} models` : ""}
        />
        <SummaryCard
          label="All-time cost"
          value={totals ? fmtCost(totals.estimatedCostUsd) : "—"}
          sub={totals ? `${totals.eventCount} total calls` : ""}
        />
      </div>

      {/* Agent leaderboard */}
      <section style={{ marginBottom: "2rem" }}>
        <h3 style={{ marginBottom: "0.75rem", fontSize: "1rem", color: "#ccc" }}>
          Top agents by usage — last {period}
        </h3>
        {byAgent.length === 0 ? (
          <p style={{ color: "#666", fontSize: "0.9rem" }}>No usage data for this period.</p>
        ) : (
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "0.9rem" }}>
            <thead>
              <tr style={{ color: "#888", textAlign: "left" }}>
                <th style={thStyle}>Agent</th>
                <th style={thStyle}>Tokens</th>
                <th style={thStyle}>Input</th>
                <th style={thStyle}>Output</th>
                <th style={thStyle}>Calls</th>
                <th style={thStyle}>Est. cost</th>
              </tr>
            </thead>
            <tbody>
              {byAgent.map((a) => (
                <tr key={a.agentId} style={{ borderBottom: "1px solid #2a2a3a" }}>
                  <td style={tdStyle}>{a.agentId}</td>
                  <td style={tdStyle}>{fmtTokens(a.totalTokens)}</td>
                  <td style={tdStyle}>{fmtTokens(a.totalInputTokens)}</td>
                  <td style={tdStyle}>{fmtTokens(a.totalOutputTokens)}</td>
                  <td style={tdStyle}>{a.eventCount}</td>
                  <td style={tdStyle}>{fmtCost(a.estimatedCostUsd)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      {/* Recent usage events table — fetched via queryUsage (no direct WS handler yet, use totals fallback) */}
      {recent.length > 0 && (
        <section>
          <h3 style={{ marginBottom: "0.75rem", fontSize: "1rem", color: "#ccc" }}>Recent events</h3>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "0.875rem" }}>
            <thead>
              <tr style={{ color: "#888", textAlign: "left" }}>
                <th style={thStyle}>Agent</th>
                <th style={thStyle}>Model</th>
                <th style={thStyle}>Tokens</th>
                <th style={thStyle}>Cost</th>
                <th style={thStyle}>When</th>
              </tr>
            </thead>
            <tbody>
              {recent.map((r) => (
                <tr key={r.id} style={{ borderBottom: "1px solid #2a2a3a" }}>
                  <td style={tdStyle}>{r.agentId}</td>
                  <td style={tdStyle}>{r.model}</td>
                  <td style={tdStyle}>{fmtTokens(r.totalTokens)}</td>
                  <td style={tdStyle}>{r.estimatedCostUsd != null ? fmtCost(r.estimatedCostUsd) : "—"}</td>
                  <td style={tdStyle}>{timeAgo(r.createdAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
    </div>
  );
}

function SummaryCard({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div style={{
      background: "#1a1a2e",
      border: "1px solid #2a2a4a",
      borderRadius: 10,
      padding: "1rem 1.25rem",
    }}>
      <div style={{ fontSize: "0.8rem", color: "#888", marginBottom: "0.4rem" }}>{label}</div>
      <div style={{ fontSize: "1.6rem", fontWeight: 700, color: "#e0e0ff", lineHeight: 1.1 }}>{value}</div>
      {sub && <div style={{ fontSize: "0.8rem", color: "#666", marginTop: "0.3rem" }}>{sub}</div>}
    </div>
  );
}

const thStyle: React.CSSProperties = {
  padding: "0.5rem 0.75rem",
  fontWeight: 500,
  borderBottom: "1px solid #2a2a3a",
};

const tdStyle: React.CSSProperties = {
  padding: "0.5rem 0.75rem",
  color: "#ccc",
};
