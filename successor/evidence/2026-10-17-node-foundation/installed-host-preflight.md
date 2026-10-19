# Installed client preflight and first-registration fix

October 19, 2026; Spec 059 / decision 059. Base
`2f33fbf8a2f0cf67860f7c77f320d09ae35183ae`. This is client-only installed evidence,
not host, provider, removal or TestFlight acceptance. The coordinator authorized
independent checks without new persistent startup/provider access and held the
separate G2 CSR upload pending the owner's exact disclosure approval.

## Observed defect and correction

The original signed package (report `20ed08cc2df56549234a63f09c3da9b1854cc05a08958cbdf18301cdaa03b559`)
was copied into the previously absent `~/Applications/Fort.app`. All 206 bytes/modes/
length/hash records matched. Its client opened and verified the exact bundle, but
SMAppService returned notFound. macOS service logs parsed the correct LaunchAgent
and reported `effectiveItemDisposition: record not found`; no service was registered.
An earlier /private/tmp launch was refused by the exact canonical-path check.

[Apple DTS explains](https://developer.apple.com/forums/thread/719862) that notFound
can also mean a service the system has never seen. That example uses a login item;
the matching interpretation for this LaunchAgent is supported by the local log.
It does not make notFound proof of a valid package or successful registration.

MacHostControl now permits an explicit Enable after exact bundle inspection when
the OS reports notRegistered or notFound. MacHostStartup registers only on that
explicit action, then requires enabled/requiresApproval confirmation. Inspection
still never registers. Pairing still requires enabled, changed/invalid packages
still fail revalidation, and removal still cannot infer absence from notFound.

The first revised installed screenshot exposed another condition in OwnerSetupView
that hid Enable. The view now uses the controller's eligibility rather than repeat
an OS-status rule. Its final installed screenshot/accessibility proves Enable local
host enabled, Connect this Mac disabled and unchanged no-default-expiry copy.
Neither action was pressed. [Final window](installed-host-enable.png).

## Exact final artifacts

Client-only tested copy: `/Users/tobiasgunn/Applications/Fort.app`, byte-identical to
`/tmp/fort059-host-setup-v2-signed-20261019/Fort.app`.

- 206 files / 406,324,962 bytes / 30 Mach-O objects.
- Signed-package report SHA256:
  `312d606afe53dfdea8c395949838744c3889d410cb7e919a195120e43f1dcf05`.
- Payload manifest SHA256:
  `1794e6cd1448e2b7fc31fcddc0337f30dff8bc72470be142f7e8fa9acbf6f9fe`.
- Helper retains no outgoing-network entitlement. App/client/profile identities
  and exact child sandbox/inheritance declarations remain unchanged.

The separately signed, uninstalled and unlaunched provider-test candidate is pinned
in [the authorization proposal](installed-host-proposal.md). Earlier signed packages
are preserved. Replaced task installs are retained as
`/tmp/fort059-installed-first-preflight-preserved-20261019.app` and
`/tmp/fort059-installed-controller-only-preserved-20261019.app`.

## Passed, failed and unrun checks

- Red: the three new behavioral functions failed with seven issues before the
  controller/startup fix. Green: 66 selected native functions passed after
  final changes; zero failed and two optional timing functions skipped (68 listed). Includes cancellation,
  failed registration, approval state, identity revalidation, model credential
  gating and compiled synthetic core setup/recovery. No real SMAppService backend
  mutation or provider execution is used by these tests.
- First broader restricted run had three core connection issues. Rerunning with
  explicit compiled fixtures and normal local IPC access passed. An intervening
  build stalled in clang-stat-cache and was stopped; the final run used the already
  rebuilt test executable with `--skip-build`. It is not a failed product verdict.
- Final Release builds passed: FortMac 27.572 s, FortHost 2.479 s. Single samples,
  not performance benchmarks. Initial clean Mac build was 51.616 s.
- Both unsigned assemblies failed signature verification as expected. Both final
  signed candidates passed every file membership/hash/mode/length check, all 30
  strict all-architecture signatures/public leaf fingerprints, strict nested
  verification, exact entitlements and native MacHostBundle.inspect. Restricted
  trust-service verification refused; normal read-only macOS access passed without
  changing trust settings. All leaves match the approved existing development cert.
- Client accessibility and screenshot passed at the installed canonical path.
  Normal NSRunningApplication quit requests and the exact enabled Quit Fort menu
  action did not exit while setup was open. Verified task-only SIGTERM cleanup
  was required. **Ordinary quit/reopen remains a product issue, not a pass.**
- Final client metadata contained only zero-byte owner-setup.lock. macOS created
  the app/client and App Group containers during normal launch, but the exact
  App Group/FortHost directory was absent. No Enable, Connect, sign-in, provider,
  CSR upload or persistent service registration action occurred. Final exact
  process/service absence was checked independently; installed files remain inert.
- Actual helper startup, effective shared-group/XPC access, provider authentication,
  account/model discovery, live messages, stop/resume/removal, privacy declarations,
  TestFlight and physical iPhone acceptance remain unrun.

Reproduction records remain outside Git: `/tmp/fort059-host-startup-red.log`,
`/tmp/fort059-host-startup-final-green.log`, `/tmp/fort059-host-setup-v2-builds.json`,
`/tmp/fort059-final-candidate-verification.json`, final candidate package reports,
`/tmp/fort059-installed-client-final-window.txt` and
`/tmp/fort059-installed-client-final-quit.txt`. The manifest payload hash is SHA256
of the report's files array serialized with sorted keys and compact JSON separators;
each file entry pins path, mode, size and SHA256. Reports also pin the three changed
product-source file hashes against the base commit; the slice commit records them.
