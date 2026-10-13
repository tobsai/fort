# Remaining input inspection and runtime work

R3 completed the authorized retrieval of three pinned UTM Swift files. Provenance
and schema findings are in [UTM-SOURCE.md](evidence/r3/UTM-SOURCE.md). Those public
text reads require no further permission or repeat retrieval.

The following artifact operation remains outside the present authorization.
Its acquisition/decompression orchestration and selected verifier-module loading
must become concrete reviewed source before seeking approval. This document is
an inventory and plan, not an executable or approved downloader.

Acquire only the eight non-UTM entries in unchanged
`../utm-synthetic-packet-v1/inputs.json`: the dated Ubuntu ARM64 tar, checksum list,
detached signature, publisher public key and package manifest, plus pinned
OpenPGP 6.3.2 metadata/archive and npm public signing keys. Preserve exact URLs,
digests, caps, redirect hosts and no-credential/no-proxy policy. Combined maximum
payload is **814,088,192 bytes**. Omit UTM.dmg; reuse the accepted static app
assessment as historical evidence. No alternate image/key, package installation,
account, credential, mount or guest program is part of input inspection.

Inspect the verifier archive without package hooks or dependencies. Authenticate
its selected module before loading it with the existing pinned Node 26.5.0.
Verify Ubuntu signed checksums against fingerprint
`D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81`. Run the existing negative-control
classes with in-memory mutations. The supplied verifier API in today's fixtures
is a crypto double, not an authenticated library or a signature-verification pass.

List/hash the authenticated Ubuntu archive without extracting a guest filesystem.
Record exact member paths; count physical tar headers and reject unsupported
metadata before preprocessing. Caps: compressed 768 MiB, raw 20 GiB, combined
auxiliary 192 MiB, at most 64 physical entries. Include retained compressed
kernel plus decoded output and other auxiliary members in peak accounting.
Record source and decoded kernel hashes/sizes. Use a bounded worker for large
hash/decompression work; the current parser's logical budget is not a process
RSS/deadline guarantee. Preserve bounded receipts and stop on any resource,
authentication or format failure. An unsupported raw layout requires observed
facts and a reviewed decoder, never inferred offsets, resizing or a replacement
kernel. No host mount, disk conversion or VM boot belongs in this operation.

UTM schema decoding is now implemented against verified text. Fresh live capture
of persisted and in-memory registry state, plus no-UI/no-mount bookmark resolution,
remains a separate missing adapter. The parser accepts supplied evidence only;
the Mac path refuses to proceed without that provider. Do not infer no shares
from an absent entry or trust a bookmarked path without resolution.

UTM registration/launch, PTY access, guest seed application, process control and
outside-root app-state cleanup remain later runtime work with exact owner
execution authority and independent acceptance. No provider cleanup is authorized.
Retain uncertain state; remove only exact positively owned objects after fresh
quiescence evidence. T07 and installed-product/provider acceptance remain open.
