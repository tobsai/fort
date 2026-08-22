import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const root = new URL("..", import.meta.url);
const page = readFileSync(new URL("app/page.tsx", root), "utf8");
const layout = readFileSync(new URL("app/layout.tsx", root), "utf8");
const styles = readFileSync(new URL("app/globals.css", root), "utf8");

describe("the stable-Agent product shell", () => {
  it("makes durable Agents and Groups the primary navigation", () => {
    expect(page).toContain("createSignedFortControlAgentClientFromEnvironment");
    expect(page).toContain("Your Agents");
    expect(page).toContain("Groups");
    expect(page).toContain("Home");
    expect(page).not.toContain("listMachines");
    expect(page).not.toContain("RevokeButton");
    expect(layout).toContain('className="brand-context">AGENT CHAT');
    expect(layout).toContain('href="/groups"');
    expect(layout).toContain('href="/legacy"');
  });

  it("discloses exact binding evidence without making a machine the identity", () => {
    expect(page).toContain("agent.binding.provider");
    expect(page).toContain("agent.binding.requested_model");
    expect(page).toContain("agent.binding.computer_id");
    expect(page).toContain("agent.agent.id");
    expect(styles).toContain(".agent-shell");
    expect(styles).toContain(".agent-card");
    expect(styles).toContain(".agent-binding");
  });
});
