# Spec 058 implementation record

## Current implementation — October 17

The owner requested continued app testing and implementation through the MVP on
October 16 at 23:47:34 UTC. [MVP-20261017.md](MVP-20261017.md) is the active milestone
and remaining-gate record. Routine local work proceeds under that request.

Recovery source `7c3287298bb0ee52c2eab8054465b9fdb9119b5f` adds authenticated
queued/active cancellation and explicit unknown closure to the actual OpenAI
service, exposes the native controls there, and preserves runtime interruption
truth. Synthetic evidence now covers service/client restart, lost acceptance,
idempotent recovery and same-Home continuation. It is recorded separately from
installed UI and real-provider acceptance.

Bundle source `9d4aaef92630698b79a2a7a4885ea2dc8f46172f` adds complete inert
bundle import, immutable file/digest revisions, native review/enable/disable/export,
and fail-closed current-runtime compatibility. The full synthetic suite now passes
631 TypeScript tests and 26 Swift tests, including the expanded native HTTP journey;
fresh Mac and iPhone Simulator builds passed. See the
[bundle milestone](evidence/2026-10-17-mvp-bundles/README.md). Complete runtime
packaging, historical instruction-editor migration and installed signoff remain open.

Both owner-trial packets are consumed: the first failed before READY; the
replacement opened, received zero owner messages and finished cleanup. All six
historical packets are retained. The following October 16 account and proposals
are historical; the active plan does not reuse or reset them.

## Historical native-provider progress — October 16

The approved fourth packet completed one actual native Mac send/reply through the
pinned official Codex runtime and existing ChatGPT sign-in. The UI operator paired
automatically, created Writer with `gpt-6-astra` selected, sent the sole approved
Writer Home sample without a skill, and observed `cedar` plus **Reply saved**.
The exact receipt/run and saved reply were independently checked through an immutable
read-only SQLite connection; execution source/app/runtime pins matched before this
documentation update. The packet has one consumed start, one completed receipt and
reported credential deletion. Zero service enrollment/device rows, empty native
pending, removed connection/enrollment/lock files, and app/runtime/listener absence
were independently confirmed. The ledger/state were unchanged by verification.

Execution source `916fd00e84a75250425d7cd152cfd15252a7c0e5`, evidence
`ea37f2b4e5934c36303b7142f7eaf9dc4c9ef322`; see the
[acceptance record](evidence/2026-10-16-fourth-native-acceptance/README.md).
This accepts the one-message native Mac path and scoped cleanup only. Effective
model was not reported; full isolation, whole-spec completion, iPhone/provider
signoff, durable restart behavior with the real provider, and distribution remain
open. All four packets are consumed and must be preserved without reset or restart.

The practical next increment is a separately approved short owner trial in the
existing Mac app: proposed ten minutes total, five plain-text messages maximum in
one Writer Home, and one minute for cleanup. It requires a fresh one-start packet,
automatic local pairing, preserved archival of the completed native cache, and a
narrow controller change from literal sample text to bounded user text. Existing
ChatGPT/Astra selection, no-tools permissions and same-user host limits remain.
This bounded trial uses the existing app/runtime and current access scope; broader
isolation and release work remain separate. This documentation authorizes no live action.

Toby approved “Begin implementing spec 58” on 2026-10-12 at 02:43 UTC, recorded
in the delegated request with reference Sentinel_716f9d1af39c8191b02a147d20a17e46.
This authorizes local product code, native project scaffolding, builds/tests,
necessary in-scope fixes and local commits. External deployment, push, provider
sign-in, credential copying, new OAuth grants, paid services, public ingress,
startup services and unrelated teardown remain outside this increment.

## Stage 0 selection

