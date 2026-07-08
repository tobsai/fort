# Subagent + Tool Events (spec 030) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The native `claude` provider parses its stream-json into typed `tool`/`subagent` run events (instead of dropping structure), so the UI can show "a subagent is working".

**Architecture:** `core/runtime` gains two `EventType` constants. `exec/native.Provider` gains an optional `Classify func(line) ([]Classified, bool)` that supersedes `Parse` when set (a line can yield multiple typed events, e.g. text + two tool_use blocks). Only the claude provider gets a classifier: complete `assistant` lines → `message` + `tool`/`subagent` events; `user` tool_result lines → a lightweight `tool` result event; partials (`stream_event`), `system`, `result`, and anything malformed → raw `stdout` unchanged. CRITICAL regression guard: the terminal `{"type":"result"}` line MUST keep landing as a raw `stdout` event — spec 026's planner extraction reads it from there.

**Tech Stack:** Go 1.26, `encoding/json` (loose structs), existing `exec/native` test pattern (stub providers running `sh -c` scripts — see `native_test.go:14`).

**Behavior change (intentional, documented):** today `jsonTextParser` matches the `"text":` key in *partial* `stream_event` lines too, so one assistant sentence produces many `message` events (the doubling 026 documented). The classifier emits `message` only for complete `assistant` lines — fewer, cleaner messages; partials degrade to `stdout` exactly like unknown lines.

---

## File Structure

- `core/runtime/runtime.go` — `EventTool`, `EventSubagent` constants.
- `exec/native/native.go` — `Classified` type, `Provider.Classify` field, `nativeRun.classify` plumbing, scan-loop branch.
- `exec/native/claude.go` (new) — `classifyClaude(line)` — the stream-json classifier + input summarizer (kept out of `providers.go` so the file stays focused).
- `exec/native/claude_test.go` (new) — table-driven classifier tests + regression guards.
- `exec/native/providers.go` — claude provider sets `Classify: classifyClaude`.
- `docs/notes/runtime-recon.md` — the claude stream-json → event map.

---

### Task 1: Event types + Classify plumbing

**Files:**
- Modify: `core/runtime/runtime.go` (the `EventType` const block, ~line 29)
- Modify: `exec/native/native.go` (Provider ~line 25; nativeRun ~line 141; Dispatch run literal ~line 122; scan ~line 195)
- Test: `exec/native/native_test.go`

- [ ] **Step 1.1: Write the failing test** — append to `exec/native/native_test.go`:

```go
func TestClassifyEmitsTypedEvents(t *testing.T) {
	// A provider with Classify turns one stdout line into N typed events;
	// unclassified lines fall through to raw stdout.
	p := Provider{
		Name:    "clsfy",
		Command: func(_ runtime.RunSpec) []string { return []string{"sh", "-c", `printf 'KNOWN\nnoise\n'`} },
		Classify: func(line string) ([]Classified, bool) {
			if line == "KNOWN" {
				return []Classified{
					{Type: runtime.EventTool, Data: `{"name":"Read"}`},
					{Type: runtime.EventSubagent, Data: `{"description":"sub"}`},
				}, true
			}
			return nil, false
		},
	}
	rt := New(t.TempDir(), p)
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "c1", Agent: "clsfy"})
	if err != nil {
		t.Fatal(err)
	}
	var got []runtime.RunEvent
	for ev := range run.Stream() {
		got = append(got, ev)
	}
	var tools, subs, stdouts int
	for _, e := range got {
		switch e.Type {
		case runtime.EventTool:
			tools++
		case runtime.EventSubagent:
			subs++
		case runtime.EventStdout:
			if e.Data == "noise" {
				stdouts++
			}
		}
	}
	if tools != 1 || subs != 1 || stdouts != 1 {
		t.Fatalf("tools=%d subs=%d stdouts=%d (want 1,1,1); events=%+v", tools, subs, stdouts, got)
	}
}
```

- [ ] **Step 1.2: Run — expect FAIL** (no `Classified`, no `Classify` field, no `EventTool`).

Run: `go test ./exec/native/ -run TestClassifyEmitsTypedEvents`
Expected: compile errors.

