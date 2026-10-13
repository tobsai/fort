# UTM source preparation revision R5

**Fixed source candidate; input acquisition and guest execution remain disabled.**
Base: `d379e6fe6ffadecc32e3282226d0a7859c9a9495`. The R4 review confirms the R3 boot
configuration and UUID corrections, then reports four P2 lifecycle/filesystem
findings. All four are reproduced in preserved tests and corrected here. Independent
acceptance of this new source/seal is pending. Scope remains this directory.

## Worker lifetime

`Worker.run()` supervises until the direct child is independently observed exited
and both output pipes have drained. EOF and an early terminal JSON frame do not
remove RSS, per-input or stage-deadline checks. Non-reaping exit observations use
at most 50 ms slices; deadline checks follow each observation. Missing RSS while
the child cannot be shown exited fails closed. Cancellation still precedes the
sole reap; the unreaped child remains the PID/process-group identity anchor.

Fixtures cover the original 650-second closed-pipe reproduction, elevated or
unobservable RSS after EOF, input and stage deadlines, missing result, nonzero
exit and normal delayed exit. Process/clock/selector/RSS boundaries are entirely
doubled. Sampled RSS permits overshoot; real scheduling, resource-limit setup,
process cancellation and Node permissions remain unrun.

## Retained filesystem identity

`DirectoryChain` in the existing `lifecycle.py` retains each directory descriptor
from `/` to the target, with a 32-component bound. It opens each component without
following symlinks, compares named and opened identities, and rechecks every edge
in reverse order. Creation occurs relative to a checked parent descriptor. A new
private leaf is observed and compared when opened; parent replacement or a later
leaf mismatch vetoes use. Failure closes owned descriptors and preserves uncertain
state. Existing roots are still refused; no adoption, retry or broad removal exists.

`LocalStore` uses that chain before creating the attempt marker and around durable
writes/publication. Receipts and published inputs are rechecked after syncing.
If a partial name is replaced after linking, it is preserved rather than unlinked.
The Python archive reader and selected-verifier writer use the same directory
boundary, including known identities for reused nested output directories. This
addresses the adjacent instances of the same path-following pattern without
changing the existing VM cleanup protocol or adding an execution component.

`verify_boot_files()` retains and revalidates the whole Data-directory chain;
checking only files in an already-open, detached directory is insufficient.
`MacKernel.prestart()` holds an additional chain through the final configuration
readback. Replacement of Data, a package ancestor or an opened file fails even
when replacement bytes are identical. Complete configuration, UUID, file-set,
size/hash, hardlink and mutation checks remain in place.

Boot files, Python archive/receipt inputs and Node verifier inputs are opened
nonblocking before type validation. A FIFO with no writer therefore fails promptly.
Regular files continue through their hash/identity checks. This is a source-level
observation boundary; it does not freeze the filesystem after return or isolate
code from a trusted owner who can modify it. No VM Start API was introduced.

## Fixtures and preserved evidence

All R1–R4 reviewer function/class ASTs and the frozen R2 missing-C05 fixture remain
unchanged. R4's four original tests were first run against unchanged R4 and all
four failed. Additional temporary-file tests cover symlinked ancestors, parent and
leaf replacement during creation/open/sync, package/Data detachment, receipt and
publication replacement, preserved unowned partials, hardlinks, archive readback,
selected-member output and descriptor cleanup after observation failure.

One existing pinned Node process runs only the repository's `hashRegular` helper
against a fixture FIFO. Its initial three-second timeout was killed/reaped by the
test harness and retained as red evidence; after the nonblocking correction it
rejects promptly. No acquired JavaScript, real publisher crypto or supervised
inspection worker ran. Synthetic verifier member bytes are written and inspected,
never imported. Other process/HTTP/crypto boundaries remain doubled.

`validation.json` records commands, counts, hashes, red/green evidence, disabled
entrypoints and unrun checks. All 11 retained official UTM Swift files and their
Apache/copyright evidence remain byte-identical to R3 at source commit
`048ca7498ea3a374439149d51739d94c5300bcda`. No public source or artifact was retrieved.
The unchanged packet, tool/input pins and earlier evidence remain referenced by
the source seal. Full OS/shared-library closure is not claimed.

## Separate remaining dependencies

1. Independently accept this fixed source and seal. No activation authority is
   implied by source tests, this commit or the earlier seals.
2. Before the eight-input operation, inspect actual tool/storage/memory prerequisites
   and obtain exact owner authority. Record and review the changed activation
   source/seal with guest execution still disabled. R5's disabled seal cannot stand
   in for changed activation source.
3. Acquire/authenticate the same eight non-UTM inputs within 814,088,192 retained
   bytes; verify real module closure, registry/publisher signatures, negative
   controls and Ubuntu member inventory. The initial CLI still ends with
   `image_bound: false` and `runtime_authorized: false`. Actual TLS, worker bounds,
   Node permissions and archive compatibility require evidence during that scope.
4. Later bind real members, decoded kernel/initrd/label facts, seed/ISO identity and
   retained ownership. Fresh registry/bookmark capture, app/helper/PTY attribution,
   owned cleanup and exact attended runtime authority remain separate prerequisites
   to any UTM/VM trial. Real canaries, Stop/quiescence, T07 and installed-product/
   provider acceptance remain open.

The planned host root remains absent. No actual download, downloaded-module
execution, live adapter, resource-limit setup, installation, mount, UTM/VM launch,
credential/account operation, host security change, provider cleanup, push or
merge occurred in this revision.
