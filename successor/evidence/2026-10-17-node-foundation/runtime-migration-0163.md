# Explicit runtime migration to Codex 0.163.0

Scope: accepted [Spec 059](../../../specs/059-modular-node-v1.md), “Explicit offline
runtime migration and attended 0.163.0 verification.” The coordinator relayed the
owner’s October 20 12:03:35 UTC approval of its 04:11:59 proposal in thread
01a1572d-841d-76d3-949d-a8dd3328d037. That permits this migration/install, normal
managed device-code sign-in to the same personal ChatGPT account, at most three
further synthetic prompts, paired Settings/Cancel/Quit and persistence/recovery,
then removal of local startup/pairing/provider credentials while keeping app/history.
Remote grants are not revoked. No alternate authentication, credential extraction,
permission expansion, provider cleanup, upload, push or merge is authorized.

## Source and fixture checkpoint

The explicit core export/CLI accepts an exact installation, recovery epoch, owner,
provider, host, source/target version and operation ID. Normal startup still refuses
a changed runtime. It requires a private owned ledger, known schema, exclusive lock,
disconnected owner, no active pairing/enrollment, no unfinished setup, no unresolved
run/outbox work, and paused schedules. It changes current agent revisions and runtime
metadata atomically, preserving historical bindings, messages, receipts, runs,
models and account selection. Old bindings are disabled for future pinned use.

The bounded local journal contains reversible Agent configuration and operation
metadata, not message bodies or credentials. Exact retries create no extra revisions.
Rollback checks the complete logical post-migration ledger digest before restoring
old rows/schema. Later changes refuse rollback; a stale backup must never replace
newer history. Current explicit history deletion accepts the optional journal while
retaining Agent configuration. No new normal startup/request route is added.

Observed red → green at the public API/CLI: implementation absent; old history
deletor rejects the new journal; unfinished setup/missing run state initially pass;
compiled core omits migration export. Each now has a passing behavioral regression.
Tests cover exact historical bytes, models/enabled state, old-runtime refusal,
new-runtime public reopen/submission, foreign identities, malformed paths/schema,
exclusive locks, pending work, retries and rollback. Four process-crash cases kill
the public CLI immediately before/after COMMIT through a test-only preload and prove
recovery through retry. Production code has no failure-injection switch.

Validation:

- Focused migration/history/package suite: 80 passed, two optional benchmark skips.
- Strict typecheck: passed. Full portable: 1,398 passed, 16 optional skips, zero failed.
- First portable run: 1,397 passed, one failed, 16 skipped. The measurement harness
  omitted package.json from its temporary incremental source tree, so TypeScript
  classified the new import.meta CLI as CommonJS (TS1470). Copying the existing
  package descriptor repairs that fixture. Final full suite includes its passing
  smoke/overwrite test; no package script is executed by copying the descriptor.
- Synthetic maintenance sample: 10,002 messages, 55.996 ms against the five-second
  budget; Node 26.5.0, arm64 Apple M1 Max, Darwin 27.2.0, one warm freshly written
  fixture, no provider latency. One sample is not a production percentile benchmark.
- Fresh host resources and unsigned 204-file review app assembled with the reviewed
  native inputs, exact 0.163.0 binary and unchanged attribution. Signing/install are
  the next separate gates.

The source SHA for Codex remains 83086188ae0614bd882818ff1aa23f3194962012; original
binary SHA256 8813367285524ac798dd96d1b071aea30d58708c7226fd236cd275cf1cfdee31.
The previous [offline/legal evidence](codex-0163-offline-update.md) remains applicable.

## Read-only installed preflight

All 206 installed files matched the Quit-fixed signed report
5d24d0c8e3595334e2bb0b0becb10b84d698b7516eb25c22f4802de0017b98f8. No owned process,
startup job, provider credential file or active client connection was found. The
closed ledger has one Agent, two historical bindings, six Messages and three completed
runs, zero devices/enrollment/outbox, and disconnected owner intent. Current binding
is still 0.157.1/gpt-6-astra; migration must preserve that model until an explicit
native selection. Ledger bytes: 114,688; SHA256
 db2980b2e5f5a93f84fe2124a2a3b4d76fb1f962401a8689aa4a6716851c0892.
Only identifiers/counts/states were read into evidence; no historical message body
or credential was printed. System Python’s SQLite read-only open failed; the pinned
Node SQLite reader succeeded without a permission change or write.

Raw source-test/preflight artifacts: /tmp/fort059-runtime-migration-20261020.
At this checkpoint the real ledger/app are unchanged, no sign-in/activation occurred,
and zero of the three prompts were used. Installed/native/provider acceptance,
effective-model reporting, physical iPhone, TestFlight and G2 remain unrun here.

## Installed update and owner sign-in checkpoint

Source commit d8d76bc3. The signed candidate contains 208 files and 30 Mach-O
objects. Every file, signature, certificate, entitlement and native bundle check
passed. Signed report SHA256:
b0d2151d36d53ca947e5dc79f70a0b156dad1366772b99ef6cdbe4e5f287c623.
Payload SHA256: 61572624b555b8897f73babf7fbcfce9db1ebc2987d0feeb171802756f1df700.
Signing uses the existing identity/profiles and unchanged entitlements. Normal
sandboxed signature verification lacked system trust access; the same read-only
verification passed with the supported escalation. No permissions were broadened.

A private closed copy of the real ledger passed apply/retry/exact logical rollback
and repeated rollback. Original schema/rows were restored on that copy. The real
migration then applied once: one current Agent revised, two prior bindings disabled.
The complete old installed app and closed ledger were preserved before an atomic
same-filesystem bundle swap. Independent post-migration comparison confirmed all
historical rows and all unrelated metadata exactly preserved. Current model stayed
gpt-6-astra; only runtime version became 0.163.0. Post-migration ledger SHA256:
2af02c82dd93012983cde2ded7493576c4fd39eda20ebc996624070b575d95a6.
After subsequent setup writes, do not restore that old backup over current history.

Normal installed setup enabled the local host and connected this Mac. Settings →
Pairing and local host opened. One initial dialog disappeared between checks and
Stop state was observed; the initiating actor was not established. Explicit Resume
succeeded. A bounded observed flow then opened the stop confirmation, verified both
Cancel/Confirm, pressed Escape and confirmed cancellation with pairing intact.
Direct AX Cancel returned an unsupported-action diagnostic and is not counted as
passing. Normal foreground Command-Q with the Pairing sheet open succeeded; reopening
restored the paired workspace without setup. A screenshot was visually inspected;
it included dimmed conversation content behind the sheet, so the raw image was
removed instead of retained as evidence. Menu enumeration was also narrowed to menu
controls after it included row descriptions. No transcript is copied into this note.

Fort’s normal managed device-code sign-in now displays its challenge; the official
verification link was opened for the owner. The owner must complete it using the
same personal ChatGPT account and code visible in Fort. No code/token was copied.
Zero of three prompts used. Exact 6.1 selection/inference and final cleanup await
that owner step. The app and local host are currently running/paired for this
pending flow; startup, pairing and any newly created local provider credential
must be removed after smoke. Remote grants are not revoked by local cleanup.
No TestFlight/G2/upload, push or merge occurred. Raw status and durable prompt budget
are in /tmp/fort059-runtime-migration-20261020/attended-state.json and prompt-budget.json.
