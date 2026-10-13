# R7 fixed activation and smoke packet

Prepared for one future informed decision. Actual smoke, activation in this
checkout, acquisition and downloaded-code execution remain unrun. The reviewed
source directory remains byte-identical to disabled commit
`5ab910e24a0591b6f8ac7762c73e719671ebc160`.

The separate local candidate commit
`4bbabbd013bee9eba2900a1d273fc49c50f78bdf` at
`refs/heads/codex/fort-r7-activation-candidate` exists only as a Git object/ref.
It has not been checked out. Its source seal is
`2c44160aca964317d9c8904513f426c42561f1180f09235294fee498847ccc4d`.
It changes only the Python inspection gate, Node inspection gate and source seal.
Guest execution remains false. `activation.patch` produces those exact source
bytes; applying it later does not make the enclosing worktree HEAD that candidate
commit. The packet commit and activation source commit are distinct identities.

The adjacent `../utm-r7-activation-approval.md` is the consolidated owner-facing
scope with literal hashes and commands. It is outside this packet's hash closure
so it can name the packet seal without a circular self-hash. Git records both.
`seal.json` covers every file in this directory except itself; both source seals
include all 272 source files and 14 references. No runtime mutation, environment
override, digest rewriting or guard bypass activates the code.

## Fixed sequence

1. After source review and owner approval, apply the exact activation patch and
   verify the activated source, packet closure and existing executable pins.
2. Run `smoke.py` once with the literal packet digest in the approval note. It
   exclusively creates the separate `successor/evidence/utm-r7-smoke-once`
   directory, writes its consumed marker, and launches four sequential trusted
   synthetic children. It contains no acquisition entrypoint and accesses neither
   operation root. A failed or interrupted smoke leaves consumed evidence.
3. Only a successful smoke permits `continue.py`. It requires the exact six
   success records, matching identities and all four case results, with completion
   no more than 300 seconds earlier by both monotonic and wall clocks. It writes
   an exclusive continuation marker before fresh preflight. Any failure ends the
   sequence. There is no deletion, adoption, parameter substitution or retry.
4. Fresh preflight rechecks closure, tool selection, smoke-proven permission
   support with the same pinned binaries, current capacity, and the R7 root's
   absence through retained no-follow ancestor descriptors. The root is checked
   absent again after observations. Existing/appearing R7 state is a veto. The
   existing acquisition controller independently requires exclusive creation.
5. Success replaces the continuation process with the exact original inspection
   CLI and activated source seal. It performs the unchanged acquire → registry →
   verifier archive → publisher → Ubuntu inventory sequence once. Dispatch
   intent is not a completion receipt; acquisition receipts own that result.

## Smoke observations and bounds

`packet.json` pins full child argv, trusted binaries, resolved runtime paths,
environment, output directory, counts, time/output bounds, profiles and cleanup.
The controller extracts only `PROFILES` and `child_limits` from the hash-verified
reviewed source AST; it never imports the acquisition worker. Every setter gets
an immediate exact readback through a resource proxy that rejects RSS/AS and any
unexpected or repeated resource on macOS. A dedicated bounded pipe reports the
readbacks or the failing setter without forwarding exception/environment values.
That pipe is closed before exec.

| Child / stage coverage | CPU soft/hard seconds | File size bytes | RSS stop bytes |
| --- | ---: | ---: | ---: |
| Python acquisition | 120 / 121 | 805,306,368 | 268,435,456 |
| Node registry and publisher | 80 / 81 | 0 | 1,073,741,824 |
| Python verifier archive | 25 / 26 | 25,165,824 | 268,435,456 |
| Python Ubuntu inventory | 280 / 281 | 0 | 3,221,225,472 |

All four use CORE 0/0 and NOFILE 64/64. The parent snapshots these limits plus
RSS/AS before and after and requires equality. No test allocates large memory or
exhausts CPU/file limits. The helpers report their actual Python/Node identity and
environment, close stdout/stderr, and remain alive for owned cancellation.

