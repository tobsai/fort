# Guarded offline history maintenance

Spec 059; source and disposable synthetic fixtures, October 18, 2026. Parent
checkpoint c28720dec6ae89390fad8e274f618c22161d3bae. No account, actual owner
history, startup registration, provider, network access or installed application
was used. The production removal control still cannot delete host history.

A separately compiled entry imports the offline ledger operation without starting
a service, provider, enrollment or socket. It accepts one exact 4 KiB private pipe
command against the process working directory and returns a correlated receipt.
Invalid controls fail closed. Lifecycle ownership reuses the existing native parent
exit guardian. A lost reply is unconfirmed; only the identical request may retry.

The native compiled journey creates synthetic settings/history, revokes pairing,
confirms core exit, runs maintenance, checks both worker PID and process group
absence, retries the exact receipt and reopens the same installation. Settings
remain; history is empty and the old epoch command is rejected. The deletion plus
clean exit took 125.127333 ms against a 5,000 ms bound. This one integration sample
is not a percentile benchmark. Node 26.5.0, Swift 6.4/Xcode 27.1, macOS 27.2,
MacBookPro18,2, 10 CPU cores and 64 GiB; local warm build, no provider latency.

## Verification

- Initial public maintenance stub failed; required packaged entry test then failed
  before the packager was updated. A test waiter was bounded by channel completion
  after the initial red run waited for a ready frame. These failures are preserved.
- Final controls/packaging: 18 passed, one optional measurement skipped.
- Strict TypeScript passed. Swift package build and the compiled native journey passed.
- Full portable suite after all ledger format guards: 1,158 passed, six optional
  measurement skips, zero failures (1,164 total). This supersedes the earlier
  portable run that preceded the two final ledger guards.
- Full native suite: 253 executed functions passed, ten optional measurements skipped
  (263 discovered), 50.335 seconds. The compiled maintenance journey was enabled.
- Fresh host resources: 94 payload files, 381,634 bytes, all manifest hashes verified;
  manifest SHA256 fede7e18652f525ad479e4954218ad8cc75dedb1bdcc950020c7a0b2ec780290.
  Location /tmp/fort059-maintenance-host-resources. No new app assembly/signing.

Commands used pinned Node with verification/test-suite.mjs --suite portable and
node_modules/typescript/bin/tsc -p tsconfig.json from successor. Native tests used
xcrun swift test --package-path successor/apple/FortKit --scratch-path
/tmp/fort059-maintenance-swift --no-parallel with FORT_TEST_HISTORY_RESOURCES and
FORT_TEST_HOST_RESOURCES set to the above resources, FORT_TEST_CORE_ARTIFACT to
its core directory, FORT_TEST_NODE to /opt/homebrew/opt/node@26/bin/node, and
FORT_TEST_GUARDIAN to /tmp/fort059-assembly-guardian.

## Remaining boundaries

Native durable confirmation, completed startup/host/executor/credential absence
checks and client cache cleanup must be composed before enabling host deletion.
The receipt alone proves neither process absence nor filesystem sanitization of
backups/SSD snapshots. No HTTP/XPC deletion route was added. iPhone cache deletion,
actual App Group provisioning, installed UI/provider and TestFlight remain unrun.
Same-local-network synthetic acceptance is the immediate test scope; later user-owned
VPN transport stays possible without changing the domain contracts. This work does
not activate access or alter network/security settings.

## Local raw log integrity

- `/tmp/fort059-maintenance-red.log`: SHA256 `16b38c271cb83e8d37435d4628ac2e265ac3d2220da5f75a83f20b9ab99c842d`.
- `/tmp/fort059-maintenance-green.log`: SHA256 `70d0a391e41c625de02fa4de7bf2c184c7d9236fbadb4f33082b2521969108db`.
- `/tmp/fort059-maintenance-artifact-red.log`: SHA256 `6d72f3cdc61732855e9b5d4e86ce8b4e29edff3506d6fd1a2eb953cfda587e88`.
- `/tmp/fort059-maintenance-controls.log`: SHA256 `1a7ab76f0bbb349eb2d2ddd1d1110f0d98ef607bca7cd7ec82f609bd4518b7dc`.
- `/tmp/fort059-maintenance-typecheck.log`: SHA256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- `/tmp/fort059-maintenance-native-build.log`: SHA256 `ebcad83310c37b5cd6d620a9ff1eca18db1f0f9e8c0d0e264feeb6ec6ecf1907`.
- `/tmp/fort059-maintenance-native-journey.log`: SHA256 `a171c91fae6e0d831fdc9c6ec3144f33fd256b4004bd79e145c012ab413ef6b2`.
- `/tmp/fort059-maintenance-portable.log`: SHA256 `d4240ea705ba8ff614ac40e1e7564cb584efe0ac487cf08a01c23b96cd44b019`.
- `/tmp/fort059-maintenance-native-full.log`: SHA256 `41c43ce541f993e6cd5440a7398bf252bef3e58acb6a4811766d01cf90b40724`.
