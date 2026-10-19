# Local provider credential removal — October 19, 2026

Governing [Spec 059](../../../specs/059-modular-node-v1.md), decision 059 and the
coordinator's explicit continuation to repair the cleanup finding without live
access. Base `41b650a227ad6b968e7df325466398bc4aa0a953` on
`codex/fort-openai-rebuild-20261016`. This slice changes native source and synthetic
fixtures; no real credential, account, startup registration or Apple state was used.

## Implemented behavior

Ordinary Mac removal now requires four confirmations: owned host stopped, pairing
revoked, startup unregistered, and Fort's local provider credential removed. Only
then can it remove the device Keychain token and prepare the exact app URL for
uninstallation. A missing provider-cleanup proof fails the generic lifecycle seam.
The Mac callback runs after recovery/runtime disposal, helper exit and a final
startup-status check, while the existing owner-operation lease is held. Failed
cleanup retains retriable progress. Every retry checks cleanup again, including
already-confirmed journals; no host or account is restarted. Ordinary disconnect
and stop preserve the provider login.

The callback resolves only the verified app's App Group, inspects the existing
`FortHost` root, and deletes only `Runtime/state/auth.json`. Directory descriptors,
no-follow opens, owner/mode/ACL checks, inode/link validation and a final absence
check constrain deletion to this private file. Credential access uses `O_EVTONLY`
metadata operations: contents are neither read nor decoded. Hardlinks, symlinks
(including broken links), special files, executable/shared files, extended ACLs and
unsafe/missing runtime directories are refused without repair. Missing auth is
idempotent; the directory is fsynced even on an absence retry. History, runtime
caches and every other file remain unchanged. This does not promise secure erasure
of filesystem snapshots or protection against an adversarial same-user race at the
last unlink instruction. No general directory traversal or recursive deletion exists.

The official pinned [Codex file auth store](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/login/src/auth/storage.rs)
uses that exact filename for file-backed credentials. Fort's launch plan fixes
CODEX_HOME to its private state directory and selects file storage. No Keychain or
ambient Codex store is inspected. The official account logout path also attempts
provider revocation, so this offline cleanup deliberately does not invoke it.
Local file absence and remote revocation are different outcomes.

Native removal confirmation and result copy now state local saved sign-in removal,
retained conversation/provider history and unchanged provider-side grants. Actual
installed UI inspection is still unrun; the source/model tests and unsigned builds
are not that signoff.

## Account correlation remains unresolved

The pinned [account protocol](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/v2/account.rs)
provides optional ChatGPT email and experimental `workspaceRouting.chatgptAccountId`.
These are supported nonsecret evidence fields, not tokens. Fort's current account
result carries only a result kind and does not expose, store or compare those values
with an owner-selected identity. An email alone would not establish the workspace.
The native result now says the runtime reports signed in and **account identity is
not verified**. A public model test confirms that label without starting a login.

The pinned [account-read implementation](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server/src/request_processors/account_processor/workspace_routing.rs)
can perform accounts/routing discovery and unauthorized-auth recovery even when
`refreshToken:false` suppresses proactive refresh. Therefore it was not executed
against an existing account as an allegedly offline metadata check. Only public
source text was fetched. No Apple email was adopted as a ChatGPT identity.

Remaining: resolve the intended account with the owner, carry validated nonsecret
email/workspace evidence through the existing account/executor/native flow, and
correlate/pin that exact selected account before dispatch and after restart. Missing
or changed identity must remain unresolved/refused. That integration and actual
account acceptance are not implemented by this cleanup slice. No session/account
switch, credential parsing or broader account grant was introduced.

## Verification