The requested smoke wall deadline is 30 seconds; each child has a five-second
startup alarm and five-second supervisor deadline, 8 KiB combined setup/output,
and a one-second reap wait. Four synthetic children run serially. At most one
owned `ps` command runs alongside one synthetic child; no more than 50 `ps`
commands per child (200 total), each with 250 ms and 128-byte output bounds.
At least two positive RSS observations and one after every pipe reaches EOF are
required. Samples request 100 ms spacing. These are scheduled controls, not hard
real-time guarantees; setup, observation and cleanup can be delayed by the host.

Cancellation sends SIGKILL only to the owned group while its unreaped direct
child reserves the PID. Then exactly one wait reaps it. Zero-signal child and
group probes must both report absence. A remaining or uncertain process/group
fails the smoke; no post-reap kill or broad process search occurs. The reviewed
bounded command runner supervises the transient `ps` children separately.

Python children receive exactly PATH, LANG, LC_ALL and TZ from Popen. The trusted
Apple Python launcher can add CPATH, LIBRARY_PATH, MANPATH, SDKROOT and
`__CF_USER_TEXT_ENCODING`; only these extra key names are allowed and recorded,
never their values. Node permits no extra environment keys. The reviewed `ps`
and preflight command runner has its separately pinned PATH/LANG/LC_ALL allowlist.

The Node helper uses the actual fixed permission argv and V8 old-space flag. It
reads its own pinned source and requires ERR_ACCESS_DENIED when reading the
pinned public `denied-probe.txt`. It also reports denied global read, write,
child and worker scopes. It does not attempt network access or subprocess
creation, and makes no claim about untested permission scopes or OS isolation.
The Python helpers are trusted local code, not hostile-code sandboxes.

The exclusive smoke journal uses private no-follow directory descriptors and
exclusive 0600 records, capped at 16 KiB each and 64 KiB total. It is separate
from the future R7 input tree. Evidence is preserved on failure. The packet and
binary pins establish checked files/paths at observation time; they do not seal
the host OS, shared libraries or framework installation against concurrent owner
changes. Actual startup remains part of the future smoke gate.

The read-only clean-environment verifier exited successfully in the restricted
tool sandbox, but Apple's Python launcher emitted a `confstr()` warning about
`DARWIN_USER_TEMP_DIR` and used `/tmp`. This was not a smoke attempt. The smoke
continues to reject any child stderr; the same diagnostic during its future
authorized run would stop before acquisition. No warning suppression, runtime
substitution or environment expansion was added to make that check pass.

## Capacity and remaining risk

Fresh preflight uses the existing read-only `sysctl -n hw.memsize` and
`memory_pressure -Q` invocations, each bounded to two seconds and fixed output
caps. Available disk must cover the unchanged 839,843,840 logical-byte acquisition
budget. Physical memory must cover the largest RSS stop threshold; observed
free-memory percentage is recorded. These observations reserve no capacity.
Filesystem metadata/allocation rounding are additional; smoke evidence adds at
most 65,536 retained logical bytes in its separate directory.

macOS does not request the RSS/AS setter. The same sampled parent stop thresholds
remain, with no proven bound on overshoot; starvation or failure of the supervisor
loses that active control. Node's 256 MiB setting limits V8 old space, not total
process memory. The conditional publisher stage executes authenticated OpenPGP
code on the Mac host. These are material owner-decision facts.

The earlier R7 operation manifest remains sealed historical source and contains
its source-only status text. This packet supplies the precise later activation
identity and fixed prerequisite commands; it changes no acquisition URL, digest,
limit, root, stage, negative control or evidence requirement. The consumed R6
tree is neither read nor altered. Successful inspection still leaves
`image_bound: false` and `runtime_authorized: false`. VM/UTM execution, mounts,
conversion, guest extraction, providers/accounts, credentials, installation,
host-wide settings and automatic retries remain excluded.
