package native

import (
	"strings"

	"github.com/tobsai/fort/core/runtime"
)

// DefaultProviders returns the built-in provider set encoding the AO-002 recon
// contract (docs/notes/runtime-recon.md). openclaw is intentionally absent until
// the CLI is installed and probed.
func DefaultProviders() []Provider {
	return []Provider{claudeProvider(), codexProvider(), hermesProvider()}
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
		Parse: jsonTextParser,
	}
}

// codex: non-interactive exec subcommand emitting JSONL events, no approvals.
//
//	codex exec "<prompt>" --json --ask-for-approval never --sandbox workspace-write
func codexProvider() Provider {
	return Provider{
		Name: "codex",
		Command: func(s runtime.RunSpec) []string {
			return []string{"codex", "exec", s.Prompt,
				"--json", "--ask-for-approval", "never", "--sandbox", "workspace-write"}
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
