import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const page = readFileSync(new URL("../app/agents/[agentId]/page.tsx", import.meta.url), "utf8");
const settings = readFileSync(new URL("../app/agents/[agentId]/AgentSettings.tsx", import.meta.url), "utf8");

describe("stable Agent owner settings", () => {
  it("embeds presentation and Behavior revision controls in the existing detail page", () => {
    expect(page).toContain('import AgentSettings from "./AgentSettings"');
    expect(page).toContain("<AgentSettings");
    expect(settings).toContain('"use client"');
    expect(settings).toContain("createAgentMutationClient");
    expect(settings).toContain('action: "profile"');
    expect(settings).toContain('action: "behavior"');
    expect(settings).toContain("expectedProfileRevisionID");
    expect(settings).toContain("expectedBehaviorRevisionID");
    expect(settings).toContain("expectedBindingRevisionID");
  });

  it("does not add execution identity, Agent creation, or Rebind inputs", () => {
    for (const forbidden of [
      "provider", "requestedModel", "resolvedModel", "machine", "computerID",
      "cloudRuntime", "adapterID", "executionSourceID", "sourceAgentID", 'action: "rebind"',
    ]) {
      expect(settings).not.toContain(forbidden);
    }
    expect(settings).not.toMatch(/<button[^>]*>\s*(Create Agent|Rebind)/);
  });
});
