# Remaining inspection and execution gates

R4 now contains the bounded acquisition, worker, archive-selection and offline
verifier source previously missing. Read [INPUT-INSPECTION.md](INPUT-INSPECTION.md)
for exact operations, limits and retained-state behavior. It is disabled and has
not acquired or executed artifacts. No further UTM source retrieval is needed.

The next separately authorized operation is input acquisition/authentication:
exactly eight non-UTM entries from unchanged `inputs.json`, maximum retained payload
814,088,192 bytes, existing pinned tools, a fresh private owned root, no credentials,
no proxy, bounded redirects, no retry and no package hooks. It includes importing
only the authenticated, inspected OpenPGP entry in the restricted Node worker.
It excludes UTM.dmg, installation, mounting, VM/app launch and providers.

Before that operation, obtain independent acceptance of the fixed source/seal and
record exact owner authority plus the activation revision. Inspect available disk
and memory reserves and the existing tool/runtime trust boundary. Tool entrypoint
hashes are checked; full OS/shared-library closure is not claimed. Unsupported
resource-limit or permission behavior must stop rather than weaken the profile.

Actual archive contents, module closure, publisher/registry cryptography, negative
controls, raw layout, names, decoded kernel and initrd facts remain unrun. The
initial CLI returns inventory with `image_bound: false`; bind exact observed
members and ownership in a reviewed follow-up. Unknown formats require observed
evidence and a reviewed decoder, never guessed offsets, replacement inputs,
mounting or conversion. A failed or consumed root cannot be adopted or retried
implicitly. Verified inputs remain for a recorded handoff; uncertain files remain
quarantined.

After real inputs are bound, complete live registry/bookmark capture, actual host
process/helper/PTY attribution, seed/ISO identity and owned cleanup. The new
pre-start gate requires the full accepted configuration and all four Data-file
hashes before Start. Runtime authority is a later separate decision. Real canaries,
Stop/termination evidence, cleanup, T07 and installed-product/provider acceptance
remain open. No provider cleanup is authorized.
