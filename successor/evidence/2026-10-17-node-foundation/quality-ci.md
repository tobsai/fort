# Test discovery and prepared quality CI

Design: [Spec 059](../../../specs/059-modular-node-v1.md).
This slice replaces the default hand-maintained test list with discovery and
prepares local and GitHub commands. It uses the existing Node decision authority.

The old command omitted existing agent settings, owner pairing, migration,
artifact, owner-demo and startup-diagnostic checks. The new regression failed
before the discovery runner existed, then passed. It verifies the formerly
omitted files, unique partitioning, invalid-suite rejection and failure when the
signed suite has no selected identity. The runner found 33 files: 29 portable,
2 macOS and 2 signed. Synthetic child processes now use the running pinned Node
binary instead of a Homebrew-only fixture path. Official runtime paths are unchanged.

Local results on October 18, 2026, Node 26.5.0, macOS 27.2, Xcode 27.1:

| Check | Result |
| --- | --- |
| Discovery regression | 1 passed after the expected missing-runner failure |
| `npm --prefix successor test` | 792 passed, 0 failed/skipped |
| `npm --prefix successor run test:macos` | 2 passed, 0 failed/skipped |
| `npm --prefix successor run typecheck` | Passed |
| `npm --prefix successor run test:specs` | 18 passed |
| Workflow YAML parse with Ruby Psych | Passed |
| Swift `--show-bin-path` | Resolved the existing compiled check's actual directory |
| Signed packaging suite | Not rerun in this slice; prior 3-test result remains separate |
| Linux execution, GitHub execution, Xcode Cloud | Unrun |

The macOS denial canary still reports `productionIsolationProven: false` and
shows that an inherited descriptor bypasses a pathname denial. Its restrictive
startup probe terminates with SIGABRT as previously documented. Passing the
diagnostic's expected behavior does not turn it into a production sandbox proof.

The workflow pins [checkout](https://github.com/actions/checkout/commit/3d3c42e5aac5ba805825da76410c181273ba90b1)
and [setup-node](https://github.com/actions/setup-node/commit/949feb2413d6458794dcd2491c4babbbce0c15c1).
The [official runner image](https://github.com/actions/runner-images/blob/main/images/macos/xcode-27-arm64-Readme.md)
lists the selected Xcode 27.1 path for the `xcode-27` runner. Image availability
and the workflow itself have not been exercised remotely. Installation disables
dependency hooks, checkout does not retain Git credentials, and permissions are
read-only. There are no release credentials or upload steps.

The local staged-path gate remains required. The workflow does not authenticate
owner decisions, enforce a PR's complete scope, configure branch protection, or
prove a signed Apple lane. Nothing was pushed, deployed, signed or published by
this slice. No provider, credential or startup registration was activated.
