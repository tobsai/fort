# Exact installed Mac host/provider test proposal

**Prepared, not approved or executed.** October 19, 2026. Spec 059 / decision 059.
The existing development provisioning approval is fulfilled. This proposal is the
separate persistent startup, network and provider-credential scope required by
CLAUDE.md: “Mandatory per-action security, credential/persistent-access approvals”.
The G2 CSR upload is separate and remains held pending the parent's approval flow.

## Artifact to authorize

Install the exact inert candidate at `/Users/tobiasgunn/Applications/Fort.app`,
preserving the task-owned current copy, then perform one attended Mac-only test:

`/tmp/fort059-provider-proposal-v2-signed-20261019/Fort.app`

- 206 files / 406,325,058 bytes / 30 Mach-O objects.
- `signed-package-report.json` SHA256:
  `9e2f1cdda9d1b969cc960ca48cf51fbf35282542ba8603e9b9692ba264095778`.
- Payload manifest SHA256:
  `f6025029e974b447ec39e4908d8f5df92a50a1c4250a47090d0c33346e18ceb1`.
- Source base `2f33fbf8a2f0cf67860f7c77f320d09ae35183ae`; report pins the exact
  MacHostStartup, MacHostControl and OwnerSetupView source hashes for this slice.
- Every file/hash/mode, exact embedded profiles/entitlements, all 30 signatures and
  public signing leaf fingerprints, and native bundle inspection passed. This
  candidate has not been installed or launched. Client-only installed evidence
  belongs to the separately hashed candidate in [preflight](installed-host-preflight.md).

Team Maple Tree Enterprises LLC **T3JB5MYZ93**. Client **io.mtree.fort.mac**,
helper/LaunchAgent label **io.mtree.fort.mac.host**, App Group
**group.io.mtree.fort.mac**, Mach service **group.io.mtree.fort.mac.fort.host**.
Existing Apple Development certificate SHA1
`58F40D1AB480565B89109B853235AC3A7B2E7AD1`, expires September 29, 2027.
Profiles `baa209da-ae30-4a3f-9be3-c43ab5abb071` and
`fb9d5710-2b88-47fe-9001-a5556b9ead6b` bind the exact app IDs, group, existing
certificate and Mac device `00006001-000429402202801E`.

Client/helper each request App Sandbox, the one exact group, their own application
identifier and team. Client keeps its existing outgoing-network entitlement.
**Proposed delta: helper com.apple.security.network.client = true.** Node, guardian
and Codex retain exactly app-sandbox plus inherit. No JIT, debugger, inbound-network,
library-validation bypass, wildcard group or broad file entitlement is added.
Outgoing access applies to the host and its inherited children and is not an OS
domain allowlist. The computer-tool permission profile separately denies files and
network and disables shell, browser, computer, MCP/skills/plugins and other tools.
That tool policy does not sandbox the host's provider communication.

## Proposed bounded authority

One attended session, **30 minutes from first Enable**, up to **six synthetic plain
text requests** including a deliberate cancellation, one explicit Stop/Resume work
cycle and one client quit/reopen attempt. Reserve the final five minutes for cleanup.
No automatic retry/relaunch loop, alternate account/provider/model, API billing,
subscription purchase, sensitive files, optional skills/routines/voice/connectors,
other-device pairing, LAN listener or shared hosting. Stop on a new permission
prompt outside this scope, identity mismatch, unsupported control or unsafe cleanup.
Earlier consumed demo grants do not apply. Failed messages still count toward six.

1. Use the installed UI to enable only the exact per-user LaunchAgent through
   SMAppService.agent. Any macOS approval must be explicit. Confirm actual status,
   shared-group access, signed XPC peer checks and the owned Node/guardian processes.
2. Pair only this Mac through Fort, generating its new device token in Keychain
   (service Fort.Spec058.Device, installation/device-scoped account, ThisDeviceOnly).
   Pairing has no default expiry; the session limit is an operator cleanup deadline,
   not a hidden change to the product's pairing policy.
