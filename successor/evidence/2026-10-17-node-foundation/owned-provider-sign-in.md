# Owned executor device sign-in preparation

Spec 059, October 18, 2026. Source base d69cfb3e72247c1a7ccd3ff976f3f9cbbc3f4d8b.
This source slice carries an explicit optional account operation through the same
private executor and Mac guardians as chat. It does not initiate real sign-in.

The neutral request binds provider, runtime version, host, request ID, digest and
deadline. One executor slot excludes chat/model discovery/other sign-in. One bounded
challenge is followed by correlated terminal evidence, accepted only after clean
worker exit and owned group absence. Cancellation retains completed-login races;
uncertain or forced cleanup stays unconfirmed. Work is limited to 115 seconds with
five seconds reserved inside the unchanged 120-second process deadline. No browser,
credential copy, account fallback, service registration or network grant is added.
The agent host remains distinct from its computer-tool sandbox.

Observed red tests: missing public owned capability and Codex composition; a pending
300 ms operation initially consumed 5,310 ms and returned unconfirmed because the
worker only enforced the final deadline; mutation of either public callback could
redirect the displayed code. The worker now aborts at the work deadline, and both
boundaries capture the original callback. Final regressions pass. A TypeScript
assertion initially narrowed a getter to never; the assertion was corrected without
a cast. No production failure was hidden by fixture substitution.

Validation on macOS 27.2, Node 26.5.0, TypeScript 6.0.3:

| Check | Result |
| --- | --- |
| Full portable | 1,259 passed, zero failed, 11 documented skips; 35.471 s |
| Focused account/worker/artifact | 129 passed, zero failed, four opt-in skips |
| Guarded Mac executor/runtime/artifact | 35 passed, zero failed, three opt-in skips; 7.509 s |
| Strict TypeScript | Passed, exit 0 |
| Isolated process benchmark | 20 serial samples after five warmups; p50 90.124 ms, p95 93.669 ms, maximum 94.068 ms; budget 2,000 ms |
| Fresh host resources | 102 files, 433,837 bytes; every hash/size and exact membership independently verified |

Benchmark alternates independent-provider-shaped success and cancellation, including
process launch, cleanup and disposal. No concurrent task builds/tests ran. Earlier
hardware inventory was MacBookPro18,2, 10 CPU, 64 GiB; a fresh sandboxed hardware
query was denied and is not claimed as a new measurement. Provider/network/user
interaction latency is excluded. Fixture tests also prove malformed/foreign/oversized
frames, changed/reordered/duplicate progress, canceled display, callback failure,
forced cancellation, invalid worker scope and absent process groups.

Artifact: /tmp/fort059-owned-account-host-resources. Manifest SHA256:
85c8960b28cd9534ad4256e2cdf2475af663f2f1d48e70bb1dae4da82ef1a2e1.
The core artifact still excludes the optional provider implementation; historical
openai service filenames/database names are retained compatibility names. The prior
190-file review app has not been replaced or relabeled by this resource build.

Remaining: primary-owner service action and lost-response status, native challenge/
cancel controls, exact Apple records/profiles, helper network authority, authorized
real-provider operation, signed installed UI and TestFlight. No actual Codex binary,
account, credential, browser, startup registration, signing or upload ran. Fixtures
establish ownership/contracts, not provider or installed-product acceptance.

## Local evidence

- `/tmp/fort059-owned-account-red.log` — SHA256 `ec220bec509f5d8c0d444bea50c050ed3336958adb4be00f53529d59c42cd8ea`
- `/tmp/fort059-owned-account-first-green.log` — SHA256 `162e24f7460e05d864a357b10d155311f9d933688d7e19fa607b6fe5f92e5855`
- `/tmp/fort059-owned-account-codex-red.log` — SHA256 `85ce28cb7806d6bbeb8cbf92ad793687a78c80eeef70e3ea08a0c8f586d0cd24`
- `/tmp/fort059-owned-account-codex-green.log` — SHA256 `dde71f23dc88b6b7acc04bef4b943b309fe7e91c19ae8757a3a3c23dcd436e33`
- `/tmp/fort059-owned-account-deadline-red.log` — SHA256 `c0bf5cd659fbefbc73088587fd83c7ad5bf72f4591569d9b1904491c57aea9d4`
- `/tmp/fort059-owned-account-deadline-green.log` — SHA256 `d96a3e41efd716a2dbaade65cc998c1dce33bfb638c9cd18ba2e3461ac3227fd`
- `/tmp/fort059-owned-account-callback-red.log` — SHA256 `2d17f95a8ad42876741df844a278078d1dafb3f75d847a95e4ff986a7d6809fd`
- `/tmp/fort059-owned-account-protocol-callback-red.log` — SHA256 `bde1f4488e1a6ec4f93692a436c005995d5bffaba9209bfafaac825a7edfd480`
- `/tmp/fort059-owned-account-focused.log` — SHA256 `7b60005013ee0f83cb0389133d2d9771e967b098620d34247ce8a182778debc7`
- `/tmp/fort059-owned-account-portable.log` — SHA256 `745ee26e96c7cf8f51d03948287b0c9e158a8ffbe246ca35b4123358d0a4bb8b`
- `/tmp/fort059-owned-account-macos.log` — SHA256 `d61b236aaeda782db602f140c5d6bbe35d00e10f9ddde4371291904f78a5c3cc`
- `/tmp/fort059-owned-account-typecheck-final.log` — SHA256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
- `/tmp/fort059-owned-account-budget.log` — SHA256 `bfd61d0ff3b0c5fe07086deb00c57a98c357e4915d83114d37304e1f160c236e`
- `/tmp/fort059-owned-account-host-resources.log` — SHA256 `c25389e044d665dc023af0a51dcfb3fc45092a4a08a0d6af55ea18e54b563e1a`
- `/tmp/fort059-owned-account-samples.json` — SHA256 `824efcdb6068712dd900bee1ff6eabe66e88e3d7f59d4ec2c141485ff3e326ef`
