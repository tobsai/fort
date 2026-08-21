import { describe, expect, it } from "vitest";

import {
  issueNativeToken,
  nativeSessionLifetimeSeconds,
  verifyNativeToken,
} from "@/lib/native-token";

describe("native gateway tokens", () => {
  it("keeps a native identity valid for 30 days", async () => {
    const token = await issueNativeToken("owner@example.com", "secret", 1_000);
    expect(nativeSessionLifetimeSeconds).toBe(30 * 24 * 60 * 60);
    await expect(
      verifyNativeToken(token, "secret", 1_000 + nativeSessionLifetimeSeconds - 1),
    ).resolves.toEqual({
      email: "owner@example.com",
    });
  });

  it("rejects tampering and expiry", async () => {
    const token = await issueNativeToken("owner@example.com", "secret", 1_000);
    await expect(verifyNativeToken(token + "x", "secret", 1_030)).rejects.toThrow();
    await expect(
      verifyNativeToken(token, "secret", 1_000 + nativeSessionLifetimeSeconds + 1),
    ).rejects.toThrow("expired");
  });
});
