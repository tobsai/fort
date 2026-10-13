# UTM source preparation revision R4

**Fixed source candidate; input acquisition and guest execution remain disabled.**
Base: `d965abb76d20a17879925ae0e3fedd504ecc6a57`. The R3 review closes all earlier
findings and reports two unique P2 readback defects. Both are fixed here. This
revision also supplies the previously missing bounded input-inspection code.
Review artifacts, prior revisions, source evidence and logs remain preserved.

## Boot integrity and identity

`boot.py` is the common policy for candidate generation and configuration readback.
It compares the complete accepted configuration, including every boot field,
exact command line and masks, package-relative kernel/initrd/drive names, resources,
virtualization settings and empty device/share lists. Missing and unexpected
fields fail. UUID spellings are canonicalized for validation, returned topology
and the host pin. Case alone is equivalent; another UUID is a different identity.

`MacKernel.prestart()` and `Host.prestart()` provide validation before Start,
independent of later process/PTY binding. The pre-start gate requires the accepted
configuration and all four `Data/` file identities, including the eventual seed
ISO. It walks without following symlinks, rejects unexpected files and hardlinks,
streams hashes, checks sizes and before/after identities, and rechecks file set
and configuration. The three-file candidate plan alone cannot satisfy this gate
because no real seed ISO exists yet. No Start/launch API was introduced.

The preserved reviewer tests now accept the untouched candidate and equivalent
uppercase UUID, and reject all four reported boot mutations. Nearby fixtures
cover every boot field missing/changed, extra EFI/platform inputs, altered masks,
absolute/traversal/subdirectory paths, unexpected drive bookmarks, writable seed,
resource/virtualization changes, different UUIDs, replaced/symlinked/missing boot
files and extra saved-state data. The pre-start API is fixture-checked before any
PID/PTY binding. These are source and regular-file fixtures, not UTM readback.

## Bounded acquisition implementation

[INPUT-INSPECTION.md](INPUT-INSPECTION.md) describes the concrete source, ordering,
worker profiles, retained-state policy and next authority boundary. The new modules
provide streamed fixed-input HTTPS, single-attempt durable storage, physical tar
validation, exact verifier-member selection, offline cryptographic verification,
bounded workers and a staged controller. Python and Node inspection entrypoints
exit 78; guest `LIVE_EXECUTION_ENABLED` remains false. No environment override
or automatic retry was added.

The controller authenticates registry/archive provenance before downloaded-module
loading and publisher checks before Ubuntu inspection. It carries the exact
selection-manifest digest across workers and rechecks the module file closure.
Parent acceptance binds typed results to the fixed archive and publisher. The
initial CLI records observed archive names and leaves `image_bound` false. An
optional reviewed-name policy path additionally requires full image facts and
the same authenticated archive; no names or offsets are guessed.

Workers have concrete wall, per-input, CPU, sampled RSS, descriptor,
file-write and output limits. Cancellation precedes child reap. Node receives only
explicit read permissions and a bounded heap. Actual subprocess/resource-limit,
TLS and downloaded-verifier behavior is unrun; all those boundaries are doubled
in tests. A sampled RSS threshold permits overshoot and is not a hard Mac quota.

Synthetic tests cover HTTP status/framing/length/hash/cap failures, unsafe and
looping redirects, stalled input progress, fixed-input ordering, receipt failure,
partial-file replacement, archive path/type/size/dependency/entry failures, wrong
worker authentication identities, stage failures, deadlines, RSS/output failures,
selected-member binding and disabled entrypoints. Temporary verifier fixture files
are hashed and inspected; their JavaScript is never imported or executed.

## Preserved trust and execution boundaries

R1 S1–S3 and R2 STD-1–3/SPEC-1–2 remain fixed. All original regression bodies and
the frozen missing-C05 Linux fixture remain unchanged. Runtime serial denial,
journal endpoint attribution, canaries, actual command cancellation, broker OS
loops and host/guest cleanup still require real evidence after later authorization.

The 11 retained official UTM Swift files still match their recorded identities at
`048ca7498ea3a374439149d51739d94c5300bcda`. No source or artifact was retrieved in
R4. Existing copyright/Apache attribution is unchanged. Version-4 schema knowledge
does not prove an actual Swift decode, installed UTM readback or bootability.
Fresh live persisted/in-memory registry capture and bookmark resolution are still
missing adapters; the default host path refuses without them.

Cloud-init directives would execute if someone separately packaged and booted
the generated seed. Disabled Python gates are workflow controls, not a sandbox
against trusted-owner changes. No package, ISO, mount, UTM/VM, account, credential,
provider, startup service, host security change, push or deployment occurred.
The planned host root remains absent. Earlier app-symlink, outside-root-state and
interrupted-root cleanup refusals remain; no implicit adoption was introduced.

## Validation and remaining gates

`validation.json` records current counts, commands, retained red/green evidence
and unrun checks. Both R3 defects were reproduced against the unchanged base;
the corrected originals and all older regressions pass. Full fixtures use the
existing Python/Node tools and the same narrowly scoped temporary Unix-socket
allowance. Supervisor processes, HTTP and provider cryptography are doubled.
Red logs retain the initial failures; any trailing-space normalization records
original and normalized hashes without changing test results.

The next review is of this source and its seal. After acceptance, a separately
recorded authority/activation revision may permit only the eight pinned input
acquisitions and bounded authenticated verifier execution. Real artifact/member
facts must then bind the image and seed. Fresh registry/bookmark capture,
process/helper/PTY attribution, owned cleanup and exact attended runtime authority
remain later requirements. T07, installed-product and provider acceptance remain
open. No current fixture pass is a live confinement or working-application claim.
