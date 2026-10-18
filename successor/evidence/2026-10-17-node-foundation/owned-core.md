# Production owner-local core composition

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing
[Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
October 18, 2026. Synthetic runtime, tokens, identities, private storage and child
processes only. No provider/account access, persistent credential, startup registration,
profile, entitlement, ordinary-app activation or external service change occurred.

`serveOwnedCore` is the production composition entry for the private supervised core.
It requires an explicit runtime implementation/descriptor, existing absolute private
data directory, and input/output streams. The directory must belong to the current
user, have no group/other permissions, be a real directory rather than a symlink,
and resolve to the process's working directory. Preflight does not create a directory,
ledger or repair permissions. Invalid/missing runtime implementations fail before
creating a ready host. No provider, fake runtime or optional module is selected here.

The function composes the existing transactional core, `core.sock` and bounded pipe,
including owner setup/cancel/recovery. It opens no TCP listener, issues no enrollment
and creates no device token. Pairing defaults to no expiry, with an explicit optional
policy. Ordinary restore respects persisted stop/disconnect/cancellation intent;
resume is an explicit caller choice and retains the core's existing safeguards.

The call owns a valid supplied runtime. EOF, abort, malformed input and startup
failure await core shutdown. When construction never finishes, it disposes the
transferred runtime directly. Failed disposal remains a failure. A hung runtime may
still need the native guardian's process deadline; this function does not supply a
new timer or claim success without cleanup. History, startup registration and app
removal remain outside this function. Canonical App Group resolution, signed bundle
configuration and exact provider grants belong to the eventual native composition.
Agent-host access and computer-tool sandbox access remain distinct.

The core artifact now exports `./owned-host` with JavaScript and declaration files.
It still omits provider implementations, optional context implementations, donor and
prototype code. The existing Swift guarded-launch function now executes this entry
through a synthetic runtime fixture. The fixture alone owns its echo behavior, PID
marker, bounded lifetime and control messages; none is a production default.

## Verification

Preserved failures and fixes:

- Initial source test failed because the production entry did not exist:
  `/tmp/fort059-owned-core-red.log`; first green is `...-first-green.log`.
- JavaScript could initially omit the runtime and still get a ready core. The
  regression is `...-runtime-red.log`; explicit runtime validation passes in
  `...-runtime-green.log` before any ledger creation.
- The expanded source journey passed while the artifact lacked the new entry:
  `...-boundaries-red.log`, five passed and one failed. Adding it to core compilation
  then exposed an optional endpoint type error in `...-compile-red.log` and
  `...-typecheck-red.log`. Explicit narrowing fixed that without a type assertion.
- Seven final composition tests passed in `...-startup-boundaries.log`.
- Strict TypeScript checking passed in `...-typecheck-pass.log`.
- **900 portable tests passed with zero skips/failures** in
  `/tmp/fort059-owned-core-portable.log`.
- **166 native functions executed and passed, with one existing startup timing
  function excluded**, in `/tmp/fort059-owned-core-swift.log`. Swift Testing reports
  167 registered functions. The explicitly enabled guardian factory test uses the
  current compiled production entry, confirms exact recovery and child/group absence,
  and rejects a missing guardian without fallback.

The composition checks cover exact Unicode/whitespace send and replay without a
second execution, retained pairing/history across EOF, stopped restore versus explicit
resume, no fixture/control HTTP route, unauthenticated refusal, pending setup cancel,
exact recovery/foreign rejection, abort, malformed input, failed runtime cleanup,
pre-aborted/preflight disposal, wrong working directory, shared permissions, symlink,
file, absent/relative path and preservation of an unknown socket path. A blocked
socket startup closes its ledger/runtime; an explicit later reopen succeeds after
the test removes only its own known marker.

Current retained core:

- `successor/.build/owned-core-059-20261018`.
- 30 files, 190,826 payload bytes.
- Manifest SHA-256 `c1a19bd8e8f0def68efa45f2b913dc5b2123466a3da6a5c14ce0eb8fded7f527`.

Native command:

```sh
FORT_TEST_GUARDIAN=/tmp/fort059-mac-core-host-guardian \
FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node \
FORT_TEST_CORE_ARTIFACT="$PWD/successor/.build/owned-core-059-20261018" \
swift test --package-path successor/apple/FortKit \
  --scratch-path /tmp/fort059-owner-setup-xpc-native
```

## Startup measurement

MacBookPro18,2, M1 Max, 64 GiB, 10 CPU; macOS 27.2 (26B5112a), Node 26.5.0,
TypeScript 6.0.3, Xcode 27.1 / Swift 6.4. Five serial compiled-artifact journeys use
five fresh ledgers and fifteen reopens: normal, stopped and explicitly resumed.
Binary/filesystem caches are warm and were not purged. Timing covers child spawn,
Node startup, production composition and its ready pipe frame. Build, initial pairing,
message operations and provider latency are excluded. This is a direct isolated
child measurement, excluding native guardian/XPC/ServiceManagement/Keychain and UI.
No other tests or builds ran during measurement.

Each run used `FORT_OWNED_CORE_BENCHMARK=1 node --test` with
`--test-name-pattern='compiled artifact runs the owned core journey'` and
`successor/host/owned-core.test.ts`. The fixture asserts the existing two-second
fresh and 500-ms reopened budgets. Percentiles use nearest rank. Fresh-ledger p95
was **102.054125 ms**; reopened p95 was **97.336208 ms**.
Raw logs/samples: `/tmp/fort059-owned-core-budget.log` and `...-budget.json`.

```json
{
  "fresh": {
    "samplesMs": [
      99.692875,
      102.054125,
      97.762875,
      99.360042,
      98.702541
    ],
    "p50Ms": 99.360042,
    "p95Ms": 102.054125
  },
  "reopened": {
    "samplesMs": [
      95.002042,
      94.869333,
      96.988,
      97.003875,
      94.522459,
      97.336208,
      95.268209,
      94.429875,
      94.299917,
      94.953083,
      96.235041,
      96.232,
      95.429083,
      93.421208,
      95.096791
    ],
    "p50Ms": 95.096791,
    "p95Ms": 97.336208
  },
  "budgetsMs": {
    "fresh": 2000,
    "reopened": 500
  }
}
```

## Remaining evidence

All four signed sandbox/XPC scenarios passed with zero skips/failures in
`/tmp/fort059-owned-core-signed.log`, using the existing development identity and
unchanged entitlements. Their successful temporary bundles were removed by the
verification tool; the preceding retained signed artifact remains historical.
Full/implementation spec gates and whitespace checks passed. Native product
source has not changed since [the preceding host slice](mac-core-host.md), whose
fresh unsigned client builds remain separate evidence; this slice changes its
native integration test, not ordinary-app wiring. The signed embedded-XPC fixture
still uses its diagnostic bootstrap, even though its artifact contains the new
production entry. Its successful execution cannot prove the new entry's signed
composition until the production wrapper is wired and exercised.

A production app-like agent wrapper, canonical storage and package/profile validation,
startup/removal integration, official runtime packaging, authorized providers,
reachable iPhone transport and installed accessibility/screenshots remain open.
No installed application, TestFlight availability or complete v1 is claimed.
