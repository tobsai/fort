# UTM input-inspection activation candidate

R5 disabled source is independently accepted. R6 prepares the two input-inspection
gates for one bounded eight-input operation. **Do not invoke the activation before
independent review and exact owner approval of the new commit/seal.** No approval
or execution occurred during preparation. Guest execution remains false.

[Exact approval details](APPROVAL.md) · [Operation and pins](activation-operation.json)
· [Revision report](EXECUTION-REVISION.md) · [Remaining gates](OUTSTANDING-INSPECTION.md)

Safe fixture/static validation:

```sh
/usr/bin/python3 -I -B tests/run.py
/usr/bin/env -i PATH=/usr/bin:/bin /opt/homebrew/opt/node@26/bin/node tests/test_verifier.mjs
/usr/bin/env -i PATH=/usr/bin:/bin /opt/homebrew/opt/node@26/bin/node tests/test_inspection_verifier.mjs
/usr/bin/python3 -I -B verify.py
```

Tests use temporary files/FIFOs/socketpairs/listeners and repository helpers;
HTTP, cryptography and supervised workers remain doubles. The guest entrypoint
still exits 78. Both prepared acquisition entrypoints are deliberately unrun.
Actual artifacts, downloaded-module compatibility and OS enforcement are untested.
