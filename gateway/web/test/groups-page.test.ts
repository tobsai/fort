import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const page = readFileSync(new URL("../app/groups/page.tsx", import.meta.url), "utf8");

describe("Groups product page", () => {
  it("renders explicit multi-Agent membership and bounded collaboration language", () => {
    expect(page).toContain("createFortControlGroupClient");
    expect(page).toContain("createSignedFortControlAgentClientFromEnvironment");
    expect(page).toContain("Multi-Agent Groups");
    expect(page).toContain("2–6 Agents");
    expect(page).toContain("10 Agent messages");
    expect(page).toContain("depth 3");
    expect(page).toContain("group.membership.members");
  });
});
