# Native helper service lifecycle

October 18, 2026. [Spec 059](../../../specs/059-modular-node-v1.md) and
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md)
govern this local composition. No real named Mach listener, App Group lookup,
startup registration, credential store, provider or installed app was activated.

## Ownership

`MacAgentService` composes the existing exact App Group storage, guarded
`MacCoreHost`, and same-user/exact-code `MacHostListener`. Construction does
nothing externally. An explicit start resolves storage, validates its identity,
waits for the core's private-channel readiness, revalidates the directory, then
creates/resumes the listener for the inspected bundle's exact service identity.
The trusted helper composition supplies executable, arguments, environment and
guardian. Incoming XPC data never selects those inputs.

The lifecycle permits one start. Explicit shutdown prevents another start,
invalidates the listener once, and awaits/closes any core launch already in
progress. Concurrent callers share the same shutdown task. Canceled startup,
changed storage or listener failure closes the core before returning failure.
Failed launch remains conservatively cleanup-unconfirmed, even when no child may
have been created; the wrapper has no separate absence proof in that case.
Forced or uncertain core shutdown cannot produce a clean receipt or auto-restart.
Failure stays retained for the lifecycle's callers.

Successful owner recovery does not immediately destroy the helper listener. Its
reply can return before the client requests OS unregistration. The wrapper later
closes on explicit shutdown. This is core/listener ownership, not proof of startup
unregistration or helper-process exit. Existing durable removal still verifies
those separately. The real helper executable/bootstrap and its signal-to-shutdown
composition remain to be wired.

No concrete provider is imported by this Swift wrapper. It adds no new dependency,
timer, idle polling, credential, permission or registration path. Its public
constructor requires an already-inspected bundle; the eventual production bootstrap
still owns fresh package validation and executable provenance.

## Verification and limits

Logs share `/tmp/fort059-agent-service-`.

- The initial public lifecycle tests failed to compile because the wrapper and
  listener seam did not exist (`red.log`). Six functions then passed
  (`first-green.log`) without weakening Swift concurrency or adding unchecked
  sendability.
- Expanded behavioral checks passed: inert construction, storage failure,
  concurrent/repeated start refusal, listener construction/resume failures,
  canceled startup, concurrent shutdown during launch, failed launch, storage
  identity replacement after launch, and forced cleanup retained as unconfirmed.
  The forced synthetic core ignored TERM; its owned process and descendant were
  independently proven absent after bounded force cleanup. Returning no clean
  receipt in that case is intentional (`behavior.log`).
- Recovery ordering passed separately: the synthetic core confirmed revocation
  and exited, while the fixture listener remained available until explicit
  service shutdown. No real credential was used.
- A synthetic storage resolver and listener exercised the actual native guardian,
  compiled `serveOwnedCore`, and Node runtime fixture. Core readiness preceded
  listener activation. The recorded core PID, guardian PID and process group were
  independently confirmed absent after shutdown. Actual system listener and
  container APIs were not called by these tests.
- Full native regression: **222 test functions passed**, zero failures
  (`native-final.log`). Isolated timing functions and the retained real-signature
  probe were excluded. The new timing function was then run alone, below.
- Strict TypeScript and the structural gate passed (`types.log`). No TypeScript
  product code changed in this slice; the previous 1015-test portable result is
  retained separately rather than claimed as a new run.
- Fresh unsigned Debug Mac and iPhone Simulator builds passed in separate
  DerivedData directories (`mac-build.log`, `ios-build.log`). Artifacts:
  `/tmp/fort059-agent-service-mac-build/Build/Products/Debug/FortMac.app` and
  `/tmp/fort059-agent-service-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.
- Xcode still reports CoreSimulator 1171.7.0 older than required 1174.9.2; device
  execution remains unavailable. App Intents metadata extraction was skipped
  because these targets do not depend on that framework. These warnings were not
  reclassified as installed acceptance.
- Structural, accepted-implementation, exact staged-path and whitespace gates
  passed before committing.

## Isolated lifecycle measurement

Recorded environment: M1 Max, 10 cores, 64 GiB, macOS 27.2/26B5112a, Node 26.5.0,
Swift 6.4 and Xcode 27.1/27A9275. Hardware is the earlier inventory, not a new read.
Twenty serial fresh storage/core lifecycles used the existing compiled core at
`successor/.build/owned-core-059-20261018` and guardian
`/tmp/fort059-mac-core-host-guardian`. Installed toolchain/artifact caches were warm;
each private core directory was new. No test or build ran concurrently.

Timer includes App Group storage preflight using a synthetic container, core/guardian
spawn, private-channel readiness, one setup-offer check without pairing, fixture
listener resume/invalidation, core shutdown and PID/group absence. Compilation,
signing, actual App Group/launchd work, provider/network time and root test-directory
creation/removal are excluded. First sample retained.

p50 **156.979 ms**, p95 **161.830 ms**, maximum **240.382 ms**; target p95 <=2000 ms.
Raw samples and successful log: `/tmp/fort059-agent-service-budget.json` and `.log`.
The opt-in `FORT_AGENT_SERVICE_BENCHMARK` runs only
`agentServiceGuardedStartupAndShutdownBudget` with explicit `FORT_TEST_NODE`,
`FORT_TEST_GUARDIAN` and `FORT_TEST_CORE_ARTIFACT` paths.
Rounded display samples in milliseconds:

```text
240.382 151.256 157.399 156.483 154.506 155.065 156.979 152.727 154.203 157.768
156.967 154.170 158.177 154.733 161.830 159.123 157.149 157.658 157.659 157.462
```

The production helper target/bootstrap, complete runtime package/profiles,
owned-installation history/reinstall composition, actual provider and installed
TestFlight acceptance remain. No provider cleanup, remote publication or consumed
authorization reset occurred.
