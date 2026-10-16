# Accepted: one real native Mac chat and scoped cleanup

The approved fourth attempt passed its **one-message native-chat test**. The UI
operator paired automatically, created Writer with `gpt-6-astra` selected, sent the
sole approved Writer Home sample with no skill, and observed **`cedar`** and
**Reply saved**. The retained ledger independently contains that exact input, reply,
receipt and completed run. Effective model is **unreported**, not attested.

| Identity | Exact value |
| --- | --- |
| Executed source | `916fd00e84a75250425d7cd152cfd15252a7c0e5` |
| Execution evidence | `ea37f2b4e5934c36303b7142f7eaf9dc4c9ef322` |
| Run | `9e75b401-1511-4483-b9ca-7e0031c23458` |
| Request | `9772a6a0-a99b-45b4-aeb6-a7fbc08c6f9f` |
| UI thread / turn | `01a1451b-81d7-76cf-8e5c-a85d7164201c` / `01a146a9-2d59-729b-8481-21afbc8eb16b` |
| Source list, 468 files | SHA-256 `3a398e49c4c7e2494f7969c1f772e5cf2fd2d322380c60343e45ad74b8a04a32` |
| App, five files | SHA-256 list `9c32247c3c77b1618515aec7faad3944826ec9e11d4ecc0d58483c8ecd23ad2b`; built from `de9e482bac99920d52a03b6904542a78226c5ba2` |
| Official Codex 0.157.1 | Executable SHA-256 `27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d` |

All execution source/app/runtime pins matched **before** the documentation update.
The sealed [execution packet](../2026-10-16-candidate-b-fourth/README.md), its approval,
pins and logs remain unchanged. Current documentation is not a new executed build.

## What was independently checked

[Read-only verification](verification.json) used SQLite `mode=ro&immutable=1` and
`query_only`, after confirming no WAL/SHM. The database passed `quick_check`; its
sole receipt matches the finished state exactly. There is one completed run, one
user message, one assistant reply, Writer Home, zero selected skills, and the exact
selected model. Database/state hashes, file set and mtimes stayed unchanged.

Cleanup: `finished`, `starts:1`, one receipt, `credentialCleanup:deleted`; zero
enrollment/device rows and closed enrollment; native pending is empty; connection,
enrollment and lock files are absent. [Process/port checks](process-checks.json)
confirmed no exact Fort app or pinned Codex process and no listener on port 60425.
Keychain deletion is reported by the controller and UI operator; this follow-up
did not read or operate on Keychain. All three earlier ledgers/states remain unchanged.

The start inferred from the credential deadline is 21:41:15.441 UTC; the final-state
mtime is 21:42:56.758 UTC, about 101.3 seconds later. This corroborates the operator's
within-five-minutes completion; it is not independent monotonic timing attestation.

[UI provenance](ui-observation.json) records the completed operator turn and successful
accessibility/screenshot call. Visible results are attributed to that operator;
pixels were not independently reinspected or the journey reenacted here.

This accepts automatic local pairing, one native Mac send/save/reply, correlation
and scoped cleanup for these exact pins. It does **not** complete Spec 058, prove
effective model or full isolation, or accept real-provider restart/multiple-turn
reliability, iPhone/web use, distribution or unattended operation. The same-user agent
host boundary and pre-opened descriptor limits remain. All four attempts are consumed.

## Smallest practical owner trial — proposal only

Use this existing Mac app for **one ten-minute session**, including setup and cleanup,
with **up to five owner-chosen plain-text messages** in one Writer Home. Reserve the
last minute for cleanup. Keep exact Astra/existing ChatGPT sign-in, no skill or computer
tools, no-files profile, empty environments and every startup/failure check.

Necessary preparation is narrow: a fresh one-start controller packet admitting bounded
user text, automatic local pairing, and archival of the completed native candidate
cache so freshness checks can pass without deleting history. Preserve the fourth
ledger/state/cache. The same Mac-user host access, temporary loopback service, scoped
native Keychain enrollment and transmission of trial text/context to the existing
provider account are the requested access scope. Cleanup removes only new trial
authorization/connection material and retains history.

This changed duration/message/start/enrollment/archive scope needs owner approval;
[the exact proposal](next-user-trial-proposal.json) is inert. No runtime restart,
message, enrollment, cache move or permission change occurred in this follow-up.
The fourth packet cannot be reused. Broad hardening is not proposed as a prerequisite
to this explicitly bounded trial; isolation and release gates remain open.

Documentation-only validation: receipt/pin/residue assertions, file preservation,
process absence and whitespace checks. No new runtime, UI, provider, build or test-suite
execution was needed. The prior 434-test preparation result remains historical evidence.
