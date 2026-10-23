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
App Store records, API keys or profiles needs Toby's approval. The Mac archive
doesn't embed the host resources yet, so Mac betas run without a local host for now.

## v1 checklist

Update this list in the PR that changes it.

- [x] Chat with a ChatGPT-plan model on the Mac, with history kept across quit and relaunch
- [x] Pick the model per agent and keep that choice through runtime updates
- [x] Remove local credentials and the host cleanly, keeping history unless deletion is chosen
- [ ] Sign in with ChatGPT from inside the app (runtime merged, sign-in flow not composed)
- [ ] Claude runner using the Claude subscription
- [ ] Pair the iPhone with the Mac over a private HTTPS endpoint
- [ ] iPhone build in TestFlight
- [ ] Mac build in TestFlight
- [ ] Toby uses both apps for a day on real devices
- [ ] Simplified UI (Crew, Conversation, Settings)

History before October 23, 2026 (ledger, evidence, delivery diaries) is archived
at commit `a4152e8`. See [decision 061](../docs/architecture/decisions/061-lean-dev-cycle.md).
