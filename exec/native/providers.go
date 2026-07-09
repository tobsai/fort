package native

import (
	"strings"

	"github.com/tobsai/fort/core/runtime"
)

// DefaultProviders returns the built-in provider set encoding the AO-002 recon
// contract (docs/notes/runtime-recon.md) for all four agents.
func DefaultProviders() []Provider {
	return []Provider{claudeProvider(), codexProvider(), hermesProvider(), openclawProvider()}
}

// claude: headless print mode with a streaming JSON output format.
//
//	claude -p "<prompt>" --output-format stream-json --include-partial-messages --verbose
func claudeProvider() Provider {
	return Provider{
		Name: "claude",
		Command: func(s runtime.RunSpec) []string {
			return []string{"claude", "-p", s.Prompt,
				"--output-format", "stream-json", "--include-partial-messages", "--verbose"}
		},
		// Structured classification (spec 030): typed tool/subagent/message
		// events; result/partials/system fall through to raw stdout (the
		// planner's result-line contract, spec 026, is preserved).
		Classify: classifyClaude,
		// Parse retained as documentation of the legacy path; Classify wins.
		Parse: jsonTextParser,
	}
}

// codex: non-interactive exec subcommand emitting JSONL events.
//
//	codex exec "<prompt>" --json --sandbox workspace-write --skip-git-repo-check
//
// Verified against codex-cli 0.143.0 (2026-07-09, live run): `codex exec` has no
// --ask-for-approval flag — exec is already non-interactive, and passing it
// aborts with "unexpected argument". --skip-git-repo-check is required because
// Fort runs each agent in a scratch workdir that is not a git repository.
func codexProvider() Provider {
	return Provider{
		Name: "codex",
		Command: func(s runtime.RunSpec) []string {
			return []string{"codex", "exec", s.Prompt,
				"--json", "--sandbox", "workspace-write", "--skip-git-repo-check"}
		},
		Parse: jsonTextParser,
	}
}

// hermes: one-shot mode prints the final text; unattended flags added.
//
//	hermes --oneshot "<prompt>" --accept-hooks --yolo
func hermesProvider() Provider {
	return Provider{
		Name: "hermes",
		Command: func(s runtime.RunSpec) []string {
			return []string{"hermes", "--oneshot", s.Prompt, "--accept-hooks", "--yolo"}
		},
		// hermes --oneshot prints plain final text; treat every line as a message.
		Parse: func(line string) (string, bool) {
			if strings.TrimSpace(line) == "" {
				return "", false
			}
			return line, true
		},
	}
}

// openclaw: one-shot errand runner (spec 023). BEST GUESS — the openclaw CLI is
// not yet installed here and docs/notes/runtime-recon.md §4 is still TODO, so the
// argv mirrors the sibling providers and is isolated to this one line for easy
// correction once probed (FORT_LIVE_CLI=openclaw FORT_LIVE_PROBE=1). If the CLI
// is absent, dispatch fails at spawn time like any missing binary; multi-machine
// placement (spec 022) keeps openclaw tasks on machines that list `openclaw`.
//
//	openclaw run "<prompt>" --headless --accept-hooks
func openclawProvider() Provider {
	return Provider{
		Name: "openclaw",
		Command: func(s runtime.RunSpec) []string {
			return []string{"openclaw", "run", s.Prompt, "--headless", "--accept-hooks"}
		},
		// Lenient: extracts text from JSON output, else falls through to raw
		// stdout — robust whether openclaw emits JSONL or plain text.
		Parse: jsonTextParser,
	}
}

// jsonTextParser pulls a human-readable "text" field out of a provider's JSONL
// event line, if present. It is intentionally lenient: unknown shapes fall
// through to a raw stdout event.
func jsonTextParser(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "{") {
		return "", false
	}
	// Cheap, dependency-free extraction of a top-level "text" or "message" value.
	for _, key := range []string{`"text":`, `"message":`, `"content":`} {
		if i := strings.Index(line, key); i >= 0 {
			rest := strings.TrimSpace(line[i+len(key):])
			if strings.HasPrefix(rest, `"`) {
				if v, ok := unquoteJSONString(rest); ok && v != "" {
					return v, true
				}
			}
		}
	}
	return "", false
}

// unquoteJSONString reads a JSON string literal at the start of s and returns
// its decoded value (handling \" and \\ escapes) plus ok.
func unquoteJSONString(s string) (string, bool) {
	if len(s) == 0 || s[0] != '"' {
		return "", false
	}
	var b strings.Builder
	esc := false
	for i := 1; i < len(s); i++ {
		c := s[i]
		if esc {
			switch c {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(c)
			}
			esc = false
			continue
		}
		if c == '\\' {
			esc = true
			continue
		}
		if c == '"' {
			return b.String(), true
		}
		b.WriteByte(c)
	}
	return "", false
}
