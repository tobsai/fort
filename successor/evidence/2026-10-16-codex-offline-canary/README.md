# Offline Codex parser and macOS canary results

**The actual parser accepts Fort's named profile. The tested deny rule blocks pathname
access but does not revoke pre-opened descriptors. Codex's complete generated sandbox
was not applied: macOS refused a second sandbox inside the protective test wrapper.**

Source commit: `fcc3685dcf1828553b27fb2d27c4e75ca76b383c`, based on
`a1bc6486bb41ee1ca5f54b89e0c8c34b8c674dfb`.
Branch: `codex/fort-openai-rebuild-20261016`.
Worktree: `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
Host reported macOS 27.2, build 26B5112a. This packet is evidence, not provider activation
authority, a fourth attempt packet or installed-product acceptance.

## Exact checks

| Check | Result and limit |
| --- | --- |
| Pinned actual CLI, invalid filesystem value | **Passed negative control.** Exit 1; `FilesystemPermissionToml` rejected the value. |
| Pinned actual CLI, undefined selected profile | **Passed negative control.** Exit 1; reported undefined `fort_absent_canary`. |
| Pinned actual CLI, exact Fort profile | **Parser/selection accepted through sandbox launch.** The same declaration and `-P fort_candidate_b_no_files` reached `sandbox-exec`. This is an inference from the observed launcher failure and the pinned source path, not an app-server selected-profile report. |
| Complete generated child policy | **Blocked/unverified.** Exit 71: `sandbox-exec: sandbox_apply: Operation not permitted`. No claim that the generated policy was applied or a tool command ran under it. |
| Source-derived deny-glob rule, loaded C process | **9 expected syscall denials passed**: ordinary and workspace reads, symlink, hardlink, Unicode/newline reads, file creation, rename and connection to the owned Unix pathname socket. Each returned EPERM. |
| Source-derived network rule | **2 kernel permission queries passed**: inbound/outbound denied. No IP socket or packet-delivery probe was used; no external endpoint was contacted by the harness. |
| Pre-opened capabilities | **Revocation is not provided.** Synthetic read/write file descriptors, Unix socket pair and pipe still worked after applying the rule. This is a boundary limitation, not successful confinement. |
| Positive controls | All 16 observations matched the unsandboxed synthetic baseline; no real account material was used. The diagnostic rule's general Mach query remains allowed because that experiment intentionally isolates filesystem/network rules. No real Mach service was contacted. |
| Temporary wrapper | Final control passed `/usr/bin/true`, with outbound permission denied before exec. Earlier controls aborted; the PID-specific sandbox log identified `file-read-data /`. Allowing only the root directory listing fixed wrapper startup without changing Fort's profile. |
| Contract tests | **394 passed, 0 failed**, exit 0. In-memory protocol/launch-plan tests only; [log](contract-tests.log). |
| Strict TypeScript and Python syntax | Both passed; [TypeScript output](typecheck.log) is empty on success. The C probe compiled with warnings treated as errors. |
| Preservation/cleanup | All three consumed attempt file sets, ledgers and states unchanged; each remains blocked with one start and zero receipts. Official executable, complete five-file native app and main-checkout status unchanged. Synthetic HOME/config/fixtures/executables removed; final owned process groups observed absent. |

The final asserted run is [iteration 05](iterations/05/validation.json), with
[raw outcomes](iterations/05/results.json), [observations](iterations/05/observations.json)
and [transcript](final-run.log). Exact stderr with blank trailing lines is stored as
deterministic gzip; [raw-byte hashes](compressed-logs.json). Iterations 01–04 preserve intermediate controls;
they are exploratory revisions, not claimed to use the final committed harness.
[The scoped wrapper diagnostic](wrapper-denial.json) records the root-directory denial.
The process log query read only that synthetic PID's sandbox entries. No logging
configuration, security setting, entitlement, permission or startup registration was changed.

## Isolation and provenance

The [test-only harness](../../runner/offline-canary/README.md) used fresh private `/tmp`
roots and explicit HOME, CODEX_HOME, XDG and TMPDIR values; no authenticated HOME or
provider-owned storage was loaded. Each CLI case had a fresh empty config scope,
explicit file-backed auth mode, disabled telemetry/update options and closed inherited
descriptors. The wrapper denied user-file reads, network and Mach/POSIX IPC before
exec. Its limited system runtime/root-directory reads were for process startup.
No app-server, account/model/thread/turn API, Fort enrollment or provider request was
initiated by the harness. The six diagnostic CLI invocations across the two final
iterations were local non-provider tests, not six sessions or consumed attempts.

The independent rule canary applies the pinned `/**` lowering (`^/.*$`) after loading
the probe. It deliberately uses an allow-default diagnostic base, not Codex's full
base/helper/daemon policy. Its observations therefore cannot attest the complete
runtime policy, Keychain/privileged-daemon isolation, or the same-user agent host.
No full host-isolation claim is made. Existing descriptors are capabilities and must
not be confused with pathname grants. The production launcher was not changed.

[pins.json](pins.json) seals all 467 source files and the unchanged runtime/native app.
The source list SHA-256 is `8f5c12b8595ccc58c9c28b1f3b7616e54096de4f378098469209fef39f413a36`.
Official Codex `rust-v0.157.1` resolves to
`36650394c5b38c2990ccf2a3457165ca3e9d9726`; executable SHA-256 is
`27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d`.
All 85 cached immutable public sources were hash-verified; [manifest](upstream-sources.json).
No upstream source was compiled or copied into the product. Donor attribution,
historical Fort/specs, provider teardown scope, old evidence and manual activation
pins remain unchanged. No push, merge, deployment or remote visibility change occurred.

## Smallest remaining gate

A useful native text prototype still needs the actual app-server startup/selection
contract and one correlated native send/reply through a separately authorized fresh
packet. Keep empty environments, disabled tools, exact model/profile checks and the
same-user host boundary explicit. Stop at the first failed preflight check; do not
adopt/reset any consumed ledger. A new live attempt and real provider-owned HOME are
outside this offline authorization. This report does not ask to execute one.

If complete generated-policy enforcement must be measured first, the minimal further
test is an official offline `codex sandbox` launch in a disposable context where its
own sandbox can apply, with empty configuration, synthetic files and network isolation
outside that child boundary. The present enclosing Seatbelt wrapper cannot provide
that test. Removing its host protection on this Mac would risk ambient startup
reads/IPC and network attempts; that relaxation was **not** performed. A separately
reviewed safe arrangement is needed, not provider sign-in or persistent host changes.

Unrun: actual app-server config/catalog/thread/turn reports; complete generated-policy
OS application; IP packet tests; real Keychain/privileged-daemon access; provider
authentication/turns; native app launch/build/pairing/UI/screenshot signoff; full test
suite with its network fixtures; independent review of this delta. Current failure
and limitations are retained rather than converted into broader acceptance.
