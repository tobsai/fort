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


## Completed GPT-6.1 Sol smoke and local cleanup, October 20

The owner completed managed sign-in and confirmed the reported workspace, then
selected GPT-6.1 Sol. Accessibility inspection of the installed d8d76bc3 app showed
GPT-6.1-Sol in the saved picker, the successful connected-runtime catalog message,
and no pending account-confirmation action. No second sign-in was requested.

Two of the three approved synthetic no-tool prompts were reserved durably before
sending through the normal composer. Both produced the expected synthetic marker
and a saved reply. Normal foreground Command-Q/relaunch between them restored the
existing connection, saved model and first reply without setup or sign-in. The
closed durable ledger confirms both requested gpt-6.1-sol and completed:

- fdc595ea-5d2a-47b8-bfe0-330af6bb1069, request 78ed22e9-6662-41b7-93ea-3423895bf6f8.
- 06a47823-c404-4311-8d0d-b9376425e21d, request 05955090-c378-4c79-9d3a-d131ab34d3d0.

Both use binding efc53a69-561f-46bb-bae7-509b8121f31e. Both effectiveModel fields
remain null and the native transcript says it was not reported. This proves the
selected-model inference path, not an independent serving-model attestation. The
third allowance was unused; do not send it merely to exhaust the budget.

The normal Prepare to remove Fort confirmation completed local cleanup. AXPress
returned unsupported even though the action subsequently completed; reconciliation
found the dialog gone and the confirmed resulting state. No duplicate cleanup
was submitted. Independently at 13:25:36 and again 13:28:33 UTC, launchctl returned
113 for the exact startup job, no helper/guardian/Node/Codex process remained, the
provider auth file and client connection file were absent, and devices, enrollment
and outbox tables each contained zero rows. The client app remained open only for
inspection. Remote grants were not revoked.

The retained ledger contains one Agent, twelve Messages and six completed runs,
including owner activity outside these two prompts. Every original Message and run
matches the pre-migration backup byte for byte. The closed ledger SHA256 is
 e8005f96248cb27d48a74c446ab5f15f22baaf48e468dffc1f9b9ca59e68c561.
A live readonly SQLite probe could not acquire the exclusive application lock; it
was deferred. After shutdown, immutable readonly inspection of the checkpointed
ledger avoided creating sidecars. No credential contents or history bodies were
read into evidence. Raw bounded receipts are under
/tmp/fort059-runtime-migration-20261020/sol61-cleanup-result.json and prompt-budget.json.

## Exact-account preference and model refresh repair

The owner reported the redundant workspace-confirmation step at 13:02/13:03 UTC.
Spec 059 records the same-scope repair: retain one explicit choice bound to the
installation, recovery epoch, provider and host. A fresh unexpired, uncanceled
managed result may reuse it only when both account ID and user label match exactly.
A new explicit choice replaces the old one atomically. A valid legacy confirmation
can be retained across owner re-pairing within the same installation/recovery scope;
new pairing alone never authenticates the provider. Catalog and inference retain
independent runtime account verification. No additional credential store or route
was introduced.

Public service regressions cover matching reauthentication/restart, changed account
or workspace, missing/multiple identities, cancellation races, expiry, stale or
malformed preference scope, interrupted operations, explicit replacement choice,
and legacy owner re-pairing. The legacy test initially failed false versus true;
removing the obsolete device comparison from preference retention fixes it while
current operation authorization still requires the exact newly paired owner.

Native status reconciliation refreshes the catalog once after a newly confirmed
account, including a matched reauthentication; repeated status polls do not repeat
discovery. Refresh is disabled with a visible prerequisite while confirmation is
required. Unchecked saved models say not checked instead of unavailable. Changing
account confirmation to an unconfirmed state clears cached catalog eligibility,
including a lost sign-in receipt; a public native red/green test covers that case.
Saved Agent bindings and drafts remain unchanged.

