# Exact installed Mac host/provider test proposal

The explicit override in Sentinel_cc6a553555a481919adde09325fb740c resolved the
activation rejection below. Same-scope testing exposed and repaired the private
MCP startup configuration. The installed replacement is now pinned by report
0ab05d939bcbdc7eb1d0905156790a42a1db4375b5bb1ac63d6106cb95d563fc, with unchanged
permissions, personal ChatGPT and six total prompts. The official authentication
handoff was reached at 00:06:29 UTC on October 20; user verification and subsequent
acceptance/cleanup are pending. [Current evidence](installed-host-test.md#empty-mcp-repair-and-authentication-handoff).
Older rejected/inactive status below is historical; G2 remains separately held.

The repaired candidate pinned by report
b887864ad524a4cd312130b1f55cc2eb7abfec666a19dcaeb50ad29541fbecae is now installed
and stopped. The owner extended testing time as needed at 23:43:54 UTC
(Sentinel_15479ff398d081918411da08295352de); the original deadline below is
historical. The same six-prompt total, personal ChatGPT selection, unchanged
permissions and cleanup apply. A changed hash alone is not a new approval gate.
Automatic approval review nevertheless rejected startup activation before execution,
including reconsideration using the recorded prior approval. It requires explicit
trusted confirmation of persistent activation. No registration, pairing or sign-in
occurred; exact absence and normal Quit passed. See the
[continuation evidence](installed-host-test.md#extended-time-continuation).
No G2 upload approval follows.

**Historical proposal, approved and attempted; cleanup complete.** October 19, 2026.
See [the attempted test outcome](installed-host-test.md). It stopped before pairing
and provider sign-in; no allowance or clock may be reset. The details below preserve
the original approval scope. Spec 059 / decision 059.
The existing development provisioning approval is fulfilled. This proposal is the
separate persistent startup, network and provider-credential scope required by
CLAUDE.md: “Mandatory per-action security, credential/persistent-access approvals”.
The G2 CSR upload is separate and remains held pending the parent's approval flow.

## Artifact to authorize

Install the exact inert candidate at `/Users/tobiasgunn/Applications/Fort.app`,
preserving the task-owned current copy, then perform one attended Mac-only test:

`/tmp/fort059-provider-proposal-quit-signed-20261019/Fort.app`

- 206 files / 406,509,378 bytes / 30 Mach-O objects.
- `signed-package-report.json` SHA256:
  `983ccb10307b07256daba171bee084568fc82d0e8c1772d4c3dffec0a7d1e439`.
- Payload manifest SHA256:
  `0c0023122c1d57818917559eff2334087e1e5a59a31f7ea03b9ce1cc27aa2ebc`.
- Source base `7a0713642acb921ae54b98bc5273e98095d8c133`; report pins the exact
  MessengerView source hash for the setup-sheet Quit/Close correction.
- Every file/hash/mode, exact embedded profiles/entitlements, all 30 signatures and
  public signing leaf fingerprints, and native bundle inspection passed. This
  candidate has not been installed or launched. Client-only installed evidence
  belongs to the separately hashed candidate in [Quit/Close evidence](installed-setup-quit.md).

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

One-time attended test only, **30 minutes from first Enable**, up to **six synthetic plain
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
and supported removal remain acceptance gates. The setup-sheet Quit defect is now
fixed. Three client-only installed cycles
passed six normal Quits, nine setup close/reopens and three window close/reopens,
without activation. This is not host/provider lifecycle acceptance. New or changed
signed bytes must be repinned and assessed before activation rather than silently
treated as the artifact approved here.

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

The parent should present this once with the refreshed exact artifact and its
remaining activation gates. The earlier candidate hashes are superseded for this
proposal; preserve their evidence. This document does not itself grant approval.
