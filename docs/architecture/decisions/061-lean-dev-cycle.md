# 061: Lean development cycle

**Decision:** Toby chose "All six cuts, one PR" on October 23, 2026, in the
Fort project thread "Leaner dev cycle and UI".

## What changed

1. The spec gate is retired for routine work. `ops/architecture/validate-specs.mjs`,
   its exact-path declarations and content digests are removed. A spec is needed
   only for billing/provider path, credentials, persistent access or destructive
   changes (see `specs/README.md`).
2. The Ponytail review evidence requirement (decision 060) is retired, along with
   `validate-reviews.mjs` and `docs/architecture/reviews/ponytail/current.md`.
   Code review happens on the PR.
3. CI runs typecheck and tests only. The Xcode job runs only when Swift, host or
   packaging files change.
4. Committed evidence is archived: `successor/evidence/` (except one pins file
   that `prototype/manual.ts` reads), the v1 ledger, the TestFlight checkpoint
   diary, the Apple host activation log and the dated handoff docs. PR descriptions
   carry results from now on.
5. `make -C successor beta` archives and uploads a TestFlight build.
6. `CLAUDE.md`, `AGENTS.md` and `successor/AGENTS.md` are cut down to the rules
   that still matter.

## What stays

No API-billing fallback. Toby's per-action approval for new credentials,
persistent grants, legal terms and destructive actions. Test-first at public
interfaces. Node/TypeScript core with Swift clients.

## Recovering archived material

Everything removed here is intact at commit
`a4152e8a` (`git show a4152e8:successor/V1-ACCEPTANCE.md`, or
`git checkout a4152e8 -- successor/evidence`). Specs 001–059 and earlier
decisions stay in place as history.
