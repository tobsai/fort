package native

import (
	"encoding/json"
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
		Name:  "claude",
		Probe: []string{"claude", "-p", "--help"},
		Command: func(s runtime.RunSpec) []string {
			return withModel("claude", []string{"claude", "-p", s.Prompt,
				"--output-format", "stream-json", "--include-partial-messages", "--verbose"}, s.Model)
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
		Name:  "codex",
		Probe: []string{"codex", "exec", "--help"},
		Command: func(s runtime.RunSpec) []string {
			return withModel("codex", []string{"codex", "exec", s.Prompt,
				"--json", "--sandbox", "workspace-write", "--skip-git-repo-check"}, s.Model)
		},
		Failure: codexFailure,
		Parse:   jsonTextParser,
	}
}

// hermes: one-shot mode prints the final text; unattended flags added.
//
//	hermes --oneshot "<prompt>" --accept-hooks --yolo
func hermesProvider() Provider {
	return Provider{
		Name:  "hermes",
		Probe: []string{"hermes", "--help"},
		Command: func(s runtime.RunSpec) []string {
			return withModel("hermes", []string{"hermes", "--oneshot", s.Prompt, "--accept-hooks", "--yolo"}, s.Model)
		},
		Failure: prefixedFailure("API call failed after "),
		// hermes --oneshot prints plain final text; treat every line as a message.
		Parse: func(line string) (string, bool) {
			if strings.TrimSpace(line) == "" {
				return "", false
			}
			return line, true
		},
	}
}

// openclaw: one local, embedded agent turn (spec 023).
// Verified against OpenClaw 2026.7.1-2 on the enrolled execution host
// (2026-07-23). Local mode runs the configured agent without depending on the
// separate OpenClaw gateway daemon. The explicit main-agent selector makes the
// headless session deterministic; JSON reserves stdout for the structured
// result.
//
//	openclaw agent --local --agent main --session-id "<fort-invocation-id>" --message "<prompt>" --thinking off --timeout 60 --json
func openclawProvider() Provider {
	return Provider{
		Name:  "openclaw",
		Probe: []string{"openclaw", "agent", "--help"},
		Command: func(s runtime.RunSpec) []string {
			// Saved playbooks currently carry the design label "Fable", not an
			// OpenClaw provider/model id. Let the configured main agent select
			// its verified model until that label has an approved mapping.
			return []string{
				"openclaw", "agent", "--local", "--agent", "main",
				"--session-id", s.RunID, "--message", s.Prompt,
				"--thinking", "off", "--timeout", "60", "--json",
			}
		},
		// Lenient: extracts text from JSON output, else falls through to raw
		// stdout — robust whether openclaw emits JSONL or plain text.
		Parse: jsonTextParser,
	}
}

func prefixedFailure(prefix string) func(string) (string, bool) {
	return func(line string) (string, bool) {
		line = strings.TrimSpace(line)
		return line, strings.HasPrefix(line, prefix)
	}
}

func codexFailure(line string) (string, bool) {
	var event struct {
		Type  string `json:"type"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(line), &event) != nil || event.Type != "error" {
		return "", false
	}
	if message := strings.TrimSpace(event.Error.Message); message != "" {
		return message, true
	}
	return strings.TrimSpace(line), true
}

func withModel(agent string, argv []string, model string) []string {
	if model == "" {
		return argv
	}
	return append(argv, "--model", providerModel(agent, model))
}

// providerModel translates the approved handoff's human-readable model labels
// at the native-provider boundary. All other values are passed through so a
// saved, provider-specific model identifier remains exact.
func providerModel(agent, model string) string {
	switch {
	case agent == "claude" && model == "Sonnet":
		return "sonnet"
	case agent == "claude" && model == "Opus":
		return "opus"
	case agent == "codex" && model == "5.6 Sol":
		return "gpt-5.6-sol"
	case agent == "hermes" && model == "Codex 5.6 Sol":
		return "openai-codex/gpt-5.6-sol"
	default:
		return model
	}
}

// jsonTextParser pulls a human-readable "text" field out of a provider's JSONL
// event line, if present. It is intentionally lenient: unknown shapes fall
// through to a raw stdout event.
func jsonTextParser(line string) (string, bool) {
	line = strings.TrimSpace(line)
	jsonLine := strings.HasPrefix(line, "{") ||
		strings.HasPrefix(line, `"text":`) ||
		strings.HasPrefix(line, `"message":`) ||
		strings.HasPrefix(line, `"content":`)
	if !jsonLine {
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
