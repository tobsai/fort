# Synthetic Swift and Node sandbox proof

[Spec 059](../../../specs/059-modular-node-v1.md) governs this slice. Its evidence
paths were refined under the unchanged implementation authority in decision 059.
The [result record](sandbox.json) pins source bytes and observed checks. The
[earlier result](results.json) remains evidence for the preceding load-path test.

One actual artifact test passed with zero failures or skips. Strict TypeScript
checking also passed. The test compiles a Swift app, packages the installed Node
26.5.0 dependency closure and legal notices, and signs temporary copies with the
existing Apple Development identity. It launches the app executable and waits for
its child. SQLite writes inside the private temporary directory succeeded. Reads
and writes outside the container and a loopback listener were denied. The process
group and newly created UUID app container were absent after completion.

The parent has only App Sandbox. The child has App Sandbox plus inheritance.
Library validation stays enabled; no JIT, network or filesystem exception is
added. This follows Apple's [helper entitlement guidance](https://developer.apple.com/library/archive/documentation/Miscellaneous/Reference/EntitlementKeyReference/Chapters/EnablingAppSandbox.html).
These negative controls establish the observed boundaries, not complete host or
computer-tool isolation for a future provider runtime.

The first real launch exposed a global OpenSSL configuration dependency that
Mach-O relocation had not removed. The sandbox correctly denied the read.
The app now supplies a bundled Node-specific empty configuration through the
[explicit OpenSSL configuration option](https://nodejs.org/api/cli.html#--openssl-configfile).
It does not edit Homebrew configuration, enable legacy algorithms or disable TLS
verification. Attribution belongs in Resources; executable images belong in
MacOS and Frameworks. An earlier signing failure caught the incorrect placement.

Only synthetic data was used. No provider, account, startup job or external
service was accessed. The one container left by the earlier failed launch was
identified by its exact test UUID and removed; the current proof cleans its own
container in a finally block. Each test removes its own temporary artifact.

This is development-signing and sandbox feasibility evidence. Production IPC,
official runtime authentication, lifecycle, benchmarks, TestFlight processing and
physical device acceptance remain open. Apple's [review requirements](https://developer.apple.com/app-store/review/guidelines/)
still require separate distribution assessment; a local pass is not approval.
