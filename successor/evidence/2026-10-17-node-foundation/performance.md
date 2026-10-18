# Core and native build measurements

The measured core baseline and native build budgets pass. These results cover
the compiled development core and native client builds, not the final bundled
host, official runtime processes, installed UI or TestFlight delivery.
[Spec 059](../../../specs/059-modular-node-v1.md) defines the unchanged targets.

Source base is `699896761fc62bb146f476093d2815e39cf4da0f`. No service, runtime or
Swift product source changed during this slice. All three core runs produced
the same 20-file, 150,239-byte artifact. Its manifest SHA256 is
`29b7c5f2b0c6c641be03f358a62ded349343fe9c43c1e60ce166ca8bd7459fae`.
The [raw reports](performance.json) preserve every sample, the initial failed
combined-process readings, native source/app hashes and measurement script hashes.

## Conditions

MacBookPro18,2, Apple M1 Max, 10 CPUs, 64 GiB; macOS 27.2. Node 26.5.0,
TypeScript 6.0.3, Apple clang 21.0.0, Xcode 27.1 build 27A9275, Swift 6.4 and
XcodeGen 2.45.4. Core execution uses compiled JavaScript with JIT disabled and
Node permissions restricted to the generated directory and networking. The
service binds loopback only. No provider/account, startup registration or
external endpoint was activated.

The controller runs the synthetic HTTP client. Its owned worker runs the core
and an immediate in-process provider fixture. Acceptance/page timings include
the client's loopback HTTP round trip; execution timings include dispatch IPC
and fixture completion. They do not estimate actual provider latency.

Each core run uses five fresh compile/package outputs, five copied-source
incremental edits, five empty and five populated process starts, 100 acceptance
requests and 100 page reads. Cold starts include Node process launch/import.
Warm starts reopen the application in an already loaded Node process. OS file
caches are uncontrolled; no cache purge or forced garbage collection was used.
Percentiles use nearest rank, so p95 of five samples is the maximum.

The workload has 100 Homes with 50 user/reply turns each, 10,000 total messages
of 1,024 bytes each. The measured acceptance requests add 200 messages. Pages
contain 100 messages. This is not 10,000 messages in a single conversation;
provider-specific context limits remain separate. A future schedule is enabled
during the 60-second idle observation. The controller hashes ledger/WAL files
outside the CPU/wakeup interval, without allocating those buffers in the worker.

## Results

| Measurement | p50 | p95 | Target |
| --- | ---: | ---: | ---: |
| Core clean compile/package | 1.428 s | 1.440 s | 10 s |
| Core incremental compile | 0.673 s | 0.683 s | 3 s |
| Populated core cold process start | 111.35 ms | 115.62 ms | 2,000 ms |
| Populated core warm application reopen | 1.49 ms | 1.53 ms | 500 ms |
| Local acceptance round trip | 14.12 ms | 14.67 ms | 25 ms |
| 100-message page round trip | 12.90 ms | 13.94 ms | 50 ms |
| Dispatch IPC and immediate fixture execution | 23.81 ms | 26.19 ms | Reported separately |
| Mac clean Debug build | 25.53 s | 28.06 s | 120 s |
| Mac incremental Debug build | 2.45 s | 2.53 s | 20 s |
| iPhone Simulator clean Debug build | 14.99 s | 15.18 s | 120 s |
| iPhone Simulator incremental Debug build | 2.45 s | 2.54 s | 20 s |

Idle RSS was 100.27 MiB before and 94.42 MiB after 60.001 seconds, below 128 MiB.
These are endpoint samples, not a continuous peak-memory trace. CPU used 0.0103%
of one core, below 0.5%. The owned process recorded 0.467 interrupt wakeups/s,
below 1/s, and 0.0167 package-idle wakeups/s. Those counters do not measure energy.
Both ledger and WAL hashes stayed unchanged. At the 10,000-message baseline,
database plus WAL occupied 51,724,320 bytes, 49.33 MiB, below 64 MiB.

## Measurement corrections and checks

Run A reported 182.72 MiB and failed the memory budget while the same process
also ran the client and hashed its 48 MB ledger. Run B moved hashing out but
still included the client, reporting 138.00 MiB and failing. Run C separates
both from the core process and passes. The workload, service artifact and target
did not change. Both failed reports remain in the JSON; they are not relabeled
as passing core-only measurements. No SQLite caching change was made. The pinned
[Node SQLite documentation](https://nodejs.org/download/release/v26.5.0/docs/api/sqlite.html)
was checked while investigating allocation, before the measurement boundary was corrected.

The CLI smoke test first failed with the command absent, then passed. A separate
red test required distinct client/core process identities; it failed before
separation and passed afterward. Smoke mode labels itself ineligible for budgets,
and the test confirms that repeating a destination refuses overwrite while
preserving the existing report. The final full command exited 0 with all eleven
core budget checks true; both earlier commands exited 2 for memory. All twenty
native build invocations succeeded.

Final strict TypeScript checking, all 828 discovered portable tests and all
18 architecture-gate tests passed, with no skipped portable tests. The full and
accepted spec gates passed. Swift unit tests, signed sandbox probes, macOS-only
diagnostics and native HTTP journeys were not repeated for this tooling-only
slice; their product sources are unchanged. The ten clean native builds compile
the current Swift clients.

Native timing uses the [captured procedure](native-measurement.py), tracked-source
copies and separate DerivedData directories. Signing was disabled. The preserved
clean apps are fixture identities `io.fort.spec058.fixture.mac` and
`io.fort.spec058.fixture.ios`, version 0.1.0 build 1. Xcode warned that its running
CoreSimulator service was older than the selected build and skipped App Intents
metadata because there is no dependency. Generic Simulator compilation passed;
no simulator boot or UI acceptance is claimed.

## Reproduction and remaining gates

Use the [verification commands](../../verification/README.md). The final raw
core output is `successor/.build/performance-core-059-20261018-c`; A and B remain
beside it. Native inputs, logs, report and clean apps are under
`successor/.build/performance-native-059-20261018-a`. Only that measurement's
redundant DerivedData directories may be discarded after preserving its clean
apps and hashes. The captured native procedure refuses an existing destination.

Final self-contained host size/startup/idle measurements, official runtime
process latency/memory, installation/removal, physical iPhone connectivity,
accessibility/screenshots and TestFlight remain unrun. The core uses the selected
development Node executable; these results do not remove that shipping gate.