3. Start the pinned official Codex **0.157.1** integration and its official ChatGPT
   device-code flow. **The owner chooses the exact existing ChatGPT account and
   workspace in the official sign-in UI, then explicitly selects the reported
   account in Fort.** Do not infer it from Apple identity or import ambient ~/.codex
   credentials. Verify reported identity before discovery/dispatch and fail closed
   on a change. No API key path or new paid account.
4. Discover models and select **gpt-6-astra only if reported for that exact account**.
   No fallback. Use the existing subscription and its usage limits. Verify effective
   controls/model, basic reply and follow-up, cancellation, history after client
   reopening and explicit Stop/Resume work within the six-request bound.
5. Stop work and revoke only this test pairing, confirm host/runtime exit and exact
   SMAppService unregistration, then remove only its Fort-owned provider credential
   and device Keychain token. Verify the exact registration/processes and credential
   metadata are absent. Keep the app installed inert and retain test history.
   If normal removal fails, report the precise incomplete state; do not claim
   success or delete unrelated state. This approval includes bounded termination
   of this session's exact owned processes if normal shutdown fails, not broad
   process kills, OS-setting changes or provider-account cleanup.

The LaunchAgent definition contains BundleProgram and MachServices, with **no
RunAtLoad or KeepAlive**. Registration still persists for this login user and lets
macOS launch the helper on demand/across later logins until it is unregistered;
closing the client does not revoke that persistence. The actual installed behavior
and supported removal remain acceptance gates. Ordinary unpaired client Quit failed
in preflight and required exact-process SIGTERM; do not mark quit/reopen accepted
until corrected/retested. New or changed signed bytes must be repinned and assessed
before activation rather than silently treated as the artifact approved here.

## Data sent and retained

Official authentication sends the normal account/device-code protocol to OpenAI.
Each model request sends the new synthetic message, selected agent instructions,
conversation identifiers and prior conversation text/role/agent metadata. The
adapter can also send selected memory and skill instructions; leave these empty
for this test. History is capped at 256 messages and context at 65,536 bytes in the
current adapter. Evidence records statuses/IDs/hashes, never credentials, device
codes or message bodies. Source/config checks disable analytics, startup auto-update,
OTel export and prompt telemetry; actual runtime controls must pass before dispatch.

Local native state lives under
`~/Library/Containers/io.mtree.fort.mac/Data/Library/Application Support/FortCandidateB`.
Host history, ledger/outbox/runs and runtime data live under
`~/Library/Group Containers/group.io.mtree.fort.mac/FortHost`:
`openai-candidate-b.sqlite` (including WAL/SHM), private core.sock, and
`Runtime/{home,state,workspace,tmp}`. Provider credentials use private
`Runtime/state/auth.json`; device tokens use the scoped Keychain entry. Runtime
session/log state can contain text; do not promise zero local prompt retention.
Fort history/native caches and other runtime session state remain after the scoped
credential removal unless deletion is separately chosen. OpenAI retention follows
the chosen account's terms/settings and is not independently verified here. Local
credential deletion does not prove remote sign-out, grant revocation or deletion
of provider-held data; no remote account cleanup is included.

## Ready-to-present approval wording

“Approve one attended 30-minute test of the exact Fort Mac candidate pinned above,
including its proposed helper outgoing-network entitlement, one per-user host
registration and Mac pairing, official Codex sign-in to the existing ChatGPT
account/workspace I choose and explicitly select, and at most six synthetic text
requests using gpt-6-astra if available? Include the bounded lifecycle checks and
removal of that test's startup registration/local credentials afterward, retaining
history and the inert app. I understand the host can connect outward, registration
persists until removed, the provider receives the described test context, and local
cleanup does not revoke remote account grants.”

The parent should present this once with the exact artifact and remaining known
quit issue. This document and preparation do not themselves grant approval.
