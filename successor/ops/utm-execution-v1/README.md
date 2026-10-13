# Disabled UTM source preparation

R4 fixes both R3 readback defects: complete boot configuration is checked before
Start, and equivalent UUID spellings bind canonically. It adds the bounded,
disabled input-inspection pipeline and synthetic HTTP/archive/worker error tests.

[Revision report](EXECUTION-REVISION.md) · [Input-inspection source](INPUT-INSPECTION.md)
· [Remaining gates](OUTSTANDING-INSPECTION.md)

```sh
/usr/bin/python3 -I -B tests/run.py
/usr/bin/env -i PATH=/usr/bin:/bin /opt/homebrew/opt/node@26/bin/node tests/test_verifier.mjs
/usr/bin/env -i PATH=/usr/bin:/bin /opt/homebrew/opt/node@26/bin/node tests/test_inspection_verifier.mjs
/usr/bin/python3 -I -B verify.py
```

Python fixtures use ordinary temporary files, anonymous socketpairs and temporary
Unix listeners. HTTP, worker processes, Linux/Mac effects and publisher crypto
remain doubled. The package, Python inspection and Node inspection entrypoints
exit 78. No binaries/archives were acquired, no downloaded module executed and no
UTM/VM, mount, installation, provider or security change occurred. This is a
source candidate for review, not a tested VM or working product.
