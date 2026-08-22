import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const page = readFileSync(new URL("../app/agents/new/page.tsx", import.meta.url), "utf8");

describe("new Agent closed state", () => {
  it("explains the eligible-option gate without exposing execution controls", () => {
    expect(page).toContain("No eligible execution source");
    expect(page).toContain("opaque eligible option");
    expect(page).toContain("Provider and machine choices never come from this page");
    expect(page).not.toMatch(/<input[^>]+(?:provider|model|machine|adapter|authority)/i);
    expect(page).not.toContain('fetch("/api/v2/agents"');
  });
});