- [ ] **Step 1.3: Add the event types** — in `core/runtime/runtime.go`, extend the const block:

```go
	// EventTool: the agent invoked a tool (spec 030). Data is a compact JSON
	// object {"name":..., "summary":...}.
	EventTool EventType = "tool"
	// EventSubagent: the agent spawned a sub-task (spec 030; claude's Task
	// tool). Data is {"description":..., "agent":...}.
	EventSubagent EventType = "subagent"
```

- [ ] **Step 1.4: Plumb Classify** — in `exec/native/native.go`:

Add next to the `Provider` type:

```go
// Classified is one typed event extracted from a provider stdout line.
type Classified struct {
	Type runtime.EventType
	Data string
}
```

Add to `Provider` (after `Parse`):

```go
	// Classify optionally turns a stdout line into typed events (spec 030). A
	// line may yield several (text + tool_use blocks). When set it supersedes
	// Parse; ok=false falls through to a raw EventStdout.
	Classify func(line string) ([]Classified, bool)
```

Add to `nativeRun` (after `parse`):

```go
	classify func(string) ([]Classified, bool)
```

In `Dispatch`, the run literal gains one field:

```go
	run := &nativeRun{
		spec:     spec,
		parse:    p.Parse,
		classify: p.Classify,
		events:   make(chan runtime.RunEvent, 64),
		done:     make(chan struct{}),
		stdin:    stdin,
		cancel:   cancel,
		status:   runtime.Status{State: runtime.StateRunning},
	}
```

In `scan`, insert the classify branch BEFORE the parse branch:

```go
		switch {
		case isErr:
			n.emit(runtime.EventStderr, line, 0)
		case n.classify != nil:
			if evs, ok := n.classify(line); ok && len(evs) > 0 {
				for _, ce := range evs {
					n.emit(ce.Type, ce.Data, 0)
				}
			} else {
				n.emit(runtime.EventStdout, line, 0)
			}
		case n.parse != nil:
			if msg, ok := n.parse(line); ok {
				n.emit(runtime.EventMessage, msg, 0)
			} else {
				n.emit(runtime.EventStdout, line, 0)
			}
		default:
			n.emit(runtime.EventStdout, line, 0)
		}
```

- [ ] **Step 1.5: Run — expect PASS**

Run: `go test ./exec/native/ -run TestClassifyEmitsTypedEvents -race`
Expected: PASS. Then guard: `go test ./exec/native/ ./core/... -race` — all green.

- [ ] **Step 1.6: Commit**

```bash
git add core/runtime/runtime.go exec/native/native.go exec/native/native_test.go
git commit -m "feat(native): typed Classify path + tool/subagent event types (spec 030)"
```

---

### Task 2: The claude classifier (TDD)

**Files:**
- Create: `exec/native/claude.go`
- Create: `exec/native/claude_test.go`

- [ ] **Step 2.1: Write the failing tests** — create `exec/native/claude_test.go`:

```go
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
```

- [ ] **Step 2.2: Run — expect FAIL** (`classifyClaude` undefined).

Run: `go test ./exec/native/ -run TestClassifyClaude`
Expected: compile error.

- [ ] **Step 2.3: Implement** — create `exec/native/claude.go`:

```go
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

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > summaryMax {
		return s[:summaryMax-1] + "…"
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
```

Note: `truncate` slices bytes; a multibyte rune at the boundary could split. Keep it simple but safe — if the byte-slice is invalid UTF-8 at the cut, trim back: replace the body of the `if` with:

```go
		cut := s[:summaryMax-1]
		for len(cut) > 0 && !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}
		return cut + "…"
```

and import `"unicode/utf8"`.

- [ ] **Step 2.4: Run — expect PASS**

Run: `go test ./exec/native/ -run TestClassifyClaude -race -v`
Expected: PASS (all table cases incl. the result-line and partial regression guards).

- [ ] **Step 2.5: Commit**

```bash
git add exec/native/claude.go exec/native/claude_test.go
git commit -m "feat(native): claude stream-json classifier -> tool/subagent events (spec 030)"
```

---

### Task 3: Wire the provider + end-to-end + docs

