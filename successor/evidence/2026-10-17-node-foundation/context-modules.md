# Removable local context compiler

Design: [Spec 059](../../../specs/059-modular-node-v1.md). This implements the
first optional module under the adopted Node design. It does not activate a
provider, account, startup service or external connector.

`contracts/context.ts` owns the versioned compiler contract. Composition may
omit a compiler, disable it with `null`, or supply a static implementation.
`modules/local-context.ts` prepares owner notes and complete reviewed text skill
bundles. It checks the exact selected provider instead of assuming Codex. It
grants no tools, credentials or external authority. Bundle contents remain task
data. The old `runner/skill-context.ts` implementation was removed after its
callers and behavioral checks moved to the new boundary.

The core and Codex adapter import contracts, not the optional implementation.
The prototype composition points explicitly select the local compiler. An
accepted context-bearing request pins its compiler ID/version. Dispatch requires
that same compiler, prepares immutable content once, and includes the content and
configuration digest in the sealed executor input. The Codex adapter consumes
prepared context and rejects a missing or mismatched configuration. It does not
silently rebuild context with another implementation.

The authenticated `/v1/modules/context` response distinguishes missing, disabled,
ready and failed state. Missing or failed preparation leaves ordinary text usable.
Requests requiring context fail explicitly. Retained notes, bundles, receipts and
messages stay readable. A compiler exception fails the affected turn, reports a
redacted module failure, wakes clients, and prevents further context-bearing work
until the module is restored. It does not block an explicitly context-free turn.

## Verification

The initial missing/disabled API test failed before the boundary existed, then
passed. The prepared-input test reproduced the missing handoff/version pin before
the service supplied it. The removal/re-addition journey initially failed before
the local compiler existed at its new path. Existing Codex and packet checks were
migrated to construct the new explicit input contract; no provider was started.

Local checks on October 18, 2026:

| Check | Result |
| --- | --- |
| Focused context, Codex and packet contracts | 415 passed before the final additional module checks |
| Module lifecycle, historical pin and independent-provider checks | 8 passed, 0 failed/skipped |
| Full discovered portable suite | 826 passed, 0 failed/skipped |
| Strict TypeScript | Passed with the final tests |
| Compiled native HTTP journey | Passed; ten synthetic executions, six service restarts, lost-response/cancellation reconciliation and no duplicate execution |
| Core artifact build and removal contract | Passed; 20 files, 149,588 bytes excluding manifest |

The artifact is `successor/.build/core-059-context-20261018`. Its manifest records
each file's bytes and SHA256. The earlier artifact is preserved. The compiled
artifact test asserts absence of optional module, Codex, prototype, donor and
dependency implementations, then runs both independent runtime contracts under
Node permissions that cannot read the repository. Node is supplied separately.
This is an isolated core artifact, not a complete native application.

Public module checks cover absent/disabled composition, prepared immutable input,
removal and re-addition without replay, redacted compiler failure, explicit plain
text after clearing a note, changed compiler version, a historical queued routine
without a compiler pin, and complete reviewed bundles for a non-Codex fixture.
The historical case edits only a stopped synthetic ledger to reproduce the old
configuration shape, then verifies behavior through restart and HTTP. It neither
modifies real history nor infers authority for old work.

## Limits

The local compiler is statically selected trusted code with synchronous local
work. This is not a mechanism for downloading arbitrary plugins or running an
untrusted compiler. Core storage retains the public note/bundle formats and their
validation so history, editing and export do not require the compiler. Module
removal does not delete those owner records.

Voice, connector operations and VM placement still need their own contracts and
scoped-authority/lifecycle proof. The executor's plain-text policy still contains
no external grants. This slice does not establish official-provider process
packaging, OS confinement, installed native presentation, performance budgets or
TestFlight/physical acceptance. Existing real-provider grants remain consumed.
Accepted runs pin their compiler. Saved routines currently select that compiler
at each new acceptance; pinning the routine revision for future scheduled runs
is the next versioning correction.

Follow-up: [routine revision pins](routine-context-pins.md) now cover that future
invocation case, with separate red/green and full-suite evidence.
