import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const page = readFileSync(new URL("../app/agents/[agentId]/page.tsx", import.meta.url), "utf8");

describe("stable Agent detail page", () => {
  it("opens Home by default and exposes pinned secondary conversations", () => {
    expect(page).toContain("createFortControlAgentDetailClient");
    expect(page).toContain("Canonical Home");
    expect(page).toContain("Pinned conversations");
    expect(page).toContain("item.pinned");
    expect(page).toContain("agent.binding.provider");
    expect(page).toContain("agent.binding.requested_model");
    expect(page).toContain("agent.binding.computer_id");
  });
});
