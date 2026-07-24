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
openclaw agent --local --agent main --session-id "<fort-invocation-id>" --message "<prompt>" --thinking off --timeout 60 --json
```

This contract was verified on the enrolled execution host against OpenClaw
2026.7.1-2 on 2026-07-23:

- `agent` is the supported one-shot subcommand; `run` is not a command.
- `--local` runs the configured agent directly and does not require the
  separate OpenClaw gateway daemon to be healthy.
- `--agent main` selects a deterministic configured agent.
- `--session-id <fort-invocation-id>` isolates each concrete Fort invocation
  from OpenClaw's shared `agent:main:main` session. Direct assignments use their
  Fort run ID. Graph task nodes use the deterministic
  `<parent-run-id>:<node-id>:<one-based-attempt>` form, so separate stages and
  retries cannot contend on or inherit one another's OpenClaw session. Before
  dispatch, Fort durably records the running node attempt and its input. A
  crash/resume advances from that stored attempt and reuses the stored input,
  so it cannot reuse an already-dispatched OpenClaw session ID.
- `--message` carries the Fort prompt without stdin or a PTY.
- `--thinking off` uses the bounded, verified execution profile instead of
  leaving extended thinking to the agent's ambient configuration.
- `--timeout 60` gives OpenClaw a provider-side 60-second deadline.
- `--json` reserves stdout for the structured result.

The isolated-session contract was added after a live Fort smoke assignment
entered the shared main session and emitted no output for several minutes. A
later deployed smoke using an isolated session but not the bounded execution
flags still produced no result within 60 seconds; cancellation completed and
left no orphan. On the enrolled execution host, OpenClaw 2026.7.1-2 completed a
direct probe with an explicit unique session ID, `--thinking off`, and
`--timeout 60` in under 10 seconds.

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
- OpenClaw uses the runtime invocation ID as an isolated session ID.
- OpenClaw disables thinking and sets the verified 60-second provider timeout.
- Graph stages and retries derive distinct deterministic invocation IDs while
  their events and node state remain attached to the parent Fort run.
- Graph attempts are durably claimed before dispatch; claim failure prevents
  dispatch, and crash/resume advances the stored attempt and input.
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
