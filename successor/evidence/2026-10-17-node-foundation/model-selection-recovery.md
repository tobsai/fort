# Installed model selection and smoke test — October 20

Base: 04afae3b0bc535a3b361d5c4e5f3a3e9ef6b2b75, isolated branch
codex/fort-openai-rebuild-20261016. Spec 059 and the owner's model-selector report,
6.1 follow-up and full-smoke request govern this slice. G2 remains separately held.
The existing personal ChatGPT/Astra authority permits at most six synthetic prompts,
time as needed, unchanged permissions and final local cleanup with history kept.

## Diagnosis and repair

The installed New agent form had an empty model picker. Supported status reported
an authenticated account that had not been explicitly selected. Selection was on a
different Settings page. The operator retained the two unsaved form fields in RAM,
confirmed the reported account/workspace through the normal control, restored and
compared the fields, selected runtime-listed GPT-6-Astra and saved Lewis Too.
The temporary form values were released. No credential contents were read.

The editor now reuses AccountSignInView for an unselected owning-Mac account, so
confirmation no longer requires leaving the form. Confirming an account refreshes
the model catalog. Account selection and discovery refuse overlapping operations.
Account identity, provider identity, saved-model preservation and companion
restrictions remain enforced. Nothing signs in or selects an account automatically.

Three public-model regressions cover the automatic refresh and both operation
orderings. The first two failed with five assertions before implementation; the
reverse-order regression then failed separately. All three pass after the repair.
The first complete fresh native run passed 339 functions with 16 optional skips;
the expanded run passed 340 with 16 skips, zero failures, 59.255 seconds.
Fresh Mac and iPhone Simulator Release builds passed. Simulator execution was not
tested; Xcode reports an older installed CoreSimulator service.

## Missing 6.1 models

The owner confirmed the selector was visible, then asked about missing 6.1 models.
The active official runtime is Codex 0.157.1. Its dedicated model-cache metadata
reported a fresh fetch at 2026-10-20T02:37:34.275694Z for that version:

| Runtime model ID | Display name | Runtime visibility |
| --- | --- | --- |
| gpt-6-astra | GPT-6-Astra | list |
| gpt-6-sol | GPT-6-Sol | list |
| gpt-6-luna | GPT-6-Luna | list |
| gpt-5.6-sol | GPT-5.6-Sol | list |
| gpt-5.6-terra | GPT-5.6-Terra | list |
| gpt-5.6-luna | GPT-5.6-Luna | list |
| gpt-reserve | GPT-Reserve | hide |
| codex-auto-review | Codex Auto Review | hide |

All eight report text/image input. No 6.1 ID is present, including hidden entries.
The refreshed installed picker exactly matches the six visible entries. Fort asks
model/list with includeHidden:false, follows pagination and preserves each exact
model slug; there is no name/version allowlist and no Fort catalog cache. The
service returns the adapter catalog without removing individual models.

Pinned upstream source at 36650394c5b38c2990ccf2a3457165ca3e9d9726 confirms:

- [Model visibility](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/models.rs):
  includeHidden:false applies the runtime's show_in_picker flag.
- [Model manager](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/models-manager/src/manager.rs):
  five-minute cache TTL, client-version and authentication-scope checks, remote
  catalog refresh for Codex-backend authentication.
- [Feature declarations](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/features/src/lib.rs):
  remote_models is a removed legacy compatibility flag. The initial suspicion
  about Fort disabling it was ruled out; no flag was changed.

The omission is in the catalog exposed by this runtime while this account was
selected. The cache timestamp does not establish entitlement or prove that its
contents are a current account-specific catalog. These observations alone do not
distinguish bundled/catalog staleness from account rollout or version gating. No guessed 6.1 ID, hidden-model override, alternate account, paid
API path or runtime upgrade was attempted. This is discovery evidence, not a
claim that 6.1 is unavailable in every OpenAI product.

## Verified update path and authentication boundary

