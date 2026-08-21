import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/auth", () => ({ auth: vi.fn() }));

import { auth } from "@/auth";
import { POST as renewNativeSession } from "@/app/api/native/session/route";
import { issueNativeToken } from "@/lib/native-token";
import { requireSession } from "@/lib/session";

const mockedAuth = vi.mocked(auth);

describe("native bearer sessions", () => {
  beforeEach(() => {
    process.env.AUTH_SECRET = "native-secret";
    process.env.FORT_ALLOWLIST = "owner@example.com";
    mockedAuth.mockReset();
  });

  it("authorizes a valid allowlisted native credential without a browser cookie", async () => {
    const token = await issueNativeToken("owner@example.com", "native-secret");
    const request = new Request("https://gateway.test/api/machines", {
      headers: { Authorization: `Bearer ${token}` },
    });
    await expect(requireSession(request)).resolves.toBeNull();
    expect(mockedAuth).not.toHaveBeenCalled();
  });

  it("rejects invalid and no-longer-allowlisted native credentials", async () => {
    const invalid = new Request("https://gateway.test/api/machines", {
      headers: { Authorization: "Bearer invalid" },
    });
    expect((await requireSession(invalid))?.status).toBe(401);

    const token = await issueNativeToken("former@example.com", "native-secret");
    const removed = new Request("https://gateway.test/api/machines", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect((await requireSession(removed))?.status).toBe(401);
  });

  it("renews a valid allowlisted native credential", async () => {
    const token = await issueNativeToken("owner@example.com", "native-secret");
    const request = new Request("https://gateway.test/api/native/session", {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });

    const response = await renewNativeSession(request);

    expect(response.status).toBe(200);
    const body = (await response.json()) as { token?: string };
    expect(body.token).toBeTypeOf("string");
    expect(body.token).not.toBe(token);
    const renewedRequest = new Request("https://gateway.test/api/machines", {
      headers: { Authorization: `Bearer ${body.token}` },
    });
    await expect(requireSession(renewedRequest)).resolves.toBeNull();
    expect(mockedAuth).not.toHaveBeenCalled();
  });

  it("does not renew an invalid native credential", async () => {
    const request = new Request("https://gateway.test/api/native/session", {
      method: "POST",
      headers: { Authorization: "Bearer invalid" },
    });

    expect((await renewNativeSession(request)).status).toBe(401);
  });
});
