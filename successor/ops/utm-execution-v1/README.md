# Disabled UTM source preparation

R5 fixes all four R4 lifecycle/filesystem findings. Worker supervision continues
through observed exit after pipe EOF; acquisition and pre-start checks retain
and revalidate directory ancestry; FIFO inputs are opened nonblocking and rejected.
Input acquisition and guest execution remain disabled.

[Revision report](EXECUTION-REVISION.md) · [Input-inspection source](INPUT-INSPECTION.md)
· [Remaining gates](OUTSTANDING-INSPECTION.md)

```sh
/usr/bin/python3 -I -B tests/run.py
/usr/bin/env -i PATH=/usr/bin:/bin /opt/homebrew/opt/node@26/bin/node tests/test_verifier.mjs
/usr/bin/env -i PATH=/usr/bin:/bin /opt/homebrew/opt/node@26/bin/node tests/test_inspection_verifier.mjs
/usr/bin/python3 -I -B verify.py
```

Fixtures use temporary regular files, symlinks, renames, FIFOs, a joined FIFO
reader thread, socketpairs and Unix listeners. One existing-Node fixture process
checks a repository helper against a temporary FIFO; it loads no downloaded code.
HTTP, supervised worker processes, Linux/Mac effects and publisher crypto remain
doubled. The three public entrypoints exit 78. No inputs, app/VM package, mount,
installation, provider or security change occurred. Runtime acceptance remains open.
