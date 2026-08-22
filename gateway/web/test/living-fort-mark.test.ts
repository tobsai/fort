import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const layout = readFileSync(new URL("../app/layout.tsx", import.meta.url), "utf8");
const styles = readFileSync(new URL("../app/globals.css", import.meta.url), "utf8");

describe("the persistent Fort product mark", () => {
  it("is a decorative orbital core beside the visible Fort wordmark", () => {
    expect(layout).toContain('<span className="fort-mark" aria-hidden="true"');
    expect(layout).toContain('className="fort-mark-core"');
    expect(layout).toContain('className="fort-mark-orbit fort-mark-orbit-a"');
    expect(layout).toContain('className="fort-mark-orbit fort-mark-orbit-b"');
  });

  it("moves continuously in ambient foreground state and keeps a slow non-spatial reduced-motion glow", () => {
    expect(styles).toContain("@keyframes fort-mark-orbit");
    expect(styles).toContain("@keyframes fort-mark-breathe");
    expect(styles).toMatch(/\.fort-mark-orbit-a\s*\{[^}]*animation:\s*fort-mark-orbit[^;}]*infinite/s);
    expect(styles).toMatch(/\.fort-mark-core\s*\{[^}]*animation:\s*fort-mark-breathe[^;}]*infinite/s);
    expect(styles).toContain("@keyframes fort-mark-reduced-glow");
    expect(styles).toMatch(
      /@media \(prefers-reduced-motion: reduce\)[\s\S]*\.fort-mark-core\s*\{[^}]*fort-mark-reduced-glow[^}]*infinite/s,
    );
  });
});
