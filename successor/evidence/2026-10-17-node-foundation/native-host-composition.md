# Fixed native host composition — October 18, 2026

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md), fixed native
host resource composition revision. This is ordinary implementation under
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
Base: `ee02e19315b1b262c39e96a6ea612648b4c8f668` on
`codex/fort-openai-rebuild-20261016`.

## Implemented and verified scope

`host/main.mjs` fixes the compiled core, optional Codex executor and separate local
context module. It accepts no arguments, derives exact bundled Codex/guardian
paths, uses the private working directory, and retains bounded owned cleanup.
Runtime construction launches nothing; the synthetic native test verifies that
provider protocol processes begin only with client discovery/dispatch.

`MacHostProgram` now checks the bundled Codex executable's exact identifier/team,
regular executable file metadata, and inheritance-only sandbox entitlements.
`MacAgentService` takes that fixed program. The unused public arbitrary-argv
constructor was removed after migrating its sole fixture caller. Internal lifecycle
injection remains for tests. No new storage class, identity record, bootstrap state
machine, provider dependency or credential mechanism was introduced.

`MacAgentStorage` prepares `Runtime/{home,state,workspace,tmp}` under its existing
validated FortHost directory. Descriptor-relative operations check current owner,
0700 mode, no links/extended ACLs, matching identities and original-root revalidation;
existing files/contents/modes are preserved. These are bounded local checks, not an
atomic defense against a hostile same-UID process changing paths after inspection.
The executor independently rechecks its private plan before launching a runtime.

The ledger already owns the installation UUID and provider/version/host binding.
`local-codex` stays installation-scoped; it is not physical machine attestation.
The service regression now verifies reopen refusal for each changed provider,
runtime version and host. No separate host-UUID file or copied-ledger adoption exists.

## Synthetic native journey

`MacHostJourneyTests.swift` uses the actual production program composition,
Swift FortClient, MacCoreHost, fixed compiled bootstrap, compiled core and executor,
and the real native guardian chain. Only container/signature/listener observations
and the official-runtime protocol endpoint are authored fixtures. The endpoint is
a Node script, not Codex. The copied Node launcher uses its installed Homebrew
libraries, including a copied libnode; this is not binary dependency closure.

The isolated successful journey took **1303.799042 ms**, excluding fixture copying
and compilation (<=5000 ms target). It verifies:

- Inert service construction and no runtime process before discovery.
- Owner setup with no default expiry, exact model discovery and saved context.
- One exact whitespace/Unicode text request and reply through the compiled worker.
- Clean shutdown, then reopening with unchanged installation/epoch/runtime identity,
  pairing receipt and history; duplicate submission reuses its receipt without
  launching another runtime.
- Exact pairing revocation, core shutdown, listener invalidation and socket removal.
  Six owned process identities per runtime launch and their groups were independently
  checked absent. The ledger remains; no provider auth file is created.

No named system XPC listener, App Group capability lookup, startup registration,
Keychain mutation, real provider, installed UI or OS sandbox claim is established.

## Validation

All checks below ran with Node 26.5.0, TypeScript 6.0.3, Xcode 27.1 and Swift 6.4,
macOS 27.2 arm64. Historical machine inventory is MacBookPro18,2, 10 cores/64 GiB;
no fresh hardware inventory is claimed.

| Check | Result |
| --- | --- |
| Full portable suite, `npm --prefix successor test` | 1087 passed, 0 failed, 5 opt-in budget tests skipped |
| Strict TypeScript and architecture structure/accepted implementation gates | Passed |
| Full native fixture regression | 229 test functions passed; timing/signature/journey checks excluded and relevant new timing/journey run separately |
| Focused storage/program checks | Passed: 25 unsafe runtime-directory cases, 18 tool-authority cases, 28 file-metadata cases, 3 executable cases, exact identity and inert public construction |
| Affected compiled executor/guarded Mac suite | 27 passed, 0 failed, 3 opt-in budgets skipped; synthetic process groups absent |
| Resource builder behavior | Complete manifests/licenses, separate compiled context, core independence, no ambient preload, no TypeScript source imports, fixed bootstrap argument refusal and existing destination preservation passed |
| Five serial fresh resource builds | Passed, p50 3725.724 ms, p95/max 3767.635 ms (each <=10000 ms) |
| Twenty private runtime-directory preparations | Passed, p50 0.2445 ms, p95 0.325708 ms, max 0.551208 ms (p95 <=50 ms) |
| Fresh unsigned FortHost, FortMac, FortiOS Simulator Debug builds | All passed |
| Real signatures/profiles/provider, registered helper, physical devices, accessibility/screenshots, archive/upload/TestFlight | Unrun; no completion claim |

