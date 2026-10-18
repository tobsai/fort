# Primary-owner account service preparation

Spec 059, October 18, 2026. Source base c5364af. Adds explicit authenticated
GET /v1/account/sign-in and POST /v1/account/sign-in/start and /cancel. Only the
exact primary owner-local device may read or operate this optional capability.
Companions and temporary installations cannot use it. No status/startup/pairing
path automatically starts provider login.

A start stores operation ID, expected revision, exact device/recovery epoch and
absolute deadline before calling the runtime. One current metadata record supports
exact lost-response retry without repeated login or extended time. New requests need
a new ID and current revision. Restart turns unfinished work into unconfirmed.
Codes and URLs remain in memory; replay carries only an invalidation. Host stop,
disconnect and close abort and await the operation before confirming shutdown.
Completed sign-in races remain signedIn; cancel never implies logout or revocation.

Account work excludes dispatch, discovery and restart at the service boundary even
when a runtime has no readiness method or still reports ready. The neutral challenge/
result parsers moved to contracts/account.ts so core does not import provider code.
No dependency, listener, executable, credential, account or startup service was added.
The existing private channel accepts these authenticated /v1 paths without a new
transport. Native controls remain the next slice. Agent host and tool sandbox remain
separate boundaries; actual host network authority is still unverified.

## Test-first and review findings

The first fixture omitted its explicit provider descriptor; after correcting fixture
setup, the public HTTP test failed with 404 before implementation. It now passes.
Two subsequent public races reproduced sign-in during an existing catalog call and
ready/restart availability during sign-in. Service-owned operation exclusion fixes
both. Initial implementation used non-erasable TypeScript parameter properties and
a narrowed local type; both were corrected. A catalog fixture used an unsupported
state literal and was corrected before the final strict check. No failing final
check is omitted or called a provider result.

Semantic review covered authenticated routes, post-await auth, companion identity,
record persistence/recovery, deadlines, ephemeral challenge, lifecycle disposal,
shared runtime operations, executor parser consumers and private-channel routing.
Parser relocation keeps executable vendor modules out of core. No dead-code claim
or new whole-repository audit is made; prior Ponytail baseline remains preserved.

## Validation

- Full portable: 1,275 passed, zero failed, 12 documented skips, 35.288 seconds.
- Focused account service: 16 passed, zero failed, one opt-in budget skip.
- Guarded Mac executor/runtime/artifact: 35 passed, zero failed, three opt-in skips.
- Strict TypeScript: exit 0, empty final diagnostic log.
- Isolated HTTP benchmark: 20 serial start/cancel journeys after five warmups;
  p50 3.980 ms, p95 4.683 ms, maximum 10.844 ms; budget 100 ms. Synthetic runtime,
  temporary SQLite and loopback HTTP; no concurrent build/test during measurement.
  Provider/network/user interaction excluded. Node 26.5.0, TypeScript 6.0.3,
  macOS 27.2; prior hardware inventory MacBookPro18,2/10 CPU/64 GiB.
- Fresh /tmp/fort059-owner-account-host-resources: 108 files, 449,608 bytes;
  all hashes/sizes and exact membership independently verified. Manifest SHA256
  d7ff3a8a42b9078d353ed55d65ab351646a82f27190f8a96932e6220ccbadc79.

No actual Codex/account/browser/credential, network grant, startup registration,
signing, upload, installed native UI or TestFlight action ran. The existing unsigned
190-file review app was not replaced. Remaining: native durable intent/challenge/
cancel/status, exact Apple profiles and activation authority, real-provider Mac
acceptance and TestFlight delivery. Source and synthetic tests do not meet those gates.

## Local evidence

- `/tmp/fort059-owner-account-red.log` — SHA256 `d564cb88fee843ff6be09b1d548dba3d3f28b53a4f088e1c78988b9abb1d72dd`
- `/tmp/fort059-owner-account-first-green.log` — SHA256 `5b68cef339c52d5e90676b7dfcbe574ac8afa6349b32d54f5491d9932b76a7e6`
- `/tmp/fort059-owner-account-races-red.log` — SHA256 `fb583c5382b8a0b9689fd4a0fb05eebc7c9ab02223ebe49c8f61f6563b0a964d`
- `/tmp/fort059-owner-account-controls.log` — SHA256 `189330e19037d2494c74e44320c749dd0a1bc30f252bf1899edc295d58baa443`
- `/tmp/fort059-owner-account-portable.log` — SHA256 `944caf3f60f9c2fccea281155efec62b751f851a4be1cb4886121d83505a673e`
- `/tmp/fort059-owner-account-macos.log` — SHA256 `746c6a2baf7fe95313fe67253e39a131a8aead717f878c3db61535e16e7a9eb1`
- `/tmp/fort059-owner-account-typecheck-final.log` — SHA256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
- `/tmp/fort059-owner-account-budget.log` — SHA256 `8a2f7712c525caca8fb5af9e9ee29ddb3b509b5e570af74cebe910789de48c1b`
- `/tmp/fort059-owner-account-host-resources.log` — SHA256 `5c4d4b80f047aea3ef4f0fbd6295fde059cafc1d9055e4804f0b8e1b85910e22`
- `/tmp/fort059-owner-account-samples.json` — SHA256 `31c1b020b5cf51b9c5ef371e008b94a20265bb0aa51528066e34472dfcf067a0`
