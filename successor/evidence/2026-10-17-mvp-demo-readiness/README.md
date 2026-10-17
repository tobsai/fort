# Demo readiness — October 17, 2026

The native prototype has a working, bounded Mac/provider path from the completed
batch and passing local recovery/routine/schedule fixtures. It is not a completed
MVP. The new transcript layout and read-only preview build successfully, but their
installed screenshot/accessibility check did not execute because macOS UI automation
timed out before the test began. No permissions or OS settings were changed.

Source commit: `6fe7120919b660ffc030521839cba8a8ba23706e` on
`codex/fort-openai-rebuild-20261016`. The source/app file hashes are in
[checks.json](checks.json).

## Completed live batch

The coordinator relayed the designated UI operator's completed session-3 outcome.
The exact operator task is `01a1451b-81d7-76cf-8e5c-a85d7164201c`, completed turn
`01a14ab9-5563-7252-b725-63d5bc2f9c51`. Its reported observations include:

- Writer answered the requested notebook question. One Reviewer turn was canceled
  without a reply and never replayed. A new Reviewer turn completed after restart.
- Supported app quit, native exit confirmation and service/runtime restart worked.
  Relaunch retained the same installation and history without reenrollment.
- Both recipients recalled the shared-memory fact from the same immutable revision;
  their individual Homes had empty memory. The final checklist completed before
  deliberate user cancellation could take effect.
- Cleanup finished before expiry. No further session was started by this work.

The immutable read-only ledger checks in [batch-outcome.json](batch-outcome.json)
independently show ten accepted requests across three sessions: nine completed and
one canceled. Session 3 has six requests, five completed runs, one canceled run,
eleven nonempty message bodies, two launches and one restart. Evidence contains
message metadata, never message bodies or credentials. All three ledgers have zero
device/enrollment rows. Session-3 state SHA-256 is
`dc8b65b019c22e559b70dbbcdd23ae4817643e506595555806f610414a8a8d7f`.

Sessions 1 and 2 retain their original blocked states and separate cleanup
reconciliations. Session 3 records `finished` and confirmed cleanup. The original
approval, scope digest, activation pins, old app build and every consumed packet
remain intact. No fourth session or unused allowance is available. The completed
batch used native source `996e19849f5104e71b26b7c14d201aaa56b31c87` and launcher
source `e5a4ef54e67cfd2df506f42b191b8f80378581d8`.

## Transcript finding and correction

The status-only final viewport is not evidence of missing message bodies. The
ledger has every saved body. Source inspection found that the view rendered all
messages, followed by every run status, then scrolled to the end. Six detached run
rows can fill the viewport below the latest reply. We did not receive the original
operator screenshot pixels, so the precise live scroll position is not independently
verified here.

The old rendering order was extracted unchanged into the presentation seam used by
the actual view. [The regression failed](transcript-order-red.txt): the final two
rows were both run statuses. [It passes after the fix](transcript-order-green.txt).
Each status now follows its request's last message, with its approval controls.
Tests preserve interleaved message order, exact text, canceled requests without
replies, orphan runs/approvals and unique presentation IDs. No ledger, API or replay
behavior changed. Pixel-level viewport acceptance is still required.

## Offline native preview

Open [open-preview.command](open-preview.command), or run:

```sh
/usr/bin/open -n /tmp/fort058-offline-preview-mac-20261017/Build/Products/Debug/FortMac.app --args --offline-preview
```

The Debug-only launch path builds a labeled, six-turn sample conversation before
looking up the normal application-support directory. It uses a unique temporary
directory and an inert credential implementation. Restore, enrollment and command
submission are disabled. There is no client, polling loop, runtime, provider login,
pairing or existing transcript access. Send is disabled and Connection is hidden.
The sample includes five replies and one canceled request. The wrapper is specific
to this local build; the build path is temporary. It does not launch automatically.

Use the command with `--offline-preview`; launching the ordinary app directly uses
its normal connection workflow. This is a read-only conversation preview, not an
interactive service demo. No installed pixel/accessibility result is claimed.

## Checks

