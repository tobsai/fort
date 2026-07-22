import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import {
  chatRequestForRoute,
  checkpointCaption,
  crewAssignments,
  DeckLoadGate,
  DeckOperationGate,
  latestRunByAgent,
  recentFailedRuns,
  relayIdentityTrusted,
  routePreviewMatchesDraft,
  shouldRefreshPlaybookCatalog,
  runState,
  shouldShowOfflineDeck,
  sortRunsForDeck,
  type DeckGate,
  type DeckMachine,
  type DeckRoutePreview,
  type DeckRun,
} from "@/lib/command-deck";

const run = (overrides: Partial<DeckRun> = {}): DeckRun => ({
  id: "r1",
  title: "Search indexing",
  agent: "claude",
  status: "running",
  ...overrides,
});

const gate = (runID = "r1"): DeckGate => ({
  run_id: runID,
  node_id: "review",
});

describe("gateway Command Deck presentation", () => {
  it("uses the shared attention-first status grammar", () => {
    expect(runState(run(), [gate()])).toBe("needs-you");
    expect(runState(run({ status: "failed" }), [])).toBe("failed");
    expect(runState(run({ status: "running" }), [])).toBe("working");
    expect(runState(run({ status: "succeeded" }), [])).toBe("delivered");
    expect(runState(run({ status: "queued" }), [])).toBe("idle");
  });

  it("describes progress only with human checkpoint decisions", () => {
    expect(checkpointCaption({ total: 5, accepted: 3, waiting: 1, rejected: 0, done: 4 })).toBe(
      "3 of 5 checkpoints accepted · 1 awaiting sign-off",
    );
    expect(checkpointCaption(undefined)).toBe("No checkpoint plan yet");
  });

  it("selects each agent's most recently updated assignment", () => {
    const selected = latestRunByAgent([
      run({ id: "old", updated_at: "2026-07-20T10:00:00Z" }),
      run({ id: "new", updated_at: "2026-07-21T10:00:00Z" }),
      run({ id: "hermes", agent: "hermes", updated_at: "2026-07-19T10:00:00Z" }),
    ]);
    expect(selected.map((item) => item.id)).toEqual(["new", "hermes"]);

    const liveBeforeDone = latestRunByAgent([
      run({ id: "still-working", status: "running", updated_at: "2026-07-20T10:00:00Z" }),
      run({ id: "newer-done", status: "succeeded", updated_at: "2026-07-21T10:00:00Z" }),
    ]);
    expect(liveBeforeDone.map((item) => item.id)).toEqual(["still-working"]);

    const recovery = latestRunByAgent([
      run({ id: "old-failure", status: "failed", updated_at: "2026-06-01T10:00:00Z" }),
      run({ id: "new-success", status: "succeeded", updated_at: "2026-07-21T10:00:00Z" }),
    ]);
    expect(recovery.map((item) => item.id)).toEqual(["new-success"]);
  });

  it("builds Crew from the machine roster without presenting flow ids as agents", () => {
    const machines: DeckMachine[] = [
      { name: "studio", agents: ["claude", "codex", "hermes"], local: true, reachable: true },
    ];
    const crew = crewAssignments(
      [
        run({ id: "flow", agent: "flow:feature-work" }),
        run({ id: "direct", agent: "claude", status: "succeeded" }),
        run({ id: "live", agent: "hermes", status: "running" }),
      ],
      machines,
    );
    expect(crew.map((member) => member.agent)).toEqual(["claude", "codex", "hermes"]);
    expect(crew.map((member) => member.run?.id)).toEqual(["direct", undefined, "live"]);
    expect(crew.map((member) => member.attributionUnknown)).toEqual([true, true, false]);
  });

  it("keeps waiting work ahead of newer quiet assignments", () => {
    const gates = [gate("waiting")];
    const ordered = sortRunsForDeck(
      [
        run({ id: "delivered", status: "succeeded", updated_at: "2026-07-22T12:00:00Z" }),
        run({ id: "waiting", status: "blocked", updated_at: "2026-07-20T12:00:00Z" }),
        run({ id: "working", status: "running", updated_at: "2026-07-22T11:00:00Z" }),
      ],
      gates,
    );
    expect(ordered.map((item) => item.id)).toEqual(["waiting", "working", "delivered"]);
  });

  it("only raises recent failed assignments in Needs you", () => {
    const now = Date.parse("2026-07-22T12:00:00Z");
    const failed = recentFailedRuns(
      [
        run({ id: "recent", status: "failed", updated_at: "2026-07-22T11:00:00Z" }),
        run({ id: "old", status: "failed", updated_at: "2026-07-19T11:00:00Z" }),
      ],
      [],
      now,
    );
    expect(failed.map((item) => item.id)).toEqual(["recent"]);
  });

  it("fails closed when pin verification belongs to a previous machine identity", () => {
    expect(relayIdentityTrusted("m1:key-new", "m1:key-old", "pinned")).toBe(false);
    expect(relayIdentityTrusted("m1:key-new", "m1:key-new", "checking")).toBe(false);
    expect(relayIdentityTrusted("m1:key-new", "m1:key-new", "mismatch")).toBe(false);
    expect(relayIdentityTrusted("m1:key-new", "m1:key-new", "first")).toBe(true);
    expect(relayIdentityTrusted("m1:key-new", "m1:key-new", "pinned")).toBe(true);
  });

  it("keeps a loaded deck visible if a later connectivity check fails", () => {
    expect(shouldShowOfflineDeck(false, false)).toBe(true);
    expect(shouldShowOfflineDeck(false, true)).toBe(false);
    expect(shouldShowOfflineDeck(true, false)).toBe(false);
  });

  it("coalesces quiet polls while allowing a foreground refresh to supersede one", () => {
    const gate = new DeckLoadGate();
    const finishQuiet = gate.begin(true);
    expect(finishQuiet).not.toBeNull();
    expect(gate.begin(true)).toBeNull();

    const finishForeground = gate.begin(false);
    expect(finishForeground).not.toBeNull();
    finishQuiet?.();
    expect(gate.begin(true)).toBeNull();

    finishForeground?.();
    const finishNext = gate.begin(true);
    expect(finishNext).not.toBeNull();
    finishNext?.();
  });

  it("keeps a consequential operation locked until its owning request finishes", () => {
    const gate = new DeckOperationGate();
    const oldMutation = gate.begin("mutation");
    expect(oldMutation).not.toBeNull();
    expect(gate.begin("snapshot")).toBeNull();
    expect(gate.current()).toBe("mutation");
    gate.reset();
    const newMutation = gate.begin("mutation");
    expect(newMutation).not.toBeNull();
    expect(gate.end(oldMutation!)).toBe(false);
    expect(gate.current()).toBe("mutation");
    expect(gate.end(newMutation!)).toBe(true);
    expect(gate.begin("snapshot")).not.toBeNull();
  });

  it("pins the resolved route and suppresses plan gates for direct answers", () => {
    const answer: DeckRoutePreview = {
      playbook_id: "quick-answer",
      playbook_revision: 3,
      playbook_name: "Quick answer",
      task_type: "question",
      source: "trigger",
      plan_gate: false,
      delivery: "answer",
      stages: [{ order: 1, name: "Answer", agent: "hermes", model: "Codex 5.6 Sol", memory: false }],
    };
    expect(chatRequestForRoute("Why was the sweep skipped?", answer, true)).toEqual({
      text: "Why was the sweep skipped?",
      task_type: "question",
      plan_gate: false,
      playbook_id: "quick-answer",
      playbook_revision: 3,
    });

    const assignment: DeckRoutePreview = {
      ...answer,
      playbook_id: "feature-work",
      playbook_revision: 7,
      playbook_name: "Feature work",
      task_type: "feature",
      plan_gate: true,
      delivery: "assignment",
    };
    expect(chatRequestForRoute("Add CSV export", assignment, false)).toEqual({
      text: "Add CSV export",
      task_type: "feature",
      plan_gate: false,
      playbook_id: "feature-work",
      playbook_revision: 7,
    });
  });

  it("refuses a route preview after its direction draft changes", () => {
    expect(routePreviewMatchesDraft("Old brief", true, "Old brief", true)).toBe(true);
    expect(routePreviewMatchesDraft("Old brief", true, "New brief", true)).toBe(false);
    expect(routePreviewMatchesDraft("Old brief", true, "Old brief", false)).toBe(false);
  });

  it("refreshes the playbook catalog for every new handoff", () => {
    expect(shouldRefreshPlaybookCatalog(false, 4)).toBe(true);
    expect(shouldRefreshPlaybookCatalog(true, 4)).toBe(false);
    expect(shouldRefreshPlaybookCatalog(true, 0)).toBe(true);
  });

  it("pins the approved visual and encrypted-load contracts in source", () => {
    const root = fileURLToPath(new URL("..", import.meta.url));
    const client = readFileSync(`${root}/components/board-client.tsx`, "utf8");
    const surface = readFileSync(`${root}/components/command-deck-surface.tsx`, "utf8");
    const commandDeckSource = `${client}\n${surface}`;
    const css = readFileSync(`${root}/app/globals.css`, "utf8");

    for (const endpoint of [
      "/api/summary",
      "/api/board",
      "/api/backlog",
      "/api/machines",
      "/api/route",
    ]) {
      expect(client).toContain(endpoint);
    }
    for (const label of ["NEEDS YOU", "PROJECTS", "UP NEXT", "CREW", "Give direction"]) {
      expect(commandDeckSource).toContain(label);
    }
    for (const routeLabel of [
      "Preview route",
      "CONFIRM ROUTE",
      "routePreview.stages",
      "/api/playbooks",
      "Change route",
    ]) {
      expect(client).toContain(routeLabel);
    }
    expect(client).toContain("setBoardHtml(null)");
    expect(client).toContain("requireTrustedIdentity");
    expect(client).toContain("const [tailing, setTailing]");
    expect(client).toContain(
      "if (tailAbort.current === controller) {\n        tailAbort.current = null;\n        setTailing(false);\n      }",
    );
    expect(client).toContain("const [deckError, setDeckError]");
    expect(client).toContain("const [operationError, setOperationError]");
    expect(client).toContain("const [tailError, setTailError]");
    expect(client).toContain("directionRevision.current");
    expect(client).not.toContain("setError(null)");
    expect(client).not.toContain('setBusy("tail")');
    expect(client).toContain('sandbox=""');
    expect(client).not.toContain('sandbox="allow-scripts"');

    for (const token of [
      "--bg-canvas: #07090e",
      "--bg: #0b0e14",
      "--panel: #12161f",
      "--brass: #c9a35c",
      "--needs-you: #e0a458",
      "--working: #6fa8ff",
      "--accepted: #57b98a",
    ]) {
      expect(css).toContain(token);
    }
  });
});
