package native

// classifyClaude turns one line of claude's --output-format stream-json into
// typed run events (spec 030). It is TOTAL and additive-safe:
//   - complete "assistant" lines -> EventMessage (text blocks, joined) plus one
//     EventTool per tool_use block (EventSubagent when the tool is Task);
//   - "user" lines with tool_result blocks -> a lightweight EventTool result
//     with a short output preview;
//   - EVERYTHING else (the terminal "result" line — which spec 026's planner
//     extraction reads from raw stdout and MUST keep reaching it — partial
//     stream_event frames, system lines, non-JSON, malformed JSON) -> ok=false,
//     so the scan loop emits the raw line as EventStdout exactly as before.

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/tobsai/fort/core/runtime"
)

const summaryMax = 160

type claudeBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Content json.RawMessage `json:"content"` // tool_result payload (string or blocks)
}

type claudeLine struct {
	Type    string `json:"type"`
	Message struct {
		Content []claudeBlock `json:"content"`
	} `json:"message"`
}

func classifyClaude(line string) ([]Classified, bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "{") {
		return nil, false
	}
	var l claudeLine
	if json.Unmarshal([]byte(s), &l) != nil {
		return nil, false
	}
	switch l.Type {
	case "assistant":
		var out []Classified
		var texts []string
		for _, b := range l.Message.Content {
			switch b.Type {
			case "text":
				if strings.TrimSpace(b.Text) != "" {
					texts = append(texts, b.Text)
				}
			case "tool_use":
				out = append(out, classifyToolUse(b))
			}
		}
		if len(texts) > 0 {
			msg := Classified{Type: runtime.EventMessage, Data: strings.Join(texts, "\n")}
			out = append([]Classified{msg}, out...)
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	case "user":
		var out []Classified
		for _, b := range l.Message.Content {
			if b.Type == "tool_result" {
				out = append(out, Classified{
					Type: runtime.EventTool,
					Data: toolJSON("tool_result", resultPreview(b.Content)),
				})
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	default:
		// result / stream_event partials / system / unknown -> raw stdout.
		return nil, false
	}
}

// classifyToolUse maps a tool_use block: Task -> subagent, else tool.
func classifyToolUse(b claudeBlock) Classified {
	var in map[string]json.RawMessage
	_ = json.Unmarshal(b.Input, &in)
	str := func(k string) string {
		var v string
		if raw, ok := in[k]; ok && json.Unmarshal(raw, &v) == nil {
			return v
		}
		return ""
	}
	if b.Name == "Task" {
		d, _ := json.Marshal(map[string]string{
			"description": truncate(firstNonEmpty(str("description"), str("prompt"))),
			"agent":       str("subagent_type"),
		})
		return Classified{Type: runtime.EventSubagent, Data: string(d)}
	}
	// summary: the most human-meaningful input field, else compact raw input.
	sum := firstNonEmpty(str("command"), str("file_path"), str("path"),
		str("pattern"), str("description"), str("query"), str("url"), str("prompt"))
	if sum == "" && len(b.Input) > 0 {
		sum = string(b.Input)
	}
	return Classified{Type: runtime.EventTool, Data: toolJSON(b.Name, sum)}
}

func toolJSON(name, summary string) string {
	d, _ := json.Marshal(map[string]string{"name": name, "summary": truncate(summary)})
	return string(d)
}

// resultPreview extracts a short text preview from a tool_result content value,
// which claude emits either as a plain string or as an array of text blocks.
func resultPreview(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var texts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				texts = append(texts, b.Text)
			}
		}
		return strings.Join(texts, " ")
	}
	return ""
}

const ellipsis = "…" // 3 bytes in UTF-8; reserve its width in the cut point

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > summaryMax {
		cut := s[:summaryMax-len(ellipsis)]
		for len(cut) > 0 && !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}
		return cut + ellipsis
	}
	return s
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
