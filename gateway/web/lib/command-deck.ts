export interface DeckCheckpointSummary {
  total: number;
  accepted: number;
  waiting: number;
  rejected: number;
  done: number;
}

export interface DeckRun {
  id: string;
  title: string;
  body?: string;
  agent: string;
  status: string;
  machine?: string;
  flow_id?: string;
  created_at?: string;
  updated_at?: string;
  checkpoints?: DeckCheckpointSummary;
}

export interface DeckGate {
  run_id: string;
  node_id: string;
  input?: string;
  since?: string;
}

export interface DeckSummary {
  total: number;
  running: number;
  queued: number;
  blocked: number;
  succeeded: number;
  failed: number;
  execution: boolean;
  gates: DeckGate[];
}

export interface DeckBoard {
  runs: DeckRun[];
  gates: DeckGate[];
}

export interface DeckBacklogItem {
  id: string;
  title: string;
  body?: string;
  agent?: string;
  machine?: string;
  source: string;
}

export interface DeckMachine {
  name: string;
  url?: string;
  agents: string[];
  local: boolean;
  reachable: boolean;
}

export interface DeckPayload {
  summary: DeckSummary;
  board: DeckBoard;
  backlog: DeckBacklogItem[];
  machines: DeckMachine[];
}

export interface DeckRoutePreview {
  playbook_id: string;
  playbook_revision: number;
  playbook_name: string;
  task_type: string;
  source: string;
  plan_gate: boolean;
  delivery: "answer" | "assignment";
  stages: DeckResolvedStage[];
}

export interface DeckPlaybook {
  id: string;
  name: string;
  revision: number;
  is_default?: boolean;
  plan_gate?: boolean;
  delivery: "answer" | "assignment";
  trigger: DeckPlaybookTrigger;
  stages: DeckPlaybookStage[];
}

export interface DeckPlaybookTrigger {
  kind: string;
  enabled: boolean;
}

export interface DeckPlaybookAssignment {
  task_type?: string;
  profile?: string;
  agent: string;
  model?: string;
}

export interface DeckPlaybookStage {
  order: number;
  name: string;
  prompt?: string;
  description?: string;
  assignments: DeckPlaybookAssignment[];
  memory?: boolean;
}

export interface DeckResolvedStage {
  order: number;
  name: string;
  prompt?: string;
  profile?: string;
  agent: string;
  model?: string;
  memory?: boolean;
}

export interface DeckChatRequest {
  text: string;
  task_type: string;
  plan_gate: boolean;
  playbook_id: string;
  playbook_revision: number;
}

export interface DeckCrewAssignment {
  agent: string;
  run?: DeckRun;
  attributionUnknown: boolean;
}

export class DeckLoadGate {
  private inFlight = 0;

  begin(quiet: boolean): (() => void) | null {
    if (quiet && this.inFlight > 0) return null;
    this.inFlight += 1;
    let finished = false;
    return () => {
      if (finished) return;
      finished = true;
      this.inFlight = Math.max(0, this.inFlight - 1);
    };
  }
}

export interface DeckOperationLease<T extends string = string> {
  operation: T;
  token: number;
}

export class DeckOperationGate<T extends string = string> {
  private active: DeckOperationLease<T> | null = null;
  private nextToken = 1;

  begin(operation: T): DeckOperationLease<T> | null {
    if (this.active !== null) return null;
    const lease = { operation, token: this.nextToken++ };
    this.active = lease;
    return lease;
  }

  end(lease: DeckOperationLease<T>): boolean {
    if (this.active?.token !== lease.token) return false;
    this.active = null;
    return true;
  }

  current(): T | null {
    return this.active?.operation ?? null;
  }

  reset(): void {
    this.active = null;
  }
}

export type DeckRunState = "needs-you" | "working" | "delivered" | "failed" | "idle";
export type DeckPinState = "checking" | "first" | "pinned" | "mismatch";

export function relayIdentityTrusted(
  currentIdentity: string,
  verifiedIdentity: string,
  state: DeckPinState,
): boolean {
  return (
    currentIdentity === verifiedIdentity &&
    (state === "first" || state === "pinned")
  );
}

export function shouldShowOfflineDeck(online: boolean, hasDeck: boolean): boolean {
  return !online && !hasDeck;
}

export function chatRequestForRoute(
  text: string,
  route: DeckRoutePreview,
  planGate: boolean,
): DeckChatRequest {
  return {
    text,
    task_type: route.task_type,
    plan_gate: route.delivery === "answer" ? false : planGate,
    playbook_id: route.playbook_id,
    playbook_revision: route.playbook_revision,
  };
}

export function routePreviewMatchesDraft(
  previewText: string,
  previewPlanGate: boolean,
  currentText: string,
  currentPlanGate: boolean,
): boolean {
  return previewText === currentText && previewPlanGate === currentPlanGate;
}

export function shouldRefreshPlaybookCatalog(
  switchingRoute: boolean,
  catalogCount: number,
): boolean {
  return !switchingRoute || catalogCount === 0;
}

export function runState(run: DeckRun, gates: DeckGate[]): DeckRunState {
  const status = run.status.toLowerCase();
  if (gates.some((gate) => gate.run_id === run.id)) return "needs-you";
  if (status === "failed" || status === "error") return "failed";
  if (status === "running") return "working";
  if (status === "succeeded" || status === "done") {
    return (run.checkpoints?.rejected ?? 0) > 0 ? "idle" : "delivered";
  }
  if (status === "blocked" || status === "paused") return "needs-you";
  return "idle";
}

