import { useState } from "react";
import { useFortSocket } from "../contexts/FortSocketContext";
import type { ToolCallEvent } from "../types";

interface Props {
  event: ToolCallEvent;
  eventType: "tool.executed" | "tool.denied" | "tool.error";
}

export default function ToolCallBlock({ event, eventType }: Props) {
  const [expanded, setExpanded] = useState(false);
  const { send } = useFortSocket();

  const isApprovalRequired =
    eventType === "tool.denied" &&
    (event.denialReason?.toLowerCase().includes("approval") ?? false);

  const statusIcon =
    eventType === "tool.executed" ? "✓" :
    eventType === "tool.error" ? "✗" :
    isApprovalRequired ? "⏳" : "✗";

  const statusClass =
    eventType === "tool.executed" ? "success" :
    eventType === "tool.error" ? "error" :
    isApprovalRequired ? "pending" : "denied";

  return (
    <div className={`tool-call-block ${statusClass}`}>
      <button className="tool-call-header" onClick={() => setExpanded(!expanded)}>
        <span className="tool-call-chevron">{expanded ? "▾" : "▸"}</span>
        <span className="tool-call-icon">⚙</span>
        <span className="tool-call-name">{event.toolName}</span>
        <span className={`tool-call-status ${statusClass}`}>{statusIcon}</span>
        {event.durationMs != null && eventType === "tool.executed" && (
          <span className="tool-call-duration">{event.durationMs}ms</span>
        )}
      </button>
      {expanded && (
        <div className="tool-call-body">
          {event.input !== undefined && (
            <div className="tool-call-section">
              <div className="tool-call-label">Input</div>
              <pre className="tool-call-json">
                {JSON.stringify(event.input, null, 2)}
              </pre>
            </div>
          )}
          {event.result?.output && (
            <div className="tool-call-section">
              <div className="tool-call-label">Output</div>
              <pre className="tool-call-json">{event.result.output}</pre>
            </div>
          )}
          {event.result?.error && (
            <div className="tool-call-section">
              <div className="tool-call-label">Error</div>
              <pre className="tool-call-json tool-call-error-text">{event.result.error}</pre>
            </div>
          )}
          {event.denialReason && (
            <div className="tool-call-section">
              <div className="tool-call-label">Reason</div>
              <div className="tool-call-reason">{event.denialReason}</div>
            </div>
          )}
          {isApprovalRequired && (
            <div className="tool-call-actions">
              <button
                className="tool-call-approve-btn"
                onClick={() =>
                  send("tool.approve", { toolCallId: event.id, toolName: event.toolName })
                }
              >
                Approve
              </button>
              <button
                className="tool-call-deny-btn"
                onClick={() =>
                  send("tool.deny", { toolCallId: event.id, toolName: event.toolName })
                }
              >
                Deny
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
