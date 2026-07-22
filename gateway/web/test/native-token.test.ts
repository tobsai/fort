import { describe, expect, it } from "vitest";

import { issueNativeToken, verifyNativeToken } from "@/lib/native-token";

describe("native gateway tokens", () => {
  it("round-trips an allowlisted identity with a bounded lifetime", async () => {
    const token = await issueNativeToken("owner@example.com", "secret", 1_000);
    await expect(verifyNativeToken(token, "secret", 1_030)).resolves.toEqual({
      email: "owner@example.com",
    });
  });

  it("rejects tampering and expiry", async () => {
    const token = await issueNativeToken("owner@example.com", "secret", 1_000);
    await expect(verifyNativeToken(token + "x", "secret", 1_030)).rejects.toThrow();
    await expect(verifyNativeToken(token, "secret", 1_901)).rejects.toThrow("expired");
  });
});
