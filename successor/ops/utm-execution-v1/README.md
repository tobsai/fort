# Disabled UTM source preparation

R2 adds Linux/host adapter code, fixed guest programs, a timed serial broker and
host session, in-memory NoCloud/plist generation and bounded archive inspection.
The three source-review findings against R1 are fixed with regressions.

Read [EXECUTION-REVISION.md](EXECUTION-REVISION.md) for evidence limits and
[OUTSTANDING-INSPECTION.md](OUTSTANDING-INSPECTION.md) for the precise missing
inputs. Real UTM schema and authenticated archive facts are unresolved. This is
not an accepted execution packet or a working tested VM.

Fixture commands use existing tools only:

```sh
/usr/bin/python3 -I -B tests/run.py
/opt/homebrew/opt/node@26/bin/node tests/test_verifier.mjs
/usr/bin/python3 -I -B verify.py
```

Python fixtures use temporary ordinary files, anonymous socketpairs and temporary
AF_UNIX listeners. Guest/host/kernel/crypto observations are explicitly doubled.
No live adapter runs. The package entry exits 78; the live-construction guard is
false and has no environment override. Candidate seed/plist output exists only
as in-memory test bytes; no ISO command or guest bootstrap executes.

Only this directory changes. Old specs, source, packets and reviewer-owned files
are preserved. No dependency installation, download, credentials, provider call,
security change, deployment or source publication is included.
