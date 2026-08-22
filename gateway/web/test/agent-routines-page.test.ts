import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const page = readFileSync(new URL("../app/agents/[agentId]/page.tsx", import.meta.url), "utf8");
const manager = readFileSync(new URL("../app/agents/[agentId]/RoutineManager.tsx", import.meta.url), "utf8");

describe("Agent-owned Routine surface", () => {
  it("loads Routines with Agent detail and limits result choices to open Agent Conversations", () => {
    expect(page).toContain("createFortControlRoutineClient");
    expect(page).toContain("routineClient.list");
    expect(page).toContain("routineClient.listRuns");
    expect(page).toContain('conversation.state === "open"');
    expect(page).toContain("<RoutineManager");
  });

  it("shows exact Routine evidence and supports create, revalidate, and Test Routine", () => {
    for (const label of [
      "Routine state",
      "Behavior Revision",
      "Binding Revision",
      "Schedule",
      "Timezone",
      "Next occurrence",
      "Result Conversation",
      "Run history",
      "Failure",
      "Next action",
      "Create Routine",
      "Revalidate",
      "Test Routine",
    ]) {
      expect(manager).toContain(label);
    }
    expect(manager).toContain("commandClient.create");
    expect(manager).toContain("commandClient.revalidate");
    expect(manager).toContain("commandClient.test");
    expect(manager).toContain("fort_cloud");
    expect(manager).toContain('placeholder="0 0 9 * * 1"');
    expect(manager).not.toMatch(/\b(provider|model|machine|adapter|source-native)\b/i);
    expect(manager).not.toMatch(/"authority"\s*:/);
  });
});
