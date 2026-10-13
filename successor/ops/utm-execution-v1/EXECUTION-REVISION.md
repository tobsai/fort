# Disabled macOS memory-policy revision R7

This revision implements only the authorized source preparation. Independent
acceptance, owner authority, corrected live validation and acquisition remain
pending. Both inspection gates and the guest execution gate are false.

Base: R6 commit `7ef4215c7b850fc27918b71cc9b562301d305dd1`, seal
`fd4c3110c319a1097d7bc419a7bd692be3d8f50b64d946a696076421b3f3b88b`.
R6's actual attempt is consumed. Its state and evidence are preserved; this slice
uses the previously recorded outcome without accessing that host root.

## Minimal source correction

`child_limits` omits the RSS/AS setter only when `sys.platform == 'darwin'`.
Core, CPU, file and descriptor limits remain exact, and their errors propagate.
Linux and other non-Darwin paths retain the RSS call. There is no error-swallowing
fallback, replacement setter, larger number, infinity assignment or parent change.

All profiles, worker argv, RSS observation, missing/malformed RSS handling, EOF and
deadline supervision, group cancellation and reap order remain unchanged. The
macOS stop thresholds still depend on a live, scheduled parent. Overshoot has no
proven byte bound; this is not hard memory enforcement. Node's 256 MiB old-space
limit is not its total heap or process-memory limit.

## Separate proposed operation

The fixed root becomes
`/Users/tobiasgunn/Library/Application Support/FortUTMInputInspectionR7`.
Its proposed operation ID is `utm-input-inspection-r7-attempt-1`. It was absent
when inspected; no directory was created. Existence at future preflight is a veto,
not authority to adopt, delete, reuse or select a different path.

The consumed root `/Users/tobiasgunn/Library/Application Support/FortUTMIsolation`
is excluded from access and effects. Original input inventory, eight URLs, hashes,
limits, authentication, archive checks and retained-state semantics are unchanged.
Source checks still precede state creation and each worker. No CLI override or
runtime environment variable can enable this disabled preparation.

## Evidence

The new fake-resource regression first failed on the observed RSS/AS alias and
now proves the exact four macOS calls, all five Linux calls, unchanged thresholds
and fail-fast propagation across all six existing profiles. Existing worker,
acquisition and pipeline fixtures preserve excessive/unobservable RSS refusal,
EOF supervision, deadlines, cancel-before-reap, retained failure state and refusal
to reuse a root. No real OS resource setter or corrected worker was run.

The previous diagnostic and reviewer decision are copied with provenance under
`evidence/r7/`; originals are unchanged. Historical R6 preparation fields are
preserved there and in Git, not relabeled as current execution evidence.
`validation.json` records the new exact safe checks and their limitations.

## Approval boundary

[APPROVAL.md](APPROVAL.md) is the precise revised draft, including explicit sampled
memory risk, all eight inputs, budgets, the new root and failure/cleanup behavior.
It requires independent review of this disabled source, a concrete fixed
activation/smoke command packet, then owner approval bound to those identities.
The network-free smoke command is a required next preparation gate, not code
executed or approved by this revision. Actual remaining setters, RSS observation,
owned cancellation/reap and unchanged parent limits must pass before acquisition.

No automatic repeat of the old attempt, marker reset, root removal, corrected live
worker, download, downloaded module, mount, UTM/VM, provider or global host setting
change occurred. The preparation commit/seal are returned separately; neither
self-reference nor this document creates execution authority.
