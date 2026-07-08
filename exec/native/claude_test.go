package native

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/runtime"
)

func TestClassifyClaudeTable(t *testing.T) {
	asst := func(blocks string) string {
		return `{"type":"assistant","message":{"content":[` + blocks + `]}}`
	}
	cases := []struct {
		name string
		line string
		want []Classified // nil => expect ok=false (raw stdout fallback)
	}{
		{"tool_use", asst(`{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}`),
			[]Classified{{runtime.EventTool, `{"name":"Bash","summary":"go test ./..."}`}}},
		{"subagent", asst(`{"type":"tool_use","name":"Task","input":{"description":"write a repro test","subagent_type":"general-purpose"}}`),
			[]Classified{{runtime.EventSubagent, `{"description":"write a repro test","agent":"general-purpose"}`}}},
		{"text", asst(`{"type":"text","text":"patch applied"}`),
			[]Classified{{runtime.EventMessage, "patch applied"}}},
		{"text-plus-tool", asst(`{"type":"text","text":"reading"},{"type":"tool_use","name":"Read","input":{"file_path":"a.go"}}`),
			[]Classified{{runtime.EventMessage, "reading"}, {runtime.EventTool, `{"name":"Read","summary":"a.go"}`}}},
		{"tool_result", `{"type":"user","message":{"content":[{"type":"tool_result","content":[{"type":"text","text":"ok: 3 files"}]}]}}`,
			[]Classified{{runtime.EventTool, `{"name":"tool_result","summary":"ok: 3 files"}`}}},
		// REGRESSION GUARDS — all must fall through to stdout (ok=false):
		{"result-line", `{"type":"result","subtype":"success","result":"[{\"title\":\"a\"}]"}`, nil},
		{"partial", `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"par"}}}`, nil},
		{"system", `{"type":"system","subtype":"init"}`, nil},
		{"not-json", `plain noise`, nil},
		{"malformed", `{"type":"assistant","message":`, nil},
	}
	for _, c := range cases {
		got, ok := classifyClaude(c.line)
		if c.want == nil {
			if ok {
				t.Errorf("%s: ok=true, want fallthrough; got %+v", c.name, got)
			}
			continue
		}
		if !ok || len(got) != len(c.want) {
			t.Fatalf("%s: ok=%v len=%d, want %d: %+v", c.name, ok, len(got), len(c.want), got)
		}
		for i := range got {
			if got[i].Type != c.want[i].Type {
				t.Errorf("%s[%d]: type=%q want %q", c.name, i, got[i].Type, c.want[i].Type)
			}
			if got[i].Type == runtime.EventMessage {
				if got[i].Data != c.want[i].Data {
					t.Errorf("%s[%d]: msg=%q want %q", c.name, i, got[i].Data, c.want[i].Data)
				}
				continue
			}
			// tool/subagent payloads: compare as JSON (field order agnostic)
			var g, w map[string]string
			if json.Unmarshal([]byte(got[i].Data), &g) != nil || json.Unmarshal([]byte(c.want[i].Data), &w) != nil {
				t.Fatalf("%s[%d]: non-JSON data %q", c.name, i, got[i].Data)
			}
			for k, v := range w {
				if g[k] != v {
					t.Errorf("%s[%d]: %s=%q want %q (full: %q)", c.name, i, k, g[k], v, got[i].Data)
				}
			}
		}
	}
}

func TestClassifyClaudeSummaryTruncation(t *testing.T) {
	long := strings.Repeat("x", 500)
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"` + long + `"}}]}}`
	evs, ok := classifyClaude(line)
	if !ok || len(evs) != 1 {
		t.Fatalf("evs=%+v ok=%v", evs, ok)
	}
	var d map[string]string
	_ = json.Unmarshal([]byte(evs[0].Data), &d)
	if len(d["summary"]) > 160 {
		t.Errorf("summary not truncated: %d chars", len(d["summary"]))
	}
}