All new product files live under `successor/`. Existing root Swift/Go code was
inspected as historical evidence and is not imported. The donor stays quarantined.
The first slice changes `contracts/`, `service/`, `runner/`, `apple/`, this record,
the local instructions and README. No donor file was copied: this slice is newly
authored from Spec 058. Its extraction list is empty; upstream Apache LICENSE,
NOTICE, quarantine manifest and source archives remain untouched. Any future
copied protocol logic needs its own path/hash/license extraction entry.

Installed tools selected: Node 26.5.0 at `/opt/homebrew/opt/node@26/bin/node`,
Node's built-in SQLite and TypeScript stripping, Xcode 27.0 (27A266a), Swift 6.4.
The first increment installed no packages. The follow-up installed pinned development
checks only: TypeScript 6.0.3, @types/node 26.6.5 and its undici-types 8.9.0 dependency.
Registry metadata/hooks were inspected first; installation used empty npm user/global
configuration and `--ignore-scripts --no-audit --no-fund`. No lifecycle hook ran.
Production still uses no external npm or Swift package dependencies. iOS 17 / macOS 14 are source deployment minima, not a validated
physical-device support matrix. Local checks use arm64 Mac and the iPhone 18 Pro
iOS 27 Simulator. The owner's actual physical-device matrix remains unverified.

The service is an explicit temporary loopback fixture, with a fresh named data
root and no startup registration. It cannot serve a physical iPhone until a
separately configured authenticated TLS endpoint is available. Native clients
permit HTTPS and an explicitly selected loopback HTTP fixture only. Each device
has separately scoped service auth; real native device credentials use Keychain.
Fixture HTTP test credentials are synthetic, temporary and deleted after testing.

Astra was explicitly selected with `gpt-6-astra`, high reasoning, for backend,
contract and runner implementation. Tool configuration is observable; no runtime
model attestation is claimed. Native code was implemented concurrently by the
coordinating agent. No external agents were contacted.

## Stage 1 evidence and limits

The service accepts exact UTF-8 into one SQLite transaction with immutable binding,
request receipt, message/run/outbox/events. It exposes a small authenticated v1
API. A visibly labeled fixture runtime supplies deterministic reply evidence.
This proves no official provider or subscription support.

FortKit is newly authored: native wire decoding, persistent immutable submissions,
installation-scoped local cache, Keychain credential storage, HTTP transport and
snapshot reduction. Both SwiftUI clients share the conversation UI. The composer
persists before networking, retains the original ID on retry and will not resend
under a different recovery epoch. Unknown run states remain informative. Pending
conflicts remain visible while history refresh continues. Accepted snapshots persist
before pending records are removed; cold offline launch restores installation-scoped
Agent/Home selection, cache and pending work before networking. Every HTTP redirect is
refused, including enrollment and authenticated reads.

The fixture now supports exact-bound owner approvals, persisted decision winners,
queued/active cancellation, and explicit closure of unknown outcomes without retry.
Parked deterministic fixture replies can continue after restart with a new fenced
generation; this is not proof of official-provider continuation. SwiftUI exposes
these controls, with an explicit uncertainty confirmation before closure.

Tests currently cover pending persistence and exact bytes, installation separation,
byte limit, insecure endpoint refusal, epoch retry gate, stale/foreign snapshot
handling, unknown states and shared Swift/TypeScript fixtures. The native HTTP
conformance executable enrolls two synthetic devices, sends text, awaits a durable
fixture reply, compares snapshots, closes/reopens the actual service, retries the
persisted request, replays events and verifies shared read state.

Local Mac and iPhone Simulator builds first succeeded without signing. Installed
Simulator tests now use explicit local ad-hoc signing to access Keychain; this is
configured only for the Simulator SDK. No account or signing certificate was added. These are build
and fixture results; no physical iPhone, official provider, App Store/TestFlight,
notarization or release acceptance is implied. Stage 1 now has passing installed fixture evidence for reconnect/restart/recovery
on both native clients; see the attended acceptance record below.

## Gates before runtime expansion

