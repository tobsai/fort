import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const root = new URL("..", import.meta.url);
const detail = readFileSync(new URL("app/groups/[groupId]/page.tsx", root), "utf8");
const commands = readFileSync(new URL("app/groups/[groupId]/GroupCommands.tsx", root), "utf8");
const lifecycle = readFileSync(new URL("app/groups/[groupId]/GroupLifecycleControls.tsx", root), "utf8");
const create = readFileSync(new URL("app/groups/new/page.tsx", root), "utf8");
const createForm = readFileSync(new URL("app/groups/new/NewGroupForm.tsx", root), "utf8");

describe("multi-Agent Group chat pages", () => {
  it("renders frozen membership, attributed turns, wave-zero targets, and exact revision evidence", () => {
    expect(detail).toContain("client.read");
    expect(detail).toContain("projection.group.membership.members");
    expect(detail).toContain("projection.turns");
    expect(detail).toContain("projection.messages");
    expect(detail).toContain("author_agent_id");
    expect(detail).toContain("initial_targets");
    expect(detail).toContain("binding_revision_id");
    expect(detail).toContain("GroupCommands");
  });

  it("sends one explicit bounded fan-out wave without client-selected execution identity", () => {
    expect(commands).toContain('"use client"');
    expect(commands).toContain("recipient_agent_ids");
    expect(commands).toContain("concurrency_policy");
    expect(commands).toContain("hard_deadline");
    expect(commands).toContain("/turns");
    expect(commands).toContain('useState<string[]>([])');
    expect(commands).toContain("selected.length === 0");
    expect(commands).not.toContain("provider:");
    expect(commands).not.toContain("binding_revision_id:");
  });

  it("creates a Group from two to six stable Agent IDs only", () => {
    expect(create).toContain("createSignedFortControlAgentClientFromEnvironment");
    expect(create).toContain("NewGroupForm");
    expect(createForm).toContain("agent_ids");
    expect(createForm).toContain("/api/v2/groups");
    expect(createForm).toContain("selected.length < 2");
    expect(createForm).toContain("selected.length > 6");
  });

  it("renames, archives or reopens, and explicitly replaces an ordered membership", () => {
    expect(detail).toContain("GroupLifecycleControls");
    expect(detail).toContain("membershipRevisionID={projection.group.membership.id}");
    expect(lifecycle).toContain('action: "rename"');
    expect(lifecycle).toContain('action: archived ? "reopen" : "archive"');
    expect(lifecycle).toContain("expected_membership_revision_id: membershipRevisionID");
    expect(lifecycle).toContain("agent_ids: orderedAgentIDs");
    expect(lifecycle).toContain("moveMember");
    expect(lifecycle).toContain("orderedAgentIDs.length < 2");
    expect(lifecycle).toContain("orderedAgentIDs.length > 6");
    expect(lifecycle).not.toContain("binding_revision_id:");
    expect(lifecycle).not.toContain("behavior_revision_id:");
    expect(lifecycle).not.toContain("provider:");
    expect(lifecycle).not.toContain("machine:");
  });
});
