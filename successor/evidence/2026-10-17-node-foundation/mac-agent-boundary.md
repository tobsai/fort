# Persistent Mac agent address boundary

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing
[Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
This slice prepares source contracts. No launch agent, App Group grant, provisioning
profile, startup registration, provider access or production package was created.

## Platform evidence and remaining authority

Apple documents SMAppService registration/control for per-user LaunchAgents and
other helper types. The SDK requires an agent plist under the calling app's
`Contents/Library/LaunchAgents` and supports a bundle-relative `BundleProgram`.
Registration is subject to user approval. The asynchronous unregister completion
is delivered after the managed process has been killed; it does not independently
prove that arbitrary descendants are absent. Sources:
[SMAppService](https://developer.apple.com/documentation/servicemanagement/smappservice)
and the installed Xcode 27.1 macOS SDK's `ServiceManagement.framework/Headers/SMAppService.h`.
No register/unregister method was called during this investigation.

Apple's DTS explains that sandboxed app/agent named XPC requires the endpoint name
within an App Group, claimed by both programs and authorized by their profiles.
A helper using the restricted/profile-backed configuration needs an app-like wrapper.
[Apple DTS, June 2026](https://developer.apple.com/forums/thread/835003).
Apple recommends the registered `group.` naming style for new code and describes
profile validation and group-container protections separately.
[App Groups guidance](https://developer.apple.com/forums/thread/721701).

The entitlement reference specifies `<group identifier>.<unique name>` for Mach/XPC.
The SDK defines `BOOTSTRAP_MAX_NAME_LEN` as 128 and `name_t` as a 128-byte char array;
the Fort name allows space for the terminator.
[App Groups entitlement](https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.application-groups).
Public documentation copies were fetched to `/tmp/fort059-apple-{groups,service}.md`.
The web reader rejected Markdown content type; the sandboxed Python fetch could not
resolve the host. An approved public read fetched the documents successfully.

Read-only local profile inventory on October 18, 2026 decoded 91 existing profiles:
50 under `~/Library/Developer/Xcode/UserData/Provisioning Profiles` and 41 under
`~/Library/MobileDevice/Provisioning Profiles`, considering `.mobileprovision` and
`.provisionprofile`. The initial sandboxed CMS decode failed with certificate-import
parameter errors. The approved metadata-only read decoded all profiles successfully.
Only app identifiers/group names containing Fort were returned; unrelated profile
contents, device lists and certificates were not output or retained.

| Historical profile | App identifier | Platform | Expiry (UTC) | App Groups |
| --- | --- | --- | --- | --- |
| `a3d51570-6019-4362-88b2-6e2cc9e82099` | `T3JB5MYZ93.io.mtree.fort` | iOS/xrOS/visionOS | 2027-05-25 00:32:52 | none |
| `28f48236-f80b-43d9-aa23-a6e0a678442b` | `T3JB5MYZ93.io.mtree.fort.watch.widgets` | iOS/xrOS/visionOS | 2027-05-25 00:32:52 | none |
| `81441a69-7364-4cd8-907d-ab420bc47bce` | `T3JB5MYZ93.io.mtree.fort.watch` | iOS/xrOS/visionOS | 2027-05-25 00:32:52 | none |

These historical Store profiles do not authorize a successor Mac host or App Group.
No Fort-specific Mac profile/group was found; this is a local inventory, not a query
of all remote account capabilities. Real successor app/agent identifiers, their
App Group, profiles and startup activation remain unverified external gates. No
historical identity has been adopted or changed. Source preparation continues without
registering or signing an agent with unverified grants.

## Address and storage behavior

`MacAgentEndpoint` validates the exact code identifier, signing team and explicit
`group.` identifier. Its Mach name is derived as `<appGroup>.fort.host`, under 128
ASCII bytes. JSON decoding revalidates and rejects unknown fields. The address
carries configuration, not a claim that Apple has granted its entitlement.

`DeviceEndpoint.macAgent` is distinct from the existing embedded `macHost` case.
Agent connections encode as schema 3; network/embedded connections remain schema 2.
Legacy network records still read without rewriting. Wrong schema/kind pairings,
unsafe pins, mixed fields and future versions fail while stored bytes remain.
On iPhone the value can retain history, while transport selection remains unavailable.

The Mac transport uses the unprivileged named-Mach-service initializer for agents,
and the embedded-service initializer for old endpoints. Its code-signing requirement
uses the code identifier/team independently of lookup name. Existing bounds, timeout,
cancellation and receipt validation remain unchanged. There is no fallback.
The setup coordinator and explicit native model construction accept either validated
local address; ordinary app construction still has no production host configured.

## Verification

- Address red: missing type and enum case, `/tmp/fort059-mac-agent-boundary-red.log`.
- Four address/storage test functions passed, `...-first-green.log`. A test's maximum
  valid byte-count expectation was corrected from 126 to 127 before that first run;
  128 bytes is rejected. No product failure is claimed for that arithmetic correction.
- Model red: the coordinator rejected agent endpoints as wrong-host in both explicit
  connection and cancellation/reopen tests, `...-model-red.log`.
- **145 Swift test functions passed** after local endpoint support. The configured
  connection test now runs for both embedded and agent addresses. A separate agent
  cancellation journey preserves the exact endpoint and identity across an uncertain
  host reply and client reopen. It uses synthetic control, credentials and transport,
  without sending a Mach request. `/tmp/fort059-mac-agent-boundary-swift.log`.

- Strict TypeScript passed: `/tmp/fort059-mac-agent-boundary-typecheck.log`.
- **Four signed sandbox scenarios passed serially**, none skipped, using the
  already authorized development identity and unchanged entitlement sets.
  `/tmp/fort059-mac-agent-boundary-signed.log`. This covers embedded peer rejection,
  exact setup/cancellation, restart/crash recovery, original receipt retention,
  file/TCP denials and owned-process/container absence. Successful temporary probe
  bundles were removed by their existing harness. It does not exercise a registered
  named Mach agent or prove an App Group grant.

## Inspection measurement

MacBookPro18,2 / M1 Max / 64 GiB / 10 CPU, macOS 27.2 (26B5112a), Xcode 27.1
(27A9275), Swift 6.4. After signed tests completed and before fresh client builds,
run the existing inspection test for each endpoint kind. Twenty separate model
constructions per kind read the same ready private record; construction/preparation
are outside the timed region. Synchronous MainActor inspection loops run serially.
No system-cache purge, credential reads, host requests or background work. p95 is
the nineteenth sorted sample. Each target is <=50 ms. Schema 2 denotes embedded
XPC; schema 3 denotes the new agent address. These are warm local metadata results,
not installed, Keychain, XPC or provider timings.
`/tmp/fort059-mac-agent-boundary-budget.log`.

```json
[
  {
    "samplesMs": [
      0.522209,
      0.503833,
      0.666292,
      0.565208,
      0.480958,
      0.47575,
      0.458417,
      0.455459,
      0.458833,
      0.45675,
      0.447208,
      0.441583,
      0.44275,
      0.4415,
      0.440292,
      0.440167,
      0.44625,
      0.4395,
      0.575292,
      0.538875
    ],
    "p95Ms": 0.575292,
    "budgetMs": 50,
    "connectionSchemaVersion": 3
  },
  {
    "samplesMs": [
      0.551917,
      0.477417,
      0.466209,
      0.461292,
      0.456833,
      0.454625,
      0.452083,
      0.451791,
      0.460208,
      0.451084,
      0.4495,
      0.432458,
      0.429209,
      0.429291,
      0.4285,
      0.428291,
      0.427209,
      0.428,
      0.434792,
      0.431875
    ],
    "connectionSchemaVersion": 2,
    "p95Ms": 0.477417,
    "budgetMs": 50
  }
]
```

## Build and remaining acceptance

Fresh unsigned engineering builds passed with `CODE_SIGNING_ALLOWED=NO`:

- Mac: `/tmp/fort059-mac-agent-boundary-mac-build/Build/Products/Debug/FortMac.app`.
- iPhone Simulator: `/tmp/fort059-mac-agent-boundary-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.

The existing `FortSpec058.xcodeproj`, Debug configuration, `FortMac` / `FortiOS`
schemes and generic macOS / iOS Simulator destinations were used. Logs:
`/tmp/fort059-mac-agent-boundary-{mac,ios}-build.log`. CoreSimulator remains outdated
(1171.7.0 installed, 1174.9.2 required); device support was disabled. Neither app
was installed/run; no service, entitlement or OS setting was changed. iPhone's
local-transport refusal is compiled source, not a Simulator/device execution claim.

Full and implementation spec gates passed; exact staged coverage is required before
commit. The last portable result remains 892; this slice changes native addresses
and a Swift compile list, so portable service tests were not rerun. The retained
compiled core is unchanged: 26 files / 186,928 bytes, manifest SHA256
`5c7dbb937a80b48ce44e5e922909e08cc9a5f9b2ff120be1986dd031d46c00d5`.

Actual named-Mach agent communication, profile-backed sandbox/container behavior,
startup registration/unregistration, update/rollback, process absence after removal,
provider execution, installed UI and TestFlight acceptance remain unverified.