Official Codex is the first candidate, but remains disabled. A fixture process or
Node permission model is not full OS isolation. Prove an enforced runner boundary
for filesystem, IPC/network, descriptors, sibling runs, host Keychain and credential
canaries before provider execution. Use an explicit environment allowlist and an
exact provider-owned config scope, fresh data root, no legacy discovery, and no
ambient API-key fallback. No app-driven login is allowed in this increment.

Remain open: official-provider feasibility and authorization; official-provider approval
and cancellation; hostile-runner OS isolation; TLS enrollment on physical iPhone;
backup/restore fencing; richer replay/pagination/negative contract compatibility;
native error recovery/accessibility on exact installed devices; skills/routines,
schedules, APNs, self-hosted topology, distribution and full T01–T20 acceptance.
No scope reduction or architecture substitution is approved by these limitations.

## Independent review follow-up

Astra reviewed fixed commit `f2a520660d17b84a743ab8de4a447aaeb573a663` and reproduced
four P2 defects. `d8085f2` fixes late device revocation/expiry with transaction-bound
authority checks across all six owner mutations. `86137fb` fixes native redirect
forwarding, cold offline navigation/cache restoration, and acceptance persistence.
Regressions first reproduced the failures, then passed. The new record is
[evidence/2026-10-12-review-followup.md](evidence/2026-10-12-review-followup.md).
These commits also add bounded fixture controls and strict TypeScript verification.

A second independent review of `ee150e0` confirmed the four earlier fixes and
reproduced two new P2 defects. `c48e970` wakes dispatch after an expired approval
decision commits, and fences every native owner mutation when the installation
contract is unsupported. Tests first reproduced both defects, then the combined
46 Node / 18 Swift suites and strict TypeScript checks passed. The additional
negative wire fixtures do not claim old-installed-binary upgrade acceptance.

The current [T01–T20 map](evidence/2026-10-12-spec058-matrix.md) records each
passing subset and open gate. [Mac signing and native evidence](evidence/2026-10-12-mac-and-contracts.md)
records the separately authorized use of one existing matching development
identity, without new credentials, provisioning or security exceptions.

The [installed iPhone closure and race follow-up](evidence/2026-10-12-iphone-closure-and-race.md)
adds a passed two-Simulator approval race and uncertainty recovery journey.
`d044da9` retains a confirmed closure across background refresh, with a failing-then-
passing public HTTP regression and 19 passing Swift tests. `b99d89e` records the
installed test harnesses. The earlier preserved Mac build predates this model fix; the separate current
build below includes it. Independent review of `986afd3` reports no new actionable findings.
The [current-source Mac build](evidence/2026-10-12-mac-current-build.md) now provides
separate signed app/test artifacts for that exact source, with verified signatures,
unchanged entitlements and an owner-attended command. That preparation step launched no UI. The subsequent
[owner-attended Mac verification](evidence/2026-10-12-mac-attended-stage1.md) passed
recovery, approval/cancellation and uncertainty closure against the byte-identical
`986afd3` app, completing the Stage 1 native fixture exit. `8cfc9ca` fixes only a
Mac test selector that matched both the dialog and Touch Bar Cancel buttons.
The corrected runner passed; all broader provider, physical-device and release
gates remain open, and no provider execution is enabled.

## Stage 2 protocol preparation

`f6a968e` adds an unwired, in-memory Codex turn adapter and private runtime evidence
types, based on current official protocol documentation and synthetic fixtures.
It copies no donor/provider source. It preserves exact input/attempt correlation,
bounded framing, final versus provisional output and reported message phases;
unsupported authority/tool requests, rerouting and uncertain outcomes fail closed.
All 21 new tests and the complete 67-test Node suite pass, along with strict
TypeScript checking. Red failures, source/log hashes and exact limits are in the
[protocol evidence record](evidence/2026-10-12-codex-protocol.md).

