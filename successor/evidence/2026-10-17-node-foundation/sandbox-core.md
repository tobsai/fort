# Compiled core inside the signed sandbox bundle

The actual compiled Fort core accepted, dispatched and replayed exact synthetic
messages inside the signed Swift/Node sandbox probe. Restart retained the same
receipt without a second execution. Authentication and the existing filesystem
and TCP denial controls passed. [Spec 059](../../../specs/059-modular-node-v1.md)
governs this local packaging proof.

The [result](sandbox-core.json) identifies the retained bundle, source inputs,
processes and extracted signing entitlements. Source base is
`8ff2dc9a3ac6003ee82aff23b20a0ec7cd6ed135`; input hashes identify the subsequent
probe changes. The artifact is
`successor/.build/sandbox-core-059-20261018-a/FortSandboxProbe.app`.
Its independently checked core manifest contains 20 files and 153,171 bytes,
with SHA-256 `4308dea66dc3c4dd3392de0a7b17bd94de9f4f86165dc7a4a348aa8ac66fca62`.
All produced core file hashes matched that manifest.

The Swift parent retains only `com.apple.security.app-sandbox`; the Node child
retains that entitlement and `com.apple.security.inherit`. Strict/deep signature
verification passed with authorized access to the existing development signing
configuration. No entitlement was added. The compiled core runs with JIT disabled
and an explicitly selected stateless fixture provider. A bounded HTTP client in
the same Node process uses the private Unix socket. No provider account or startup
registration is involved.

Verification:

- Red: the new core scenario failed with `Unknown synthetic scenario` before
  its implementation.
- Green: all three signed sandbox scenarios passed, with zero skips: compiled
  core, existing normal SQLite/socket controls, and hung-child cleanup.
- The retained artifact passed authentication rejection, exact message/replay,
  restart, duplicate receipt, socket removal, outside read/write denial and TCP
  listening denial. Parent, child and owned process group were absent before its
  synthetic container was removed.
- Strict TypeScript passed. The preceding transport slice passed 831 portable
  tests and the compiled Swift HTTP journey; those were not repeated for this
  packaging-only change. Runtime relocation and native UI were not rerun.

Reproduce with the pinned Node 26.5.0 executable and the previously selected
existing identity: run `node --test successor/packaging/sandbox-feasibility.test.ts`
with `FORT_SIGNING_IDENTITY` set to that local identity. A retained artifact is
produced by `successor/packaging/sandbox-feasibility.ts --runtime <pinned-node>
--identity <existing-identity> --destination <fresh-absolute-directory> --scenario
core`. Logs are `/tmp/fort059-sandbox-core-red.log`,
`/tmp/fort059-sandbox-core-green.log`, `/tmp/fort059-sandbox-core-artifact.log`
and `/tmp/fort059-sandbox-core-typecheck.log`.

This is synthetic application and package evidence. It does not establish Swift
peer authorization, XPC, persistent host lifecycle, real-provider authentication,
an isolated executor, reachable iPhone transport or TestFlight acceptance. The
inherited child shares its parent's sandbox privileges; the agent-host boundary
and computer-tool sandbox remain separate. No installed UI or physical-device
acceptance is claimed.