| Check | Observed result |
| --- | --- |
| Initial public removal test | Failed: synthetic auth remained after confirmed host removal; passed after cleanup composition |
| Unsafe credential and native status cases | Failed for executable auth and missing unverified-identity label; corrected and passed |
| Retry and lifecycle receipt | Synthetic failed cleanup retains recovery phase, retry succeeds without registration, repeat rechecks, missing provider proof refuses app handoff |
| Focused removal/storage/model suite | 32 passed, 1 opt-in budget skipped; 33 reported |
| Complete serial native suite | **326 passed, 16 skipped, 342 reported**, zero issues, 53.223 seconds |
| Isolated cleanup budget | Five warmups, 20 samples: p50 **0.384583 ms**, p95 **0.403667 ms**, max **0.6165 ms**; target p95 <=25 ms |
| Fresh engineering builds | Unsigned Release FortMac and FortHost, unsigned Debug iPhone Simulator all passed |
| Package | 200 files / 404,165,556 bytes; exact membership, every output hash/mode/length and all 198 input hashes/modes independently matched; zero profiles |
| Exact package probe | Passed: public signature inspection refuses unsigned app; explicitly synthetic signature callback reaches real missing-profile refusal |
| Live account/cleanup, installed accessibility/screenshots, Apple/profile/startup, TestFlight | Unrun; no completion claim |

The full native skips are 14 opt-in budgets and two optional package/signature
probes. The new cleanup budget and exact package probe were subsequently supplied
and passed separately. The retained signed probe and unrelated budgets were not
repeated. Unchanged TypeScript/provider suites were not rerun for this native slice.

Intermediate failures remain recorded: two new test assertions initially failed
Swift compilation and were split into correctly awaited/throwing assertions. An
eager App Group inspection added during review broke existing inert setup/reinstall
construction (38 issues); inspection was deferred to explicit confirmed removal.
The affected regression subset and complete native suite then passed. The first
assembly invocation used the shell's nonmatching Node and was refused; explicit
pinned Node 26.5.0 succeeded without changing the tool version gate. CoreSimulator
1171.7.0 remains older than Xcode's required 1174.9.2; Simulator UI was not run or
repaired. These failures are not relabeled as passes.

Toolchain/hardware context remains Node 26.5.0, Swift 6.4/Xcode 27.1, macOS 27.2,
MacBookPro18,2 (10 CPUs, 64 GiB). Timing was isolated from builds/tests and measures
warm metadata/unlink/fsync against synthetic files, not provider/Keychain latency.

## Exact review artifact and logs

New artifact: `/private/tmp/fort059-provider-cleanup-review-20261019/Fort.app`.
It is **unsigned, unlaunched and not a working installed application**. Existing
engineering IDs/team/group and outbound-network-disabled helper are unchanged.
Prior review apps are preserved. Node/guardian/Codex and legal material use the
previously verified inputs; no credential was copied. This new package supersedes
the older package only for source currency, not signing or installed acceptance.

Package report SHA256:
`dabd400861db883297722a8a647a383dd0d8af5c138ba5165822c703bea91caf`.
Independent check: `/tmp/fort059-provider-cleanup-package-verification.json`.
Unchanged host resources: `/tmp/fort059-mac-demo-host-resources-20261019`;
manifest SHA256 `2e7463a21456f569a243de770a1f77bee75a5babbdeee6449b8cce672bfff1a5`.

All new logs have prefix `/tmp/fort059-provider-cleanup-`:
`red.log`, `green.log`, `negative-red.log`, `negative-red2.log`, `focused.log`,
`native.log`, `composition.log`, `native-final.log`, `budget.log`, `samples.json`,
`mac-build.log`, `host-build.log`, `ios-build.log`, `assembly.log`, and
`package-inspection.log`. The three build directories use that prefix plus
`mac-build`, `host-build`, and `ios-build`.

The Apple/1Password handoff was left undisturbed. G2 certificate approval remains
valid but unexecuted; the Program License Agreement remains unapproved. Exact Apple
records/profiles, final signature/entitlements, host/provider activation approval,
account correlation and installed Mac/TestFlight acceptance remain open. No live
provider cleanup, new grant, deployment, merge or push was performed.