The adapter does not launch or authenticate a runtime, initialize a thread, read
provider configuration, implement runner enrollment, or commit provider events.
Correlation metadata and requested read-only policy are not authentication or OS
isolation. The service/public/native contract remains fixture-only and unchanged.
Candidate CLI symlink metadata suggests 0.157.1; the binary was not executed and
its protocol compatibility is unverified. Stage 2 still requires enforced OS and
credential confinement, authenticated/fenced runner integration, exact-runtime
conformance and provider-native feasibility/real-turn evidence. See the
[enclosing-boundary requirements](runner/README.md). No new owner intervention is
needed for further deterministic work; provider activation remains gated.

## Synthetic runner-to-ledger increment

After independent acceptance of `f6a968e` / `43d0ca8`, the parent authorized a
backend-only synthetic bridge and crash/replay tests. `1804345` implements that
bounded slice: a persisted fresh-ledger fixture mode, separately scoped runner
sessions and dispatch capabilities, immutable accepted binding snapshots,
transaction-bound event authentication, exact turn/generation/model/runtime
fencing, canonical replay receipts and atomic terminal persistence. No donor or
provider source was copied and no public/native contract was expanded.

The deterministic transport state machine verifies the dispatch's text digest,
orders writes before close actions and bounds handshake, turn, interruption and
teardown deadlines. Actual temporary fixture service/runner processes were killed
at recorded boundaries. Unknown outcomes retain the barrier; only committed terminal
evidence or explicit owner closure releases eligible work. Session credentials
cannot act as owner credentials, and IDs/text alone authorize nothing.

`611521e` adds the final concurrent-terminal and first-cancel race verification.
The complete backend suite passes **116 Node tests**, including the prior 67,
41 bridge tests and 8 transport tests. Strict TypeScript checking passes. Source
hashes, raw red/green logs, command outcomes, crash cases and limits are in the
[synthetic bridge record](evidence/2026-10-12-synthetic-runner-bridge.md).
Native/Swift validation was deliberately not rerun in this backend-only increment.
No UI session, real provider invocation, credential read/copy, login, persistent
permission, deployment or push occurred. Stage 2 remains incomplete; production
host enrollment/TLS, enforced OS isolation, actual process I/O/deadlines and exact
runtime/provider/native feasibility evidence remain open.

## Bounded synthetic process I/O and confinement experiment

The next backend-only increment adds a fixed synthetic process supervisor with
bounded pipe writes, output and evidence queues, explicit environment, separate
cancellation/upload handling, and observed owned-group cleanup before terminal
evidence. Authenticated bridge tests preserve exact text, acknowledgement replay,
owner cancellation, restart fencing and the existing uncertainty barrier. The
service still enables no official runtime; public/native contracts are unchanged.

Temporary C syscall canaries test synthetic actor boundaries outside Node's
permission checks. The restrictive macOS profile aborts; a separate allow-default
diagnostic profile proves only its named denials. An inherited descriptor bypasses
a path denial, and supervisor death can leave its worker running until an outer
owner reaps it. Neither result is production isolation. `sandbox-exec` is deprecated;
no supported enclosing arrangement, provider credential boundary or crash-proof
process container is accepted. See the [exact results and remaining gates](evidence/2026-10-12-process-confinement.md).

## Process review correction and isolation proposal

The process review found a P2 race: allowed slow evidence retries paused stdout
long enough for the root-exit timer to discard an already-emitted terminal. Stdout
now parses bounded chunks synchronously, independently of the ordered upload
queue. Existing timeouts, caps and cleanup-before-terminal semantics remain.
The default-limit reproduction and combined progress/EOF/ledger regressions pass;
see the [correction evidence](evidence/2026-10-12-process-backpressure-fix.md).
The [Linux VM decision proposal](design/2026-10-12-managed-linux-isolation.md) pins
a candidate environment and future approval scope using read-only host/public
metadata. Nothing was provisioned; OS confinement and providers remain gated.
