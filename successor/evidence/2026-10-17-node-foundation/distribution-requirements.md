# Distribution requirements checked October 18, 2026

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md). This is a
read-only source and retained-artifact assessment. No account was connected,
credential read, agreement accepted, binary downloaded, provider invoked, startup
registered or artifact uploaded. No external agent was contacted.

## Source licenses are permission with obligations

[Node v26.5.0's complete license](https://github.com/nodejs/node/blob/v26.5.0/LICENSE)
permits redistribution subject to its copyright/permission notices. The file also
contains dependency licenses; preserve the complete file, not just its opening MIT
paragraph. The Homebrew runtime additionally links separately packaged libraries,
whose installed notices must accompany the exact shipped closure. No additional
blanket owner/legal approval is needed merely to copy software under these grants.
This conclusion does not certify an incomplete binary inventory.

[Codex's exact source LICENSE](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/LICENSE)
is Apache-2.0. Its redistribution conditions include supplying that license,
preserving applicable attribution/NOTICE and identifying modifications. It grants
no general trademark license. Fort branding therefore coexists with the original
legal attribution. The [pinned NOTICE](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/NOTICE)
also identifies MIT-derived Ratatui code. Those two files alone are not a verified
inventory of every statically linked dependency. The
[pinned Cargo workspace](https://github.com/openai/codex/blob/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/Cargo.toml)
declares version 0.157.1 and many dependencies, including Ratatui 0.30.2.
The next packaging task is to bind complete third-party notices to the precise
binary/source build, not to request generic permission to use Apache-2.0.

The installed Codex package has separate voice licenses, including LGPL-2.1; that
voice payload is not copied into Fort's current runtime artifact. Its presence
must not be misreported as the license of the separately selected Codex executable,
or as proof that the executable's own license closure is complete. The exact
installed package's bin/resource layout and optional companion payloads remain
packaging inputs; no companion is automatically enabled.

## Concrete Apple requirements

Apple's [App Review Guidelines](https://developer.apple.com/app-store/review/guidelines/),
sections 2.4.5 and 2.5.2, require a sandboxed, self-contained Mac bundle submitted
using Xcode, consent for background/startup execution, and Store-delivered updates.
Downloading code that adds or changes app functionality is restricted. Bundling
fixed reviewed JavaScript is not by itself evidence of rejection; dynamic code
features and each nested runtime still need review against those rules. Fort must
not silently add an installer, Homebrew requirement, updater, or sandbox exception.
Section 5.2.2 requires permitted use of third-party services. These rules do not
establish approval of Fort or of its proposed helper/App Group topology.

OpenAI's [app-server documentation](https://developers.openai.com/codex/app-server/)
explicitly describes embedding the protocol into another product. Its
[authentication documentation](https://developers.openai.com/codex/auth/) distinguishes
ChatGPT subscription access from API billing and describes credential storage.
These are official integration seams, not authority to copy the owner's existing
credentials or connect a new Fort sandbox. Exact account/workspace/model eligibility
and effective permission behavior still require an authorized provider check.

The exact external preparation action is a read-only lookup in the owner's existing
Apple account for team `T3JB5MYZ93`, the Mac/iPhone app records, identifiers
`io.mtree.fort.mac` / `io.mtree.fort`, helper `io.mtree.fort.mac.host` and group
`group.io.mtree.fort.mac`. No authenticated Apple tool or permitted desktop account
inspection is available in this task. Do not create replacement credentials or
provisioning records by inference. Actual missing identifier/group/profile mutations
can be proposed only after that lookup. No new product/language choice is required
for the local work in this report.

## Retained Node artifact and failed checks

Read-only audit of `successor/.build/codex-version-059-20261018/`:

- Staging manifest SHA-256:
  `0fe874c5ddb903feddb65d33c8f2bfa9d4b9a296b692c89189bc9c36d19d7b0a`.
- All **26** staging images match their recorded bundled hashes, pass strict
  signature verification and contain no Homebrew dynamic-load path.
- All **39** attribution files match the manifest. The assembler moved them into
  `FortSandboxProbe.app/Contents/Resources/NodeLicenses`; an initial check against
  the old staging location failed, then the correct assembly location passed.
- Signature verification inside the execution sandbox returned
  `CSSMERR_TP_NOT_TRUSTED`. Read-only verification with macOS trust-service access
  passed for all images. No signing/private-key operation was performed.
- Audit: `/tmp/fort059-distribution-retained-audit.json`. This rechecks retained
  engineering bytes, not a new distribution package or runtime execution.

A fresh empty-environment Node packaging test with explicit ad-hoc identity `-`
failed at SQLite launch because macOS hardened-runtime library validation rejected
Team IDs. Log: `/tmp/fort059-distribution-node-test.log`. This reproduces the
previously recorded ad-hoc limitation in `results.json`; it is not a newly discovered
production regression. The fixture removed only its own temporary directory.
Library validation was not disabled. The earlier explicitly development-signed
execution proof remains historical evidence; this turn did not replay its grant.

The 20 retained dependency packages are ada-url, brotli, c-ares, hdrhistogram_c,
icu4c, libffi, libnghttp2, libnghttp3, libngtcp2, libuv, llhttp, merve, nbytes, Node,
OpenSSL, simdjson, simdutf, SQLite, uvwasi and zstd. Versions and exact per-file hashes
are bound by the retained manifest. A complete fresh app package still needs the
current core resources, helper, runtime signatures/profiles and exact Codex notices.
