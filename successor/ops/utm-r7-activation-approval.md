# Prepared R7 one-decision scope

Status: source/fixture preparation complete. Independent review of this exact
packet and an informed owner decision remain required. This document grants no
execution authority. Both inspection gates in the checkout remain disabled.

The proposed single decision covers the exact activation patch, one network-free
local synthetic smoke, and **only if that smoke and fresh preflight pass**, one
new eight-input acquisition/authentication/inventory attempt. Any failure ends
the sequence; changed source, tools, parameters, inputs, paths or another attempt
require a new decision. The previous R6 attempt stays consumed and untouched.

The owner must accept this macOS control change: the RSS/AS setter is omitted.
Sampled RSS stops remain 256 MiB for acquisition/verifier archive, 1 GiB for
registry/publisher, and 3 GiB for Ubuntu inventory. There is **no proven bound on
memory overshoot**. Startup, sampling, scheduling and termination can delay the
stop; a failed or starved supervisor loses that active control. Node's 256 MiB
setting bounds V8 old space only. The conditional publisher stage executes the
authenticated OpenPGP 6.3.2 module **on the Mac host**. This is not hostile-code OS
isolation or VM execution.

| Identity | Fixed value |
| --- | --- |
| Preparation worktree | `/private/tmp/fort-core-openmaus-20261011` |
| Preparation branch | `codex/fort-core-openmaus-20261011` |
| Accepted disabled source commit | `5ab910e24a0591b6f8ac7762c73e719671ebc160` |
| Accepted disabled seal | `882eebb2e20b6b4937cc794e09837512c446fe5b5bd717dd7e55e6c2d0546d0e` |
| Inert activation candidate commit | `4bbabbd013bee9eba2900a1d273fc49c50f78bdf` |
| Candidate local ref | `refs/heads/codex/fort-r7-activation-candidate` |
| Activated source seal | `2c44160aca964317d9c8904513f426c42561f1180f09235294fee498847ccc4d` |
| Activation patch SHA-256 | `6510c370db9d1a5bfeff2bebe5d62df39c1e7fc3d8e53e6d0c894012a8c7dfa1` |
| Packet seal SHA-256 | `17d19668ad7fa24dbc957328d6f1e4bc1694eb16ae73c518a76850c43df61d55` |
| Smoke controller SHA-256 | `c854d4ac3656eec9b9a84ed4374814dab2a0459d4b8b39a4af1b35632fd19393` |

The candidate is a local Git object/ref and is not checked out. Its sole delta is
the two inspection gates and their source seal; guest execution remains false.
The patch gives the inspected source directory exactly that candidate's bytes.
The enclosing preparation commit remains a separate identity. The packet seal
covers all 26 packet files plus its two embedded source seals; those bind 272
source files and 14 references. The adjacent approval note is committed alongside
the packet but is outside its closure to avoid a circular self-hash.

The [fixed manifest](utm-r7-activation/packet.json) pins every synthetic child's
full argv, current executable/resolved path/hash, exact environment, profile and
bound. The [implementation note](utm-r7-activation/README.md) explains the checks.
All commands below are prepared text only. Stop on the first nonzero exit; no
command here has been used to activate or execute the smoke/acquisition.

1. Read-only verification of the disabled starting state:

```sh
/usr/bin/env -i PATH=/usr/bin:/bin LANG=C LC_ALL=C TZ=UTC /usr/bin/python3 -I -B /private/tmp/fort-core-openmaus-20261011/successor/ops/utm-r7-activation/verify.py disabled 17d19668ad7fa24dbc957328d6f1e4bc1694eb16ae73c518a76850c43df61d55
```

2. Apply the exact reviewed patch after the decision, then verify the activated
   source and packet. No checkout, merge, source rewrite or environment override:

```sh
git -C /private/tmp/fort-core-openmaus-20261011 apply /private/tmp/fort-core-openmaus-20261011/successor/ops/utm-r7-activation/activation.patch
/usr/bin/env -i PATH=/usr/bin:/bin LANG=C LC_ALL=C TZ=UTC /usr/bin/python3 -I -B /private/tmp/fort-core-openmaus-20261011/successor/ops/utm-r7-activation/verify.py activated 17d19668ad7fa24dbc957328d6f1e4bc1694eb16ae73c518a76850c43df61d55
```

3. Run the fixed network-free smoke exactly once:

```sh
/usr/bin/env -i PATH=/usr/bin:/bin LANG=C LC_ALL=C TZ=UTC /usr/bin/python3 -I -B /private/tmp/fort-core-openmaus-20261011/successor/ops/utm-r7-activation/smoke.py --run-once 17d19668ad7fa24dbc957328d6f1e4bc1694eb16ae73c518a76850c43df61d55
```

