import { useEffect, useState, useCallback } from "react";
import { useFortSocket } from "../contexts/FortSocketContext";
import { timeAgo } from "../utils/time";

// ─── Types ────────────────────────────────────────────────────────────────────

interface CheckDetail {
  status: "healthy" | "degraded" | "unhealthy";
  message?: string;
  details?: Record<string, unknown>;
}

interface HealthResponse {
  status: "healthy" | "degraded" | "unhealthy";
  timestamp: string;
  uptime: number;
  checks: Record<string, CheckDetail>;
}

interface MetricsResponse {
  uptime: number;
  heapMb: number;
  dbSizeMb: number;
  tasksToday: number;
  activeTasks: number;
  errorsToday: number;
}

interface ErrorEntry {
  id: string;
  subsystem: string;
  message: string;
  stack: string | null;
  context: Record<string, unknown> | null;
  createdAt: string;
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function statusColor(status: "healthy" | "degraded" | "unhealthy"): string {
  switch (status) {
    case "healthy": return "#22c55e";
    case "degraded": return "#f59e0b";
    case "unhealthy": return "#ef4444";
  }
}

function statusBg(status: "healthy" | "degraded" | "unhealthy"): string {
  switch (status) {
    case "healthy": return "rgba(34,197,94,0.12)";
    case "degraded": return "rgba(245,158,11,0.12)";
    case "unhealthy": return "rgba(239,68,68,0.12)";
  }
}

function formatUptime(secs: number): string {
  if (secs < 60) return `${secs}s`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`;
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  return `${h}h ${m}m`;
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function StatusPill({ status }: { status: "healthy" | "degraded" | "unhealthy" }) {
  return (
    <span style={{
      display: "inline-block",
      padding: "4px 14px",
      borderRadius: 20,
      background: statusBg(status),
      color: statusColor(status),
      border: `1px solid ${statusColor(status)}40`,
      fontWeight: 700,
      fontSize: 13,
      letterSpacing: 1,
      textTransform: "uppercase",
    }}>
      {status}
    </span>
  );
}

function CheckCard({ name, detail }: { name: string; detail: CheckDetail }) {
  const icon = detail.status === "healthy" ? "✓" : detail.status === "degraded" ? "⚠" : "✗";
  return (
    <div style={{
      background: "#1a1a24",
      border: `1px solid ${statusColor(detail.status)}30`,
      borderRadius: 8,
      padding: "0.85rem 1rem",
      minWidth: 180,
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 4 }}>
        <span style={{ color: statusColor(detail.status), fontWeight: 700 }}>{icon}</span>
        <span style={{ color: "#ccc", fontSize: 12, fontWeight: 600, textTransform: "capitalize" }}>
          {name.replace(/_/g, " ")}
        </span>
      </div>
      <div style={{ color: "#888", fontSize: 11, lineHeight: 1.4 }}>
        {detail.message || detail.status}
      </div>
    </div>
  );
}

function MetricBox({ label, value }: { label: string; value: string | number }) {
  return (
    <div style={{
      background: "#1a1a24",
      border: "1px solid #2a2a36",
      borderRadius: 8,
      padding: "0.75rem 1.1rem",
      minWidth: 130,
      textAlign: "center",
    }}>
      <div style={{ color: "#eee", fontWeight: 700, fontSize: 20 }}>{value}</div>
      <div style={{ color: "#666", fontSize: 11, marginTop: 3 }}>{label}</div>
    </div>
  );
}

function ErrorRow({ entry, expanded, onToggle }: {
  entry: ErrorEntry;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <div
      style={{
        borderBottom: "1px solid #1e1e2a",
        padding: "0.55rem 0.75rem",
        cursor: entry.stack ? "pointer" : "default",
      }}
      onClick={onToggle}
    >
      <div style={{ display: "flex", gap: "0.75rem", alignItems: "flex-start" }}>
        <span style={{
          background: "#2a2030",
          color: "#c084fc",
          borderRadius: 4,
          padding: "1px 6px",
          fontSize: 11,
          fontWeight: 600,
          whiteSpace: "nowrap",
          flexShrink: 0,
        }}>
          {entry.subsystem}
        </span>
        <span style={{ color: "#ddd", fontSize: 12, flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: expanded ? "normal" : "nowrap" }}>
          {entry.message}
        </span>
        <span style={{ color: "#555", fontSize: 11, whiteSpace: "nowrap", flexShrink: 0 }}>
          {timeAgo(new Date(entry.createdAt))}
        </span>
      </div>
      {expanded && entry.stack && (
        <pre style={{
          marginTop: "0.5rem",
          background: "#111118",
          borderRadius: 4,
          padding: "0.5rem",
          fontSize: 10,
          color: "#f87171",
          overflowX: "auto",
          whiteSpace: "pre-wrap",
          wordBreak: "break-all",
        }}>
          {entry.stack}
        </pre>
      )}
    </div>
  );
}

// ─── Main page ────────────────────────────────────────────────────────────────

export default function DiagnosticsPage() {
  const { send, subscribe } = useFortSocket();

  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [metrics, setMetrics] = useState<MetricsResponse | null>(null);
  const [errors, setErrors] = useState<ErrorEntry[]>([]);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null);

  const refresh = useCallback(() => {
    send("diagnostics.health");
    send("diagnostics.metrics");
    send("diagnostics.errors", { limit: 50 });
    setLastRefresh(new Date());
  }, [send]);

  useEffect(() => {
    const unsubHealth = subscribe("diagnostics.health.response", (msg) => {
      setHealth(msg.payload as HealthResponse);
    });
    const unsubMetrics = subscribe("diagnostics.metrics.response", (msg) => {
      setMetrics(msg.payload as MetricsResponse);
    });
    const unsubErrors = subscribe("diagnostics.errors.response", (msg) => {
      setErrors((msg.payload as ErrorEntry[]) ?? []);
    });

    refresh();
    const interval = setInterval(refresh, 30_000);

    return () => {
      unsubHealth();
      unsubMetrics();
      unsubErrors();
      clearInterval(interval);
    };
  }, [refresh, subscribe]);

  const toggleExpanded = (id: string) => {
    setExpanded((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  const overallStatus = health?.status ?? "healthy";

  return (
    <div style={{ padding: "1.5rem 2rem", maxWidth: 1000, margin: "0 auto" }}>

      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "1.5rem" }}>
        <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
          <h1 style={{ margin: 0, fontSize: 20, color: "#eee" }}>Diagnostics</h1>
          <StatusPill status={overallStatus} />
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
          {lastRefresh && (
            <span style={{ color: "#555", fontSize: 11 }}>
              refreshed {timeAgo(lastRefresh)}
            </span>
          )}
          <button
            onClick={refresh}
            style={{
              padding: "5px 14px",
              background: "#2a2a36",
              border: "1px solid #333",
              borderRadius: 6,
              color: "#ccc",
              fontSize: 12,
              cursor: "pointer",
            }}
          >
            Refresh
          </button>
        </div>
      </div>

      {/* Metrics row */}
      {metrics && (
        <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap", marginBottom: "1.5rem" }}>
          <MetricBox label="Uptime" value={formatUptime(metrics.uptime)} />
          <MetricBox label="Heap" value={`${metrics.heapMb} MB`} />
          <MetricBox label="DB Size" value={`${metrics.dbSizeMb} MB`} />
          <MetricBox label="Tasks Today" value={metrics.tasksToday} />
          <MetricBox label="Active Tasks" value={metrics.activeTasks} />
          <MetricBox label="Errors Today" value={metrics.errorsToday} />
        </div>
      )}

      {/* Health check cards */}
      {health && Object.keys(health.checks).length > 0 && (
        <div style={{ marginBottom: "1.5rem" }}>
          <div style={{ color: "#666", fontSize: 11, fontWeight: 600, textTransform: "uppercase", letterSpacing: 1, marginBottom: "0.6rem" }}>
            Health Checks
          </div>
          <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap" }}>
            {Object.entries(health.checks).map(([name, detail]) => (
              <CheckCard key={name} name={name} detail={detail} />
            ))}
          </div>
        </div>
      )}

      {/* Error log table */}
      <div>
        <div style={{ color: "#666", fontSize: 11, fontWeight: 600, textTransform: "uppercase", letterSpacing: 1, marginBottom: "0.6rem" }}>
          Error Log ({errors.length})
        </div>
        <div style={{ background: "#15151e", border: "1px solid #1e1e2a", borderRadius: 8, overflow: "hidden" }}>
          {errors.length === 0 ? (
            <div style={{ padding: "1.5rem", textAlign: "center", color: "#444", fontSize: 13 }}>
              No errors recorded
            </div>
          ) : (
            errors.map((entry) => (
              <ErrorRow
                key={entry.id}
                entry={entry}
                expanded={!!expanded[entry.id]}
                onToggle={() => toggleExpanded(entry.id)}
              />
            ))
          )}
        </div>
      </div>
    </div>
  );
}
