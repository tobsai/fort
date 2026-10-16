# Fourth attempt: one sample, five minutes including cleanup

Executor only: UI task `01a1451b-81d7-76cf-8e5c-a85d7164201c`.
Authority: [owner approval at 21:29 UTC](../../evidence/2026-10-16-candidate-b-fourth/approval.json).
Verify [source/app/runtime pins](../../evidence/2026-10-16-candidate-b-fourth/pins.json).
The preparation task has not started this packet. Preserve all three consumed roots.

1. Prepare CUA handles first. Fully quit the pinned FortMac app. Confirm
   `~/Library/Application Support/FortCandidateB` is absent or empty. The fixed new
   root `/Users/tobiasgunn/dev/fort/.local/prototype-candidate-b-fourth-20261016`
   must have no state, ledger, lock or enrollment. Preserve unexpected contents and
   stop; do not reset or substitute another root. Use the existing approved execution
   permission path if read-only process inspection is sandbox-blocked.
2. In a PTY at the worktree, run the command below. When ready, send `start` **once**.
   Any startup failure consumes it: retain only the redacted `{stage,check}`, state
   and cleanup result, then stop without editing settings or retrying.
3. After readiness, open the exact pinned app below and pair automatically through
   **Connection**, using this root's `local-enrollment.json`. Keep values in CUA
   variables with `getAXState({emit:false})`; bind TextEdit before opening the file
   if using the existing method. Never emit the code, screenshot credential fields,
   read/copy provider credentials or initiate sign-in. Close TextEdit without saving.
4. In **Workspace**, create only **Writer**, model **gpt-6-astra**, instructions
   **Reply briefly using supplied context.** Open Writer Home, select **no skill**,
   and send the one literal sample below once. Expect `cedar`. Capture safe normal-UI
   accessibility/screenshot and receipt/run evidence. An unexpected response or
   failure gets no corrective message, approval override, retry or close-unknown.
5. Quit Fort fully, then send `done`. Work cuts off at 292 seconds; all cleanup must
   finish by 300 seconds. Use `abort` immediately on a failure. Confirm `starts:1`,
   one completed receipt for success, native exit/empty pending, scoped credential
   cleanup and owned-runtime/listener shutdown. Failed/late cleanup remains blocked.

```sh
cd /Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016
/opt/homebrew/opt/node@26/bin/node successor/prototype/manual.ts
```

App: `successor/.build/candidate-b-review-mac/Build/Products/Debug/FortMac.app`

Exact sole sample, Writer Home with no skill:

```text
Remember the sample word cedar. Reply with that word only.
```

The five-minute clock starts with `start` and includes preflight, pairing, the message
and cleanup. `status` emits only bounded run metadata. Terminal closure, SIGINT and
SIGTERM cancel pending startup/running work. One start means no restart after any
outcome. Preserve the fourth ledger/state after cleanup; never reuse an older one.

The owner acknowledged that the agent host has the Mac user's access, pre-opened
descriptors survive pathname restrictions and full generated-policy isolation is
unverified. Keep the exact no-files profile and every remaining check. This approval
does not permit new sign-in, security/OS changes, provider cleanup, services,
deployment, public ingress or another attempt. If a protected password/Touch ID/
Keychain prompt needs the owner, stop and hand off rather than bypassing it.
