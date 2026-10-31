# Fort

A Node/TypeScript core with one local ledger and optional runtime modules, plus
Swift Mac and iPhone apps in `apple/`. Rules for agents are in [../CLAUDE.md](../CLAUDE.md).
The donor import in `../foundation/` is quarantined and is not a runtime dependency.

## Test

Use Node 26.5.0.

```sh
make -C successor install   # npm ci with lifecycle scripts disabled
make -C successor test      # typecheck + portable tests
```

On a Mac, `make -C successor test-mac` adds the macOS suite. Swift tests need
synthetic host fixtures first. The exact steps and environment variables are in
the native job of [the CI workflow](../.github/workflows/fort-quality.yml). Then run:

```sh
xcrun swift test --package-path successor/apple/FortKit --scratch-path /tmp/fort-swift --no-parallel
```

Keep `--no-parallel`. Otherwise unrelated tests compete for the main actor and
break timing checks.

## Ship a beta

```sh
export FORT_TEAM_ID=… FORT_BUNDLE_ID_IOS=… ASC_KEY_ID=… ASC_ISSUER_ID=… ASC_KEY_PATH=…
make -C successor beta                 # iPhone
make -C successor beta PLATFORM=mac    # Mac (needs FORT_BUNDLE_ID_MAC)
```

This archives, signs and uploads to App Store Connect. The build shows up in
TestFlight once Apple processes it. A beta counts as accepted when Toby has
used it, not when the upload succeeds.

The project's default bundle IDs are fixture IDs (`io.fort.spec058.*`) with
signing turned off, so the script needs the real App Store IDs passed in. Creating
App Store records, API keys or profiles needs Toby's approval.

### Mac beta (with the local runner)

The Mac build carries the FortHost helper (Node 26.5.0, Codex 0.163.0, the process
guardian), so `PLATFORM=mac` runs `release/mac-testflight.sh` instead of an Xcode
archive. It builds both apps unsigned, assembles them with `packaging/mac-application.ts`,
signs inside out for the Mac App Store with `packaging/mac-sign.ts` (each executable
in its App Sandbox, the runtimes inheriting the helper's), checks every signature
against `packaging/mac-signing-plan.ts`, builds `Fort.pkg` and uploads it.
`make -C successor mac-package` does the same with ad-hoc signing and no upload; CI runs it.

One-time setup in the Apple developer account and App Store Connect:

- App IDs `io.mtree.fort` (already the iPhone app) and `io.mtree.fort.host`, both with
  the App Groups capability and the group `group.io.mtree.fort`.
- Add macOS to the existing Fort app record (same bundle ID, so it's one app on both).
- An Apple Distribution certificate and a Mac Installer Distribution certificate in the
  login keychain of the Mac that runs the script.
- Two Mac App Store provisioning profiles, one per App ID.

```sh
export FORT_TEAM_ID=… FORT_MAC_PROFILE_APP=…/Fort.provisionprofile FORT_MAC_PROFILE_HOST=…/FortHost.provisionprofile
export ASC_KEY_ID=… ASC_ISSUER_ID=… ASC_KEY_PATH=…
make -C successor beta PLATFORM=mac
```

Other bundle IDs or group: `FORT_BUNDLE_ID_MAC`, `FORT_HOST_BUNDLE_ID_MAC`, `FORT_APP_GROUP_MAC`.
`FORT_SKIP_UPLOAD=1` stops after `Fort.pkg`. The helper reads the owner's Claude Code
through a temporary sandbox exception, which internal TestFlight accepts but public
App Review may question.

## Deploy the hub

Railway builds `successor/Dockerfile` from `main` for fort.mtree.io. The Fort quality workflow also
runs on every push to `main`, and its `hub-image` job builds that same image and checks that the hub
starts and answers. With **Wait for CI** on in the hub service's Railway settings, a deploy waits for
that workflow and is skipped if it fails. Runs on `main` are never cancelled, because Railway treats
a cancelled run as a block.

The hub keeps everything in SQLite on its `/data` volume. Railway can back that volume up on a
schedule (the hub service's **Backups** tab): daily copies are kept 6 days, weekly 27 and monthly 89.
SQLite survives a snapshot taken mid-write the same way it survives a power cut. Turn on Daily, plus
Weekly if you want a longer reach. Backups are billed as volume storage for the changed data only.
Two things to know: deleting history in Fort doesn't reach the backups, so deleted messages live on
until the copies age out, and wiping the volume deletes its backups too. A restore mounts the copy as
a new volume and stages a redeploy for you to review.

## v1 checklist

Update this list in the PR that changes it.

- [x] Chat with a ChatGPT-plan model on the Mac, with history kept across quit and relaunch
- [x] Pick the model per agent and keep that choice through runtime updates
- [x] Remove local credentials and the host cleanly, keeping history unless deletion is chosen
- [ ] Sign in with ChatGPT from inside the app (runtime merged, sign-in flow not composed)
- [ ] Claude runner using the Claude subscription
- [ ] Pair the iPhone with the Mac over a private HTTPS endpoint (built over Tailscale in Menu → Devices; first real pairing not yet done)
- [x] iPhone build in TestFlight (`io.mtree.fort`)
- [ ] Mac build in TestFlight
- [ ] Toby uses both apps for a day on real devices
- [ ] Simplified UI (Crew, Conversation, Settings)

History before October 23, 2026 (ledger, evidence, delivery diaries) is archived
at commit `a4152e8`. See [decision 061](../docs/architecture/decisions/061-lean-dev-cycle.md).
