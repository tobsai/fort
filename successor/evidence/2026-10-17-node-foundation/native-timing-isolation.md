# Native removal timing isolation, October 20

Spec 059 / decision 059. Source base 1af93191f4f7c712229f3e3a1723a0df3029269f.
The coordinator requested investigation of the two parallel failures from the
Settings repair without disturbing installed authentication. The Settings source
fix remains committed but has not been installed. Zero of six prompts used.

## Result and cause

The older focused set, excluding the new Settings test, reproduces the timing
failure in all three repetitions. The same test executable passes the budget alone
and alongside Settings. The old focused set passes all three runs with explicit
--no-parallel. This rules out Settings as a necessary trigger; its window bridge
is not called by the timing test. The measured host/model/test files are unchanged
from 6ffca1e0, before the Settings change.

ordinaryMacRemovalCompositionMetadataBudget measures elapsed time around model
construction plus await inspectLocalHost. That path enters MacHostControl.inspect,
then MacHostStartup.inspect, awaits the synthetic SetupHostBackend actor and resumes
on MainActor. Concurrent tests do synchronous work and file operations on that
same actor while the sample is suspended. Scheduling delays count toward its p95.
Separate temporary directories do not isolate the shared actor. Removing only the
other synchronous setup benchmark does not cure the failure. This is an aggregate
test-load/isolation problem, not evidence of slower production removal. These
controls do not establish installed UI responsiveness or actual host latency.

## Repeated controls before changing the validation command

Same Mac, macOS 27.2 (26B5112a), arm64, Apple Swift 6.4.0.34.1, Testing Library 2084.
Same compiled test executable and scratch directory /tmp/fort059-host-startup-tests;
--skip-build avoids compilation in these comparisons. Cases run sequentially in
fresh test processes, three repetitions each, with 20 original samples per budget
invocation. No build or deliberate external load runs concurrently. Normal desktop
load is uncontrolled; this is an isolation comparison, not a machine benchmark.
No installed app/host/account operation is part of these controls.

| Selected tests | Removal p95 milliseconds (three runs) | Verdicts |
| --- | --- | --- |
| alone | 0.303, 0.384, 0.324 | pass, pass, pass |
| with-settings | 0.370, 0.327, 1.967 | pass, pass, pass |
| with-reinstall | 2.315, 2.727, 3.002 | pass, pass, pass |
| old-focused | 97.153, 84.724, 97.040 | fail, fail, fail |
| with-setup-budget | 0.754, 0.726, 0.764 | pass, pass, pass |
| old-without-setup-budget | 98.454, 95.470, 66.152 | fail, fail, fail |
| old-serial | 0.381, 0.379, 0.402 | pass, pass, pass |

The original red command, with the same module-cache variables as the earlier
Settings test, is:

```sh
swift test --skip-build --disable-sandbox --package-path successor/apple/FortKit --scratch-path /tmp/fort059-host-startup-tests --filter 'agentSettings|modelPicker|avatarMotion|providerNeutralCatalog|ownerSetupModel|ordinaryMacModel|ordinaryMacReinstall|ordinaryMacRemovalComposition'
```

The without-setup-budget case additionally skips
ownerSetupModelInspectionMeetsLocalBudgetWithoutActivation. The alone case selects
ordinaryMacRemovalCompositionMetadataBudget. Full argument arrays and all samples
are in matrix.json and minimized-matrix.json beside the raw logs under
/tmp/fort059-timing-isolation-20261020. Earlier 52.55/65.75 ms failures remain
preserved under /tmp/fort059-settings-quit-20261020.

## Broader control and final correction

An initial two-stage invocation excluded the budget from ordinary tests, then ran
it separately. Parsed workflow commands passed all three focused pairs: 24 ordinary
functions and one unchanged budget, with isolated p95 0.306/0.306/0.331 ms. Those
results and the initial command are preserved in split-matrix.json and
workflow-test-steps.json. That narrower approach was not committed.

