# AO-043 · Packaging & distribution

One-command install for `fort`, matching the existing OSS + Homebrew posture.

## Local build
```sh
make build           # -> ./bin/fort
./bin/fort version
```

## Cross-platform snapshot (no publish)
```sh
brew install goreleaser   # once
make snapshot             # -> ./dist/ for darwin+linux, amd64+arm64
```

## Cut a release (publishes the Homebrew formula)
Requires a git tag and a `GITHUB_TOKEN` with access to both `tobsai/fort` and the
tap repo `tobsai/homebrew-tap`:
```sh
git tag v0.1.0 && git push origin v0.1.0
export GITHUB_TOKEN=…            # repo + tap scope
make release                     # goreleaser builds, creates the GitHub release,
                                 # and pushes Formula/fort.rb to tobsai/homebrew-tap
```

## Install from the tap
```sh
brew tap tobsai/tap
brew install fort
fort version
```

## Status / what's automated
- `.goreleaser.yaml` — cross-compiles `fort`, builds archives + checksums, and
  templates the Homebrew formula into `tobsai/homebrew-tap`.
- `Formula/fort.rb` — a checked-in reference formula (build-from-source) so you
  can `brew install --build-from-source ./Formula/fort.rb` without a release.
- **Requires the user:** the actual `git tag` + `GITHUB_TOKEN` + `make release`
  step publishes the release and the tap. This build wires everything up to that
  point; cutting the live release is a credentialed action left to Toby (there is
  also a `cut-fort-release` skill that automates it).