Validation artifacts: /tmp/fort059-account-refresh-20261020. Final service: 37 pass,
two optional benchmark skips. Full portable: 1,417 pass, 16 optional skips. Strict
TypeScript check passes. The earlier full native run passed 344 tests with 16 skips;
final native/build results are recorded below after the lost-receipt correction.
Initial native attempts omitted required fixture variables, and the old compiled
host journey expected redundant confirmation after re-pairing. Correct fixture
inputs and a behavioral expectation for automatic exact-match reuse corrected
those failures. The attempted NSHostingView unit accessibility tree was empty, so
that invalid test approach was removed; native model tests and installed inspection
remain separate evidence. No production debug logging remains.

The two live turns above used the previously installed migration build. The account
repair uses synthetic service/native proof and a separately checked disconnected
installation. No new provider activation or sign-in is authorized by its installation.
TestFlight, physical iPhone, G2, independent effective-model reporting and remote
provider closure remain unverified.

Final source verification: full native 345 passed, 16 optional skips; fresh Release
Mac build passed. Full portable 1,417 passed, 16 optional skips; strict typecheck
passed. These include the final legacy re-pair and lost-receipt fixes. No additional
provider prompt or activation was used. Signed packaging and disconnected installed
checks follow as a separate evidence stage.


## Account repair installed while disconnected

Source commit 35882ab7d0aa1c969550c507080b1fbd3440ffef is now installed at
/Users/tobiasgunn/Applications/Fort.app. The reviewed package has 208 files,
418,411,073 bytes and 30 Mach-O binaries. Every file/hash/mode, every binary's
signature and development certificate, unchanged client/helper/child entitlements,
unchanged provisioning/configuration and the native MacHostBundle inspector passed.
The Codex 0.163.0 binary and its attribution remain the exact previously reviewed
inputs. No dependency, entitlement or provider path was expanded.

Signed report SHA256: 4e8440a0a565825c916423492ae5e24a0b0fed7b9d781c52d60dabe518e6f053.
Payload manifest SHA256: 99420bffa4082661ee956836e2bf00ed48381caab6e7adcf80a215eb8659d1ff.
A verified previous app is retained at
/private/tmp/fort059-account-refresh-20261020/previous-installed.app. Installation
used an atomic same-filesystem bundle swap after normal client Quit, with startup,
provider credential, pairing and all Fort processes verified absent. It did not
run a ledger migration or edit retained data.

The exact installed app passed native bundle inspection, normal launch, accessibility
inspection, a cropped setup-sheet screenshot and normal Command-Q while setup was
open. The screenshot shows the host is not enabled and removal was recorded complete.
Send was disabled, no provider sign-in action appeared, and no message element was
present in the captured view. The pre-existing duplicated removal-status sentence
is cosmetic and was not changed in this bounded account-selection slice.
Screenshot: /tmp/fort059-account-refresh-20261020/installed-disconnected.png,
SHA256 2f070b98551c1dd52e3acf8868df4c879bdaff559a16808021db7c031e8d031c.

After launch and Quit, all 208 installed files and signatures still matched. At
13:37:25 UTC there were zero Fort processes, no startup job, no local provider auth
or client connection file, and zero devices/enrollment/outbox rows. The complete
closed ledger SHA256 remained e8005f96248cb27d48a74c446ab5f15f22baaf48e468dffc1f9b9ca59e68c561.
Original history remains unchanged. Prompt use remains 2/3. No new sign-in, provider
call, activation, upload, push or merge occurred during this update.

Final package/install/UI/absence receipt:
/tmp/fort059-account-refresh-20261020/installed-final.json.
The new exact-account reuse behavior has synthetic service/native/compiled-host
proof; it was deliberately not reauthenticated against a real provider after
cleanup. This is the requested disconnected update, not new-source provider or
TestFlight acceptance. No blocker remains for this bounded slice. Independent
effective-model reporting, physical iPhone, TestFlight/G2 and remote provider closure
remain separate open gates.
