# Disabled UTM source preparation

R3 fixes the five findings against `ca29ad4`: owned command-group cancellation,
physical tar-header limits, combined archive budgets, journal descriptor
attribution and runtime serial-denial evidence. All nine supplied reviewer
regressions pass. Activation remains disabled.

The three authorized UTM text files are verified and retained. The candidate now
uses their version/information schema and checks raw registry entries. Fresh
live registry capture and bookmark resolution remain unavailable by default.

Read [EXECUTION-REVISION.md](EXECUTION-REVISION.md) for the source/evidence boundary
and [OUTSTANDING-INSPECTION.md](OUTSTANDING-INSPECTION.md) for remaining input and
runtime work. This is a source candidate for independent review, not a tested VM.

Fixture commands use existing tools only:

```sh
/usr/bin/python3 -I -B tests/run.py
/usr/bin/env -i PATH=/usr/bin:/bin /opt/homebrew/opt/node@26/bin/node tests/test_verifier.mjs
/usr/bin/python3 -I -B verify.py
```

Fixtures use ordinary temporary files, anonymous socketpairs and temporary Unix
listeners. Command, pidfd, guest, host and cryptographic effects are doubled.
The public entrypoint exits 78. No live adapter, binary/archive acquisition,
installation, mount, UTM/VM launch, provider, credential or security change ran.