| Check | Exact result |
| --- | --- |
| Fixture service and Codex adapter tests | **91 passed, 0 failed**: `node --test service/openai.test.ts runner/codex-chat.test.ts`; synthetic runtimes and temporary test-only enrollment, no provider |
| Swift suite | **40 passed, 0 failed**, including pending cancellation/lost-response/relaunch, native routine pending replay and three transcript presentation tests |
| Offline preview guard | Observed compile red before implementation; green proves restore/enroll/send/refresh leave no connection files and cannot send |
| Transcript ordering | Observed behavioral red under old layout; green after fix |
| Mac and iPhone Simulator builds | Both **BUILD SUCCEEDED** with Xcode 27.1; separate fresh derived-data directories |
| Code signatures | Both pass `codesign --verify --deep --strict` in read-only approved verification; initial sandboxed Mac verification could not establish certificate trust |
| Corrected UI test compilation | **TEST BUILD SUCCEEDED**; no test execution |
| Native UI regression | **Blocked before test execution**: timed out enabling automation mode; no corrected screenshot/accessibility result. The runner exited afterward |
| Simulator runtime | Build succeeded despite an out-of-date CoreSimulator service warning; no Simulator UI launch was attempted |
| Preservation | All 32 current batch/cache files retain bytes, size and mtime; all 12 historical packet file hashes and six completed-batch Mac app file hashes match; main checkout status unchanged |
| Cleanup observation | Exact recorded ports 54275/55280/55651 closed; app, UI runner and pinned runtime absent; active lock, local enrollment files and native connection file absent |

The fixture run includes routine checkpoint restart, owner-gate expiry, cancellation
barriers, immutable configuration, exact schedule consent, occurrence deduplication,
multi-year downtime, and queued/active cancellation races where completion or
uncertainty can win. These are fixture results, not live routine/schedule signoff.
See [native-verification.json](native-verification.json),
[preservation.json](preservation.json), test/build logs and the local source/build
hashes recorded with this evidence. Checked-in text logs have trailing whitespace
and final blank lines normalized; original byte hashes are retained in `checks.json`.

Unrun in this revision: full TypeScript suite and fresh TypeScript checking
(TypeScript source is unchanged), live provider invocation, native HTTP journey,
corrected installed UI inspection, physical iPhone, skill B invocation/export,
actual-provider routine/schedule execution and deliberate user cancellation.
The prior automatic approval rejection of the older broad fixture run is historical;
the latest delegated request explicitly authorized fixtures, and this 91-test run
was approved. No earlier rejection was bypassed.

## Smallest next live acceptance plan — not activated

First inspect this exact offline Mac build through accessibility and a screenshot,
scroll to the bottom, and verify the latest sample reply remains visible beside its
status. This needs no provider, pairing or consumed packet. Resolve the automation
availability issue through the supported UI operator; do not weaken security settings.

Then prepare one new bounded Mac acceptance packet for a future explicit decision:
one fresh expiring local device enrollment; ten minutes including a final one-minute
cleanup reserve; at most five accepted Fort requests and six provider-turn attempts;
at most two owned runtime launches with one explicit restart and the existing
per-process ceiling. Preserve all previous history. Use the existing exact runtime,
ChatGPT sign-in, selected `gpt-6-astra` and no-tools profile. No new provider sign-in,
spending, startup registration or permission changes belong in that packet.

| Step | Bound and evidence |
| --- | --- |
| Skill B | Invoke one reviewed revised instruction bundle; inspect the saved configuration/output, export its complete bundle and compare the digest |
| Manual routine | One request with two text-only task steps separated by an explicit owner gate; inspect pinned revisions, gate decision and durable outputs |
| One-time schedule | Explicit preview/enable consent for one occurrence of a one-task text routine; inspect its occurrence key and single receipt, then pause it |
| User cancellation | One bounded request; press Cancel while active, record the exact terminal outcome, and send one follow-up only after confirmed cancellation/closure permits it |
| Finish | Capture final viewport/accessibility, quit via supported UI, confirm native exit, stop the session, verify cleanup and immutable history |

If cancellation loses to completion, record it as unproven and stop; do not extend
the budget or replay. Report effective model only if the runtime supplies evidence;
selected `gpt-6-astra` is not attestation. The one scheduled dispatch and routine
turns require an explicit extension beyond the consumed plain-text batch's scope.
This note creates no packet, approval, enrollment or scheduled job.

Physical iPhone/shared history and read state, sleep/reboot behavior, host confinement,
backup/restore, notifications, accessibility and distribution remain separate MVP
gates. The agent host still has the Mac user's access; computer-tool restrictions
do not establish host isolation. The quarantined Apache-2.0 donor remains separate
from this newly authored successor. No merge, push, deployment or provider cleanup
occurred.
