# Mac XCTest authorization diagnosis — 2026-10-12

Reviewed source: `51bbeaf9fa683cf2074f0405aebbdbd50bc19526`. Tested app source:
`e47730c025285d1b15c14a5df6d9d277055938a1`. No product code, signing, entitlements
or test orchestration was changed during this diagnosis.

## Current requirement

Apple's `/usr/bin/automationmodetool`, invoked **without arguments**, reports:

```text
Automation Mode is disabled.
This device requires user authentication to enable Automation Mode.
```

Its installed manual (`/usr/share/man/man1/automationmodetool.1`) documents that
the no-argument invocation only prints status and exits. No configuration command
was run. This is a concrete authentication requirement for the next Mac UI test;
the timeout alone did not establish it.

The next dependent action needs the owner at the logged-in Mac for one normal
Fort fixture test invocation. The owner should inspect the actual macOS prompt
for enabling UI Automation for the test session and, if it matches that purpose,
authenticate directly in macOS. Do not send a password to the agent. If a different
permission or scope is requested, stop and preserve the prompt text for review.
The exact prompt was not captured in this diagnostic session, so no button label
or additional Privacy & Security setting is asserted.

Do not disable automation authentication, reset TCC, change Accessibility grants,
enable developer-tools policy, alter Keychain access, remove quarantine, disable
Gatekeeper, kill/restart system authorization daemons, or re-sign as a workaround.

## Comparison and one normal retry

| Observation | Evidence and implication |
| --- | --- |
| Earlier recovery passed | `recovery-4.xcresult` completed five app launches. The Fort runner received automation mode and an accessibility backend endpoint for each launched app. |
| Final app approval/cancel passed | `approval-2.xcresult` used the final `e47730c` app. Signing and ordinary native controls worked. |
| Final recovery lost authorization | `recovery-5.xcresult` received automation mode, completed the first three launches, then failed to load AX on the fourth launch. Xcode's automatic replacement runner timed out enabling automation mode. |
| Artifact identity unchanged | The executable, debug dylib, Info.plist and CodeResources still match all four recorded hashes. Both app and runner have valid signatures. |
| One fresh normal attempt failed | `recovery-6.xcresult` used the same signed app and runner, with no rebuild, policy change, workaround or competing Fort runner. It timed out after 60 seconds enabling automation mode before a test began. No further Mac retry was made. |
| Debugger permission was granted | The fresh attempt's scoped system log reports `task_control_t` authorization success for the test runner. This does not imply UI Automation authentication succeeded. |
| Console session exists | CoreGraphics's documented keys report on-console `1` and login-done `1`. The lock-state key was unavailable, so unlocked state is not claimed. |
| Developer-tools mode disabled | `DevToolsSecurity -status` reports disabled. Its local manual describes debugger/performance-tool authentication policy; this separate setting was not changed or prescribed as a fix. |
| Current automation mode needs authentication | The explicit status above supports an owner-attended attempt. Scoped logs did not identify a TCC denial or a need for a particular permanent grant. |

The original mid-run loss of automation authority is still unexplained. This
diagnosis does **not** prove a product-code defect, an expired authorization, or a
specific persistent permission being revoked. It establishes that a fresh ordinary
invocation did not recover and that the current host requires authentication.
The dependent Mac UI gate stops here until an attended attempt can preserve and
resolve the actual prompt. [Scoped metadata and result](native-2026-10-12/mac-authorization-diagnosis.json).

The authorized next invocation, once the owner is present, can use the existing
harness from this worktree with a fresh result path:

```sh
/opt/homebrew/opt/node@26/bin/node successor/apple/native-ui-check.ts \
  /tmp/fort058-mac-trusted-build/Build/Products/FortMac_macosx27.0-arm64.xctestrun \
  'platform=macOS,arch=arm64' /tmp/fort058-mac-owner-attended-recovery-1.xcresult
```

This command has **not** been run. It requires the owner-attended step above.
All fixture processes from the completed diagnostic attempt exited, and its
harness closed the temporary loopback listeners and removed temporary test input.

## Signing route that resolved the earlier Gatekeeper block

The earlier runner was ad-hoc signed and rejected by Gatekeeper. The successful
route was an ordinary Xcode Debug `build-for-testing` with manual signing for
**both app and test runner**, using the one matching existing identity:

- Apple Development: Tobias Gunn (76KFKR48Y3)
- SHA-1 fingerprint `58F40D1AB480565B89109B853235AC3A7B2E7AD1`
- Team `T3JB5MYZ93` (Maple Tree Enterprises LLC)
- `CODE_SIGNING_ALLOWED=YES`, `CODE_SIGNING_REQUIRED=YES`, `CODE_SIGN_STYLE=Manual`
- Exact identity/team overrides, empty provisioning-profile settings, no
  `-allowProvisioningUpdates`

The chain verifies through Apple Worldwide Developer Relations to Apple Root CA.
The existing key was used by normal signing; no key/certificate was created,
exported or copied, and no new Keychain permission or account grant was made.
No Gatekeeper exception or other security setting changed. The later XCTest
authentication problem is distinct from that resolved artifact launch problem.

## Safe remaining Stage 1 work

The independent review of `51bbeaf` reports both P2 findings closed, with no new
actionable defect. Preserve that fixed review and the passing 46 Node/18 Swift/
strict-TypeScript evidence. Further headless contract/recovery checks and fixture
harness preparation can proceed without Mac UI authorization. The existing
iPhone Simulator route can cover installed unknown-outcome closure independently;
installed two-device races need their own explicit device setup and evidence.
The Mac parts of those journeys and exact-final-artifact recovery remain blocked.

No real provider, credential enrollment, OS-isolation substitution, deployment,
startup registration, release action or provider cleanup is authorized by this
diagnosis. Full Spec 058 acceptance remains incomplete.
