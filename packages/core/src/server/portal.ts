/**
 * Fort Web Portal
 *
 * Self-contained HTML page served by FortServer.
 * Dark theme, WebSocket-driven, no external dependencies.
 *
 * Features:
 * - Setup wizard (4-step) shown when no default agent exists
 * - Dashboard with Kanban board
 * - Agents tab with detail modal (soul content)
 * - Chat tab with agent sidebar
 */

export function getPortalHTML(): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Fort Portal</title>
<style>
  :root {
    --bg-primary: #0e0e12;
    --bg-secondary: #16161d;
    --bg-tertiary: #1e1e28;
    --border: #2a2a3a;
    --text-primary: #e0e0e8;
    --text-secondary: #8888a0;
    --accent: #6c5ce7;
    --success: #2ed573;
    --warning: #ffa502;
    --danger: #ff4757;
    --font: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    --mono: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  }

  * { margin: 0; padding: 0; box-sizing: border-box; }

  body {
    font-family: var(--font);
    background: var(--bg-primary);
    color: var(--text-primary);
    height: 100vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  /* ─── Top Bar ─── */
  .topbar {
    display: flex;
    align-items: center;
    padding: 0 20px;
    height: 52px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .logo {
    font-size: 18px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 3px;
    margin-right: 32px;
    font-family: var(--mono);
  }

  .nav-tabs {
    display: flex;
    gap: 4px;
  }

  .nav-tab {
    padding: 8px 16px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary);
    border: none;
    background: transparent;
    transition: all 0.15s;
  }

  .nav-tab:hover { color: var(--text-primary); background: var(--bg-tertiary); }
  .nav-tab.active { color: var(--accent); background: var(--bg-tertiary); }

  .topbar-right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .ws-status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-secondary);
  }

  .ws-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--danger);
  }

  .ws-dot.connected { background: var(--success); }

  /* ─── Content Area ─── */
  .content {
    flex: 1;
    overflow-y: auto;
    padding: 24px;
  }

  .tab-panel { display: none; height: 100%; }
  .tab-panel.active { display: flex; flex-direction: column; }

  /* ─── Dashboard Tab ─── */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 16px;
    margin-bottom: 24px;
  }

  .stat-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 20px;
  }

  .stat-label {
    font-size: 12px;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 1px;
    margin-bottom: 8px;
  }

  .stat-value {
    font-size: 28px;
    font-weight: 700;
    font-family: var(--mono);
  }

  .stat-value.accent { color: var(--accent); }
  .stat-value.success { color: var(--success); }
  .stat-value.warning { color: var(--warning); }

  .section-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 1px;
    margin-bottom: 12px;
  }

  /* ─── Kanban Board ─── */
  .kanban {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 16px;
    flex: 1;
    min-height: 0;
  }

  .kanban-column {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    min-height: 200px;
    max-height: calc(100vh - 280px);
  }

  .kanban-col-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 16px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .kanban-col-title {
    font-size: 13px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-secondary);
  }

  .kanban-col-count {
    font-size: 12px;
    font-family: var(--mono);
    color: var(--text-secondary);
    background: var(--bg-tertiary);
    padding: 2px 8px;
    border-radius: 10px;
  }

  .kanban-col-body {
    flex: 1;
    overflow-y: auto;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .kanban-card {
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px;
    border-left: 3px solid var(--border);
    transition: border-color 0.15s;
  }

  .kanban-card.agent-owned { border-left-color: var(--accent); }
  .kanban-card.user-owned { border-left-color: #555; }

  .kanban-card-title {
    font-size: 13px;
    font-weight: 500;
    margin-bottom: 8px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kanban-card-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .kanban-card-assignee {
    font-size: 11px;
    padding: 2px 8px;
    border-radius: 8px;
    font-weight: 500;
  }

  .kanban-card-assignee.agent {
    background: rgba(108,92,231,0.15);
    color: var(--accent);
  }

  .kanban-card-assignee.user {
    background: rgba(136,136,160,0.15);
    color: var(--text-secondary);
  }

  .kanban-card-time {
    font-size: 11px;
    color: var(--text-secondary);
    font-family: var(--mono);
  }

  .kanban-add-btn {
    width: 100%;
    padding: 8px;
    background: transparent;
    border: 1px dashed var(--border);
    border-radius: 8px;
    color: var(--text-secondary);
    font-size: 12px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .kanban-add-btn:hover {
    border-color: var(--accent);
    color: var(--accent);
  }

  .kanban-done-toggle {
    background: none;
    border: none;
    color: var(--text-secondary);
    font-size: 11px;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 4px;
  }

  .kanban-done-toggle:hover { background: var(--bg-tertiary); }

  .kanban-empty {
    text-align: center;
    padding: 24px 12px;
    color: var(--text-secondary);
    font-size: 12px;
  }

  /* ─── Add Task Inline Form ─── */
  .add-task-form {
    display: none;
    flex-direction: column;
    gap: 8px;
    padding: 12px;
    background: var(--bg-tertiary);
    border: 1px solid var(--accent);
    border-radius: 8px;
  }

  .add-task-form.active { display: flex; }

  .add-task-form input {
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 8px 12px;
    color: var(--text-primary);
    font-size: 13px;
    font-family: var(--font);
    outline: none;
  }

  .add-task-form input:focus { border-color: var(--accent); }

  .add-task-form-actions {
    display: flex;
    gap: 8px;
  }

  .add-task-form-actions button {
    padding: 6px 14px;
    border-radius: 6px;
    border: none;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
  }

  .add-task-submit {
    background: var(--accent);
    color: #fff;
  }

  .add-task-cancel {
    background: var(--bg-primary);
    color: var(--text-secondary);
    border: 1px solid var(--border) !important;
  }

  /* ─── Agents Tab ─── */
  .agents-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 16px;
  }

  .agent-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 20px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .agent-card:hover {
    border-color: var(--accent);
    transform: translateY(-1px);
  }

  .agent-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;
  }

  .agent-emoji { font-size: 28px; }

  .agent-name {
    font-size: 16px;
    font-weight: 600;
  }

  .agent-type {
    font-size: 11px;
    color: var(--text-secondary);
    background: var(--bg-tertiary);
    padding: 2px 8px;
    border-radius: 4px;
    margin-left: auto;
  }

  .agent-desc {
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.5;
  }

  .agent-status-badge {
    display: inline-block;
    margin-top: 12px;
    font-size: 11px;
    padding: 3px 10px;
    border-radius: 10px;
    font-weight: 500;
  }

  .agent-status-badge.idle { background: rgba(108,92,231,0.15); color: var(--accent); }
  .agent-status-badge.running { background: rgba(46,213,115,0.15); color: var(--success); }
  .agent-status-badge.error { background: rgba(255,71,87,0.15); color: var(--danger); }

  .agent-default-tag {
    font-size: 10px;
    padding: 2px 6px;
    border-radius: 4px;
    background: rgba(108,92,231,0.2);
    color: var(--accent);
    margin-left: 8px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  /* Agent Detail Modal */
  .modal-overlay {
    display: none;
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.6);
    z-index: 100;
    align-items: center;
    justify-content: center;
  }

  .modal-overlay.active { display: flex; }

  .modal {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 12px;
    width: 90%;
    max-width: 600px;
    max-height: 80vh;
    display: flex;
    flex-direction: column;
  }

  .modal-header {
    display: flex;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
    gap: 12px;
  }

  .modal-close {
    margin-left: auto;
    background: none;
    border: none;
    color: var(--text-secondary);
    font-size: 20px;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 4px;
  }

  .modal-close:hover { background: var(--bg-tertiary); color: var(--text-primary); }

  .modal-body {
    padding: 20px;
    overflow-y: auto;
    font-size: 14px;
    line-height: 1.6;
    white-space: pre-wrap;
    font-family: var(--mono);
    color: var(--text-secondary);
  }

  /* ─── Chat Tab ─── */
  .chat-layout {
    flex: 1;
    display: flex;
    min-height: 0;
    gap: 0;
  }

  .chat-sidebar {
    width: 200px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px 0 0 10px;
    overflow-y: auto;
    flex-shrink: 0;
  }

  .chat-sidebar-title {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-secondary);
    padding: 14px 16px 8px;
  }

  .chat-agent-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 16px;
    cursor: pointer;
    font-size: 13px;
    color: var(--text-secondary);
    transition: all 0.15s;
    border-left: 2px solid transparent;
  }

  .chat-agent-item:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  .chat-agent-item.active {
    background: var(--bg-tertiary);
    color: var(--text-primary);
    border-left-color: var(--accent);
  }

  .chat-agent-item-emoji { font-size: 18px; }
  .chat-agent-item-name { font-weight: 500; }

  .chat-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-left: none;
    border-radius: 0 10px 10px 0;
  }

  .chat-header {
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
    font-size: 14px;
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }

  .chat-messages {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
  }

  .chat-msg {
    margin-bottom: 16px;
    display: flex;
    gap: 12px;
  }

  .chat-msg-avatar {
    width: 32px;
    height: 32px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    flex-shrink: 0;
  }

  .chat-msg.user .chat-msg-avatar { background: var(--accent); color: #fff; }
  .chat-msg.agent .chat-msg-avatar { background: var(--bg-tertiary); font-size: 18px; }

  .chat-msg-content {
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 12px 16px;
    font-size: 14px;
    line-height: 1.5;
    max-width: 80%;
  }

  .chat-msg.user .chat-msg-content {
    background: var(--accent);
    color: #fff;
    border-color: var(--accent);
  }

  .chat-msg-body { max-width: 80%; }

  .chat-task-card {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 8px;
    padding: 8px 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 12px;
  }

  .chat-task-id {
    font-family: var(--mono);
    color: var(--accent);
    font-weight: 600;
    white-space: nowrap;
  }

  .chat-task-title {
    color: var(--text-secondary);
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chat-task-status {
    font-weight: 600;
    white-space: nowrap;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
  }

  .chat-task-status.completed { background: rgba(46,213,115,0.15); color: var(--success); }
  .chat-task-status.in_progress { background: rgba(108,92,231,0.15); color: var(--accent); }
  .chat-task-status.created { background: rgba(136,136,160,0.15); color: var(--text-secondary); }
  .chat-task-status.failed { background: rgba(255,71,87,0.15); color: var(--danger); }

  .chat-input-bar {
    display: flex;
    gap: 10px;
    padding: 12px 16px;
    border-top: 1px solid var(--border);
    flex-shrink: 0;
  }

  .chat-input {
    flex: 1;
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px 16px;
    color: var(--text-primary);
    font-size: 14px;
    font-family: var(--font);
    outline: none;
    resize: none;
  }

  .chat-input:focus { border-color: var(--accent); }

  .chat-send {
    padding: 12px 20px;
    background: var(--accent);
    color: #fff;
    border: none;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: opacity 0.15s;
    flex-shrink: 0;
  }

  .chat-send:hover { opacity: 0.85; }
  .chat-send:disabled { opacity: 0.4; cursor: not-allowed; }

  .chat-empty {
    text-align: center;
    padding: 48px 24px;
    color: var(--text-secondary);
    font-size: 14px;
  }

  /* ─── Setup Wizard ─── */
  .wizard-overlay {
    display: none;
    position: fixed;
    inset: 0;
    background: var(--bg-primary);
    z-index: 200;
    align-items: center;
    justify-content: center;
    flex-direction: column;
  }

  .wizard-overlay.active { display: flex; }

  .wizard-progress {
    display: flex;
    gap: 8px;
    margin-bottom: 40px;
  }

  .wizard-progress-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--border);
    transition: background 0.2s;
  }

  .wizard-progress-dot.active { background: var(--accent); }
  .wizard-progress-dot.done { background: var(--success); }

  .wizard-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 48px;
    width: 90%;
    max-width: 520px;
    text-align: center;
  }

  .wizard-step { display: none; }
  .wizard-step.active { display: block; }

  .wizard-castle {
    font-size: 72px;
    margin-bottom: 16px;
    display: block;
  }

  .wizard-title {
    font-size: 24px;
    font-weight: 700;
    margin-bottom: 12px;
  }

  .wizard-subtitle {
    font-size: 14px;
    color: var(--text-secondary);
    margin-bottom: 32px;
    line-height: 1.6;
  }

  .wizard-btn {
    padding: 12px 32px;
    background: var(--accent);
    color: #fff;
    border: none;
    border-radius: 8px;
    font-size: 15px;
    font-weight: 600;
    cursor: pointer;
    transition: opacity 0.15s;
  }

  .wizard-btn:hover { opacity: 0.85; }
  .wizard-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .wizard-btn-secondary {
    padding: 10px 24px;
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    margin-right: 12px;
  }

  .wizard-btn-secondary:hover { border-color: var(--text-secondary); }

  .wizard-field {
    text-align: left;
    margin-bottom: 20px;
  }

  .wizard-field label {
    display: block;
    font-size: 13px;
    font-weight: 600;
    margin-bottom: 8px;
    color: var(--text-secondary);
  }

  .wizard-field input,
  .wizard-field textarea {
    width: 100%;
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px 16px;
    color: var(--text-primary);
    font-size: 14px;
    font-family: var(--font);
    outline: none;
  }

  .wizard-field input:focus,
  .wizard-field textarea:focus { border-color: var(--accent); }

  .wizard-field textarea {
    resize: vertical;
    min-height: 80px;
  }

  .wizard-actions {
    display: flex;
    justify-content: center;
    gap: 12px;
    margin-top: 24px;
  }

  /* Emoji grid */
  .emoji-grid {
    display: grid;
    grid-template-columns: repeat(8, 1fr);
    gap: 8px;
    margin-bottom: 24px;
  }

  .emoji-option {
    width: 48px;
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 24px;
    border-radius: 10px;
    border: 2px solid var(--border);
    background: var(--bg-tertiary);
    cursor: pointer;
    transition: all 0.15s;
  }

  .emoji-option:hover { border-color: var(--accent); transform: scale(1.1); }
  .emoji-option.selected { border-color: var(--accent); background: rgba(108,92,231,0.2); }

  /* Avatar upload */
  .avatar-section {
    margin-top: 24px;
    padding-top: 20px;
    border-top: 1px solid var(--border);
  }

  .avatar-section-title {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 12px;
    color: var(--text-secondary);
  }

  .avatar-row {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .avatar-preview {
    width: 80px;
    height: 80px;
    border-radius: 50%;
    border: 2px solid var(--border);
    background: var(--bg-tertiary);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 36px;
    overflow: hidden;
    flex-shrink: 0;
  }

  .avatar-preview img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .avatar-controls {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .avatar-upload-btn {
    padding: 8px 16px;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--bg-tertiary);
    color: var(--text-primary);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.15s;
  }

  .avatar-upload-btn:hover { border-color: var(--accent); }

  .avatar-hint {
    font-size: 12px;
    color: var(--text-secondary);
  }

  .avatar-use-default {
    font-size: 12px;
    color: var(--accent);
    cursor: pointer;
    background: none;
    border: none;
    text-decoration: underline;
    padding: 0;
  }

  .avatar-use-default:hover { opacity: 0.8; }

  /* Wizard summary card */
  .wizard-summary {
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 24px;
    margin-bottom: 24px;
    text-align: center;
  }

  .wizard-summary-emoji { font-size: 48px; display: block; margin-bottom: 12px; }
  .wizard-summary-name { font-size: 20px; font-weight: 700; margin-bottom: 8px; }
  .wizard-summary-goals {
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.5;
    white-space: pre-wrap;
    max-height: 120px;
    overflow-y: auto;
  }

  .empty-state {
    text-align: center;
    padding: 48px 24px;
    color: var(--text-secondary);
    font-size: 14px;
  }

  /* ─── Scrollbar ─── */
  ::-webkit-scrollbar { width: 6px; }
  ::-webkit-scrollbar-track { background: transparent; }
  ::-webkit-scrollbar-thumb { background: var(--border); border-radius: 3px; }
  ::-webkit-scrollbar-thumb:hover { background: var(--text-secondary); }
</style>
</head>
<body>

<!-- Setup Wizard Overlay -->
<div class="wizard-overlay" id="wizard-overlay">
  <div class="wizard-progress" id="wizard-progress">
    <div class="wizard-progress-dot active"></div>
    <div class="wizard-progress-dot"></div>
    <div class="wizard-progress-dot"></div>
    <div class="wizard-progress-dot"></div>
  </div>
  <div class="wizard-card">

    <!-- Step 1: Welcome -->
    <div class="wizard-step active" id="wizard-step-0">
      <span class="wizard-castle">🏰</span>
      <div class="wizard-title">Welcome to Fort</div>
      <div class="wizard-subtitle">Let's set up your first agent. This will be your primary assistant that handles tasks and conversations.</div>
      <div class="wizard-actions">
        <button class="wizard-btn" onclick="wizardNext()">Get Started</button>
      </div>
    </div>

    <!-- Step 2: Name & Goals -->
    <div class="wizard-step" id="wizard-step-1">
      <div class="wizard-title">Name & Goals</div>
      <div class="wizard-subtitle">Give your agent a name and tell it what to help you with.</div>
      <div class="wizard-field">
        <label>Agent Name</label>
        <input type="text" id="wizard-name" value="Fort" placeholder="Fort" autocomplete="off" />
      </div>
      <div class="wizard-field">
        <label>What should this agent help you with?</label>
        <textarea id="wizard-goals" placeholder="e.g. Help me manage tasks, answer questions, and automate workflows"></textarea>
      </div>
      <div class="wizard-field">
        <label>Describe this agent's personality</label>
        <textarea id="wizard-personality" placeholder="e.g. Professional and direct. Gets straight to the point without unnecessary filler. Addresses me as 'sir' and always suggests next steps."></textarea>
      </div>
      <div class="wizard-actions">
        <button class="wizard-btn-secondary" onclick="wizardBack()">Back</button>
        <button class="wizard-btn" onclick="wizardNext()">Next</button>
      </div>
    </div>

    <!-- Step 3: Emoji & Avatar -->
    <div class="wizard-step" id="wizard-step-2">
      <div class="wizard-title">Choose an Emoji</div>
      <div class="wizard-subtitle">Pick an icon for your agent.</div>
      <div class="emoji-grid" id="emoji-grid"></div>

      <div class="avatar-section">
        <div class="avatar-section-title">Agent Avatar</div>
        <div class="avatar-row">
          <div class="avatar-preview" id="avatar-preview">
            <img id="avatar-preview-img" src="/api/default-avatar" alt="Avatar" />
          </div>
          <div class="avatar-controls">
            <input type="file" id="avatar-file-input" accept="image/png,image/jpeg,image/webp" style="display:none" onchange="handleAvatarUpload(this)" />
            <button class="avatar-upload-btn" onclick="document.getElementById('avatar-file-input').click()">Upload Image</button>
            <button class="avatar-use-default" onclick="useDefaultAvatar()">Use default avatar</button>
            <span class="avatar-hint">PNG, JPG or WebP. Will be cropped to a circle.</span>
          </div>
        </div>
      </div>

      <div class="wizard-actions">
        <button class="wizard-btn-secondary" onclick="wizardBack()">Back</button>
        <button class="wizard-btn" onclick="wizardNext()">Next</button>
      </div>
    </div>

    <!-- Step 4: Done -->
    <div class="wizard-step" id="wizard-step-3">
      <div class="wizard-title">Ready to Launch</div>
      <div class="wizard-subtitle">Here's a summary of your new agent.</div>
      <div class="wizard-summary">
        <div class="avatar-preview" id="wizard-summary-avatar" style="width:64px;height:64px;margin:0 auto 12px;font-size:28px;">
          <img id="wizard-summary-avatar-img" src="/api/default-avatar" alt="Avatar" />
        </div>
        <span class="wizard-summary-emoji" id="wizard-summary-emoji"></span>
        <div class="wizard-summary-name" id="wizard-summary-name"></div>
        <div class="wizard-summary-goals" id="wizard-summary-goals"></div>
      </div>
      <div class="wizard-actions">
        <button class="wizard-btn-secondary" onclick="wizardBack()">Back</button>
        <button class="wizard-btn" id="wizard-launch-btn" onclick="wizardSubmit()">Launch Fort</button>
      </div>
    </div>

  </div>
</div>

<!-- Top Bar -->
<div class="topbar" id="topbar">
  <div class="logo">FORT</div>
  <div class="nav-tabs">
    <button class="nav-tab" data-tab="dashboard">Dashboard</button>
    <button class="nav-tab" data-tab="agents">Agents</button>
    <button class="nav-tab active" data-tab="chat">Chat</button>
  </div>
  <div class="topbar-right">
    <div class="ws-status">
      <div class="ws-dot" id="wsDot"></div>
      <span id="wsLabel">Connecting...</span>
    </div>
  </div>
</div>

<!-- Content -->
<div class="content" id="main-content">

  <!-- Dashboard Tab -->
  <div class="tab-panel" id="tab-dashboard">
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-label">Agents</div>
        <div class="stat-value accent" id="stat-agents">-</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Active Tasks</div>
        <div class="stat-value warning" id="stat-active">-</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Total Tasks</div>
        <div class="stat-value" id="stat-total">-</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Memory Nodes</div>
        <div class="stat-value success" id="stat-memory">-</div>
      </div>
    </div>

    <div class="section-title">Tasks</div>
    <div class="kanban" id="kanban-board">
      <div class="kanban-column">
        <div class="kanban-col-header">
          <span class="kanban-col-title">To Do</span>
          <span class="kanban-col-count" id="kanban-todo-count">0</span>
        </div>
        <div class="kanban-col-body" id="kanban-todo">
          <button class="kanban-add-btn" id="kanban-add-btn" onclick="showAddTask()">+ Add Task</button>
          <div class="add-task-form" id="add-task-form">
            <input type="text" id="add-task-input" placeholder="Task description..." autocomplete="off" />
            <div class="add-task-form-actions">
              <button class="add-task-submit" onclick="submitNewTask()">Add</button>
              <button class="add-task-cancel" onclick="hideAddTask()">Cancel</button>
            </div>
          </div>
        </div>
      </div>
      <div class="kanban-column">
        <div class="kanban-col-header">
          <span class="kanban-col-title">In Progress</span>
          <span class="kanban-col-count" id="kanban-progress-count">0</span>
        </div>
        <div class="kanban-col-body" id="kanban-progress"></div>
      </div>
      <div class="kanban-column">
        <div class="kanban-col-header">
          <span class="kanban-col-title">Done</span>
          <span class="kanban-col-count" id="kanban-done-count">0</span>
        </div>
        <div class="kanban-col-body" id="kanban-done"></div>
      </div>
    </div>
  </div>

  <!-- Agents Tab -->
  <div class="tab-panel" id="tab-agents">
    <div class="agents-grid" id="agents-grid">
      <div class="empty-state">Loading agents...</div>
    </div>
  </div>

  <!-- Chat Tab -->
  <div class="tab-panel active" id="tab-chat">
    <div class="chat-layout">
      <div class="chat-sidebar" id="chat-sidebar">
        <div class="chat-sidebar-title">Agents</div>
        <div id="chat-agent-list"></div>
      </div>
      <div class="chat-main">
        <div class="chat-header" id="chat-header">
          <span id="chat-header-emoji"></span>
          <span id="chat-header-name">Select an agent</span>
        </div>
        <div class="chat-messages" id="chat-messages">
          <div class="chat-empty">Select an agent to start chatting.</div>
        </div>
        <div class="chat-input-bar">
          <input class="chat-input" id="chat-input" type="text" placeholder="Type a message..." autocomplete="off" />
          <button class="chat-send" id="chat-send">Send</button>
        </div>
      </div>
    </div>
  </div>

</div>

<!-- Agent Detail Modal -->
<div class="modal-overlay" id="modal-overlay">
  <div class="modal">
    <div class="modal-header">
      <span id="modal-emoji" style="font-size:24px;"></span>
      <span id="modal-title" style="font-weight:600;font-size:16px;"></span>
      <button class="modal-close" id="modal-close">&times;</button>
    </div>
    <div class="modal-body" id="modal-body"></div>
  </div>
</div>

<script>
(function() {
  // ─── State ───
  var ws = null;
  var state = { agents: [], activeTasks: 0, totalTasks: 0, memoryStats: { nodeCount: 0 } };
  var agentsList = [];
  var tasksList = [];
  var chatMessages = {};  // keyed by agentId
  var shownTaskIds = {};  // track which task IDs have been displayed
  var selectedChatAgent = null;
  var reconnectTimer = null;
  var wizardStep = 0;
  var wizardData = { name: 'Fort', goals: '', emoji: '🏰', personality: '', avatarDataUrl: '' };
  var doneExpanded = false;
  var hasGreeted = false;
  var chatHistoryLoaded = false;
  var agentsLoaded = false;
  var initialRouteApplied = false;

  var EMOJI_OPTIONS = [
    '🏰', '🤖', '🦉', '🧠',
    '⚡', '🛡️', '🔮', '🌟',
    '🐙', '🎯', '🔥', '🌊',
    '🗡️', '🎭', '📡', '🧬'
  ];

  // ─── Utils ───
  function uuid() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
      var r = Math.random() * 16 | 0;
      return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
    });
  }

  function timeAgo(ts) {
    if (!ts) return '';
    var diff = Date.now() - new Date(ts).getTime();
    if (diff < 60000) return 'just now';
    if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
    if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
    return Math.floor(diff / 86400000) + 'd ago';
  }

  function esc(s) {
    var d = document.createElement('div');
    d.textContent = s || '';
    return d.innerHTML;
  }

  function truncate(s, len) {
    if (!s) return '';
    return s.length > len ? s.substring(0, len) + '...' : s;
  }

  // ─── Setup Wizard ───
  function initWizard() {
    fetch('/api/setup-status')
      .then(function(r) { return r.json(); })
      .then(function(data) {
        if (!data.complete) {
          document.getElementById('wizard-overlay').classList.add('active');
          renderEmojiGrid();
        }
      })
      .catch(function() {
        // If endpoint fails, skip wizard
      });
  }

  function renderEmojiGrid() {
    var grid = document.getElementById('emoji-grid');
    grid.innerHTML = EMOJI_OPTIONS.map(function(em) {
      var cls = em === wizardData.emoji ? 'emoji-option selected' : 'emoji-option';
      return '<div class="' + cls + '" data-emoji="' + em + '">' + em + '</div>';
    }).join('');
    grid.querySelectorAll('.emoji-option').forEach(function(el) {
      el.addEventListener('click', function() {
        wizardData.emoji = el.getAttribute('data-emoji');
        renderEmojiGrid();
      });
    });
  }

  // Avatar handling
  window.handleAvatarUpload = function(input) {
    var file = input.files && input.files[0];
    if (!file) return;
    if (file.size > 5 * 1024 * 1024) {
      alert('Image too large. Please use an image under 5MB.');
      return;
    }
    var reader = new FileReader();
    reader.onload = function(e) {
      wizardData.avatarDataUrl = e.target.result;
      document.getElementById('avatar-preview-img').src = e.target.result;
      // Update summary too
      document.getElementById('wizard-summary-avatar-img').src = e.target.result;
    };
    reader.readAsDataURL(file);
  };

  window.useDefaultAvatar = function() {
    wizardData.avatarDataUrl = '';
    document.getElementById('avatar-preview-img').src = '/api/default-avatar';
    document.getElementById('wizard-summary-avatar-img').src = '/api/default-avatar';
  };

  function updateWizardProgress() {
    var dots = document.querySelectorAll('.wizard-progress-dot');
    dots.forEach(function(dot, i) {
      dot.classList.remove('active', 'done');
      if (i < wizardStep) dot.classList.add('done');
      if (i === wizardStep) dot.classList.add('active');
    });
  }

  function showWizardStep(step) {
    document.querySelectorAll('.wizard-step').forEach(function(s) { s.classList.remove('active'); });
    var el = document.getElementById('wizard-step-' + step);
    if (el) el.classList.add('active');
    updateWizardProgress();
  }

  window.wizardNext = function() {
    // Save data from current step
    if (wizardStep === 1) {
      wizardData.name = document.getElementById('wizard-name').value.trim() || 'Fort';
      wizardData.goals = document.getElementById('wizard-goals').value.trim();
      wizardData.personality = (document.getElementById('wizard-personality') || {}).value || '';
    }
    // Prepare next step
    if (wizardStep === 2) {
      // Moving to summary — update all summary fields
      document.getElementById('wizard-summary-emoji').textContent = wizardData.emoji;
      document.getElementById('wizard-summary-name').textContent = wizardData.name;
      document.getElementById('wizard-summary-goals').textContent = wizardData.goals || '(No goals set)';
      // Avatar preview in summary
      var summaryImg = document.getElementById('wizard-summary-avatar-img');
      if (wizardData.avatarDataUrl) {
        summaryImg.src = wizardData.avatarDataUrl;
      } else {
        summaryImg.src = '/api/default-avatar';
      }
    }
    wizardStep = Math.min(wizardStep + 1, 3);
    showWizardStep(wizardStep);
  };

  window.wizardBack = function() {
    if (wizardStep === 1) {
      wizardData.name = document.getElementById('wizard-name').value.trim() || 'Fort';
      wizardData.goals = document.getElementById('wizard-goals').value.trim();
      wizardData.personality = (document.getElementById('wizard-personality') || {}).value || '';
    }
    wizardStep = Math.max(wizardStep - 1, 0);
    showWizardStep(wizardStep);
  };

  window.wizardSubmit = function() {
    var btn = document.getElementById('wizard-launch-btn');
    btn.disabled = true;
    btn.textContent = 'Creating agent...';

    fetch('/api/agents/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: wizardData.name,
        goals: wizardData.goals,
        emoji: wizardData.emoji,
        personality: wizardData.personality,
        avatarDataUrl: wizardData.avatarDataUrl || null,
      }),
    })
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.error) {
        btn.disabled = false;
        btn.textContent = 'Launch Fort';
        alert('Error: ' + data.error);
        return;
      }
      document.getElementById('wizard-overlay').classList.remove('active');
      // Refresh data
      wsSend('agents');
      wsSend('status');
      wsSend('tasks');

      // Switch to Chat tab and greet the user (wait for agent to finish starting)
      setTimeout(function() {
        if (data.id) {
          selectedChatAgent = data.id;
          renderChatSidebar();
          renderChatMessages();
        }
        switchToTab('chat');
        // Send a hidden greeting request — don't show the prompt in chat
        hasGreeted = true;
        wsSend('chat', { text: '__greeting__', agentId: data.id, hidden: true });
      }, 1500);
    })
    .catch(function(err) {
      btn.disabled = false;
      btn.textContent = 'Launch Fort';
      alert('Failed to create agent: ' + err.message);
    });
  };

  // ─── WebSocket ───
  function connect() {
    var protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(protocol + '//' + location.host);

    ws.onopen = function() {
      document.getElementById('wsDot').classList.add('connected');
      document.getElementById('wsLabel').textContent = 'Connected';
      clearTimeout(reconnectTimer);
      wsSend('status');
      wsSend('agents');
      wsSend('tasks');
      // Load chat history
      loadChatHistory();
    };

    ws.onclose = function() {
      document.getElementById('wsDot').classList.remove('connected');
      document.getElementById('wsLabel').textContent = 'Disconnected';
      reconnectTimer = setTimeout(connect, 3000);
    };

    ws.onerror = function() { ws.close(); };

    ws.onmessage = function(e) {
      try {
        var msg = JSON.parse(e.data);
        handleWSMessage(msg);
      } catch(err) { /* ignore parse errors */ }
    };
  }

  // ─── Hash Routing ───
  function switchToTab(tabName, pushHash) {
    document.querySelectorAll('.nav-tab').forEach(function(t) { t.classList.remove('active'); });
    document.querySelectorAll('.tab-panel').forEach(function(t) { t.classList.remove('active'); });
    var tab = document.querySelector('.nav-tab[data-tab="' + tabName + '"]');
    if (tab) tab.classList.add('active');
    var panel = document.getElementById('tab-' + tabName);
    if (panel) panel.classList.add('active');
    if (pushHash !== false) {
      var hash = '#' + tabName;
      if (tabName === 'chat' && selectedChatAgent) hash += '/' + selectedChatAgent;
      if (location.hash !== hash) history.replaceState(null, '', hash);
    }
  }

  function applyRoute() {
    var hash = location.hash.replace(/^#/, '');
    if (!hash) hash = 'chat';
    var parts = hash.split('/');
    var tabName = parts[0] || 'chat';
    var agentIdFromHash = parts[1] || null;

    // Validate tab name
    if (['dashboard', 'agents', 'chat'].indexOf(tabName) === -1) tabName = 'chat';

    switchToTab(tabName, false);

    if (tabName === 'chat' && agentIdFromHash && agentsLoaded) {
      var found = findAgent(agentIdFromHash);
      if (found) {
        selectedChatAgent = agentIdFromHash;
        renderChatSidebar();
        renderChatMessages();
      }
    }
  }

  window.addEventListener('hashchange', applyRoute);

  function maybeAutoGreet() {
    if (hasGreeted || !chatHistoryLoaded || !agentsLoaded) return;

    // Apply hash route now that data is loaded
    if (!initialRouteApplied) {
      initialRouteApplied = true;
      applyRoute();
    }

    // Auto-select default agent if none selected
    if (!selectedChatAgent && agentsList.length > 0) {
      var defaultAgent = agentsList.find(function(a) { return a.status === 'running'; });
      if (defaultAgent) {
        selectChatAgent((defaultAgent.config || defaultAgent).id);
      }
    }

    if (!selectedChatAgent) return;

    // Only greet if there's no chat history for this agent
    if (chatMessages[selectedChatAgent] && chatMessages[selectedChatAgent].length > 0) {
      hasGreeted = true;
      return;
    }

    hasGreeted = true;
    wsSend('chat', { text: '__greeting__', agentId: selectedChatAgent, hidden: true });
  }

  function wsSend(type, payload) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ id: uuid(), type: type, payload: payload }));
    }
  }

  function handleWSMessage(msg) {
    switch (msg.type) {
      case 'state':
      case 'status.response':
        state = msg.payload;
        renderDashboard();
        break;
      case 'agents.response':
        agentsList = msg.payload || [];
        renderAgents();
        renderChatSidebar();
        agentsLoaded = true;
        maybeAutoGreet();
        break;
      case 'tasks.response':
        tasksList = msg.payload || [];
        renderKanban();
        break;
      case 'chat.response':
        // Track the task ID to prevent duplicates from task.status_changed
        if (msg.payload && msg.payload.task && msg.payload.task.id) {
          shownTaskIds[msg.payload.task.id] = true;
        }
        // For greeting messages, show the agent's response directly from the completed task
        if (msg.payload && msg.payload.hidden && msg.payload.task && msg.payload.task.result) {
          var greetTask = msg.payload.task;
          var greetAgentId = greetTask.assignedAgent || greetTask.agentId || selectedChatAgent;
          if (greetAgentId) {
            addChatMessage(greetAgentId, 'agent', greetTask.result);
          }
        }
        wsSend('tasks');
        wsSend('status');
        break;
      case 'agent.acknowledged':
        if (msg.payload) {
          var ackAgent = msg.payload.agentId;
          var ackMsg = msg.payload.message;
          var ackTitle = msg.payload.title || '';
          var ackTask = {
            shortId: msg.payload.shortId,
            title: ackTitle,
            status: 'in_progress',
          };
          // Skip acknowledgment for hidden greeting tasks
          if (ackAgent && ackTitle.indexOf('Please greet me') === -1) {
            addChatMessage(ackAgent, 'agent', ackMsg, ackTask);
          }
        }
        break;
      case 'task.status_changed':
        var t = msg.payload;
        if (t && t.result && (t.source === 'user_chat' || t.source === 'background')) {
          if (t.status !== 'completed' && t.status !== 'failed' && t.status !== 'needs_review') break;
          var agentId = t.assignedAgent || t.agentId || t.agent;
          var taskId = t.id;
          var isGreetingTask = t.source === 'background' && (t.title || '').indexOf('Please greet me') !== -1;
          // Skip greetings — already handled by chat.response
          // Skip tasks already shown — prevents duplicates
          if (isGreetingTask || (taskId && shownTaskIds[taskId])) break;
          if (taskId) shownTaskIds[taskId] = true;
          if (agentId) {
            addChatMessage(agentId, 'agent', t.result, {
              shortId: t.shortId || t.id,
              title: t.title || 'Task',
              status: t.status || 'completed',
            });
          }
        }
        wsSend('tasks');
        wsSend('status');
        break;
      case 'task.created':
        wsSend('tasks');
        wsSend('status');
        break;
      case 'agent.started':
      case 'agent.error':
        wsSend('agents');
        wsSend('status');
        break;
      case 'error':
        if (msg.error && selectedChatAgent) {
          addChatMessage(selectedChatAgent, 'agent', 'Error: ' + msg.error);
        }
        break;
    }
  }

  // ─── Render: Dashboard ───
  function renderDashboard() {
    var agents = Array.isArray(state.agents) ? state.agents : [];
    document.getElementById('stat-agents').textContent = agents.length;
    document.getElementById('stat-active').textContent = state.activeTasks || 0;
    document.getElementById('stat-total').textContent = state.totalTasks || 0;
    var mem = state.memoryStats || {};
    document.getElementById('stat-memory').textContent = mem.nodeCount || 0;
  }

  // ─── Render: Kanban Board ───
  function renderKanban() {
    var todo = [];
    var progress = [];
    var done = [];

    tasksList.forEach(function(t) {
      var status = (t.status || 'pending').toLowerCase();
      if (status === 'completed' || status === 'done') {
        done.push(t);
      } else if (status === 'running' || status === 'in_progress' || status === 'active') {
        progress.push(t);
      } else {
        // pending, blocked, failed, etc
        todo.push(t);
      }
    });

    document.getElementById('kanban-todo-count').textContent = todo.length;
    document.getElementById('kanban-progress-count').textContent = progress.length;
    document.getElementById('kanban-done-count').textContent = done.length;

    // To Do column
    var todoEl = document.getElementById('kanban-todo');
    var addBtnHtml = '<button class="kanban-add-btn" onclick="showAddTask()">+ Add Task</button>' +
      '<div class="add-task-form" id="add-task-form"><input type="text" id="add-task-input" placeholder="Task description..." autocomplete="off" />' +
      '<div class="add-task-form-actions"><button class="add-task-submit" onclick="submitNewTask()">Add</button>' +
      '<button class="add-task-cancel" onclick="hideAddTask()">Cancel</button></div></div>';
    if (todo.length === 0) {
      todoEl.innerHTML = addBtnHtml + '<div class="kanban-empty">No tasks</div>';
    } else {
      todoEl.innerHTML = addBtnHtml + todo.map(renderKanbanCard).join('');
    }

    // In Progress column
    var progressEl = document.getElementById('kanban-progress');
    if (progress.length === 0) {
      progressEl.innerHTML = '<div class="kanban-empty">Nothing running</div>';
    } else {
      progressEl.innerHTML = progress.map(renderKanbanCard).join('');
    }

    // Done column (last 10, collapsible)
    var doneEl = document.getElementById('kanban-done');
    var doneVisible = doneExpanded ? done : done.slice(0, 10);
    if (done.length === 0) {
      doneEl.innerHTML = '<div class="kanban-empty">No completed tasks</div>';
    } else {
      var cards = doneVisible.map(renderKanbanCard).join('');
      if (done.length > 10 && !doneExpanded) {
        cards += '<button class="kanban-done-toggle" onclick="toggleDone()">Show all ' + done.length + ' completed</button>';
      } else if (done.length > 10 && doneExpanded) {
        cards += '<button class="kanban-done-toggle" onclick="toggleDone()">Show less</button>';
      }
      doneEl.innerHTML = cards;
    }
  }

  function renderKanbanCard(t) {
    var title = t.description || t.id || 'Unnamed task';
    var agentName = t.agentName || t.agent || '';
    var isAgent = !!agentName && agentName.toLowerCase() !== 'user';
    var ownerClass = isAgent ? 'agent-owned' : 'user-owned';
    var badgeClass = isAgent ? 'agent' : 'user';
    var agentEmoji = getAgentEmojiByName(agentName);
    var assigneeLabel = isAgent ? (agentEmoji + ' ' + esc(agentName)) : 'You';

    return '<div class="kanban-card ' + ownerClass + '">' +
      '<div class="kanban-card-title">' + esc(truncate(title, 60)) + '</div>' +
      '<div class="kanban-card-meta">' +
        '<span class="kanban-card-assignee ' + badgeClass + '">' + assigneeLabel + '</span>' +
        '<span class="kanban-card-time">' + esc(timeAgo(t.createdAt)) + '</span>' +
      '</div>' +
    '</div>';
  }

  function getAgentEmojiByName(name) {
    if (!name) return '';
    for (var i = 0; i < agentsList.length; i++) {
      var cfg = agentsList[i].config || agentsList[i];
      if ((cfg.name || '').toLowerCase() === name.toLowerCase()) {
        return agentsList[i].emoji || cfg.emoji || '';
      }
    }
    return '';
  }

  window.toggleDone = function() {
    doneExpanded = !doneExpanded;
    renderKanban();
  };

  window.showAddTask = function() {
    document.getElementById('add-task-form').classList.add('active');
    document.getElementById('add-task-input').focus();
  };

  window.hideAddTask = function() {
    document.getElementById('add-task-form').classList.remove('active');
    document.getElementById('add-task-input').value = '';
  };

  window.submitNewTask = function() {
    var input = document.getElementById('add-task-input');
    var text = input.value.trim();
    if (!text) return;
    input.value = '';
    document.getElementById('add-task-form').classList.remove('active');
    // Send as a chat message to create a task
    wsSend('chat', { text: text });
  };

  // ─── Render: Agents ───
  function renderAgents() {
    var el = document.getElementById('agents-grid');
    if (!agentsList.length) {
      el.innerHTML = '<div class="empty-state">No agents registered</div>';
      return;
    }
    el.innerHTML = agentsList.map(function(a, i) {
      var cfg = a.config || a;
      var emoji = a.emoji || cfg.emoji || '🤖';
      var name = cfg.name || 'Unknown';
      var desc = cfg.description || '';
      var status = a.status || 'idle';
      var isDefault = cfg.isDefault || a.isDefault;
      var defaultTag = isDefault ? '<span class="agent-default-tag">default</span>' : '';

      return '<div class="agent-card" data-index="' + i + '">' +
        '<div class="agent-header">' +
          '<span class="agent-emoji">' + esc(emoji) + '</span>' +
          '<span class="agent-name">' + esc(name) + defaultTag + '</span>' +
        '</div>' +
        '<div class="agent-desc">' + esc(desc) + '</div>' +
        '<span class="agent-status-badge ' + esc(status) + '">' + esc(status) + '</span>' +
        '</div>';
    }).join('');

    el.querySelectorAll('.agent-card').forEach(function(card) {
      card.addEventListener('click', function() {
        var idx = parseInt(card.getAttribute('data-index'));
        showAgentDetail(agentsList[idx]);
      });
    });
  }

  function showAgentDetail(agent) {
    var cfg = agent.config || agent;
    var emoji = agent.emoji || cfg.emoji || '🤖';
    document.getElementById('modal-emoji').textContent = emoji;
    document.getElementById('modal-title').textContent = cfg.name || 'Agent';
    var soul = agent.soul || '';
    document.getElementById('modal-body').textContent = soul || 'No SOUL.md content available for this agent.';
    document.getElementById('modal-overlay').classList.add('active');
  }

  document.getElementById('modal-close').addEventListener('click', function() {
    document.getElementById('modal-overlay').classList.remove('active');
  });
  document.getElementById('modal-overlay').addEventListener('click', function(e) {
    if (e.target === this) this.classList.remove('active');
  });

  // ─── Render: Chat Sidebar ───
  function renderChatSidebar() {
    var listEl = document.getElementById('chat-agent-list');
    if (!agentsList.length) {
      listEl.innerHTML = '<div style="padding:16px;color:var(--text-secondary);font-size:12px;">No agents available</div>';
      return;
    }
    listEl.innerHTML = agentsList.map(function(a) {
      var cfg = a.config || a;
      var agentId = cfg.id || cfg.name;
      var emoji = a.emoji || cfg.emoji || '🤖';
      var name = cfg.name || 'Unknown';
      var cls = 'chat-agent-item' + (selectedChatAgent === agentId ? ' active' : '');
      var isDefault = cfg.isDefault || a.isDefault;
      return '<div class="' + cls + '" data-agent-id="' + esc(agentId) + '">' +
        '<span class="chat-agent-item-emoji">' + esc(emoji) + '</span>' +
        '<span class="chat-agent-item-name">' + esc(name) + (isDefault ? ' *' : '') + '</span>' +
      '</div>';
    }).join('');

    listEl.querySelectorAll('.chat-agent-item').forEach(function(item) {
      item.addEventListener('click', function() {
        var agentId = item.getAttribute('data-agent-id');
        selectChatAgent(agentId);
      });
    });

    // Auto-select default agent if none selected
    if (!selectedChatAgent && agentsList.length > 0) {
      var defaultAgent = agentsList.find(function(a) {
        var cfg = a.config || a;
        return cfg.isDefault || a.isDefault;
      });
      var firstAgent = defaultAgent || agentsList[0];
      var firstCfg = firstAgent.config || firstAgent;
      selectChatAgent(firstCfg.id || firstCfg.name);
    }
  }

  function selectChatAgent(agentId) {
    selectedChatAgent = agentId;
    // Update hash
    history.replaceState(null, '', '#chat/' + agentId);
    // Update sidebar active state
    document.querySelectorAll('.chat-agent-item').forEach(function(item) {
      item.classList.toggle('active', item.getAttribute('data-agent-id') === agentId);
    });
    // Update header
    var agent = findAgent(agentId);
    if (agent) {
      var cfg = agent.config || agent;
      document.getElementById('chat-header-emoji').textContent = agent.emoji || cfg.emoji || '🤖';
      document.getElementById('chat-header-name').textContent = cfg.name || agentId;
    }
    renderChatMessages();
  }

  function findAgent(agentId) {
    return agentsList.find(function(a) {
      var cfg = a.config || a;
      return (cfg.id || cfg.name) === agentId;
    });
  }

  function getAgentEmoji(agentId) {
    var agent = findAgent(agentId);
    if (agent) {
      var cfg = agent.config || agent;
      return agent.emoji || cfg.emoji || '🤖';
    }
    return '🤖';
  }

  // ─── Render: Chat Messages ───
  function addChatMessage(agentId, role, text, taskInfo) {
    if (!chatMessages[agentId]) chatMessages[agentId] = [];
    chatMessages[agentId].push({ role: role, text: text, ts: Date.now(), task: taskInfo || null });
    if (agentId === selectedChatAgent) renderChatMessages();
  }

  function renderChatMessages() {
    var el = document.getElementById('chat-messages');
    var msgs = chatMessages[selectedChatAgent] || [];
    if (!msgs.length) {
      var agent = findAgent(selectedChatAgent);
      var name = agent ? (agent.config || agent).name : selectedChatAgent;
      el.innerHTML = '<div class="chat-empty">Send a message to start chatting with ' + esc(name) + '.</div>';
      return;
    }
    var emoji = getAgentEmoji(selectedChatAgent);
    el.innerHTML = msgs.map(function(m) {
      var isUser = m.role === 'user';
      var taskCard = '';
      if (m.task) {
        var statusClass = (m.task.status || 'created').toLowerCase();
        var statusLabel = m.task.status === 'completed' ? 'Complete' : m.task.status === 'in_progress' ? 'In Progress' : m.task.status;
        taskCard = '<div class="chat-task-card">' +
          '<div class="chat-task-id">' + esc(m.task.shortId || m.task.id) + '</div>' +
          '<div class="chat-task-title">' + esc(m.task.title || 'Task') + '</div>' +
          '<div class="chat-task-status ' + esc(statusClass) + '">' + esc(statusLabel) + '</div>' +
        '</div>';
      }
      return '<div class="chat-msg ' + (isUser ? 'user' : 'agent') + '">' +
        '<div class="chat-msg-avatar">' + (isUser ? 'You' : esc(emoji)) + '</div>' +
        '<div class="chat-msg-body">' +
          '<div class="chat-msg-content">' + esc(m.text) + '</div>' +
          taskCard +
        '</div>' +
        '</div>';
    }).join('');
    el.scrollTop = el.scrollHeight;
  }

  document.getElementById('chat-send').addEventListener('click', sendChat);
  document.getElementById('chat-input').addEventListener('keydown', function(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendChat();
    }
  });

  function sendChat() {
    var input = document.getElementById('chat-input');
    var text = input.value.trim();
    if (!text || !selectedChatAgent) return;
    input.value = '';
    addChatMessage(selectedChatAgent, 'user', text);
    wsSend('chat', { text: text, agentId: selectedChatAgent });
  }

  function loadChatHistory() {
    fetch('/api/chat-history')
      .then(function(r) { return r.json(); })
      .then(function(tasks) {
        if (!Array.isArray(tasks)) return;
        tasks.forEach(function(t) {
          var agentId = t.assignedAgent;
          if (!agentId) return;
          var isGreeting = t.source === 'background' && (t.description || '').indexOf('Please greet me') !== -1;
          if (!chatMessages[agentId]) chatMessages[agentId] = [];
          // Don't show the hidden greeting prompt as a user message
          if (!isGreeting) {
            chatMessages[agentId].push({
              role: 'user',
              text: t.description,
              ts: new Date(t.createdAt).getTime(),
              task: null,
            });
          }
          // Add agent response (greeting responses show without task card)
          if (t.result) {
            chatMessages[agentId].push({
              role: 'agent',
              text: t.result,
              ts: new Date(t.createdAt).getTime() + 1,
              task: isGreeting ? null : { shortId: t.shortId, title: t.title, status: t.status },
            });
          }
        });
        if (selectedChatAgent) renderChatMessages();
        chatHistoryLoaded = true;
        maybeAutoGreet();
      })
      .catch(function() {
        chatHistoryLoaded = true;
        maybeAutoGreet();
      });
  }

  // ─── Tab Navigation ───
  document.querySelectorAll('.nav-tab').forEach(function(tab) {
    tab.addEventListener('click', function() {
      switchToTab(tab.getAttribute('data-tab'));
    });
  });

  // ─── Init ───
  // Apply initial route from hash (or default to chat)
  applyRoute();
  initWizard();
  connect();

})();
</script>
</body>
</html>`;
}
