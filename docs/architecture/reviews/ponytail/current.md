What this repo does: Fort is a single-owner native messenger with a Node core and
optional official runtime adapters. This review covers one design-only slice: a
Spec 059 revision and decision 059 entry that let Fort hold the credential for
OpenAI's "Sign in with ChatGPT" plan-usage route, assuming one owner.

Reviewed the two changed documents against ARCHITECTURE.md, Spec 058 section 9,
the existing Spec 059 credential sections (owner setup token handling and
Fort-owned provider credential removal), the runner adapter layout and the
provider literal in contracts/runtime.ts and service/synthetic-runner.ts. Read the
provider's current documentation and terms for the route on 2026-10-21. No source,
test, build or native file changes in this slice.

Must fix (resolved): the new section first omitted service/synthetic-runner.ts,
which also hard-codes the provider literal; it is now declared. The removal path
for the new credential file is not declared because it touches native storage;
the section states that a later path revision must add it.

Should fix (open): repository guidance outside specs (CLAUDE.md, successor/AGENTS.md
and team notes) still describes subscription use as CLI-only. Update them when the
adapter lands so agents do not refuse the approved route.

The narrowing is limited to one named route. Claude and Codex credential rules,
the no-substitution rule and per-action approval for real sign-in are unchanged.
The route is a provider preview; the section requires a terms recheck before
release and an unavailable capability if the route ends.

Verdict: ready to commit as design authority for fixture-only slices.
Not checked: any implementation (none exists), real sign-in or inference, native
sign-in UI, packaging, removal on a signed installation, TestFlight, and whether
OpenAI treats a self-registered personal client as an eligible open-source app in
practice. The first attended real turn is the evidence for that.

```fort-review
{
  "format": 1,
  "reviewedCommit": "0f64b37f37aff55dec3142fad38c5e133703b856",
  "reviewer": "Claude Code (orchestration architecture thread)",
  "scope": "Design-only slice: Spec 059 section 'Fort-held OpenAI plan sign-in credential — October 22', its implementation-path additions and refreshed digest, and the matching decision 059 entry. Reviewed against ARCHITECTURE.md, Spec 058 section 9 and existing Spec 059 credential sections. Single owner; no code, release or provider acceptance.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "node ops/architecture/validate-specs.mjs passes (structure only).",
    "node ops/architecture/validate-specs.mjs --implementation specs/059-modular-node-v1.md passes; digest refreshed to the new content.",
    "Not run: portable/native test suites and typecheck, because no source changed in this slice.",
    "Not run: any provider sign-in, inference, native UI or installed check."
  ],
  "exceptions": [],
  "findings": [
    { "id": "PT-OAI-1", "detail": "Provider literal is also hard-coded in service/synthetic-runner.ts; added to declared paths.", "disposition": "fixed" },
    { "id": "PT-OAI-2", "detail": "CLAUDE.md, successor/AGENTS.md and team notes still describe subscription use as CLI-only; update with the adapter slice.", "disposition": "open" },
    { "id": "PT-OAI-3", "detail": "Removal of the new credential file needs native storage paths in a later revision before any real credential exists.", "disposition": "open" }
  ],
  "files": {
    "docs/architecture/decisions/059-node-foundation.md": "bfa4410e7fef1ce0ab28c8f2184d7e7d93facbc13608f86c52db8197f816e74f",
    "specs/059-modular-node-v1.md": "2ad473ecc0da292036bc9558fe8e6b47c69ad815f3b42a1b9684b83b1f1e0085"
  }
}
```
