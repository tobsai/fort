# Mac agent storage placement

October 18, 2026. [Spec 059](../../../specs/059-modular-node-v1.md), under the
existing [Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
This slice follows `d144a69`. All filesystem mutations and process execution use
synthetic temporary storage. No real App Group lookup, provider invocation, account,
credential, profile, entitlement, startup registration or installed-app change occurred.

`MacAgentStorage.prepare(for:)` resolves only the validated endpoint's exact group
identifier through `FileManager.containerURL(forSecurityApplicationGroupIdentifier:)`.
The injected resolver is internal to the test seam. There is no alternate storage
path and no container creation by Fort. It opens the supplied container without
following its final symlink, requires current-user ownership and owner read/write/
search permission, rejects group/other write permission and special mode bits, and
rejects extended ACL entries. The root's location comes from its descriptor's canonical
path; it is not reconstructed from HOME or assumed group-container conventions.

Only the fixed `FortHost` child is created, at mode 0700 using the parent descriptor.
An existing child must be a real current-user directory with exactly mode 0700 and
no extended ACL entries. Unknown files, FIFOs, symlinks, unsafe modes and ACLs are
preserved and refused. No chmod, ACL removal, history migration or deletion occurs
in production. New directory metadata is synced before returning. Reopen preserves
existing contents. A returned value checks both original device/inode/mode identities
and current ownership/ACLs before yielding its directory for launch.

This is placement and preflight, not a lifetime filesystem lease or protection from
a hostile same-user process. The core retains its exclusive ledger/socket ownership.
Runtime/package validation, startup ownership and history removal are separate.
Agent-host access and computer-tool sandbox access remain distinct.

## Apple API and unrun platform gates

[Apple's API reference](https://developer.apple.com/documentation/foundation/filemanager/containerurl(forsecurityapplicationgroupidentifier:))
states that macOS may return a group URL even when its entitlement is invalid. A URL
alone is therefore not authorization evidence. This adapter verifies access and
storage properties; the eventual production wrapper must separately validate its
signed package and matching profile. The actual system resolver was not called.
The documented API may cause the OS to create an authorized group container when
used; the tests only supply existing synthetic directories. The read-only public
documentation copy is `/tmp/fort059-apple-group-container.md`.

The deliberately strict no-extended-ACL policy may reject a real provisioned group
container. That compatibility remains unverified; this slice does not strip an ACL
or claim actual App Group readiness. Existing local inventory still lacks matching
Mac/agent App Group profiles. No service activation or profile change was attempted.

## Verification

Preserved initial failures:

- `/tmp/fort059-agent-storage-red.log`: sandboxed Swift invocation could not write
  its existing compiler cache. The supported escalation was approved; no sandbox
  setting was changed.
- `...-missing-red.log`: the first public test failed because `MacAgentStorage`
  did not exist.
- `...-first-green.log`: compilation exposed missing `try` annotations and Darwin
  ACL pointer/enum conversions; those were corrected.
- `...-green.log`: an empty ACL on this macOS returns null with `ENOENT`. A direct
  temporary-directory metadata probe confirmed it; only that missing-ACL case is
  accepted. Existing nonempty ACLs are independently refused by regression tests.
- `...-first-pass.log`: the canonical-path expectation used Foundation, which
  returned `/var` while `F_GETPATH` returned `/private/var`. The test now compares
  against POSIX `realpath`; `...-history-pass.log` passed.

Seven behavioral functions passed, including the guarded compiled-core journey:
exact group lookup, missing lookup/container, invalid URL, preserved history,
file/symlink/FIFO refusal, shared/read-only permissions, foreign-owned root refusal,
extended ACL preservation, changed child/container identities or permissions/ACLs,
and 20 concurrent preparations converging on one directory. The ACL fixtures alone
use `chmod` on their own temporary directories, then remove their fixture ACLs.
Production never runs chmod. The guarded journey pairs a synthetic device, reads
the exact installation, recovers that pairing, confirms child/group absence and
retains the ledger. It uses the production `serveOwnedCore` entry with an explicitly
synthetic runtime; no provider implementation is selected.

The full native suite passed in `/tmp/fort059-agent-storage-swift.log`:
**173 functions executed; two timing functions excluded**. Swift Testing reports
175 registered functions. The new timing function passed separately below; the
unchanged startup timing function retains its earlier isolated evidence.

```sh
FORT_TEST_GUARDIAN=/tmp/fort059-mac-core-host-guardian \
FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node \
FORT_TEST_CORE_ARTIFACT="$PWD/successor/.build/owned-core-059-20261018" \
swift test --package-path successor/apple/FortKit \
  --scratch-path /tmp/fort059-owner-setup-xpc-native
```

The unchanged core has 30 files / 190,826 payload bytes, with manifest SHA-256
`c1a19bd8e8f0def68efa45f2b913dc5b2123466a3da6a5c14ce0eb8fded7f527`.
Portable Node, TypeScript and signed embedded-XPC suites were not rerun for this
Swift-only addition. Their earlier results remain scoped to their recorded revisions;
the signed fixture does not use this new storage adapter.

Fresh unsigned Debug builds both passed:

- `/tmp/fort059-agent-storage-mac-build/Build/Products/Debug/FortMac.app`
- `/tmp/fort059-agent-storage-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`

Commands use the existing project/schemes, generic platform destinations, separate
fresh derived-data directories and `CODE_SIGNING_ALLOWED=NO`. Logs are
`/tmp/fort059-agent-storage-mac-build.log` and `...-ios-build.log`. Both still report
CoreSimulator 1171.7.0 versus required 1174.9.2; device support is disabled. Compilation
is not Simulator UI, installed accessibility/screenshot, provider or TestFlight proof.
No app was launched. Ordinary app composition remains unconfigured for this host.

## Storage operation timing

MacBookPro18,2, M1 Max, 64 GiB, 10 CPU; macOS 27.2 (26B5112a), Xcode 27.1 /
Swift 6.4. Twenty fresh synthetic containers, with prepare/reopen/validate measured
serially after compilation, with warm filesystem/binary caches and no cache purge.
No other tests or builds ran concurrently. Timing includes descriptor checks, ACL
reads, child creation and sync for prepare; reopen repeats the checks/sync. Parent
fixture creation and cleanup are excluded, as are Apple lookup, process launch,
provider, Keychain, ServiceManagement and UI latency. There is no idle work.

Run with `FORT_AGENT_STORAGE_BENCHMARK=1` and the Swift filter
`agentStoragePreparationAndInspectionStayWithinTheirIsolatedBudget`.
Nearest-rank p50/p95 in milliseconds, with a 50-ms p95 budget per operation:

| Operation | p50 | p95 |
| --- | ---: | ---: |
| Prepare | 0.195208 | 0.267416 |
| Reopen | 0.136667 | 0.155291 |
| Validate | 0.060209 | 0.067792 |

Raw logs and derived results: `/tmp/fort059-agent-storage-budget.log` and
`/tmp/fort059-agent-storage-budget.json`. All samples, in milliseconds:

```json
{
  "prepare": [0.390083,0.227333,0.200292,0.201125,0.195208,0.184208,0.200709,0.183916,0.188291,0.186458,0.180333,0.254666,0.190083,0.201708,0.183417,0.183916,0.267416,0.198833,0.193042,0.200125],
  "reopen": [0.148958,0.239083,0.14675,0.13625,0.133542,0.133084,0.132166,0.132,0.139542,0.131916,0.13175,0.155291,0.133958,0.137542,0.135583,0.138167,0.150583,0.136667,0.141,0.141042],
  "validate": [0.063583,0.067625,0.060209,0.060042,0.059708,0.06275,0.059625,0.059833,0.059417,0.05975,0.062333,0.061083,0.060375,0.0595,0.0595,0.059875,0.067792,0.06325,0.061459,0.074]
}
```
