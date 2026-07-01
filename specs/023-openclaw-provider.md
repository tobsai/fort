# 023 — OpenClaw native provider

**Status:** proposed · **New capability — requires Toby's approval before merge.**
**Governed by:** [021-fort-native](021-fort-native.md) · **Relates to:** [017-openclaw-import](017-openclaw-import.md)

> Author's note: implemented under Toby's "take your best guess" authorization.
> The exact `openclaw` CLI contract is **unverified** — the binary is not
> installed on this machine and `docs/notes/runtime-recon.md §4` still marks it
> TODO. The argv below is a best guess mirroring the sibling providers and is
> isolated to one function + one comment so it is a one-line fix once probed.

## Goal
Complete "a control plane for Hermes, OpenClaw, Claude Code, and Codex" by giving
Fort a fourth `native` provider so the existing `openclaw` routing rules
(`explicit-openclaw`, `errand-lane`) actually dispatch instead of failing with
"no provider registered".

## Approach (as built)
Add `openclawProvider()` to `exec/native/providers.go` and include it in
`DefaultProviders()`, mirroring the claude/codex/hermes pattern:

```go
// openclaw: one-shot errand runner. BEST GUESS — verify against the real CLI
// (docs/notes/runtime-recon.md §4). If the binary is absent, dispatch fails at
// spawn time exactly like any missing CLI; multi-machine placement (spec 022)
// keeps openclaw tasks on machines whose machines.yaml lists `openclaw`.
//
//	openclaw run "<prompt>" --headless --accept-hooks
func openclawProvider() Provider {
    return Provider{
        Name: "openclaw",
        Command: func(s runtime.RunSpec) []string {
            return []string{"openclaw", "run", s.Prompt, "--headless", "--accept-hooks"}
        },
        Parse: jsonTextParser, // lenient: extracts text from JSON, else raw stdout
    }
}
```

`jsonTextParser` is the safe parser choice: if `openclaw` emits JSON it extracts
the text field; if it emits plain text the line falls through to a raw stdout
event, so output is never dropped regardless of format.

## Decisions (best-guess; correct on review)
- **Binary `openclaw`** — HIGH confidence (all rules/docs use it).
- **`run "<prompt>" --headless --accept-hooks`** — MEDIUM/LOW; sibling-pattern
  guess. Alternatives if `run` is wrong: `--print` (claude-like) or `--oneshot`
  (hermes-like). Isolated to one line.
- **`jsonTextParser`** — robust to either JSON or plain-text output.
- **Kept in `DefaultProviders`** even though the CLI may be absent: this is the
  explicit ask ("control plane for … OpenClaw"). A missing binary fails the run
  cleanly, same as any provider; it does not affect the other three.

## Affected files
- Changed: `exec/native/providers.go` (add `openclawProvider`, register it),
  `exec/native/native.go` (docstring: openclaw now included),
  `exec/native/native_test.go` (provider registered + argv shape),
  `.env.example` (uncomment the `openclaw` block guidance).
- Verification: `exec/native/native_live_test.go` already supports
  `FORT_LIVE_CLI=openclaw FORT_LIVE_PROBE=1` to probe the real binary.

## Test criteria (`go test ./...`)
- `DefaultProviders()` includes `openclaw`; its `Command` yields the expected
  argv for a sample `RunSpec` (guards accidental argv drift).
- Dispatching `openclaw` through `exec/fake` (token-free) still routes/records —
  the errand lane no longer dead-ends.
- Live probe (opt-in) documents how to confirm the real argv once installed.

## Rollback
Revert the `openclawProvider` addition; `DefaultProviders()` returns to the
three-provider set and the `openclaw` rules fail-closed as before.
