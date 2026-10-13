# Disabled R7 macOS input-inspection correction

R7 skips the incompatible child RSS/AS setter only on macOS. Existing sampled
RSS stops, all other limits and Linux behavior remain unchanged. **Both inspection
gates and guest execution are false.** No corrected live worker or acquisition
ran. The consumed R6 root remains untouched; the proposed R7 operation has a new
fixed root and no authority to execute.

[Approval draft and limits](APPROVAL.md) · [Exact operation](activation-operation.json)
· [Revision](EXECUTION-REVISION.md) · [Remaining gates](OUTSTANDING-INSPECTION.md)

Safe fixture/static checks:

```sh
/usr/bin/python3 -I -B tests/run.py
/usr/bin/env -i PATH=/usr/bin:/bin /opt/homebrew/opt/node@26/bin/node tests/test_verifier.mjs
/usr/bin/env -i PATH=/usr/bin:/bin /opt/homebrew/opt/node@26/bin/node tests/test_inspection_verifier.mjs
/usr/bin/python3 -I -B verify.py
```

Fixtures use repository helpers and temporary files/FIFOs/socketpairs/listeners;
HTTP, cryptography and supervised workers are doubles. Disabled Python, Node and
guest entrypoints exit 78. Fixture success is not real corrected-worker validation,
Linux enforcement evidence, publisher authentication or acquisition acceptance.