The complete prepared native suite still failed in parallel: 332 passing functions,
16 optional skips, four failed functions. Three synthetic account tests threw
invalidResponse from waitUntilHeld: nativeAccountLateReplyCannotExposeCodeAfterConnectionChanges,
nativeAccountLateReplyCannotReplaceChangedSavedIntent and
nativeAccountLateChallengeStaysHiddenAfterStoppingLocalWork. Their fixture polls
for a held response while the work that should reach it needs MainActor. The fourth
failure is nativeBundledHostJourneyPreservesIdentityAndHistoryThenRevokesAndStops:
12,621.10 ms exceeds its 5,000 ms limit. Its functional and owned-process-absence
checks completed; the final elapsed-time assertion failed. The isolated removal
budget passed afterward. See full-cached-0.log, full-cached-1.log and
full-cached-results.json. No real account or provider was involved.

The same full compiled suite with --no-parallel passes 337 functions, with 16
optional skips and zero failures (353 total functions). The host journey is
2,085.27 ms; total test time is 59.571 seconds. No code, threshold, samples, or
assertions changed. full-serial.log and full-serial-results.json retain the result.
Explicit concurrency within individual tests still executes; this option schedules
separate tests/cases sequentially so they do not compete for the global main actor
or launch all synthetic process workloads simultaneously.

The final correction adds only --no-parallel to the existing native quality command.
There are no skipped selectors, extra invocation, retry, optional gate, fixture
change or new harness. Every ordinary and timing test still runs. The README records
the same command. The unchanged 20-minute CI job budget is retained; this single
local duration is not a remote-CI performance promise. Ruby Psych parses the final
YAML, and the final command is retained in final-workflow-command.json. Three focused
runs of that actual command, with only local scratch/cache, --skip-build,
--disable-sandbox and the 25-function filter, provide the final repeated regression
results in final-focused-results.json: all 25 functions pass in every run, with
removal p95 0.663, 0.841 and 0.665 ms.

## Build-planning limitation and artifact

The initial broad command without --skip-build stalled at Swift's pre-planning
phase before executing tests. After confirming its exact process and parent,
only that task-created test command and Python orchestrator were stopped, with
exact test-process exit confirmed.
The partial planner log is full-0.log. No successful fresh Swift rebuild is claimed.

All diagnosis runs use the unchanged compiled test artifact at
/tmp/fort059-host-startup-tests/out/Products/Debug/FortKitTests.xctest/Contents/MacOS/FortKitTests,
SHA256 06906004e942e384762397105f0d8a6cb5930cbada08e08356a08cd3415a2d98
(23,055,424 bytes), recorded in test-artifact.json. Product/tests are unchanged from
the source base. Normal local IPC was used for the full suite with explicit
FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node,
FORT_TEST_GUARDIAN=/tmp/fort059-assembly-guardian,
FORT_TEST_CORE_ARTIFACT=/tmp/fort059-account-correlation-host-resources/core and
FORT_TEST_HOST_RESOURCES=/tmp/fort059-account-correlation-host-resources. These are
previously prepared synthetic resources, not a current signed product package.
Optional real-package/signing and benchmark environment inputs were not enabled.

## Limits and preservation

No fresh native rebuild, remote CI, portable suite,
installed AX/screenshot/Quit, real-provider, TestFlight or physical execution is
implied. Environmental load can still affect an isolated timing test; this change
removes competing tests without raising the budget. Optional hardware/signing
benchmarks were not enabled.

The installed report remains
0ab05d939bcbdc7eb1d0905156790a42a1db4375b5bb1ac63d6106cb95d563fc. No installed UI
was read, navigated, quit, relaunched or replaced during this investigation. No
login code was generated and no credential inspected. Authentication confirmation,
personal account verification/selection, exact Astra discovery and provider tests
remain pending. Last observed host/pairing are active; final cleanup is pending.
Do not infer authentication from elapsed time. G2 upload remains separately held.