export function checkpointCaption(checkpoints: DeckCheckpointSummary | undefined): string {
  if (!checkpoints || checkpoints.total === 0) return "No checkpoint plan yet";
  const parts = [`${checkpoints.accepted} of ${checkpoints.total} checkpoints accepted`];
  if (checkpoints.waiting > 0) parts.push(`${checkpoints.waiting} awaiting sign-off`);
  if (checkpoints.rejected > 0) parts.push(`${checkpoints.rejected} redirected`);
  return parts.join(" · ");
}

export function latestRunByAgent(runs: DeckRun[], gates: DeckGate[] = []): DeckRun[] {
  const priority: Record<DeckRunState, number> = {
    "needs-you": 0,
    working: 1,
    failed: 2,
    delivered: 2,
    idle: 2,
  };
  const latest = new Map<string, DeckRun>();
  for (const run of runs) {
    if (run.agent.toLowerCase().startsWith("flow:")) continue;
    const key = run.agent.trim().toLowerCase();
    const current = latest.get(key);
    if (
      !current ||
      priority[runState(run, gates)] < priority[runState(current, gates)] ||
      (priority[runState(run, gates)] === priority[runState(current, gates)] &&
        runTime(run) > runTime(current))
    ) {
      latest.set(key, run);
    }
  }
  return [...latest.values()];
}

export function crewAssignments(
  runs: DeckRun[],
  machines: DeckMachine[],
  gates: DeckGate[] = [],
): DeckCrewAssignment[] {
  const latest = new Map(
    latestRunByAgent(runs, gates).map((run) => [run.agent.trim().toLowerCase(), run]),
  );
  const activeFlow = runs.some((run) => {
    const status = run.status.toLowerCase();
    return (
      run.agent.toLowerCase().startsWith("flow:") &&
      (status === "running" || status === "blocked" || status === "paused")
    );
  });
  const agents: string[] = [];
  const seen = new Set<string>();
  for (const machine of machines) {
    for (const agent of machine.agents) {
      const key = agent.trim().toLowerCase();
      if (!key || key.startsWith("flow:") || seen.has(key)) continue;
      seen.add(key);
      agents.push(agent);
    }
  }
  for (const run of latest.values()) {
    const key = run.agent.trim().toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    agents.push(run.agent);
  }
  return agents.map((agent) => {
    const run = latest.get(agent.trim().toLowerCase());
    return {
      agent,
      ...(run ? { run } : {}),
      attributionUnknown:
        activeFlow &&
        (!run || !["needs-you", "working"].includes(runState(run, gates))),
    };
  });
}

export function sortRunsForDeck(
  runs: DeckRun[],
  gates: DeckGate[],
  now = Date.now(),
): DeckRun[] {
  const priority = (run: DeckRun): number => {
    const state = runState(run, gates);
    if (state === "needs-you") return 0;
    if (state === "failed" && isRecentFailure(run, gates, now)) return 1;
    if (state === "working") return 2;
    return 3;
  };
  return [...runs].sort((left, right) => {
    const stateDelta = priority(left) - priority(right);
    return stateDelta || runTime(right) - runTime(left);
  });
}

export function isRecentFailure(
  run: DeckRun,
  gates: DeckGate[],
  now = Date.now(),
): boolean {
  const cutoff = now - 48 * 60 * 60 * 1000;
  return runState(run, gates) === "failed" && runTime(run) >= cutoff;
}

export function recentFailedRuns(
  runs: DeckRun[],
  gates: DeckGate[],
  now = Date.now(),
): DeckRun[] {
  const cutoff = now - 48 * 60 * 60 * 1000;
  return runs.filter(
    (run) => runState(run, gates) === "failed" && runTime(run) >= cutoff,
  );
}

export function crewFailureActivity(
  run: DeckRun,
  gates: DeckGate[],
  now = Date.now(),
): string {
  return `${isRecentFailure(run, gates, now) ? "needs attention on" : "failed"} ${run.title}`;
}

export function nextPlaybookStageOrder(stages: Array<{ order: number }>): number {
  return stages.reduce((maximum, stage) => Math.max(maximum, stage.order), 0) + 1;
}

export function projectStateLabel(
  run: DeckRun,
  gates: DeckGate[],
  now = Date.now(),
): string {
  const state = runState(run, gates);
  const labels: Record<DeckRunState, string> = {
    "needs-you": "Needs you",
    working: "Working",
    delivered: "Delivered",
    failed: isRecentFailure(run, gates, now) ? "Needs attention" : "Failed",
    idle: "Idle",
  };
  return labels[state];
}

export function meshReachability(machines: DeckMachine[]): string {
  if (machines.length === 0) return "roster not loaded";
  return `${machines.filter((machine) => machine.reachable).length} of ${machines.length} reachable`;
}

export function displayAgent(agent: string): string {
  const normalized = agent.trim().toLowerCase();
  const labels: Record<string, string> = {
    claude: "Claude Code",
    codex: "Codex",
    hermes: "Hermes",
    openclaw: "OpenClaw",
  };
  return labels[normalized] ?? agent;
}

export function relativeAge(value: string | undefined, now = Date.now()): string {
  if (!value) return "just now";
  const then = Date.parse(value);
  if (!Number.isFinite(then)) return "just now";
  const seconds = Math.max(0, Math.floor((now - then) / 1000));
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function runTime(run: DeckRun): number {
  const parsed = Date.parse(run.updated_at ?? run.created_at ?? "");
  return Number.isFinite(parsed) ? parsed : 0;
}
