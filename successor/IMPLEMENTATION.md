# Spec 058 implementation record

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
No package installation or build hook ran. The successor uses no npm/Swift package
dependencies. iOS 17 / macOS 14 are source deployment minima, not a validated
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
conflicts require review; full cancellation/approval/recovery UI is still open.

Tests currently cover pending persistence and exact bytes, installation separation,
byte limit, insecure endpoint refusal, epoch retry gate, stale/foreign snapshot
handling, unknown states and shared Swift/TypeScript fixtures. The native HTTP
conformance executable enrolls two synthetic devices, sends text, awaits a durable
fixture reply, compares snapshots, closes/reopens the actual service, retries the
persisted request, replays events and verifies shared read state.

Local Mac and iPhone Simulator builds succeeded without signing. These are build
and fixture results; no physical iPhone, official provider, App Store/TestFlight,
notarization or release acceptance is implied. Stage 1 is not fully accepted until
both installed native UIs have been exercised for reconnect/restart/recovery.

## Gates before runtime expansion

Official Codex is the first candidate, but remains disabled. A fixture process or
Node permission model is not full OS isolation. Prove an enforced runner boundary
for filesystem, IPC/network, descriptors, sibling runs, host Keychain and credential
canaries before provider execution. Use an explicit environment allowlist and an
exact provider-owned config scope, fresh data root, no legacy discovery, and no
ambient API-key fallback. No app-driven login is allowed in this increment.

Remain open: official-provider feasibility and authorization; scoped owner approval
and cancellation; hostile-runner OS isolation; TLS enrollment on physical iPhone;
backup/restore fencing; richer replay/pagination/negative contract compatibility;
native error recovery/accessibility on exact installed devices; skills/routines,
schedules, APNs, self-hosted topology, distribution and full T01–T20 acceptance.
No scope reduction or architecture substitution is approved by these limitations.
