# Bounded synthetic process I/O and native confinement experiment

**The synthetic process slice passes; production OS confinement does not.**
138 backend tests and strict TypeScript pass for source commit
`e2ecd0a0eda19e0ffac61097fbee0add4af71f47`, based on accepted bridge
`5b4d35f15165e4bef56cb927363ff1f268297f6e`. This is local synthetic evidence,
not Stage 2 acceptance or a tested official-provider application.

Worktree: `/tmp/fort-core-openmaus-20261011` (canonical `/private/tmp/...`).
Branch: `codex/fort-core-openmaus-20261011`. Donor provenance remains
`8b53c0323dba2099a953ee2d77ab9330cd5acaa7`; its source and licensing are unchanged.
No donor implementation was copied into this increment.

## Authority and scope

This continues the bounded process-I/O and temporary-canary work authorized after
[the independent bridge review](../../reviews/synthetic-bridge-5b4d35f/REVIEW.md).
The work used fixed synthetic workers, temporary private roots, synthetic canaries,
owned process groups and local test sockets. Existing Node 26.5.0, clang and
sandbox-exec were used; nothing was installed. The C source is a test-only syscall
probe compiled into temporary storage, not a production backend or app dependency.

Only named successor source/tests, package test script, runner documentation,
implementation note and this evidence were changed. Parent-owned `reviews/`, donor,
Fort specs, native/Swift source, public/private contracts, existing service
implementation and Codex codec are unchanged. Existing main-checkout work and
history are preserved. No UI was launched, no provider executable was invoked,
no existing credential/configuration was read or copied, and no sign-in, account,
OS user, VM, Keychain/TCC grant, persistent permission, startup service, network
exposure, push or deployment occurred. Provider teardown closure remains outside
this task. No external agent was contacted or spawned.

## Implemented behavior

`startSyntheticProcess` launches only a fixed reviewed fixture through explicit
argv/environment and fresh private cwd. Its trusted parent retains bridge authority;
worker protocol/argv/environment contain no runner or owner credential. It is an
opt-in test harness, not service activation or a generic executable launcher.
The fixture has host-user OS authority and therefore cannot safely be substituted
with a provider or untrusted program.

Actual I/O tests prove exact fragmented UTF-8, partial writes, whole-frame write
deadlines, blocked stdin, abrupt/partial EOF, output floods, lost acknowledgement,
cancellation, bounded upload waits and evidence backlog. Output and traces are
bounded; raw stderr/input are not included in process diagnostics. A lost receipt
retries identical normalized evidence at most once, never another process start.

Cancellation and evidence upload proceed independently of blocked writes.
Acknowledgement alone and exit zero without correlated terminal output remain
unknown. Completed/interrupted evidence is held until root exit, pipe closure and
absence of the owned POSIX group are observed. TERM and then KILL target only that
group. Pipe-holding descendants that ignore TERM are killed and group absence is
observed; a cleanup request alone is never confirmation.

Default bounds: handshake 5 s, turn 30 s, interrupt 2 s, whole outgoing frame 2 s,
teardown 2 s, cleanup observation 2 s, upload 2 s per attempt with at most two
attempts. Queue caps are 524,288 wire bytes, two waiting nonterminal evidence events
and one held terminal, in addition to one active upload. Stdout caps at 1,000,000
bytes, stderr at 65,536; the codec also enforces its existing frame/text/item limits.
Tests shorten deadlines; callers cannot enlarge them. These asynchronous bounds do
not promise hard real-time scheduling. A protocol burst exceeding the evidence
queue marks upload unconfirmed, stops the process and leaves the existing service
lease/restart fence to establish durable uncertainty, without inventing contiguous
acceptance or resending work.

Three new authenticated HTTP bridge tests verify exact output plus restart replay,
terminal acknowledgement loss with one committed result/start, owner cancellation
while stdin is blocked, and service restart fencing of a still-running process.
The original seven service-kill checkpoints and five runner kill/exit cases also
rerun in the full suite. Public/native schemas and fixture labels are unchanged.

## Confinement results and failures

The installed Apple [sandbox-exec manual](/usr/share/man/man1/sandbox-exec.1) states:
“The sandbox-exec command is DEPRECATED.” The official App Sandbox page was opened
but its text could not be retrieved through the available web tool; no conclusion
about a supported implementation is drawn from that attempted lookup.

