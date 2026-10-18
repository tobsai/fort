# Native removal composition

Source base: `48954abd3a9b632f2368c734d8ce42c995b547d5`, followed by this reviewed
local slice. [Spec 059](../../../specs/059-modular-node-v1.md) governs the change;
the [Ponytail change report](../../../docs/architecture/reviews/ponytail/current.md)
binds its exact staged files and modes. Date: October 18, 2026.

## Behavior

The ordinary verified Mac host now supplies the existing durable removal
controller to OwnerLifecycle and MessengerModel. Saved removal progress blocks
new setup but leaves removal retry available. Each action rechecks the pinned
app/helper/endpoint/client identity. Startup construction remains inert.

Removal requires exact revocation, clean core stop, startup unregistration and
independent helper-exit observation before credential cleanup or system app-removal
handoff. A failed credential cleanup can retry from confirmed progress without
starting the helper or repeating unregistration. Disconnect recovery verifies the
same pairing but does not unregister startup. History is retained. The production
controller rejects the unavailable history-deletion capability before any effect;
that accepted v1 goal remains open pending its exact deletion scope.

The unused convenience constructor was removed after the Ponytail tree-wide caller
search. Production construction uses MacHostControl's existing package identity.

## Verification

| Check | Result and limit |
| --- | --- |
| Initial public lifecycle/model regressions | Failed before implementation: unsupported history deletion performed work, ordinary removal was unavailable with and without saved progress. Preserved in `/tmp/fort059-removal-composition-red.log`. |
| Initial focused native slice | 14 functions passed in `/tmp/fort059-removal-composition-green.log`. |
| Additional removal/model behavior | Four functions passed, including the two-case ordinary model test, bundle rejection before effects, exact disconnect recovery and credential-cleanup retry. `/tmp/fort059-removal-final-focused.log`. |
| Full native regression | 234 functions passed. Isolated timing, retained signed probe and complete bundled journey were excluded from this parallel run. `/tmp/fort059-review-native-full.log`. |
| Complete synthetic bundled journey | Passed in 1,327.049083 ms against a 5,000 ms budget; owned process groups absent. Uses authored Codex protocol behavior, no provider or registration. `/tmp/fort059-review-native-journey.log`. |
| Portable suite | 1,088 passed, 5 opt-in budgets skipped, zero failures. `/tmp/fort059-review-portable-permitted.log`. |
| Strict TypeScript | Passed. `/tmp/fort059-review-typecheck.log`. |
| Spec and review gate behavior | 27 passed, comprising 18 spec and 9 review functions. `/tmp/fort059-review-gates.log`. |
| Unsigned native builds | Fresh FortMac and FortiOS Simulator Debug builds passed. `/tmp/fort059-review-mac-build.log` and `/tmp/fort059-review-ios-build.log`. |

Fresh synthetic resources are at `/tmp/fort059-review-host-resources`; the guardian
is `/tmp/fort059-review-guardian`. They were built with the same preparation used
by the corrected native CI job. Node is the selected Homebrew 26.5.0 binary; the
fixture copies its libnode when present. The standalone official Node layout and
the remote CI runner have not been executed here. These artifacts are neither a
self-contained release package nor signed/installed product evidence.

The first focused Swift invocation was blocked by compiler-cache access; the
supported permission escalation allowed compilation. New test assertions initially
combined multiple actor reads under one await; splitting those assertions repaired
the compile error without changing product code. The first portable run used the
restricted environment and failed socket startup. A direct temporary Unix socket
probe returned EPERM. That exact test process group was stopped, and the supported
local-socket run passed. The failed log remains at `/tmp/fort059-review-portable.log`.

## Removal metadata timing

Fresh metadata projection into the ordinary model, 20 samples in milliseconds,
one recorded connection, synthetic bundle inspection and startup backend. No
other task tests/builds ran concurrently. Node/provider, signature verification,
Keychain, real SMAppService and deliberate helper-exit waits are excluded. Current
host metadata: MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2, Swift 6.4.

```json
[5.878083320567384,0.2680416509974748,0.2412916801404208,0.23637499543838203,0.2354166645091027,0.23099998361431062,0.23412497830577195,0.22787501802667975,0.23387500550597906,0.23241667076945305,0.22662500850856304,0.23658332065679133,0.22533335140906274,0.2257083251606673,0.22516667377203703,0.22558332420885563,0.2246666408609599,0.23133333888836205,0.22816666751168668,0.22691665799356997]
```

Nearest-rank p95 is 0.268042 ms, maximum 5.878083 ms; p95 passes the 50 ms target.
Raw output: `/tmp/fort059-removal-composition-budget.log`.

## Remaining gates

No real credential, startup registration, helper account, app deletion or history
deletion was used. Installed accessibility/screenshots, exact Apple profiles and
TestFlight, actual provider integration and physical iPhone remain unverified.
The baseline review also found the absent ordinary native resume journey; it is
an open v1 blocker, not excused by the successful synthetic restore/recovery tests.
