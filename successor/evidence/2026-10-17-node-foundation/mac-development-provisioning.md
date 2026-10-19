# Mac development provisioning and signed package

October 19, 2026. Governed by [Spec 059](../../../specs/059-modular-node-v1.md).
Product source is `60b144706a4f58852a8a5634c12a2ba758b54f0e`; this slice changes
only the spec and evidence. [Machine-readable results](mac-development-provisioning.json)
record the exact profiles, signing identities, builds and checks.

## Authority and Apple changes

At 17:41:48 UTC the owner answered “Yes”, message
`Sentinel_0027f013f6888191af17a943d64b6802`, to the exact consolidated provisioning
request `Sentinel_d35153d258108191af6a5a13a439515b` at 17:38 UTC. The coordinator
relayed both references from thread `01a0f307-3940-7333-96b0-a3b0b854e0e8`.
This authorizes the two identifiers, one group and two development profiles below
using the existing certificate/device, followed by a local signed build. It does
not add provider-login or background-service activation authority.

The authenticated Account Holder session showed Maple Tree Enterprises LLC,
team `T3JB5MYZ93`. The pre-change lists had neither exact Mac ID nor an App Group
or Mac profile. Created:

| Record | Exact result |
| --- | --- |
| Fort Mac App ID | `io.mtree.fort.mac`, portal record `W52255PZQS` |
| Fort Host App ID | `io.mtree.fort.mac.host`, portal record `9UDXQ2ZK4T` |
| Fort Mac Shared Data | `group.io.mtree.fort.mac`, associated with only the two new IDs |
| Fort Mac Development 20261019 | UUID `baa209da-ae30-4a3f-9be3-c43ab5abb071`, created 17:53:55 UTC |
| Fort Host Development 20261019 | UUID `fb9d5710-2b88-47fe-9001-a5556b9ead6b`, created 17:54:59 UTC |

Apple presents the explicit App ID records as cross-platform. Both generated
profiles declare `OSX`, one certificate and only the existing device
`00006001-000429402202801E`. The final portal list shows one profile of each name.
The existing Fort iOS app/identifier and unrelated records were not changed.
App Groups was the only capability selected; Apple's non-editable In-App Purchase
checkbox was already selected. No purchase product or service was configured.

The existing public development certificate was downloaded and matched to local
SHA-1 `58F40D1AB480565B89109B853235AC3A7B2E7AD1`, serial
`73B4165F5E67529210DB32B75EB8A24C`, team `T3JB5MYZ93`, expiry
September 29, 2027 at 20:26:26 UTC. Both downloaded profiles contain exactly that
certificate. Profiles expire October 19, 2027 at their corresponding creation
times; the certificate's earlier expiry still limits signing validity.

The profile allowances include the explicit group and Apple's `T3JB5MYZ93.*`
App Group/keychain wildcards. These are profile allowances, not the application's
requested entitlements. Each signed app requests only `group.io.mtree.fort.mac`.
Raw profiles remain outside Git at `/tmp/fort059-development-profiles-20261019`;
public identifiers and hashes alone are recorded here. No private key was read,
exported or created. The existing certificate was used through normal codesign.
No G2 replacement or agreement acceptance was performed in this slice.

## Exact artifact and verification

Signed, unlaunched development app:
`/tmp/fort059-development-signed-20261019/Fort.app`.

It contains 206 files, 406,325,058 bytes and 30 Mach-O code objects. Full file
membership, modes, sizes and SHA-256 values are in the retained signed-package
report, SHA-256
`20ed08cc2df56549234a63f09c3da9b1854cc05a08958cbdf18301cdaa03b559`.
The unsigned assembly and every older package are preserved separately.

- Fresh unchanged-source Release builds passed: FortMac 53.339 seconds, FortHost
  3.367 seconds. Commands use explicit `PRODUCT_BUNDLE_IDENTIFIER` overrides and
  `CODE_SIGNING_ALLOWED=NO`, then the existing assembly tool. This is one build
  sample per scheme, not a benchmark or installed-performance result.
- The unsigned 202-file assembly failed the real strict signature check as expected.
  The existing assembler verified its inputs, dependency closure and legal payload.
- Profiles were decoded and checked for exact team, app ID, platform, one approved
  certificate, one device and the shared group before embedding.
- Dylibs were signed first, then Node/guardian/Codex, helper and outer app. Signing
  used the existing certificate, hardened runtime and `--timestamp=none`, with no
  `--deep` signing or entitlement-preservation shortcut. The original Codex binary
  and runtime inputs were untouched; their copies acquire new signature bytes.
- Strict nested verification passed. Every one of the 30 code objects separately
  passed strict all-architecture signature checks and exact team/development
  authority checks. All 30 extracted public leaf certificates match the approved
  fingerprint. The first extraction command used the wrong optional-argument form;
  correcting it to `--extract-certificates=prefix` passed without package changes.
  All 206 resulting file hashes, modes and lengths matched.
- Client and helper have their own App Sandbox, exact App Group, team and application
  identifier. The client's existing outgoing-network entitlement is retained.
  The helper has no outgoing-network entitlement. Node, guardian and Codex each
  have exactly App Sandbox plus sandbox inheritance, with exact helper-derived
  identifiers. No debugger, JIT, library-validation bypass or wildcard group was added.
- The existing public `MacHostBundle.inspect` accepted the real signed package.
  A temporary read-only entry point used 49 byte-identical FortKit library files;
  it did not launch an app, helper, runtime, listener or provider. The first probe
  used `/private/tmp` and was correctly refused as `unsafeFile`: Foundation on this
  Mac standardizes that alias to `/tmp`. Correcting the probe path passed without
  changing product code. Profile eligibility remains distinct from its native
  `presentUnverified` profile-presence result.
- Fort display name and compiled icon checks passed on the signed artifact.

Reproduction inputs and commands are recorded in the JSON evidence and in
`/tmp/fort059-build-development.py`, `/tmp/fort059-sign-development-package.py`
and the corresponding build/signature logs. Existing project defaults were not
changed. The temporary scripts are inspection/build tooling, not installed services.

## Remaining gates

No app was installed or opened. There is no signed installed accessibility,
screenshot, App Group container, named XPC, startup, provider or physical-device
acceptance. Actual host/provider testing requires a concrete separate grant;
this helper's no-network entitlement must not be presented as provider readiness.
The agent host and computer-tool sandbox remain different security boundaries.

Development profiles are not App Store distribution profiles. Archive, upload,
processing, tester groups, TestFlight availability and physical acceptance are
unrun. Privacy/runtime inventory, final privacy/feedback/support contacts and URLs,
and export-compliance decisions remain open. No submission or publication occurred.
The separately approved G2 direct-distribution replacement remains unexecuted and
is not a development or TestFlight prerequisite.