| Experiment | Observed result | Meaning |
| --- | --- | --- |
| Native probe without a profile | All 13 positive controls work | Probe can read synthetic targets, use its inherited descriptor/environment, signal-check the owned sentinel, fork/exec, and reach owned Unix/loopback endpoints |
| Restrictive deny-default profile | SIGABRT before a report | **Failed candidate boundary**; adding metadata/sysctl/null/random allowances still did not make it runnable |
| Diagnostic allow-default policy with explicit synthetic-target denials | Workspace read/write works; owner, ledger, sibling, provider, home and escaping-symlink reads fail; signal-check, fork/exec, Unix IPC and loopback TCP fail | Named denials work; unrelated host resources are **not** confined by this diagnostic policy |
| Separate synthetic credential host policy | Provider canary readable only for host actor; other target denials remain | Demonstrates an intended host/tool split with synthetic material, not a real credential-bearing runtime |
| Environment allowlist and closed descriptors | Synthetic environment secret absent and fd 3 unreadable | Launcher hygiene works for the tested scope |
| Deliberately inherited fd 3 under path denial | Synthetic owner canary remains readable | **Descriptor bypass exists**; a pathname policy cannot revoke that inherited capability |
| Supervisor SIGKILL with blocked worker | Worker group remains alive until the outer harness kills it and observes absence | **Host death is not cleanup proof**; a production outer lifetime boundary is still missing |

The diagnostic test deliberately asserts the descriptor leak and retains
`productionIsolationProven: false`. Its green status means the experiment matches
its explicit limited claims; it does not convert the failed restrictive profile
into accepted confinement. All normal synthetic groups and canary roots are removed.
Owned groups in the crash experiment are explicitly reaped by its outer test owner.
Group absence does not prove the absence of descendants able to escape that group.

Global Keychain/TCC, Mach IPC, unrelated/arbitrary host paths, external networks,
provider tools/configuration, real credentials and Linux confinement are unrun.
The file canaries establish no claim about global Keychain isolation. The weak
canary policy is never applied to an official runtime or enabled in the product.

## Validation and retained evidence

From `successor/`, with existing project dependencies and no install/build hooks:

```sh
/opt/homebrew/opt/node@26/bin/node --test service/service.test.ts service/synthetic-runner.test.ts runner/fixture.test.ts runner/codex.test.ts runner/synthetic-transport.test.ts runner/synthetic-process.test.ts runner/confinement-canary.test.ts
/opt/homebrew/opt/node@26/bin/node node_modules/typescript/bin/tsc -p tsconfig.json
```

- **138 passed, 0 failed, 0 skipped**, exit 0: [full log](process-confinement-2026-10-12/node-tests.log).
  This includes 17 new process tests, 3 bridge cases, 1 transport deadline case and
  1 native diagnostic test in addition to the previously accepted 116.
- Strict TypeScript: exit 0, no diagnostics: [log](process-confinement-2026-10-12/typecheck.log).
- `node --check runner/process-fixture.mjs`, `git diff --check`, and fixed-base
  unchanged-scope comparisons: exit 0. Clang runs with `-Wall -Wextra -Werror`
  inside the native test; compilation passed.
- Behavioral red→green evidence: [deadline](process-confinement-2026-10-12/deadline-red.log.gz),
  [whole-frame timeout](process-confinement-2026-10-12/frame-deadline-red.log.gz),
  [evidence backlog](process-confinement-2026-10-12/backlog-red.log.gz).
  The first process/canary missing-module failures are scaffolding, not behavioral
  red proof. An initial missing crash-host fixture left its test waiting; the exact
  owned test PIDs were stopped. The corrected harness rejects premature host exit,
  has a timeout and passes. This historical failed run is retained and labeled.
- The three restrictive-profile failed logs remain alongside the successful
  diagnostic log. They are not omitted from the reported result.
- [validation.json](process-confinement-2026-10-12/validation.json) records the exact
  source commit, 11 source hashes, 20 artifact hashes, exit codes and limitations.
  Logs record observed runs, not an independent reconstruction of historical edits.
  Raw logs with trailing whitespace are retained as deterministic gzip files with
  both compressed and decompressed hashes.

No native UI/Swift rerun was needed for unchanged native code. No installed-product,
physical-device, provider-native, self-hosted or release acceptance is claimed.

## Remaining gate

This increment is ready for independent review as synthetic I/O/lifecycle work.
T07 and Stage 2 remain blocked on a **supported, enclosing OS process and credential
boundary**, including descriptor hygiene, escaped descendants, outer-host death,
Keychain/IPC and the eventual runtime/tool split. The current deprecated diagnostic
mechanism is not a shipping solution. A concrete supported design must precede any
request for host changes; no new host security action is proposed or executed here.

Any design requiring new OS users/groups, VM/runtime provisioning, persistent ACLs,
signing entitlements, security grants, Keychain/TCC changes, startup service or
network exposure must first present exact target, action, risk, cleanup and a
least-privilege alternative for owner approval. Provider version/schema execution
and provider-native sign-in remain after that boundary and under their own gates.