It exclusively creates `/private/tmp/fort-core-openmaus-20261011/successor/evidence/utm-r7-smoke-once` with a consumed marker
and at most 65,536 retained logical bytes of structured evidence. Four trusted
synthetic children run sequentially, covering all four distinct CORE/CPU/FSIZE/
NOFILE tuples and the selected Python and Node paths. Each setter is immediately
read back. Parent limits must be unchanged. Actual owned-child `ps` observations,
including one after all pipes close, precede owned SIGKILL, sole reap and positive
child/group absence checks. No large allocations, network, downloaded modules,
provider access or either operation-root access occur in this smoke.

The requested controller deadline is 30 seconds; child startup alarm and wall
deadline are five seconds, combined output is 8 KiB per child, and reap wait is
one second. At most one synthetic child and one RSS query overlap. Each RSS query
has 250 ms / 128-byte bounds; there are at most 50 queries per child, 200 total.
Node verifies allowed self-read and denied public canary read, and reports denied
global read/write/child/worker scopes. These checks claim only those observations.
Python's known launcher metadata key names may appear; their values are never
logged. Required environment values are fixed and unexpected keys fail.

A read-only verifier passed under the restricted tool sandbox with an Apple
Python `confstr()` temporary-directory warning. If that warning appears in a
future synthetic child's stderr, this exact smoke rejects it and stops before
acquisition. No warning suppression or alternate runtime is authorized.

4. Only after smoke success, start the fixed continuation within 300 seconds:

```sh
/usr/bin/env -i PATH=/usr/bin:/bin LANG=C LC_ALL=C TZ=UTC /usr/bin/python3 -I -B /private/tmp/fort-core-openmaus-20261011/successor/ops/utm-r7-activation/continue.py --run-once 17d19668ad7fa24dbc957328d6f1e4bc1694eb16ae73c518a76850c43df61d55
```

It checks the bound smoke receipt, every case's evidence and unchanged parent
limits, consumes an exclusive continuation marker, then performs fresh closure,
tool/runtime, current-capacity and no-follow root-absence checks. Memory support
is the successful actual smoke plus fresh matching binary pins. Preflight makes
two read-only bounded queries (`sysctl -n hw.memsize`, `memory_pressure -Q`), each
with a two-second deadline. An existing or newly appearing R7 root is a veto.
Capacity observations reserve nothing; filesystem overhead is additional.

Only success permits this exact exec replacement, with the same four-variable
environment; it is not a standalone bypass command for the owner to run:

```text
/usr/bin/python3 -I -B /private/tmp/fort-core-openmaus-20261011/successor/ops/utm-execution-v1/inspect-inputs.py 2c44160aca964317d9c8904513f426c42561f1180f09235294fee498847ccc4d
```

The attempt root is
`/Users/tobiasgunn/Library/Application Support/FortUTMInputInspectionR7`.
The [sealed eight-input operation](utm-execution-v1/activation-operation.json)
fixes URLs, digests, the Ubuntu publisher fingerprint, archive authentication,
registry/publisher verification, all four rejected negative controls and durable
receipts. The order remains acquisition → registry → verifier archive → publisher
→ Ubuntu inventory. Limits remain **814,088,192 retained input bytes** and
**839,843,840 logical bytes overall**, plus filesystem overhead. At most 40 HTTPS
requests, four redirects per input, 600 seconds per input, no retry, and **no
aggregate wire-byte quota**. Source operation documents keep their historical
R7 preparation status; this note and packet supply the later fixed identities
without changing that operation's inputs or restrictions.

Retain partial/uncertain state and failure evidence; no automatic cleanup,
retry, alternate root or adoption. Completion still leaves `image_bound: false`
and `runtime_authorized: false`. The consumed R6 tree is neither read nor changed.
UTM/VM launch, mount/conversion, guest extraction, providers/accounts, credentials,
installation/hooks, host-wide settings and external publication are excluded.

Validation performed: 18 packet fixture tests; 8 focused reviewed-source fixture
tests; Python syntax for 10 files; Node syntax for one file; four default inert
entrypoints exit 78; exact candidate delta and 272+14 source closure; packet and
existing executable pins; activated-state refusal in this disabled checkout.
Current checks passed. The initial missing-seam failures and a reproduced completion-deadline edge
case are retained as red TDD evidence; all pass after implementation/correction. The complete unchanged R7 suite was not repeated; its accepted
review remains the baseline. Actual resource setters/readbacks, payload startup,
Node permission checks, RSS/cancellation/reap, fresh live preflight, acquisition,
authentication and inventory are **unrun**. See
[validation.json](utm-r7-activation/validation.json).

Final preparation check: one read-only verification invocation mistakenly used
an incorrect digest and was rejected before effects. The corrected literal
packet digest above passed closure/tool verification; activated-state validation
also correctly refused the still-disabled checkout. No live attempt was consumed.
