# Prepared bounded input-inspection activation

R5 disabled source is accepted. R6 prepares Python and Node inspection gates as
literal true for separate activation review. No acquisition or downloaded-code
execution is approved or performed. Guest execution remains separately false.
Do not invoke either prepared entrypoint before exact owner approval of its seal.

## Fixed operation

The prepared `inspect-inputs.py` requires the exact accepted activation seal
SHA-256 as its argument. Running it can create the root and start acquisition;
it is intentionally unrun in this revision. Source preparation is not authority.
`source_identity.py` checks the entire source/reference closure
before creating state and again before each worker; guest execution must remain
disabled. There is no environment override, automatic retry or implicit recovery.
Changing inspection authority never enables UTM or guest execution.

The operation creates the previously absent private root:
`/Users/tobiasgunn/Library/Application Support/FortUTMIsolation`.
Inputs are under `downloads/`; the checked verifier closure is under `verifier/`;
receipts and the selection manifest remain inside that root. Existing roots are
refused. Python creation and archive/member IO retain and recheck every ancestor
through directory descriptors, with a 32-component bound. New leaves are created
relative to verified parents and compared before use. Symlinked ancestors fail
before creation; replacement observed during the operation vetoes acceptance.
No app, VM package, host mount, provider or credential is created.

Exactly eight unchanged entries from `../utm-synthetic-packet-v1/inputs.json`
are eligible; UTM.dmg is omitted. Maximum retained download payload is
814,088,192 bytes. Each input retains its exact URL, cap and SHA-256/SHA-512 pin.
Only the recorded HTTPS hosts and at most four redirects are allowed. Redirect
queries, cookies, arbitrary headers and response bodies are not diagnostics.
Direct standard-library HTTPS uses certificate verification, fixed request headers,
no proxy, cookie jar, authentication or netrc. Transformed, ambiguous, partial,
over-limit or failed responses cannot produce an accepted input. A one-byte
oversize detection read may occur; that byte is never written as accepted data.

## Ordered stages

1. `acquire`: consume the fresh attempt, stream pinned inputs into exclusive
   partial files, verify EOF/length/digest, synchronize and publish exact owned
   files, then publish bounded receipts and completion. Root/inode changes or
   persistence failures veto acceptance. Failure stops the next request and
   preserves uncertain state; it does not delete replacement files or retry.
   Receipt and published-file identity checks extend through directory sync. A
   replaced partial name is rechecked before unlink and preserved on mismatch.
2. `registry`: with existing pinned Node, stream-rehash all eight regular inputs
   and verify npm registry signatures using built-in cryptography. No downloaded
   module has loaded at this point.
3. `verifier_archive`: verify the 8 MiB archive pin; inspect physical USTAR headers
   before preprocessing; reject links, sparse/special/PAX/GNU headers, traversal,
   duplicates, setuid/setgid entries and native addons. At most 256 entries and
   24 MiB expanded data are allowed. Check package name/version, no declared
   dependencies, exact `./dist/node/openpgp.mjs` export, and retained LICENSE.
   Copy only checked regular members into a fresh private directory. Every file
   receives a size/hash manifest. Its exact SHA-256 is carried to the next worker.
4. `publisher`: independently recheck input pins and registry authenticity, the
   supplied selection-manifest digest, exact package file closure, every member's
   size/hash and declared entry. Only then import that authenticated module.
   Node receives read permission solely for the fixed verifier sources, pinned
   input inventory, acquired inputs and checked module closure. Network, child
   processes, workers, addons, FFI, WASI and filesystem writes are not granted.
   Verify Ubuntu checksums against primary fingerprint
   `D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81`; require rejection of mutated sums,
   signature, public-key text and expected fingerprint. No other key is fetched
   or generated. Parent acceptance checks the exact archive/publisher identity
   and typed signature/negative-control results.
5. `ubuntu_inventory`: verify the exact archive pin, count physical headers and
   stream member hashes without extracting a filesystem. Apply 64-entry, 20 GiB
   raw and combined 192 MiB auxiliary bounds. An unpartitioned ext4 candidate
   must be observable; unknown layout requires a reviewed decoder. Record actual
   names rather than guessing kernel/initrd names. This does not prove bootability.