**Files:**
- Modify: `exec/native/providers.go` (claudeProvider ~line 18)
- Modify: `exec/native/claude_test.go` (append e2e test)
- Modify: `docs/notes/runtime-recon.md`

- [ ] **Step 3.1: Write the failing e2e test** — append to `exec/native/claude_test.go`:

```go
// TestClaudeProviderClassifiesEndToEnd drives a canned claude-shaped stream
// through Dispatch using the real claude provider's Classify (argv swapped for
// a script), proving the wire-up: typed events out, result line kept on stdout.
func TestClaudeProviderClassifiesEndToEnd(t *testing.T) {
	lines := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Task","input":{"description":"probe","subagent_type":"explore"}}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}
{"type":"result","subtype":"success","result":"final"}`
	p := claudeProvider()
	p.Command = func(_ runtime.RunSpec) []string {
		return []string{"sh", "-c", "cat <<'EOF'\n" + lines + "\nEOF"}
	}
	rt := New(t.TempDir(), p)
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "e2e", Agent: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	var subs, msgs int
	var resultOnStdout bool
	for ev := range run.Stream() {
		switch ev.Type {
		case runtime.EventSubagent:
			subs++
		case runtime.EventMessage:
			msgs++
		case runtime.EventStdout:
			if strings.Contains(ev.Data, `"type":"result"`) {
				resultOnStdout = true
			}
		}
	}
	if subs != 1 || msgs != 1 || !resultOnStdout {
		t.Fatalf("subs=%d msgs=%d resultOnStdout=%v", subs, msgs, resultOnStdout)
	}
}
```

- [ ] **Step 3.2: Run — expect FAIL** (claudeProvider has no Classify; the subagent line falls through jsonTextParser — the Task line contains `"description"` but not a top-level `"text":`… it DOES contain `"text"`? No: the Task line has no `text`/`message`/`content` string value at top… it has `"content":[…]` non-string, so jsonTextParser returns false → stdout → subs==0 → FAIL).

Run: `go test ./exec/native/ -run TestClaudeProviderClassifiesEndToEnd`
Expected: FAIL — `subs=0`.

- [ ] **Step 3.3: Wire it** — in `exec/native/providers.go`, `claudeProvider()` becomes:

```go
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
```

- [ ] **Step 3.4: Run — expect PASS** + full package + regression sweep

Run: `go test ./exec/native/ -race && go test ./control/ ./core/... ./ui/`
Expected: all green — especially `./control/` (the 026 planner tests prove the result-line contract still holds at the consumer).

- [ ] **Step 3.5: Docs** — in `docs/notes/runtime-recon.md`, under the claude section, add:

```
Event map (spec 030): complete assistant lines -> message (text blocks, joined)
+ tool per tool_use block (subagent when the tool is Task, data
{"description","agent"}; tool data {"name","summary"}). user tool_result lines
-> tool {"name":"tool_result","summary":<preview>}. The terminal result line,
stream_event partials, and system lines stay raw stdout (the planner reads the
result line from stdout — spec 026).
```

- [ ] **Step 3.6: Commit**

```bash
git add exec/native/providers.go exec/native/claude_test.go docs/notes/runtime-recon.md
git commit -m "feat(native): claude provider emits typed tool/subagent events (spec 030)"
```

---

## Self-review

**Spec coverage:** EventTool/EventSubagent constants (T1); classifier for tool_use/Task/tool_result/text (T2); result-line + partial + system + malformed → stdout with explicit regression tests (T2) and a consumer-level sweep of `./control/` (T3.4); claude-only wiring, other providers untouched (T3.3 touches only claudeProvider); no-doubling via complete-lines-only (T2 partial case); total parser — malformed JSON falls through, never panics (T2 malformed case); determinism — classifier is pure parsing, no Dispatch/router usage (nothing new imported); docs event map (T3.5). UI rendering: correctly out of scope (031/027).
**Placeholder scan:** none — complete code in every step.
**Type consistency:** `Classified{Type,Data}` used identically in T1 plumbing, T2 classifier, and tests; `classifyClaude` name matches T2 definition and T3 wiring; `summaryMax`/`truncate`/`toolJSON` internal to claude.go.
