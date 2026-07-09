// The email allowlist is the whole authorization model of a personal gateway
// (spec 028, D1). These tests pin the exact decision the Auth.js signIn
// callback delegates to: on-list allow, off-list deny, and fail-closed.

import { describe, expect, it } from "vitest";

import { isAllowed, parseAllowlist } from "@/lib/allowlist";

describe("parseAllowlist", () => {
  it("splits, trims, lowercases, and drops blanks", () => {
    expect(parseAllowlist(" A@x.com , b@Y.com ,,")).toEqual(["a@x.com", "b@y.com"]);
  });
  it("returns [] for undefined/empty", () => {
    expect(parseAllowlist(undefined)).toEqual([]);
    expect(parseAllowlist("")).toEqual([]);
  });
});

describe("isAllowed (the signIn callback decision)", () => {
  const list = "owner@example.com, teammate@example.com";

  it("allows an allowlisted email (case-insensitive)", () => {
    expect(isAllowed("owner@example.com", list)).toBe(true);
    expect(isAllowed("Owner@Example.com", list)).toBe(true);
    expect(isAllowed("teammate@example.com", list)).toBe(true);
  });

  it("denies a non-allowlisted email", () => {
    expect(isAllowed("stranger@evil.com", list)).toBe(false);
  });

  it("fails closed: empty allowlist allows no one", () => {
    expect(isAllowed("owner@example.com", "")).toBe(false);
    expect(isAllowed("owner@example.com", undefined)).toBe(false);
  });

  it("denies a missing email", () => {
    expect(isAllowed(null, list)).toBe(false);
    expect(isAllowed(undefined, list)).toBe(false);
    expect(isAllowed("", list)).toBe(false);
  });
});
