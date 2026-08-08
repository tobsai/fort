package conversation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCompileContextIsCanonicalAndStopsAtBoundary(t *testing.T) {
	participants := []Participant{
		{ID: "seat-codex", Profile: "codex-sol", Agent: "codex", Model: "gpt-5.6-sol", Machine: "studio", DisplayName: "Codex on Studio", Position: 1},
		{ID: "seat-claude", Profile: "claude-opus", Agent: "claude", Model: "claude-opus-4.1", Machine: "mini", DisplayName: "Claude on Mini", Position: 0, State: ParticipantRemoved},
	}
	messages := []Message{
		{ID: 3, AuthorKind: AuthorAssistant, AuthorID: "seat-codex", Body: "after", CreatedAt: time.Unix(3, 0)},
		{ID: 1, AuthorKind: AuthorHuman, AuthorID: "toby", Body: "first", CreatedAt: time.Unix(1, 0)},
		{ID: 2, AuthorKind: AuthorAssistant, AuthorID: "seat-claude", Body: "second", CreatedAt: time.Unix(2, 0)},
	}

	first, err := CompileContext("conversation-1", 2, participants, messages)
	if err != nil {
		t.Fatalf("compile context: %v", err)
	}
	second, err := CompileContext("conversation-1", 2, []Participant{participants[1], participants[0]}, []Message{messages[1], messages[2], messages[0]})
	if err != nil {
		t.Fatalf("compile reordered context: %v", err)
	}
	if first != second {
		t.Fatalf("context changed with input order:\nfirst:  %s\nsecond: %s", first, second)
	}
	if strings.Contains(first, "after") {
		t.Fatalf("context crossed the frozen boundary: %s", first)
	}
	claudeIdentity := `{"participant_id":"seat-claude","profile":"claude-opus","agent":"claude","model":"claude-opus-4.1","machine":"mini","display_name":"Claude on Mini"}`
	codexIdentity := `{"participant_id":"seat-codex","profile":"codex-sol","agent":"codex","model":"gpt-5.6-sol","machine":"studio","display_name":"Codex on Studio"}`
	if !strings.Contains(first, `"through_message_id":2`) || !strings.Contains(first, `"body":"first"`) || !strings.Contains(first, `"body":"second"`) {
		t.Fatalf("context omitted required history: %s", first)
	}
	if claudeAt, codexAt := strings.Index(first, claudeIdentity), strings.Index(first, codexIdentity); claudeAt < 0 || codexAt < 0 || claudeAt > codexAt {
		t.Fatalf("context identities are missing or non-canonical: %s", first)
	}
}

func TestCompileContextRejectsOversizeWithoutTruncation(t *testing.T) {
	_, err := CompileContext("conversation-1", 1, nil, []Message{{
		ID: 1, AuthorKind: AuthorHuman, AuthorID: "toby", Body: strings.Repeat("x", MaxContextBytes),
	}})
	if !errors.Is(err, ErrContextTooLarge) {
		t.Fatalf("error = %v, want ErrContextTooLarge", err)
	}
}

func TestTargetStateTransitionsAreExplicit(t *testing.T) {
	tests := []struct {
		from, to TargetState
		want     bool
	}{
		{TargetQueued, TargetWorking, true},
		{TargetQueued, TargetCanceled, true},
		{TargetWorking, TargetAnswered, true},
		{TargetWorking, TargetFailed, true},
		{TargetWorking, TargetCanceled, true},
		{TargetQueued, TargetAnswered, false},
		{TargetAnswered, TargetWorking, false},
	}
	for _, tt := range tests {
		if got := CanTransition(tt.from, tt.to); got != tt.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCompileParticipantPromptIncludesExactSeatAndSharedContext(t *testing.T) {
	participant := Participant{ID: "p1", Profile: "codex:gpt", Agent: "codex", Model: "gpt", Machine: "studio", DisplayName: "Codex on Studio"}
	contextJSON, err := CompileContext("c1", 1, []Participant{participant}, []Message{{ID: 1, AuthorKind: AuthorHuman, AuthorID: "human", Body: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := CompileParticipantPrompt(contextJSON, participant)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{contextJSON, `"participant_id":"p1"`, `"profile":"codex:gpt"`, `"machine":"studio"`, "Answer as this exact participant"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}

func TestProjectNameValidation(t *testing.T) {
	if got, err := ValidateProjectName("  Fort  "); err != nil || got != "Fort" {
		t.Fatalf("validated name = %q, %v", got, err)
	}
	if _, err := ValidateProjectName(""); err == nil {
		t.Fatal("blank name accepted")
	}
	if _, err := ValidateProjectName(strings.Repeat("x", 121)); err == nil {
		t.Fatal("overlong name accepted")
	}
}
