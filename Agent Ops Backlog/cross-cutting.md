# Cross-cutting (decisions)

Work that isn't phase-bound. Resolve these early — AO-090 unblocks the monorepo (AO-001) and everything after it.

---

### AO-090 · Decide Fort's language + module boundaries
- **Type:** decision · **Pri:** P0 · **Est:** S · **Labels:** decision → you · **Depends:** —
- **Do:** Pick the implementation language and how the modules split. **Recommendation:** Go for `core`/`exec` (strong concurrency; eases borrowing Multica's Go daemon), with `ui` in its own stack talking to `core` over local HTTP/WS — or all-Go if you prefer one toolchain. Confirm the `core`/`exec`/`ui` seam rules.
- **Acceptance:**
  - [ ] Decision recorded; AO-001 scaffolds in the chosen language with enforced module boundaries.

### AO-091 · Open-core / licensing decision
- **Type:** decision · **Pri:** P2 · **Est:** S · **Labels:** decision → you · **Depends:** —
- **Do:** Decide the license/business posture for Fort: open-core (OSS core + paid Pro/cloud) fits your existing OSS + Homebrew distribution and the market doc's freemium plan. One project makes this cleaner than three. Note any attribution required for vendored Multica (Apache-2.0) code (AO-003).
- **Acceptance:**
  - [ ] License chosen; README + repo headers reflect it; Apache-2.0 attributions in place if vendoring.
