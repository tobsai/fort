# Architecture reassessment — October 17, 2026

**Review input, not implementation approval.** Toby requested a fresh design
through the [four principles](../../ARCHITECTURE.md), followed by thorough
research into a different design, removable service connectors, a performant
Mac background service and CI/CD. This note inventories the current source and
proposes what to keep, adapt or replace. Independent architecture and public-source
research remain inputs to the final recommendation; no framework or backend
language has been selected here.

## Preserved checkpoint and authority

- Branch: `codex/fort-openai-rebuild-20261016`.
- Worktree: `/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
- Paused checkpoint: `e02eb571e66620e407a360584817616074730b55`.
- Latest product source: `09788c132647458cbdc683506c8bd5622f0be1af`.
- Prior evidence: [interactive QA record](../../successor/evidence/2026-10-17-interactive-qa/README.md)
  and [delivery plan](../../successor/TESTFLIGHT-DELIVERY-20261017.md).

No tests/builds were running at pause. Those records report 120 service/runtime
tests, 56 native tests, typecheck, QA/Mac/iPhone Simulator Debug builds and Release
FortKit compilation passed. These are preserved results from the earlier source,
not reruns or proof of the new principles. Installed new-QA inspection, actual
provider discovery, real host installation/removal and TestFlight acceptance are
still unverified. No application/runtime was started by this documentation slice.

The primary checkout, unrelated `.build/` and owner-approval file, source pins,
consumed demos and donor quarantine stay unchanged. Local teardown was reported
complete; provider closure remains unverified. No remote cleanup is authorized.
Historical Spec 058 approval is retained as history but its current review is
`needs-reassessment`; the implementation gate must reject it.

## Decision grid and negative controls

| Principle | Current evidence/gap | Proposed direction | Review proof still required |
| --- | --- | --- | --- |
| Vendor independent | `contracts/v1.ts` exports OpenAI/Codex literals; native `ModelCatalog.canSelect` requires Codex. Current success can hide this coupling. | Adapt public contracts to capabilities/opaque identities; isolate official runtimes. | Shared behavior with a differently shaped non-OpenAI fixture, absent-OpenAI build and separate real subscription evidence. |
| Modular | `service/openai.ts` combines lifecycle, memory, schedules, authorization, persistence and runtime orchestration; memory is in snapshots/turn configuration. | Extract bounded responsibilities around retained public behaviors. Make memory, voice, execution environments and service connectors removable. | Delete optional packages from the build graph/artifact and pass send/persist/replay plus retained-history tests. |
| Fast | Build/test success exists; controlled clean/incremental/startup/runtime/idle benchmarks and budgets do not. | Compare a small set of credible language/framework candidates at the same seam. | Hardware/toolchain/workload/cache/repetition protocol; numeric target and measured results separated from provider latency. |
| Easy install/remove | Pairing recovery has fixtures; real host lifecycle adapter is missing. iPhone connectivity and TestFlight packaging are unresolved. | Design the Mac background host and delivery lifecycle together. | Exact installed install/update/revoke/recovery/uninstall absence proof, data choices and physical TestFlight acceptance. |

These failures are intentional negative controls: passing structural validation
must not convert any row into architectural compliance. Fixture provider proof,
module removal and performance checks are **planned requirements, not tests run
in this documentation task**.

Independent review additionally identified competing attempt identifiers,
repeated full-history persistence and per-second scheduler writes. Preserve these
as redesign findings: clarify ownership of attempt identity, bound persistence
work and measure idle/write amplification. They are not benchmark results or
fixes performed by this gate task; product source remains unchanged.

## Independence by dimension

| Dimension | Current limitation | Requirement for the new proposal |
| --- | --- | --- |
| AI/runtime | Codex details reach public types and native selection. | Provider capabilities/session semantics are adapter-owned; no mandatory OpenAI dependency in core/UI. Preserve exact model and subscription preference without claiming identical runtimes. |
| Hosting | Loopback prototype needs a reachable, authenticated path for a physical iPhone. | Local/self-hosted placement without a mandatory vendor control plane; disclose transport dependencies and operational burden. |
| Storage | SQLite is the prototype's engine and implementation choice. | Own durable schemas/contracts and export/migration behavior; justify the engine and replacement cost rather than add speculative databases. |
| Optional modules | Memory is integrated; voice/VM/connectors lack proven removal boundaries. | Contract, lifecycle, scoped authority and package/artifact omission proof for each module. No disabled-only compliance claim. |
| Client/distribution | Native iPhone/Mac plus TestFlight explicitly depend on Apple. | Record that accepted product preference; keep service contracts platform-neutral and research host distribution constraints. It does not justify AI-vendor coupling. |

## Keep, adapt, replace — proposal for review

| Choice | Candidate | Reason and limit |
| --- | --- | --- |
| Keep | Exact identity, stable conversation/message IDs, durable acceptance/replay/idempotency, honest uncertain outcomes and public behavior tests. | These product invariants do not require a particular runtime or language. Verify them through any replacement seam. |
| Keep | Native iPhone/Mac priority, accessibility, history choice and TestFlight delivery requirement. | Preserve user decisions; current app packages remain engineering artifacts. |
| Keep | License/provenance, teardown records and all historical evidence. | No reset, deletion or retroactive reinterpretation. |
| Adapt | Public contract and native model catalog. | Remove provider assumptions; capabilities must express unsupported operations and different session semantics. An arbitrary provider string alone is insufficient. |
| Adapt | Core service composition and persistence access. | Preserve behavior but separate domain transitions, persistence, API, runtime adapters and optional module lifecycle. Avoid premature microservices. |
| Adapt | Pairing and host management. | Retain tested no-expiry/default, retry and revocation distinctions; connect them to a real removable host lifecycle after review. |
| Replace | Go-only instructions and automatic adoption of the current TypeScript service design. | Constitution supersedes both as mandates. Language choice follows measured comparisons and packaging research. No source rewrite is authorized. |
| Replace | Codex-specific public/native semantics and any standalone-DMG delivery assumption. | They conflict with the new AI independence principle or existing TestFlight requirement. Confirm an Apple-supported host arrangement before choosing packaging. |
| Decide | Mac background host boundary, connector API, secure iPhone transport, database and CI provider. | Compare operational/removal costs, measurable performance and delivery feasibility before accepting a new spec. |

For the host, compare a modular service process with an explicit lifetime and
owned data against alternatives identified by research. Specify process/IPC
ownership, permissions, no hidden startup, idle resource budget, crash recovery,
update compatibility and reversible uninstall. Keep agent-host privileges distinct
from computer-tool sandbox constraints. Naming an interface or using Ask mode
does not establish confinement.

## CI/build inventory — read-only

`git ls-files '.github/**' 'successor/**/.github/**' 'foundation/**/.github/**'`
returns no tracked workflows in this checkpoint. No `.github/` directory is
present. Remote Actions configuration, required checks and branch protections
were not inspected or changed. Absence of local workflows is not evidence about
remote settings.

A historical root `Makefile` is tracked but sparse-excluded (`git ls-files -v`
shows `S Makefile`). Read through `git show HEAD:Makefile`; it was not restored or
run. It targets old Go packages, `goreleaser release`, Homebrew and a Developer ID
DMG/notarization flow. Those install/release/clean targets are not successor CI or
TestFlight delivery commands and must not be reused by default.

Current source tooling:

| Purpose | Command from repository root | Evidence/limit |
| --- | --- | --- |
| Spec structure / gate tests | `npm --prefix successor run check:specs` / `npm --prefix successor run test:specs` | New local commands plus npm test/typecheck prechecks; dependency-free Node validator. Not remote enforcement. |
| Typecheck | `npm --prefix successor run typecheck` | Existing TypeScript 6.0.3 and Node 26.5.0 pins. |
| Service/runtime tests | `npm --prefix successor test` | Existing explicit test list; CI must review coverage and avoid automatic provider enrollment. Prior 120-test affected suite is not a claim that this whole command was rerun. |
| Native package tests | `xcrun swift test --package-path successor/apple/FortKit` | Prior 56 passed. Apple toolchain required. |
| Release package compilation | `xcrun swift build -c release --package-path successor/apple/FortKit` | Prior pass; Debug fixture availability is not Release behavior. |
| Generate Apple project | `xcodegen generate --spec successor/apple/project.yml` | Existing project generator; generated project is not release configuration. |
| Mac compile | `xcodebuild build -project successor/apple/FortSpec058.xcodeproj -scheme FortMac -configuration Debug -destination 'platform=macOS' -derivedDataPath /tmp/fort-ci-mac -disableAutomaticPackageResolution CODE_SIGNING_ALLOWED=NO` | Proposed unsigned CI command; earlier build used ad-hoc signing. This exact variant was not run here. |
| iOS compile | `xcodebuild build -project successor/apple/FortSpec058.xcodeproj -scheme FortiOS -configuration Debug -destination 'generic/platform=iOS Simulator' -derivedDataPath /tmp/fort-ci-ios -disableAutomaticPackageResolution CODE_SIGNING_ALLOWED=NO` | Proposed unsigned CI command; earlier build used ad-hoc signing. Simulator execution remains separate. |

`successor/package.json` has test/typecheck scripts but no production host build,
installer, archive or release command. `successor/apple/project.yml` specifies
Swift 6.0, iOS 17/macOS 14, version 0.1.0/build 1, fixture bundle IDs, signing off
by default and ad-hoc Simulator signing. No successor non-Simulator profile/team
mapping, App Store Connect export configuration or production icon catalog has
been verified. The earlier build logs include a CoreSimulator service/toolchain
version warning; compilation did not establish working simulator UI automation.

The earlier read-only signing inventory found an Apple Distribution identity for
team T3JB5MYZ93. That metadata does not prove valid profiles, App Store Connect
roles/app records, agreements, next build numbers or tester groups. No keys,
tokens or account files were read for this assessment. Historical
`ui/apple/ExportOptions.plist` is an app-store-connect/upload input for the older
app; its Mac counterpart uses Developer ID. Neither authorizes reuse of old app
IDs or uploading this prototype. See the [delivery plan](../../successor/TESTFLIGHT-DELIVERY-20261017.md).

## CI/CD proposal — pending research and owner design review

Start by evaluating GitHub Actions for ordinary repository checks because the
source already uses Git; compare Xcode Cloud or another service if its Apple
signing/delivery integration materially reduces ownership and operational cost.
Git alone is not proof that GitHub is the best host. The final recommendation must
state runner/macOS/Xcode support, cost, private-source handling, credential custody,
artifact retention and exit path. No workflows, runners or remote settings are
created by this task.

Keep portable verification commands separate from CI-vendor YAML. Proposed jobs:

1. Structural spec review and gate tests on every change; behavioral/type checks
   for the active source. Enforce the chosen spec's implementation gate before
   product changes can progress. Human review still checks the real decision.
2. Reproducible Apple project generation, package tests and unsigned builds on a
   pinned compatible Mac/Xcode toolchain. Add provider-absent/module-absent jobs
   and controlled performance comparisons when their designs are approved.
3. A separately authorized release stage validates exact app IDs, entitlements,
   profiles and source revision, archives and exports TestFlight packages, and
   records immutable checksums. Use least-privilege release access; ordinary PR
   verification must not receive production signing or provider credentials.
4. Track upload receipt, Apple processing, tester availability and physical
   installed acceptance separately. Failure at any stage blocks delivery signoff.
   Include host install/revoke/uninstall evidence, not only UI screenshots.

The research review must settle what can be packaged with TestFlight and what
host installation, background permissions or external runtimes require separate
owner action. Do not promise a removable background service based only on a
passing command-line build. No signed archive, upload, installer, background
registration, credential or CI runner is created during reassessment.

## Proposed CI enforcement of change scope

The local gate now supports an exact per-spec `Implementation paths` declaration
with `--implementation <spec> --staged [--base <full-commit-SHA>]`. This adds path
coverage, not authenticated approval or architectural compliance. A future CI job
should implement this procedure; none of it configures remote enforcement now:

1. Obtain the base-branch and head commit IDs from trusted PR event/review context,
   derive their merge-base, and use that full SHA for the comparison. Use a clean
   checkout/index of the head and retain/fetch the exact comparison object. Fail
   if it is unavailable; never silently fall back to HEAD.
2. Read the proposed applicable spec path as data. A reviewer must verify semantic
   applicability. Run the full structural check and the selected spec's staged
   gate against that trusted base. Every changed path, including deleted paths
   and both sides of a rename, must appear in its reviewed declaration (the
   selected spec itself is implicit). Current local tooling handles one spec per
   bounded slice; split unrelated scopes or design a reviewed multi-spec extension.
3. Run the validator from a trusted reviewed revision rather than blindly trust
   a PR's rewritten gate. Gate/template/baseline changes require explicit owner
   review. Reject changed scope without an updated applicable decision and digest.
   Ordinary CI must not receive signing, provider or deployment credentials.
4. Separately require an authenticated owner/reviewer decision bound to the exact
   head and spec digest, with prior approval dismissed on relevant changes.
   Verify the real record and any tradeoffs; `accepted`, a named owner and a hash
   inside the submitted Markdown can all be self-authored and are not proof.
5. A future required status check/merge policy must enforce both the automated
   result and that authenticated review. No workflow, approval service, required
   check or branch protection is configured here; portable local commands alone
   cannot prevent someone from bypassing them.

The local index check deliberately ignores unstaged/untracked work and does not
associate already committed changes unless an explicit base is supplied. It also
does not prove that code follows the listed spec, that evidence is true or that
unlisted behavioral changes are absent. Those remain human review and behavior-test
responsibilities. No product change has been marked accepted to demonstrate this
feature: accepted records in tests are clearly synthetic, temporary fixtures.

## Next review packet

Combine the independent architecture critique and primary-source research with
this inventory. Recommend a bounded target design, compare alternatives through
all four principles, choose proposed numeric budgets and show install/remove
journeys. Write a new or revised spec using the template; resolve the explicit
tradeoffs with Toby before an accepted implementation gate. Preserve this
checkpoint while the owner reviews; do not continue feature expansion in parallel.
