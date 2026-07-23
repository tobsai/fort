# 023 — OpenClaw native provider

**Status:** implemented and verified
**Governed by:** [021-fort-native](021-fort-native.md)
**Relates to:** [017-openclaw-import](017-openclaw-import.md)

## Goal

Give Fort a native OpenClaw provider so the existing `explicit-openclaw` and
`errand-lane` rules dispatch to an installed OpenClaw agent instead of
dead-ending at provider lookup or an interactive CLI.

## Verified command contract

Fort invokes the installed CLI through OpenClaw's explicit embedded,
non-interactive agent path:

```sh
openclaw agent --local --agent main --message "<prompt>" --json
```

This contract was verified on the enrolled execution host against OpenClaw
2026.7.1-2 on 2026-07-23:

- `agent` is the supported one-shot subcommand; `run` is not a command.
- `--local` runs the configured agent directly and does not require the
  separate OpenClaw gateway daemon to be healthy.
- `--agent main` selects a deterministic configured agent.
- `--message` carries the Fort prompt without stdin or a PTY.
- `--json` reserves stdout for the structured result.

The playbook label `Fable` is intentionally not passed as `--model`: it is a
Fort design label, not a verified OpenClaw `provider/model` identifier. The
configured `main` agent owns model selection until an explicit mapping is
approved.

## Runtime safety

The provider declares a token-free contract probe:

```sh
openclaw agent --help
```

`NativeRuntime` runs the probe before every dispatch. If an installed CLI
removes or renames the required subcommand, dispatch fails before consuming
tokens and reports that the provider command contract is unavailable.

Machine capability advertisement uses the same probe. Merely finding an
`openclaw` executable on `PATH` is insufficient: a machine only advertises the
provider when the non-interactive contract is callable.

`jsonTextParser` extracts response text from JSON and retains raw stdout for
unrecognized output, so a CLI output-shape addition does not silently discard
the result.

## Test criteria

`go test ./...` must cover:

- `DefaultProviders()` includes `openclaw`.
- OpenClaw argv exactly matches the verified local one-shot contract.
- All default providers declare a token-free, provider-specific help probe.
- Dispatch fails closed when an installed provider's command contract drifts.
- Machine capability discovery excludes an installed-but-incompatible CLI.
- OpenClaw does not invent a model flag from an unmapped Fort label.

The relay suite separately proves an encrypted mobile `POST /api/chat`
round-trip preserves method, content type, and body through the broker and
decrypts the daemon response.

## Rollback

Revert the `openclawProvider` registration. OpenClaw routes then fail closed
with no registered provider; no other native provider is affected.
