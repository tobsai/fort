# Offline macOS permission experiment

This is a disposable diagnostic, outside the product launch path. The 2026-10-16
delegation authorized synthetic offline canaries with an empty temporary HOME and
configuration, no provider session, IP traffic, enrollment, real credential access,
consumed-ledger changes or persistent system changes. It did not authorize another
Candidate B attempt. The existing product launcher and manual packets remain blocked.

`run.py` uses installed Python 3.14 and clang, compiles only the adjacent reviewed C
probe, and checks the official Codex executable's fixed SHA-256 before invocation.
It invokes no package/build hooks. All child environments are constructed explicitly;
stdin is null, other inherited file descriptors are closed, core dumps are disabled,
outputs/file sizes and execution times are bounded, and owned process groups are
killed if necessary and checked absent. This does not prove absence of descendants
that could escape a process group.

The probe deliberately opens synthetic files, pipes and Unix socket pairs before
applying a rule, to measure whether existing capabilities survive. No IP socket is
created by the probe. Network checks use the local kernel's `sandbox_check` query;
they are not packet-delivery tests. Unix IPC connects only to the probe's own temporary
pathname socket. No Keychain or real privileged-daemon request is sent.

The first policy is a **source-derived rule experiment**, not Codex's complete
generated policy. It applies the pinned lowering of `/**`, read/write denials using
`^/.*$`, after the probe executable has loaded. Its allow-default base intentionally
isolates those rules and leaves other operations outside its claim. Tested pathname
reads/writes are denied, including Unicode/newline paths and links. Pre-opened file
and IPC descriptors remain usable. A passing experiment records that limitation;
it is not a successful descriptor-revocation or host-isolation test.

The separate wrapper for trusted Codex diagnostics denies network, Mach and POSIX IPC,
all user/volume reads, data reads outside explicit system runtime paths, the exact
binary and owned temporary directory, and writes outside that directory. It permits
listing `/` because a process-specific macOS denial identified that as necessary for
exec startup. This does not add a filesystem grant to Fort's no-files profile.
The wrapper must successfully execute `/usr/bin/true` and report a denied outbound
permission query before the harness invokes Codex. It is experimental test protection,
not a production launcher or a claim of general host confinement.

Three fresh empty scopes invoke only `codex sandbox`: an invalid permission value,
an undefined selected profile, and the exact Fort profile. File-backed auth storage
is explicitly selected, telemetry/update hooks are disabled, and no model, account,
thread, turn or provider API is called by the harness. The actual binary rejected both
negative controls and accepted the exact profile through its sandbox launcher.
macOS then refused nested `sandbox_apply` with exit 71. The generated child policy
was therefore **not applied or exercised** in that invocation.

Run only within an explicit offline-canary authorization:

```sh
/opt/homebrew/bin/python3 successor/runner/offline-canary/run.py
```

The final run asserts each observation and records individual statuses in
`validation.json`; exit zero means the experiment completed with those recorded
limitations. Temporary HOME/config/workspace/fixtures and the compiled probe are
deleted. Only logs, inert policy text and synthetic metadata remain for evidence.
The harness does not repair or relax unexpected outcomes automatically.

See [the retained results](../../evidence/2026-10-16-codex-offline-canary/README.md)
for exact pins, failed intermediate controls and passed/unrun checks. The smallest
remaining native-chat gate is actual app-server startup/selection compatibility and
one correlated text turn through the native client in a **separately authorized fresh
packet**, retaining no tools/environments and the explicit same-user host boundary.
No old attempt can supply that authority. This experiment neither requires a new
host-isolation project for that bounded decision nor proves the broader isolation or
release requirements satisfied.
