# Input-inspection activation candidate R6

**Prepared and unrun; independent activation review and exact owner approval are
pending. Guest execution remains disabled.** Base commit:
`56916fa8f994b5f8b59ca79e2f0bc5b105d6cf5e`, accepted disabled-source seal:
`a77c433f047412ded2245daf648a518f044f05b8e97604618d04b18aa51c89bb`.
The R5 review closes all findings and requires this concrete preparation before
any acquisition decision. It supplies no acquisition or runtime authority.

## Exact source delta

The Python `INSPECTION_ENABLED` and matching Node constant change from false to
true. The CLI warning/docstring and static verifier describe a prepared activation
candidate. Two fixture expectations now check that prepared state; missing exact
source-seal identity still rejects before a worker is constructed. The accepted
worker, filesystem, authentication, archive, result and cleanup implementations
are unchanged. All input/tool pins, resource/redirect limits and guest guards are
unchanged. There is no new adapter, service, dependency or broader operation.

`input_inspection_authorized: false` records that no owner decision exists at
preparation time. Future authority must explicitly bind the immutable activation
commit/seal outside this snapshot; no environment variable or this document
creates it. Do not change source after review and silently reuse its old identity.
Only the supported CLI's unselected inventory path is proposed: `image_bound` and
`runtime_authorized` must both remain false.

## Read-only prerequisites

The exact proposed host root is absent. Every ancestor was opened without following
symlinks and matched its declared identity. User 501 owns the home/Library/Application
Support chain; the final parent is mode 0700. Safe access checks outside the
restrictive fixture sandbox report read/write/execute permission; no write probe
or new host directory was created. `df` and a read-only query of its already-mounted
volume identify writable internal APFS at `/System/Volumes/Data`.

`statvfs` and `df` report 733,184,000,000 available bytes. The conservative retained
logical-file budget is 839,843,840 bytes, leaving 732,344,156,160 bytes before
filesystem metadata/rounding. The APFS `diskutil` per-volume `FreeSpace` field is
zero; it is retained as returned and is not substituted for the consistent
`statvfs`/`df` available-space observation. No capacity was reserved.

The host reports 64 GiB RAM, 81% on the memory-pressure free metric, and no swap
in use. Exact free/inactive/speculative page counts are retained. These observations
support proposing the 3 GiB peak worker RSS threshold; they do not make RSS a hard
quota or reserve memory. No extra numeric reserve requirement is introduced.

Python, Node and ps bytes/resolved paths match all three original entrypoint pins.
Node reports v26.5.0 and documents the required permission/heap flags. Existing
Python reports 3.9.6, kqueue availability and all five required resource-limit
interfaces. Current limits were read, never set. An own-process ps RSS query
succeeded. The selected Xcode Python binary and Node linked-library paths are
recorded as supplemental trust-boundary evidence, not a new full runtime seal.
Actual Node permission/resource-limit enforcement remains for the authorized
operation and must stop if unsupported.

Initial restricted-sandbox ps/sysctl/parent-access queries were denied; the narrow
read-only supplement succeeded without changing permissions. A diskutil query
against an arbitrary parent directory failed; querying the actual mountpoint
reported by df succeeded. Raw results remain in `evidence/r6/`. No host security
setting, mount, network request or runtime process was changed or activated.

## Approval and verification

[APPROVAL.md](APPROVAL.md) states all eight URLs and caps, code origins, host trust
boundary, paths, storage/transfer limits, failure behavior and explicit exclusions.
[activation-operation.json](activation-operation.json) preserves exact pins and
machine-readable scope. Retained body bytes are capped; HTTP/TLS overhead is not
an aggregate on-wire quota. The proposal does not include UTM.dmg or a VM trial.

The preserved fixture suites, syntax checks, original regression ASTs, accepted
implementation identity, retained UTM evidence and new source seal are verified.
Only the guest entrypoint is exercised for exit 78. Neither `inspect-inputs.py`
nor the Node verifier CLI is executed after activation preparation. Imported test
helpers load only repository code; no downloaded JavaScript is available or run.
`validation.json` distinguishes these checks from unrun acquisition and runtime.

Next: independently accept this activation commit/seal, then obtain exact owner
authority for one consumed input operation and its narrow execution allowance.
Recheck changing prerequisites before that run. Actual signed inputs, selected
module, negative controls and archive inventory remain absent. Later member/seed
binding, fresh registry/bookmark capture, app/helper/PTY attribution and exact
attended runtime authority are separate VM gates. T07 and installed-product/provider
acceptance remain open. No provider cleanup is authorized.
