# Interactive owner demo — one approved start

Operator: `01a14bac-6acb-70be-926a-23ab4584a0f1`. Parent:
`01a0f307-3940-7333-96b0-a3b0b854e0e8`. Approval: owner message
`Sentinel_f671f601a3148191bae54d2de3b281c1`, 2026-10-17 21:00:19 UTC.
Read scope.json and owner-approval.json. Do not reuse any old launcher or packet.

The user is waiting to type in Fort. Prepare CUA, quit any existing Fort preview
through its supported UI, and have the private pairing flow ready before start.
Do not open a file:// chat link or the offline preview. Do not launch a provider
or issue enrollment during the readiness check.

Working directory:
`/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016/successor`

Read-only preparation check:

```sh
/opt/homebrew/opt/node@26/bin/node prototype/owner-demo-manual.ts --check
```

Before activation, check exact-root write access without a session, credential,
runtime or timer:

```sh
/opt/homebrew/opt/node@26/bin/node prototype/owner-demo-manual.ts --check-writes
```

The operator's default workspace is outside Fort. Use the supported
`exec_command` permission mechanism (`sandbox_permissions: "require_escalated"`)
for this write check and the attended launcher below. Scope the justification to
the existing approved Fort demo: exact session-root writes, existing native cache
archive, one bounded runtime/local enrollment and its cleanup. Do not change OS
permissions, security settings, the runtime profile or credential lifetime. Honor
any platform approval outcome; a denied escalation is not permission to bypass it.

When ready, run an attended PTY terminal and leave it open:

```sh
/opt/homebrew/opt/node@26/bin/node prototype/owner-demo-manual.ts
```

1. `native-status` must confirm exit before activation. Only then type `start`.
   The durable session directory starts the one-use attempt; its initial state
   establishes the 600-second absolute deadline. A refused pre-start filesystem
   check creates no session and consumes no runtime, credential or demo time.
2. The launcher archives the existing native cache, performs restricted exact
   runtime preflight, issues one expiring local enrollment and directly opens
   `/tmp/fort058-offline-preview-mac-20261017/Build/Products/Debug/FortMac.app`
   with **no arguments**. The directory name is historical; without
   `--offline-preview` this build is the normal interactive app.
3. Privately use `pairingFile` from READY to fill the normal Connection UI. Never
   print credential values, screenshot pairing values or copy provider credentials.
   Create one Agent named `Writer`, model `gpt-6-astra`, instructions exactly
   `Reply briefly using supplied context.` Open Writer Home; choose no skill and
   add no memory. Do not start routines, schedules or additional conversations.
4. Immediately focus the enabled composer and tell the owner it is ready with
   READY's exact work cutoff and expiry. The owner can send up to twelve ordinary
   plain-text requests. Do not spend requests on unsolicited operator samples.
5. Keep the operator attended. `status` prints run metadata only. There is no
   restart allowance. At completion quit Fort through its UI, check `native-status`,
   then type `done`. `abort` is available on failure. At 540 seconds the controller
   automatically stops work and uses the last minute for native/runtime shutdown,
   credential revocation and issued Keychain/connection/pairing cleanup.
6. Report final state and cleanup evidence with no message bodies. Once the
   session directory or lock exists, preserve it and do not retry/reset. A failure
   before either exists is not a consumed live session. The launcher reports its
   stage, allowlisted error code and durable-state classification without raw
   exceptions. If it reports EPERM/EACCES and no session, retry the same approved
   action with supported permission escalation after --check-writes succeeds.

This authorizes existing ChatGPT/Astra only, no sign-in, billing, new tools,
provider accounts, startup services or security changes. The host still has the
Mac user's access; the no-tool profile does not establish host confinement.
Longer use requires a separate credential-duration/lifecycle decision. Do not
extend or renew this packet. Existing nine consumed packets remain preserved.

The first operator attempt exited before creating the root. Implementation-owner
inspection and unchanged native/cache evidence confirmed zero runtime launches,
enrollments and elapsed demo time. This repair preserves that attempt's pins in
`evidence/2026-10-17-owner-demo-startup-repair/initial-pins.json`; scope and owner
approval bytes are unchanged. The exact original exception was suppressed, so its
filesystem cause remains a hypothesis until the operator runs the write check.
