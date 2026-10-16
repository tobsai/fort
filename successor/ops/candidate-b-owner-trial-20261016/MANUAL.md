# Owner trial — one start, up to five human-entered messages

Executor: UI task `01a1451b-81d7-76cf-8e5c-a85d7164201c` only.
Parent: `01a0f307-3940-7333-96b0-a3b0b854e0e8`.
Authority: [21:52 UTC owner approval](../../evidence/2026-10-16-candidate-b-owner-trial/approval.json).
Verify [exact source/app/runtime pins](../../evidence/2026-10-16-candidate-b-owner-trial/pins.json).
Preparation has not started the runtime, enrolled a device or sent a message.

1. Prepare CUA handles before starting. Verify the pinned app is exited, the old
   native cache is safely archived as recorded in `native-cache-preservation.json`,
   and `~/Library/Application Support/FortCandidateB` is absent or empty. The new
   root `/Users/tobiasgunn/dev/fort/.local/prototype-candidate-b-owner-trial-20261016`
   must contain no ledger/state/lock/enrollment. Preserve unexpected files and stop;
   never reset or substitute another root. Do not touch any of the four old packets.
2. Run the command below in a PTY. Type `start` once. This starts the ten-minute clock:
   nine minutes for preflight/pairing/chat and one minute for cleanup. Any startup
   failure consumes the start. Record only the redacted stage/check and cleanup
   outcome; no retry or settings change. Keep the PTY open while the owner chats.
3. Open the exact pinned FortMac app shown by the controller. Pair automatically in
   **Connection** from the new root's `local-enrollment.json`. Keep values in
   non-emitting CUA variables; if using TextEdit, bind it before opening the file,
   use `getAXState({emit:false})`, and close without saving. Never expose enrollment
   values in logs/chat/screenshots or read/copy provider credentials.
4. In **Workspace**, create only **Writer**, model **gpt-6-astra**, instructions
   **Reply briefly using supplied context.** Open Writer Home and select **no skill**.
   **Immediately notify the parent**, before waiting or doing further inspection:
   “Ready for you to type in Writer Home. Up to five messages; wait for each reply.
   Chat closes at <exact workCutoff>; cleanup/credential expiry is <exact
   credentialExpiry>. No messages have been sent.” Use the controller's exact ISO
   timestamps. Bring the app forward and leave the composer ready for the owner.
5. **Only the owner types/sends messages. Send no sample or corrective prompt.**
   Supervise without taking over the composer. `status` reads bounded receipt/run
   metadata without consuming a message. The controller automatically stops the app,
   runtime and listener at the work cutoff and performs scoped credential cleanup.
   Use `done` for an owner-requested early finish or `abort` on failure. Do not relaunch.
6. Report the final phase, starts=1, receipt/run count (0–5), pending count, native/
   runtime/listener exit and credential cleanup by the exact expiry. Preserve the
   resulting ledger/cache. Capture only safe UI evidence; avoid reproducing the
   owner's message bodies in evidence. Any failed/late/uncertain check remains blocked.

```sh
cd /Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016
/opt/homebrew/opt/node@26/bin/node successor/prototype/manual.ts
```

App: `successor/.build/candidate-b-review-mac/Build/Products/Debug/FortMac.app`.
Use this build only; no rebuild is needed. Protected password/Touch ID/Keychain
prompts require the owner. Do not bypass them or relax a startup check.

The host has the Mac user's access; the no-files computer-tool profile is not host
isolation. Pre-opened descriptors remain usable and full OS policy enforcement is
unverified. Keep the same permissions, existing ChatGPT sign-in and Astra. No new
sign-in, OS/security changes, provider cleanup, permanent services, public ingress,
deployment or restart is authorized.
