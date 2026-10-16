# Candidate B — bounded manual-test decision packet

**Prepared only. No execution decision has been recorded.** This packet proposes
the replacement candidate's two supervised sessions. It does not enable the
runtime, create credentials, consume a start, or transfer Packet A's approval.

The [independent follow-up review](../../evidence/2026-10-16-candidate-b-independent-acceptance.md)
confirms the cancellation, idle-disposal and stale-form findings are resolved,
with no new material code blocker in the targeted scope. The reviewer reran 15
focused tests and verified every source/app hash. This is code review, not live
runtime or native acceptance.

## Exact candidate

| Item | Pin |
| --- | --- |
| Branch | `codex/fort-openai-rebuild-20261016` |
| Worktree | `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016` |
| Reviewed source | `73f644425a74463488cdce61aab8b83384ce8722` |
| Reviewed evidence checkpoint | `772410e5b59a97d203171922d3802621550fee19` |
| Source-list SHA-256 | `0fb368a0e358a176db114a1085d83996af8c488b2f32d503eebd137970be6e45` |
| App-list SHA-256 | `9c32247c3c77b1618515aec7faad3944826ec9e11d4ecc0d58483c8ecd23ad2b` |
| App | `successor/.build/candidate-b-review-mac/Build/Products/Debug/FortMac.app` |
| Official runtime | `/opt/homebrew/Caskroom/codex/0.157.1/bin/codex` |
| Runtime SHA-256 | `27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d` |
| Requested model / account class | `gpt-6-astra` / existing ChatGPT sign-in |

Full file pins and existing validation are in
[pins.json](../../evidence/2026-10-16-candidate-b-review-fixes/pins.json).
The app was compiled at `de9e482`; its Apple source is unchanged at the reviewed
source pin. It has a linker ad-hoc signature, no team identity, bound Info.plist
or sealed resources. Launch and Keychain compatibility have not been tested.

## Proposed decision and boundaries

Approve only the five exact samples in [MANUAL.md](MANUAL.md), through this Mac
candidate and the existing provider-owned ChatGPT session. At most two supervised
session starts, at most 300 seconds each, including runtime preflight and cleanup.
The runtime work cutoff is no later than 296 seconds, leaving two seconds for
TERM and two for KILL. These are timer-based bounds, not hard-real-time guarantees.
No extra diagnostic prompt or separate provider preflight process is permitted.
A failed first preflight consumes the first start and prevents the second.

Session one contains samples 1–4. Session two is conditional: four distinct
completed receipts retained, the **actual native pending queue empty**, and
service/runtime/app cleanup confirmed. Otherwise stop. Session two repeats the
no-turn checks, restores the same installation and sends only sample 5. Do not
manually replay an earlier sample or use the synthetic harness as the live client.

This would authorize one fresh local Fort enrollment and one device credential
stored by the Mac app in Keychain, a fresh candidate SQLite ledger, and one
temporary loopback listener. The enrollment code expires after five minutes and
is single-use; the device credential currently expires after **30 days**, not at
session end. Its hash remains in the ledger and its value remains in Keychain
after shutdown. Stopping the service closes the endpoint; it does not revoke or
erase that credential. Do not print either value in evidence. This preparation
has created neither. No provider credential is created, copied or printed.

Only the provider's own runtime may use the existing ChatGPT store during a
later authorized preflight/test. No sign-in, refresh command, API-key fallback,
account switch, model substitution, credit purchase, new integration, deployment,
startup service, OS security change, provider cleanup or shared hosting is in
scope. Provider-owned normal session maintenance may write its own state; this
is not a promise that starting it is read-only.

## Risk comparison with the previously accepted prototype

The parent delegation records that the user already accepted residual same-user
host-access risk. This packet does not reopen the full OS-isolation decision or
require unrelated hardening. That acceptance is context, not execution authority
for changed pins.

| Area | Replacement candidate and change |
| --- | --- |
| Host authority | Still runs as the owner. `HOME=/Users/tobiasgunn` remains so the official runtime can find its existing provider store. It can also discover owner configuration, files and integrations. There is no demonstrated host filesystem, Keychain, IPC or non-provider network confinement. This is the same declared risk class; the lost artifact prevents a byte-for-byte risk comparison. |
| Ambient MCP | Only `spritepipe`, `node_repl` and `computer-use` are explicitly disabled by name. The complete effective set is unknown. Other enabled servers must be identified **before thread creation or MCP discovery**; any enabled/unknown entry blocks this packet. Host-risk acceptance does not authorize connecting a sensitive account or arbitrary server. |
| Requested tool policy | `untrusted`, read-only and network-disabled computer-tool policy are requested. Extra empty-environment/tool/root fields and restricted-read fields may be ignored. They are not host containment. Tool-event rejection may occur after a side effect. No claim of tools being impossible is made. |
| Lifecycle | Improved by the reviewed fixes: cancellation fence, disposal when idle, bounded TERM/KILL, observed child close and owned group absence. Escaped descendants are not contained; cleanup is still unproven against the real runtime. |
| Native artifact | Newly reconstructed app with new hashes and linker-only ad-hoc signing; the previously reviewed signed app was lost. Native launch, credential storage and visual behavior need actual inspection. |
| Model evidence | Exact `gpt-6-astra` availability and thread echo must pass. The candidate does not attest the provider's effective serving model; `effectiveModel` remains null. |

## Exact remaining prerequisites

1. **Preparation gap:** the always-refusing activation gate remains intact. The
   adapter currently combines initialization, account/model checks, thread creation
   and turn dispatch. There is no separate completed preflight receipt or attended
   session controller. Before execution, prepare and nonlive-test the minimal
   candidate-specific wiring described in [PREFLIGHT.md](PREFLIGHT.md), keeping it
   disabled. Pin and review that source delta. This packet is reviewable scope,
   not a claim that the current source is an executable runbook. Do not bypass the
   gate or run a standalone preflight process as a shortcut.
2. **Runtime evidence:** prove the exact arguments are accepted, effective
   provider/configuration and full MCP set meet the packet, existing ChatGPT auth
   is available, and Astra is selectable. The preflight itself needs later
   execution authority because it starts the official host and may access its
   provider store/network. Stop before `turn/start` on uncertainty.
3. **Owner action:** an explicit candidate-specific bounded execution decision,
   including the one local Fort credential, followed by readiness to attend both
   short sessions. No new provider login is requested. The user opens the pinned
   app, enrolls locally, sends the five samples and reports visual observations.
   A normal macOS prompt may need their interaction; never bypass it. If the app
   cannot use Keychain or launch under its current signature, stop and report the
   exact failure. Do not silently sign, replace or rebuild the approved app.

An unchanged-scope implementation revision requires pin verification, not replay
of historical approval bodies. A new scope or materially changed risk must be
reported. At present, both the live decision and the final execution-wiring pin
are absent; no start is authorized. Manual observations will be labelled
user-reported and will not satisfy screenshot/accessibility, physical-iPhone,
release or full Spec 058 isolation acceptance.

## Preparation checks

See [validation.json](validation.json) for the original-review copy, source/app
hash verification, exact sample checks, unchanged main checkout and disabled gate.
The machine-readable scope is [packet.json](packet.json). No application tests
were rerun for these documentation-only additions. No runtime/configuration/auth
preflight or native launch has occurred.
