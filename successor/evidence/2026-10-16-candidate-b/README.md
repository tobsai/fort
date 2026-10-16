# Candidate B reconstruction checkpoint

The 37 remaining preserved Git blobs yielded **zero matches** to the known missing
prototype markers. No exact lost prototype source or signed app was recovered.
The original branch, raw objects, indexes and evidence remain privately preserved
at `/Users/tobiasgunn/dev/fort/.local/prototype-recovery-20261016T021809Z/`.
The targeted scan report is `targeted-blob-scan/report.json` there. Matching was
bounded to known markers; no secret or unrelated blob contents were printed.

This is a **new candidate**, reconstructed from committed baseline
`ac2dd2ae55b3df4235d352f34937c33f1fe932c3`. It is not the lost October 15 artifact
and does not inherit that artifact's execution approval.

- Branch: `codex/fort-openai-rebuild-20261016`
- Durable worktree: `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`
- Pinned source: `385277a07c2da772bac8a0b5efa0276b4c65e2a4`
- Source list SHA-256: `3c640ab3f8a226f819a3c08c203f4c6dc24d0520b14fdf71f7b50ebe91851192`
- App list SHA-256: `05a8a2f7af6cf7f834557fef978edc0eb95e4b695d00f7f510ff835d4dbc2405`
- [Exact paths and file hashes](pins.json)

The app is under `successor/.build/candidate-b-mac/Build/Products/Debug/FortMac.app`.
It was built from `a131642`; all Apple source is byte-identical in the pinned
source revision. `CODE_SIGNING_ALLOWED=NO` was used. The Mach-O has a linker ad-hoc
signature, with no team identity, bound Info.plist or sealed resources. This is
not the previous signed build and is not a distributable or installed acceptance
artifact. Xcode registered the build path with LaunchServices during its normal
build phase; the app was not launched or copied to Applications.

## Rebuilt behavior

The native workspace creates and edits stable named Agents and instruction-only
skills, creates shared conversations, selects one explicit shared recipient and
selects exact skill revisions per message. Candidate bundle/storage identities
are separate from the historical fixture. Pending requests preserve text,
recipient, binding and skill revisions across restart; stale revisions pause
automatic retry and remain visible. Workspace saving waits for background refresh.

The separate SQLite candidate service durably accepts immutable requests and
configuration, returns original receipts on identical retries, rejects changed
retries, saves attributed replies and persists monotonic read state. Dispatch is
serialized. Each turn receives earlier user/reply pairs from its conversation,
including replies completed after a later user message was queued. Restart holds
old queued work and fences unresolved outcomes without blocking another Home.

The injected Codex transport prepares exact-model ephemeral threads and bounded
conversation context, validates returned configuration, rejects tools and model
substitution, excludes commentary from saved replies and never claims effective
model attestation. It uses the preserved reducer plus newly authored connection
and chat adapters. No donor or historical Fort implementation was copied into
this slice. Existing OpenMausBot provenance, licensing and quarantine are unchanged;
this reconstruction makes no new upstream import or publication.

## Verification

| Check | Result |
| --- | --- |
| Combined nonlive Node service/runner/protocol suites at `a131642` | 128 passed, 0 failed; [raw output](node-tests.log) |
| Final candidate HTTP suite after exact whitespace fix | 7 passed, 0 failed; [raw output](openai-final-tests.log) |
| FortKit Swift tests | 21 passed, 0 failed; tool receipt `071703` |
| Actual Swift client → temporary candidate HTTP service | Five synthetic turns passed; restart context, exact recipients/skills, receipts and empty pending queues checked; [raw output](native-http-final.log) |
| Strict TypeScript including activation gate | Passed; tool receipt `40c357` |
| Native Mac compile | Build succeeded; durable raw log `successor/.build/review/mac-build.log` |
| Activation entry | Expected refusal, exit 1, before provider activity; receipt `c24831` |
| Main checkout status | Matches recovery snapshot byte-for-byte; SHA-256 `02ec195be31c53b9078dc8ff764a1ac1bb2588aa33a192eda971483f47157297` |

Red checks reproduced missing workspace/dispatch behavior, queued context ordering,
restart fencing, revision edits, native pending metadata/selection, receipt shape,
the versioned approval-policy mismatch and whitespace-only text rejection. Each
was followed by its passing targeted check. The first sandboxed Swift build failed
on compiler-cache access; the approved compiler escalation succeeded. Offline npm
restore lacked a cached package; the three existing lockfile-pinned development
packages were then restored with empty npm config and install scripts disabled.
One combined-test command failed before execution because the log directory was
not yet present; its rerun passed. A transient Mac transport close interrupted one
patch; disk inspection confirmed it had not applied, then the retry passed.

No current required nonlive check remains failing. The full npm command including
OS confinement canaries was not run. Synthetic runner subprocesses in the selected
baseline suites are test actors, not official provider processes.

## Remaining gates and limits

No official Codex process, provider request, provider account read, native UI
session, real enrollment credential or Keychain item was created for this
candidate. No login, deployment, remote push, startup service, OS security change
or provider cleanup was performed. Local teardown is not evidence of full provider
closure. Main's unrelated dirty work is retained.

Independent candidate review, executable/configuration compatibility, enforced
host isolation, real inference, exact installed accessibility/screenshots, physical
iPhone and release validation remain unrun. No enabled provider launcher exists.
The adapter requires an explicitly supplied transport; a bounded owned-process
launcher and its exact configuration need separate review and testing before any
proposed activation. Requested tool sandbox settings do not isolate the agent host
or prove that ambient configuration and credentials cannot be accessed.

The pinned ordinary 0.157.1 schemas do not list every requested empty capability
field. Their acceptance and actual effect remain unverified; unknown fields can
be ignored. See the [execution gate and protocol references](../../prototype/README.md).
Unknown-outcome closure, held-queue resumption, pending-conflict replacement,
incremental event replay and a production enrollment/revocation interface are not
implemented in this candidate's OpenAI path. The five-sample synthetic path does
not depend on those operations. No replacement model or provider is introduced.

The artifact remains a source reconstruction with a native compile and synthetic
end-to-end checks. Fresh independent review and exact source/app/runtime pins must
precede a new live execution decision. Packet A remains unconsumed and does not
authorize candidate B.

## Local commits

`b0a04d9` workspace foundation; `48c054f` durable turns/context; `670b17d` native
workspace and recipients; `9835840` disabled Codex adapter; `a131642` native HTTP
checks and separate app identity; `085ed8d` exact whitespace; `385277a` validation
commands. The subsequent evidence commit adds this report and pins only.
