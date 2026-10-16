# Candidate B — third attempt, five minutes total

Only UI task `01a1451b-81d7-76cf-8e5c-a85d7164201c` executes this attempt after
checking the [approval](../../evidence/2026-10-16-candidate-b-third/approval.json)
and [exact source/app pins](../../evidence/2026-10-16-candidate-b-third/pins.json).
Both earlier attempts remain blocked. Do not open, delete, reset or reuse either
state or ledger. Preparation did not start the third attempt or its runtime.

Third-attempt root:
`/Users/tobiasgunn/dev/fort/.local/prototype-candidate-b-third-20261016`.
This root must have no existing state, service ledger or lock before first use.
If any exists, inspect the redacted state and stop; do not delete it or select a
new directory. There is one third-attempt start, with no restart even after success.

## Before starting

Verify the evidence source commit and pinned file bytes, and fully quit the exact
FortMac app. The native directory `~/Library/Application Support/FortCandidateB`
must be absent or empty. Preserve any unexpected contents and stop. Prepare the
UI automation handles before `start`; setup inside the session consumes its time.
The existing TextEdit method may keep the enrollment JSON inside persistent CUA
runtime variables using `getAXState({emit:false})` and transfer those values to
Fort's secure field. Bind TextEdit before opening that file so its initial AX
snapshot cannot emit the code. Never return or print the code, screenshot it,
copy provider credentials, or change OS permissions. If a protected password,
Touch ID or Keychain prompt requires the owner, stop and hand it off.

Run with a PTY in the candidate worktree:

```sh
cd /Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016
/opt/homebrew/opt/node@26/bin/node successor/prototype/manual.ts
```

Use the approved execution permission path if the sandbox prevents the read-only
process-status probe; do not weaken the freshness check. Previous default-sandbox
`pgrep` failed while the elevated probe confirmed absence. No OS setting change
is needed or authorized. Creating the terminal controller verifies pins and
prepares directories; the runtime and bounded clock begin only on `start`.
When the sole UI operator is ready, type `start` exactly once.

If preflight fails, collect only its allowlisted `{stage, check}`, retained state,
exit/cleanup evidence and elapsed time. Do not start again. A preflight failure
with zero messages is a valid diagnostic result. Do not change settings to force
acceptance. The proven feature-alias mismatch is fixed; canonical controls must still be
explicitly false. Later live checks remain unverified.

## If readiness is published

Open only:
`successor/.build/candidate-b-review-mac/Build/Products/Debug/FortMac.app`.
Enroll through **Connection** using the new root's `local-enrollment.json`.
Transfer the loopback address, installation ID and one-time code through UI
variables without emitting the code. Close TextEdit without saving. Do not
initiate any provider sign-in. Create these exact items in **Workspace**:

| Item | Name | Model / instructions / participants |
| --- | --- | --- |
| Agent | Writer | `gpt-6-astra`; `Reply briefly using supplied context.` |
| Agent | Reviewer | `gpt-6-astra`; `Check the sample claim.` |
| Skill | Check marker | `End with [CHECKED].` |
| Conversation | Sample planning | Writer and Reviewer |

Send each approved message once, in this order, within the same session. Wait
for a durable completed run after each. Use Check marker only for sample 3.

| # | Conversation / recipient | Exact message |
| --- | --- | --- |
| 1 | Writer Home / Writer | `Remember the sample word cedar. Reply with that word only.` |
| 2 | Reviewer Home / Reviewer | `In one sentence, explain why a checklist is useful.` |
| 3 | Writer Home / Writer, Check marker | `Suggest one short title for a sample packing list.` |
| 4 | Sample planning / explicitly Reviewer | `Our sample plan is to pack a red notebook. Repeat the item.` |
| 5 | Writer Home / Writer, no skill | `What sample word did I ask you to remember earlier? Reply with that word only.` |

Expected observations remain `cedar`, one short checklist sentence, a title ending
`[CHECKED]`, the red notebook, then `cedar`. Sample 5 is same-session recall;
there is no restart or persisted-restoration acceptance in this attempt.
An unexpected reply is a result: stop without a corrective prompt. Do not use
approval, retry or close-unknown controls to force completion. Capture safe
accessibility/screenshot evidence of normal UI and receipt/run IDs, never the
pairing code or credential fields. `status` returns only bounded run metadata.

## Stop and cleanup

Work stops at 292 seconds, reserving eight seconds inside the five-minute total.
Quit Fort fully before typing `done`; a closed window is insufficient. Success
requires five completed receipts, exact app exit, empty actual native pending
queue and confirmed cleanup by 300 seconds. The controller closes the listener,
disposes the owned runtime, revokes enrollment/device authorization, deletes only
issued native credentials and removes its enrollment file. No credential is
retained for another session. The immutable credential expiry is five minutes.

Use `abort`, SIGINT or SIGTERM to cancel. Terminal closure also cancels startup
and running work. Confirm exact app/runtime absence and record `phase`, `starts`,
receipt count and `credentialCleanup`, plus safe startup diagnostics if present.
Failed/late cleanup is blocked; report any remaining residue without a reset.
Timer supervision is not hard-real-time or OS confinement. No second start,
second session, extra message, login, API-key fallback, deployment, service install
or provider cleanup is authorized. Preserve the local ledger and sample history.