The [official changelog](https://learn.chatgpt.com/docs/changelog) records that
0.159.1 added GPT-6.1 Sol to bundled catalogs on September 29. The latest stable
release verified on October 20 is
[0.163.0](https://github.com/openai/codex/releases/tag/rust-v0.163.0), published
October 15 at 21:30:38 UTC. Its annotated tag
129b8ec7ee6f4511557a4856279f529025c25af4 resolves to source
83086188ae0614bd882818ff1aa23f3194962012. That source's
[model catalog](https://github.com/openai/codex/blob/83086188ae0614bd882818ff1aa23f3194962012/codex-rs/models-manager/models.json)
contains gpt-6.1-sol / GPT-6.1-Sol, visible with text/image input. It contains no
other 6.1 ID. Downloaded catalog SHA256:
943ca7fe1d19ed019054158303f0dbbd80b3bef43b3aa425a71aa7cb3fb52c2b.
This supports updating the runtime; it does not prove this account can infer with it.

The official [models and inference guidance](https://developers.openai.com/siwc/token-sharing-open-source/models-and-inference)
explicitly warns that app-server model/list may use bundled or cached choices.
Its account-specific alternative uses a token issued for the separately documented
[direct token-sharing registration flow](https://developers.openai.com/siwc/token-sharing-open-source/sign-in),
with an issued client ID and chatgpt.tokens.use.direct permission. Fort currently
uses account/login/start with chatgptDeviceCode, account/read and runtime-managed
credential storage. It does not implement that registration/token-sharing adapter.
The direct endpoint guidance is therefore not authority to extract Fort's managed
credentials or send them through another route. No such request was made.

Concrete next slice: update Fort's actual bundled Codex executable to reviewed
0.163.0, together with its version-bound adapter and packaging expectations. Updating
another Codex on PATH does not change this package. Current pins occur in
runner/codex-descriptor.ts, codex-account.ts, codex-chat.ts, codex-executor.ts,
packaging manifests and the signed runtime/license inventory. Startup checks contain
0.157.1-specific feature aliases and unified_exec behavior. The new release changes
shell execution and removes the patched zsh, so a binary-only replacement is not
an adequate update. Keep exact saved models and historical bindings; an existing
agent's runtime-version binding also needs an explicit tested migration decision.

Preparation can proceed under ordinary implementation authority: declare the exact
slice, audit source/protocol/features and legal notices, test version rejection and
compatibility, run a new private empty-home no-network startup/control probe, then
build and verify a complete signed candidate. Preserve disabled tools, telemetry,
auto-update and ambient configuration, and retain the host/tool-sandbox distinction.
No new runtime was downloaded, executed, signed or installed in this slice.
New live activation/sign-in and exact 6.1 inference need their own concrete authority:
the prior attended session is now cleaned up and must not be replayed. G2 remains held.

The owner prefers the latest models. Preserve that preference for the next design
slice without silently changing existing exact model selections or claiming that a
catalog label establishes provider access.

## Installed client and live smoke checks

First repaired package report SHA256:
c071fdbfa1ce53fa2f4b8d1496734b6198b8d60ff7880d45e3daac06e813ed9c.
Payload SHA256:
a8cda3a1cb1074246c81d8e007f14348021f38b23baa493e14aefef637e963bb.

All 206 candidate files and 30 Mach-O signatures/public signing certificates
verified; exact entitlements and native full-package inspection passed. The update
changed only Contents/MacOS/FortMac. Helper/runtime/resources/profiles and the
outer resource seal stayed identical. Installed hashes and native inspection passed.
The existing host PID and helper inode were unchanged. Full previous app retained
at /tmp/fort059-model-selection-prior-installed-20261020.app.

The initial packaging assertion expected two changed files; codesign correctly
left the unchanged resource seal identical. Inspection confirmed the sole changed
file, and the assertion was narrowed to permit only the executable and optional
outer seal. The failed check occurred on an inert temporary candidate.

Installed checks completed before the cleanup repair:

- Account selection and saved gpt-6-astra binding survived update/relaunch.
- Accessibility showed the ready model catalog. A Settings-window screenshot was
  visually inspected and shows Lewis Too / gpt-6-astra. The earlier coordinate
  crop captured avatars instead and is not model-picker evidence.
- Normal Quit passed from the Settings editor and Settings root. Foreground normal
  Quit after chat passed. One earlier background/concurrent-activity Quit attempt
  did not terminate and is retained as an unsuccessful operator attempt.
- Normal window close left the client running with zero windows; reopening restored
  the transcript.
- A synthetic no-tools Astra request returned its exact expected marker and saved
  the reply. Explicit Stop local work retained pairing/history; explicit Resume
  local work succeeded. A second synthetic recovery request returned its expected
  marker after resume. Earlier messages remained present.
- The completed run UI says effective model was not reported. Exact selected
  gpt-6-astra was verified by runtime controls/preflight; no stronger effective-model
  claim is made.

Two automated synthetic prompts were sent. One additional completed turn appeared
during the check; its contents were not inspected or copied. It is conservatively
reserved against the same limit: three of six slots accounted for. Reservations
were durable before each automated Send. No prompt was repeated after a completed
reply. An AX-only composer fill did not enable Send; normal keyboard entry into
the exact composer worked, without consuming an extra request.

## Cleanup defect and final verification

The first normal Prepare to remove Fort attempt refused with “The saved pairing
changed.” Read-only lifecycle comparisons found an exact current connection and
a confirmed removal record belonging to an earlier pairing. The host remained
registered. The UI's accessibility confirmation returned -25205; actual state was
checked before proceeding, rather than treating that return as proof of no effect.

The real MacHostRemoval boundary now treats a confirmed foreign pairing record as
completed history. It still validates the exact current/disconnected connection,
pending-setup absence and every recovery/exit/unregistration/credential proof.
Mismatched requested and locally-forgotten intents still refuse before any effect.
No installed lifecycle record was manually modified.

A parameterized public-adapter regression reproduced the confirmed-history failure;
the two unfinished-intent cases already refused correctly. It checks recovery and
unregistration counts, credential callback, and preservation of historical intent,
current connection and conversation bytes. The confirmed case failed before the
repair; the other two already refused safely. All three now pass.

The final full native run passes 341 functions, skips 16 optional functions, and
fails none (357 total), in 59.552 seconds. The portable suite passes 1,329 tests,
skips 16, and fails none (1,345 total), in 42.134 seconds. Mac and iPhone Simulator
Release builds pass after the cleanup change.

Second client-only package report SHA256:
b418cf5fb43b631339a9b1031ab162d3f0e8f774756e39b9352bfb96b50af67f.
Payload SHA256:
e97d9996306be248594c247f7f924a8376a8fdd9c0d126417188d025822baa1b.
All 206 files, 30 Mach-O signatures, certificates, entitlements and native package
inspection pass. Only Contents/MacOS/FortMac changed again. The old full package
is retained at /tmp/fort059-model-selection-pre-cleanup-installed-20261020.app.
The host PID and helper inode stayed unchanged during installation.

Normal Prepare to remove Fort then confirmed shutdown, revocation, startup
unregistration, local pairing removal and deletion of Fort's saved provider sign-in.
Independent verification at 03:04:57 UTC found the exact launchd job and owned host
processes absent, auth.json and active connection absent, matching confirmed
pairing/removal records, two saved snapshot files and retained host history.
Provider-side grants and provider history were not changed.

## Remaining Quit failure

Normal Quit passed from the initial Settings editor/root and ordinary chat. After
Settings confirmation actions, both the Quit menu and actual foreground Command-Q
failed to terminate the client. Back → Done → Command-Q succeeded. The relation to
alert dismissal is observed; the root cause is not yet proven.

A plain AppKit alert, a SwiftUI alert on an AppKit sheet, and a SwiftUI alert on a
SwiftUI-presented Settings sheet all passed their window-termination property
checks without further product changes. They do not reproduce the installed
failure and are not claimed as regression fixes. Temporary diagnostic source and
logs remain under the raw evidence directory; no ineffective test or speculative
window-policy repair was added to the product. This stays an open v1 UI finding.

After final cleanup, the exact installed client reopened in disconnected setup,
showing that the host was not enabled. Setup was closed and normal foreground
Command-Q succeeded. At 03:17:18 UTC, all 206 installed files still matched the
final report; client and host processes, launchd job, credential path and active
connection were independently absent. Startup was not re-enabled and no new
account or prompt was used. History remains retained.

## Evidence locations and limits

Raw logs, redacted AX records, source audit, screenshots, package manifests and
prompt reservations: /tmp/fort059-model-selection-20261020.
Fresh Swift scratch: /tmp/fort059-model-selection-swift-tests.
Fresh initial Mac/iPhone build outputs:
/tmp/fort059-model-selection-mac-build-20261020 and
/tmp/fort059-model-selection-ios-build-20261020.

The completed live checks used the existing personal account and owner pairing. Fresh
enrollment/new sign-in was not repeated; earlier activation evidence remains
separate. No new provider grants, computer tools, remote agents, external accounts
or permission changes. TestFlight, physical iPhone, new runtime versions and 6.1
provider availability remain unverified. Source/fixture checks do not establish
those outcomes. No push, merge, upload or provider-side grant revocation.

## Raw evidence digests

- native-cleanup-final.log: `ec3fa53071d4b851087010e47e278529448454b3b7392ea058334af72f1ab258`
- portable-smoke.log: `5887389dad8540a0069478311b757fa908d54a5d1de014eadc2ddc743747f42b`
- mac-cleanup-build.log: `d724c3f00eef408dece597d82b6c03bce9d6d9f07bab556c3398cc8969095ea7`
- ios-cleanup-build.log: `52c6a55a34e885fe75e062745dde2e4a7416acb2a9f308667160cd668fd1c405`
- cleanup-candidate-verification.json: `c51adda33666f90ca245c732bb3734cab7e9d2c0f6e068a99426d5484f60cd29`
- installed-cleanup-update.json: `c5872b3ec654b48a47bf8fdd4b0c9f1154015c3e65eeddf1b09f1aa12ecacb32`
- cleanup-absence.json: `804a51de30fa8399887e4b2532c7de0bddda8f0e3f39d09b1d39372db3578e51`
- final-disconnected-verification.json: `308543fab4dfac045e43582f267648d5209f4b86fb494c4a35845d7652009578`
- provider-prompts.json: `86f83dba97922933a21bd4534271d5bf0f149262431f046d705c61075df25910`
- alert-quit-red-2.log: `570767d4bfe9378e6e748da947a8f17676a5fa577f6a52ae52d4232a1d5e80b1`
- alert-quit-red-3.log: `5d53a8cce8ebc1ff9c9b51879d2e048e033278c117379ab05f9a829722704b15`
- alert-quit-red-4.log: `9661c88ba366ce647b36ed013328050594e4362875c4c74f1c0e5ac5a6a5b348`
- AgentSettings-alert-diagnostic.swift: `4a3007725cf45f8044ece5cec9369ed05e8213d23a4c7e408541170f1d67d7fa`


## Pairing navigation Quit repair, October 20

The faithful fixture changed the diagnosis. A temporary SwiftUI app linked to the
actual FortKit library rendered MessengerView with AccountSignInFixture, temporary
storage and no network or real credentials. Ordinary Settings navigation changed
the same SheetPresentationWindow's preventsApplicationTerminationWhenModal flag
from false at the root to true on Pairing and local host. Foreground Command-Q
failed before any confirmation appeared. Back restored false and Quit succeeded.
The earlier alert-only fixtures did not include this destination transition.

The rendered PairingSettingsView regression failed first because the containing
window still prevented termination. It now passes with unchanged draft and exact
synthetic connection bytes. The fix reuses NonBlockingSheetWindow on that destination,
just as Settings and setup already do. It adds no global termination policy, polling,
host operation or account access. The temporary real-view app then completed ordinary
Settings, Pairing, Stop local work, Cancel and foreground Command-Q successfully.
The alert remained blocking while displayed, and the parent sheet was nonblocking
after dismissal. Cancel's accessibility call returned -25205, but subsequent window
inspection confirmed dismissal; it was not blindly repeated.

Validation passed:

- Focused native policy regressions: 2 pass, after the new case failed first.
- Complete sequential native suite: 342 pass, 16 optional skips, zero failures,
  358 total in 59.766 seconds.
- Fresh Mac and iPhone Simulator Release builds.
- Signed candidate: all 206 file hashes/modes, 30 Mach-O signatures, exact existing
  certificates/entitlements and native MacHostBundle inspection.
- Installed client-only update with a complete prior app retained. Only
  Contents/MacOS/FortMac changed; helper inode, resources, runtime, profiles and
  permissions stayed unchanged.
- Exact installed disconnected setup accessibility and screenshot inspection,
  followed by successful foreground Command-Q with setup still open.
- Independent final check at 03:40:12 UTC: all 206 installed files match; no owned
  client/host process, launchd job, provider credential file or active connection.
  This client-only operation does not remove or rewrite retained history.

The installed disconnected app has no workspace and therefore does not expose the
Pairing settings route. That exact installed journey remains unrun until an authorized
pairing; the faithful synthetic real-view flow is the current reproduction and fix
proof. No host activation, re-pairing, provider prompt or credential extraction occurred.
Portable tests were not rerun for this Swift-only change; the prior 1,329-pass result
belongs to 43bd032. Physical iPhone, TestFlight, real 6.1 inference and G2 upload remain
unrun/held. Codex 0.163.0 preparation is a separate unfinished slice.

Installed candidate report SHA256:
`5d24d0c8e3595334e2bb0b0becb10b84d698b7516eb25c22f4802de0017b98f8`.
Payload SHA256:
`20f874e206208dcdb66045566a19edcdb29f7fca235ad9b5ddd9e97aff857852`.
Rollback app: `/tmp/fort059-pairing-quit-previous-installed-20261020.app`.
Raw temporary evidence: `/tmp/fort059-settings-confirmation-20261020/`.
The before/after fixture apps and their source remain there. Only the fixed fixture
exited successfully through the Pairing confirmation path. Both are now stopped.

Evidence hashes:
- pairing-red.log: `d847b5ed0d42ef276ad90b0ed2c725e1e950292b29001ba81a85a1caabe3583f`
- pairing-green.log: `f4b9b02b7e3a7fa8f5298616ff2f3e9aeb81f23db79eacce3bc57a6c53e823c4`
- windows-before.log: `17a652ff2b2a778a3f672ef24f154343069e1579967babafcfdda22266e0110a`
- windows-after.log: `977494ac5a61983e2ded2c35c8425eb5ce43403a36d6e65362b2f94fa9a3a16a`
- flow-after-cancel.json: `efc6ce2c923e184928c9bfd6ed0640fd10ae4a0fcbfcc9a3525d82baa0165003`
- quit-after-cancel.json: `5172af53e6e2c94f7d0e9ac893b9c5bee18dd89919db77f98cf66907c6bc2078`
- native-final.log: `f37f58a36dffb5506a9eb365a49757f3a5667093041375e10165abb43b2c9853`
- mac-build.log: `b963049317a6f15aac3355e4a4bc1bc7c3f84376730c1ece7cc99d16db65aa28`
- ios-build.log: `791b541706c1000c4e73147def1ae33a889cff65162c59c9a941e724d8e33891`
- candidate-verification.json: `494e599342c0a7d4f21be7aec3c2f2183450f2b188f9a6dfeedc5cb89e2e4817`
- installed-update.json: `bfaf433002884569785080dfad45e69574073742432ab2678a4d1e240696fa4e`
- installed-disconnected-ax.jsonl: `6facfc1e381f65b63caf4a8fd1eabc39c498b4b66ad632eaa698a259e4d2c529`
- installed-setup.png: `4cc746359745ecf59f1d6bc0c04132f8f0a45ea691bcd311fe8a390ac6fe46a5`
- installed-quit.json: `5172af53e6e2c94f7d0e9ac893b9c5bee18dd89919db77f98cf66907c6bc2078`
- final-disconnected-verification.json: `e818ad2b58bf3b2f8a1e509686502d72431b50e843c7a6293f75639434448bb8`
