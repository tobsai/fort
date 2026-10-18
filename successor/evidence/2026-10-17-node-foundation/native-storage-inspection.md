# Existing native host storage — synthetic inspection evidence

October 18, 2026. Governing [Spec 059](../../../specs/059-modular-node-v1.md),
base `2c1246173de396af1ac1ecafe41759eaa9adedf7`, existing implementation authority.

`MacAgentStorage.inspectExisting` opens only the exact resolved App Group's existing
FortHost directory. Preparation and inspection share canonical descriptor, inode,
owner, mode, ACL and revalidation checks. Inspection omits directory creation and
fsync; it does not create Runtime paths, repair permissions or examine history files.
The default resolver remains Apple's supported exact-group API. A returned URL alone
is not evidence of accessible storage, so the descriptor checks remain necessary.
[Apple FileManager reference](https://developer.apple.com/documentation/foundation/filemanager/containerurl%28forsecurityapplicationgroupidentifier%3A%29).

Only injected synthetic container resolvers were exercised. Actual OS App Group
resolution, sandbox grants and any OS-level container provisioning remain unverified.
No real container, Keychain, account, startup registration, provider or history was
accessed. The native removal operation is still unavailable; this is its storage
precondition, not completed deletion wiring or installed acceptance.

The first behavioral test failed because routing inspection through preparation
created an absent FortHost directory. The implementation now separates the two
entry-point intents while sharing one validator. The focused run passes ten test
functions, with three optional functions skipped (13 discovered); the parameterized
runtime guard contributes 25 cases. It includes exact group resolution and retained
bytes, missing storage, five unsafe container variants, six unsafe child variants,
and eight post-inspection replacement/permission/ACL variants. Existing preparation
and guarded lifecycle behavior are checked by the full native regression run below.

Performance: 20 repeated warm inspect-and-revalidate calls against one synthetic
prepared container, on MacBookPro18,2 / 10 CPU / 64 GiB / macOS 27.2 / Xcode 27.1,
Swift 6.4. No concurrent test/build workload. Nearest-rank p50 0.190250 ms, p95
0.195417 ms, maximum 0.197917 ms; target p95 <=50 ms. Fixture creation is excluded.
No installed App Group latency claim is made.

```json
[
  0.197917,
  0.193875,
  0.195417,
  0.191125,
  0.191083,
  0.194166,
  0.190583,
  0.19025,
  0.190375,
  0.190042,
  0.19,
  0.190125,
  0.190042,
  0.189459,
  0.189875,
  0.19275,
  0.189375,
  0.189459,
  0.190958,
  0.189958
]
```

Commands use `xcrun swift test --package-path successor/apple/FortKit
--scratch-path /tmp/fort059-storage-inspection-swift --no-parallel`.
Focused measurement adds `--filter agentStorage` and
`FORT_STORAGE_INSPECTION_SAMPLES=/tmp/fort059-storage-inspection-samples.json`.
The full regression uses the retained compiled synthetic core/resources and guardian
from the OpenSSL slice; no official runtime is invoked.

No new Mac/iPhone application artifact, installed inspection, screenshot/accessibility,
provider acceptance, signing/profile change, archive, upload or TestFlight check is
claimed. The prior unsigned application packages remain unchanged. Full native
history-deletion composition and iPhone saved-history deletion remain implementation.

- `/tmp/fort059-storage-inspection-red.log` SHA256 `2806c2ea219cde9120858405479d84887a178fa366d2461cd26ceb7c56bc04b4`
- `/tmp/fort059-storage-inspection-green.log` SHA256 `d73f8fdef1a3011a05dc41864c444809403898d5aca001049b43bc8f8cc8a382`

## Full native regression and fixture repair

The initial full run stalled in the existing ObservedChild test fixture after its
synthetic cat child had exited. A one-second sample of the exact test PID showed
Foundation waitUntilExit blocked from MacHostProcessTests.swift:33. Read-only process
inspection found no child of that test process. Only that verified test PID was
terminated. The interrupted run is a failed/incomplete check, not a pass.

The fixture now owns a POSIX-spawned cat child, closes its input for ordinary EOF,
and reaps with waitpid and a one-second deadline. Its destructor can signal only
that still-unreaped owned fixture child on failed cleanup. Close-on-exec defaults
prevent pipe inheritance. Production process observation, startup and cleanup code
are unchanged. All seven process-observation functions pass after this test-only repair.
The final serial full run passed **251 functions**, with **10 optional checks skipped**
(261 discovered), including the compiled synthetic native host journey and owned-group
absence checks. No real provider/runtime or product service was involved.

- `/tmp/fort059-storage-inspection-native.log` SHA256 `e6520114930af9cba94fbc302c9a902e5f762d9118731e5f9a7f20e89161b2a8`
- `/tmp/fort059-storage-inspection-sample.txt` SHA256 `8062dc5a79eb2398094d916f1ce82128916ac5c9b7a0517e1c7049bb4dddac07`
- `/tmp/fort059-storage-process-fixture.log` SHA256 `04c22b9eb4d3a9aa9b71f0e6039fd158bd4bd765c169df5bc20c28584c91100d`
- `/tmp/fort059-storage-inspection-native-final.log` SHA256 `4b34ead3d7a81e7ca30f8450b9371c97c5dd2e661d45c19840a6fca0fc82abcc`
