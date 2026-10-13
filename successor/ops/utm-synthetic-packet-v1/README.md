# UTM synthetic VM packet v1 — inert review draft

This packet defines **one owner-attended, synthetic-only Apple Virtualization
trial** using the inspected UTM 4.7.5 (118) distribution. It is complete as a
planning/review artifact. It is **not an executable or execution-ready bundle**:
there is no launch script, importable `.utm` package, `config.plist`, cloud-config,
seed image or runtime controller here. `scope.json` explicitly records
`execution_authorized=false`, `ready_for_execution=false` and
`approval_request_ready_to_send=false`.

The current task authorizes repository preparation only. No new payload was
downloaded; no UTM app, verifier, guest, experiment or provider was run. No host
task root, account, key, permission or security exception was created. Read-only
local source/evidence inspection and existing-tool entrypoint hashing support
this draft. Previous packets, approvals, source and reviewer-owned files remain
unchanged.

## What is now established

App inspection at `baadd3edcac5ffd02d97f90b66c12a879bf2366d` established the exact
DMG hash/size, valid stapled ticket, app version/build, Developer ID signature and
Gatekeeper **Notarized Developer ID** acceptance. Signer: **Turing Software, LLC**,
Team ID **WDNLXAD4W8**. All 80 Mach-O files, 160 architecture signatures and 78
code bundles passed static inspection. The 1,180-entry app manifest and all public
receipts remain linked and hashed. The image/app were cleaned afterwards.

The earlier outer-DMG primary-signature rejection remains recorded. The accepted
inspection exception permits examining the exact pinned container; it is not a
macOS security exception or authorization to launch the app. A future consolidated
trial request must explicitly carry that scope through while retaining the DMG
ticket, exact app manifest, signer and fresh app/nested-code trust checks.

## Review map

| File | Review purpose |
| --- | --- |
| [scope.json](scope.json) | Inert status, one-trial limits, exact paths, resource caps and exclusions |
| [inputs.json](inputs.json) | Nine exact URLs/digests/caps, verified app/binary pins, existing tool identities and unresolved archive-member binding |
| [vm-design.json](vm-design.json) | Device/boot/storage contract; explicit zero NICs and shares; no importable configuration |
| [protocol.json](protocol.json) | Two-PTY identity, framing, fixed sequence, deadlines and required negative tests |
| [canaries.json](canaries.json) | Eleven measured synthetic cases, positive controls, actual unit restrictions and claim limits |
| [RUNBOOK.md](RUNBOOK.md) | Ordered future stages, evidence gates and bounded stop/cleanup procedure |
| [HOST-EFFECTS.md](HOST-EFFECTS.md) | Plain-language host changes, owner actions, permission handling and residue limits |
| [REVIEW-GATES.md](REVIEW-GATES.md) | Source work and independent acceptance needed before presenting execution approval |
| [APPROVAL-REQUEST.md](APPROVAL-REQUEST.md) | One consolidated owner request, draft only; not sent |
| [provenance.json](provenance.json) | Exact retained inputs and important changes from the earlier proposal |
| [validation.json](validation.json), [seal.json](seal.json) | This preparation's static checks and content identity, not runtime acceptance |

The largest changes from the proposal are measured app identity/footprint,
concrete signed container candidates, explicit positive controls for every
denial, exclusion of Lima DHCP/SSH assumptions and a bounded two-channel protocol.
The inspected UTM source defaults a new configuration to **one shared NIC**.
The future renderer must encode and reread `Network=[]`; default construction
is unacceptable.

## Intended result and explicit limits

The one nominal session would establish only the measured synthetic guest UID,
namespace, device, IPC/network and cgroup-lifetime behavior. The Mac owner,
same-user host processes, signed UTM app, Apple Virtualization, guest root and
fixed broker are trusted. The tool is the untrusted guest process. A PTY path,
UUID or nonce is correlation, not cryptographic authentication.

Signing/notarization does not establish confinement. The app includes QEMU and
JIT/network-capable helpers even when the VM uses Apple Virtualization. No QEMU
backend/converter, shared hosting, provider credentials, real prompts or real
agent runtime are included. The guest shares its read-only OS root; this is not
a private per-tool root or full provider-store isolation. Host Keychain is never
read. No T07, crash-fault matrix, unattended lifecycle, deployment or release
acceptance follows from this trial.

Guest archive contents/bootability, NoCloud/PTY implementation and first-launch
state/process cleanup remain unresolved. The review should first accept or amend
this exact design; source implementation and behavioral tests must close the
pre-approval gates. Only then can the parent present the single consolidated
request bound to the reviewed execution commit/seal. A plan seal alone cannot
authorize subsequently written code.
