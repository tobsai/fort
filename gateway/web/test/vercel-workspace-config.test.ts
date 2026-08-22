import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

describe("fort-gateway Vercel workspace", () => {
  it("builds the web workspace from the gateway root with the committed lockfile", () => {
    const config = JSON.parse(
      readFileSync(new URL("../../vercel.json", import.meta.url), "utf8"),
    ) as Record<string, unknown>;
    const lockfile = readFileSync(new URL("../../package-lock.json", import.meta.url), "utf8");

    expect(config.framework).toBe("nextjs");
    expect(config.installCommand).toBe("npm ci");
    expect(config.buildCommand).toBe("npm run build --workspace=@fort/gateway-web");
    expect(config.outputDirectory).toBe("web/.next");
    expect(lockfile).toContain('"name": "fort-gateway"');
    expect(lockfile).toContain('"node_modules/@fort/gateway-web"');
  });
});
