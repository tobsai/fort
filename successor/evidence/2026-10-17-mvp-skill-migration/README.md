# Instruction skill migration — October 17, 2026

Source `a524c4a6fbe4d53e9f8e514a5e89a71f8cda8968` completes the native
instruction-authoring migration after the routine/schedule source `f6e4615`.
The ten modified files survived the execution disconnect; Git confirmed they were
unstaged and uncommitted. They were reviewed, freshly tested and committed after
reconnection. Prior `/tmp` logs/builds were absent, so this record uses new results,
not inferred success from missing artifacts.

New plain-text instruction skills are complete one-file bundles with provenance,
private-owner license metadata, immutable revisions and explicit review. Saving or
editing does not enable them. The native editor opens the exact bundle review;
legacy instructions remain readable but cannot be newly selected or submitted
until explicitly migrated and reviewed. Migration retains their exact prior text
and revision. Already accepted requests retain their recorded configuration and
receipt. Simple editing cannot discard files or requested authority from an imported
multi-file bundle. Native and service callers now follow this workflow, including
historical synthetic session fixtures; consumed launchers and scope rules did not
change.

Fresh validation after reconnect:

- **649 TypeScript tests passed**, zero failures/skips; strict typecheck exit 0.
- **29 Swift tests passed**.
- Compiled Swift HTTP conformance passed: ten synthetic runtime executions across
  four service restarts, one lost acceptance, no duplicate dispatch, an empty pending
  queue, explicit instruction review, complete bundle round trip, routine gates and
  schedule preview/enable/pause. No scheduled runtime execution occurred.
- Mac and iPhone Simulator build-for-testing passed using installed Xcode 27.1
  (27A9275) and existing signing settings. Strict/deep host signature verification
  passed for both apps and both runners; 26 files were hashed.
- All six consumed trial packets, native cache and main Git status matched the
  recorded baseline by raw hashes/stat. No historical SQLite or credential store
  was opened.

See [checks](checks.json), [HTTP output](native-http.txt), [builds and hashes](builds.json)
and [preservation](preservation.json). Raw logs are retained under
`.local/prototype-recovery-20261016T021809Z/mvp-skills-validation-20261017/`.
Before disconnect, tests first exposed automatic instruction enablement and native
legacy selection. The first broad run then exposed eighteen synthetic session
fixtures missing explicit review. Those fixtures were migrated; the fresh complete
run above passed. The earlier temporary red logs were lost during disconnect and
are not claimed as available artifacts here.

Fresh app paths:

- `/tmp/fort058-mvp-skills-mac-20261017/Build/Products/Debug/FortMac.app`
- `/tmp/fort058-mvp-skills-ios-20261017/Build/Products/Debug-iphonesimulator/FortiOS.app`

Neither app nor Simulator was launched. No live enrollment/provider turn, Keychain
change, provider sign-in, spending, trust/security change, startup registration,
remote resource, push, merge or deployment occurred. The three-session temporary
enrollment proposal remains pending. Reconnection did not approve it.

Installed accessibility/screenshots, actual-provider acceptance, physical iPhone,
provider/topology feasibility, enforced host isolation, lifecycle/notification tests,
encrypted backup/restore and distribution remain open. The no-tool adapter supports
complete UTF-8 text bundles; binary files, scripts and requested tools remain refused
before runtime dispatch. This record does not complete the MVP.