The full native command excluded the existing opt-in budget/signature functions
and the separately executed `nativeBundledHostJourney` and
`agentStorageRuntimePreparationBudget`. Relevant logs are
`/tmp/fort059-host-{portable,typecheck,native-full,macos}.log`,
`/tmp/fort059-native-journey.log` and `/tmp/fort059-host-resources-{red,green}.log`.

Serial resource-build samples (ms):
`[3766.095916,3725.723875,3656.99975,3767.634958,3655.179833]`.
Runtime preparation samples (ms):
`[0.551208,0.246334,0.241041,0.243375,0.325708,0.263084,0.245125,0.247291,0.2445,0.252667,0.253541,0.247791,0.248334,0.240167,0.241875,0.239583,0.239125,0.239333,0.239291,0.24425]`.
These measure local composition/metadata, not provider latency or idle performance.

## Retained artifact and engineering builds

Resources: `successor/.build/host-resources-059-20261018`, produced by:

```sh
/opt/homebrew/opt/node@26/bin/node successor/packaging/host-resources.ts \
  --destination "$PWD/successor/.build/host-resources-059-20261018"
```

The manifest lists **87 files / 360602 bytes**; every size/hash was independently
verified. Manifest SHA256:
`f229160e4c645cc01303512215cc2d7ffdd689c2f15f735419e969dbd2a08db0`.
Source MIT licenses remain in core/executor/context. Node, official Codex, guardian,
full binary license/dependency closure and Apple profiles are supplied separately.
This resource artifact does not authorize those dependencies or constitute an app.

Unsigned engineering builds:

- `/tmp/fort059-composed-host-helper-build/Build/Products/Debug/FortHost.app`
- `/tmp/fort059-composed-host-mac-build/Build/Products/Debug/FortMac.app`
- `/tmp/fort059-composed-host-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`

These targets do not yet embed the assembled host resources/binaries or release
profiles. No bundle was launched, signed, registered, installed or uploaded.

## Failures and limits retained

The first resource test failed because its public builder module was absent.
The first native test compile failed for the missing Codex program property.
Implementation then exposed a Swift actor-isolation conversion error; an explicit
MainActor container-lookup closure fixed it. The journey test initially used the
wrong native descriptor field (`provider` instead of optional `runtime`); corrected
without changing the contract. Its first launch failed because copying Homebrew's
Node launcher alone omitted libnode; the fixture now copies that dependency and
still discloses remaining Homebrew dependencies. A later submission was correctly
refused with `memoryChanged`; the fixture now sends the saved memory revision.
The complete journey then passed without weakening any product guard.

The sandboxed builder's Git license fallback emitted Apple's temporary-directory
warning but succeeded. Xcode again reported CoreSimulator 1171.7.0 older than
required 1174.9.2; all three builds passed, simulator UI execution remains unavailable.

The requested ponytail skill was searched in all 133 executor skill entries and
available local skill paths/content. It was not found and was not run; no substitute
skill is claimed. Its location/name clarification remains open.

## Remaining work and external gates

Continue the ordinary installed removal/history/reinstall composition and concrete
helper package assembly. Actual App Group storage compatibility, runtime package
metadata/legal closure, account authentication, provider network authority, iPhone
transport and TestFlight remain distinct gates. See
[Apple activation prerequisites](../../APPLE-HOST-ACTIVATION.md) for exact proposed
IDs and the read-only account check, conditional provisioning and later bounded
registration action. No permission expansion or activation is implied here.

The original Apache donor remains quarantined and unchanged from `ac2dd2a`, with
upstream pin `8b53c0323dba2099a953ee2d77ab9330cd5acaa7` and attribution intact.
The primary checkout, remote visibility, consumed demos and unrelated untracked
work were not modified. No push, merge, deployment or provider cleanup occurred.