The supported CLI ends with authenticated inputs and inventory, explicitly
`image_bound: false`. The policy API also supports an exact, already reviewed
member selection: it must name three distinct observed members, use the original
archive pin and satisfy the existing full image/decoded-kernel validation. That
optional path is fixture-tested; real names and image facts are still absent.
It is not an automatic fallback or a way to reuse a failed/consumed root. A later
recorded handoff must bind observed members and retained ownership before reuse.

## Worker limits

| Worker | Wall deadline | CPU soft/hard seconds | RSS stop threshold | Regular-file write limit |
| --- | ---: | ---: | ---: | ---: |
| Acquisition | 900 s total; 600 s per input | 120/121 | 256 MiB | 768 MiB per file |
| Registry verification | 90 s | 80/81 | 1 GiB | 0 |
| Verifier archive | 30 s | 25/26 | 256 MiB | 24 MiB per file |
| Publisher verification | 90 s | 80/81 | 1 GiB | 0 |
| Ubuntu inventory or selected-member inspection | 300 s each | 280/281 | 3 GiB | 0 |

All workers have a 64 KiB combined stdout/stderr cap, 64-descriptor limit and
zero core-file allowance. Node has a 256 MiB JS heap cap. The supervisor tracks
ordered fixed-input progress independently: late, skipped, duplicated or missing
progress cannot extend the 600-second input deadline, including when DNS/read
blocks inside the worker. Socket operations also use the remaining deadline,
capped at 30 seconds. The controller checks an 1800-second staging deadline.

RSS checks request 100 ms spacing, with each `ps` observation bounded to 250 ms.
This is a sampled stop threshold with possible overshoot and OS scheduling delay,
not a macOS RSS quota. The local Darwin manual describes `RLIMIT_RSS` as a
memory-pressure hint. That hint plus CPU/file limits is set in the child before
its entrypoint; unsupported setup fails before work. No virtual-address-space
limit is claimed. Actual OS enforcement remains unrun. The supervisor continues RSS and input/stage-deadline checks even after both
pipes close or a terminal result arrives. Exit observations use at most 50 ms
slices and leave the child unreaped. It retains that child as the process-group
identity anchor, cancels the group on every exit, then waits at most one additional
second.
Failures never become success because cancellation was sent. No arbitrary
untrusted child-tree confinement guarantee is claimed.

Regular-file readers open nonblocking before checking file type; unexpected FIFOs
cannot wait for a writer before rejection. Directory bindings are rechecked after
Python archive reads and verifier output writes. These checks do not prevent a
trusted owner changing files after an observation returns.

The Python archive examiner retains compressed bytes and some kernel data. The
3 GiB RSS threshold is an explicit sampled stop condition;
logical archive/extraction budgets are different. The source does not claim a
filesystem integrity check, guest execution, or protection against trusted owner
or guest-root modification.

## Evidence, cleanup and gates

Per-download receipts are capped at 8192 bytes; inspection result artifacts and
the module selection manifest at 64 KiB. Diagnostic record accounting is capped
at 256 KiB per owned store; fixed eight-input receipts are small and the parent
also bounds worker output. Public-key/error bodies are omitted. Failures preserve
partial/uncertain files and permanently consume this attempt. Successful inputs
remain for a reviewed evidence/ownership handoff; no broad cleanup, adoption or
retry path exists. Review actual storage/free-memory reserves before acquisition.

Entrypoint tool hashes are checked before workers run. These are not a full
shared-library/OS closure. Existing Node permission flags were inspected locally;
no downloaded library, live supervisor, network request or resource-limit setup
ran. An existing-Node fixture process exercises only the repository's regular-file
helper against a temporary FIFO. Real module contents, cryptography, archive formats, names and OS behavior
remain future evidence. Retain exact unsupported results rather than changing
pins, adding a decoder/dependency, broadening permissions or retrying silently.

Next gates: independent acceptance of this activation source/seal; exact owner authority
for the eight-input acquisition and bounded authenticated verifier execution;
a recorded activation revision with guest gates still false; actual receipts,
negative controls and archive/member evidence. UTM/VM execution, live registry
capture, bookmark resolution, installed-product/provider checks and T07 remain
separate later gates.
