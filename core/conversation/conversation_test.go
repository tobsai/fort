package conversation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCompileContextIsCanonicalAndStopsAtBoundary(t *testing.T) {
	messages := []Message{
		{ID: 3, AuthorKind: AuthorAssistant, AuthorID: "seat-codex", Body: "after", CreatedAt: time.Unix(3, 0)},
		{ID: 1, AuthorKind: AuthorHuman, AuthorID: "toby", Body: "first", CreatedAt: time.Unix(1, 0)},
		{ID: 2, AuthorKind: AuthorAssistant, AuthorID: "seat-claude", Body: "second", CreatedAt: time.Unix(2, 0)},
	}

	first, err := CompileContext("conversation-1", 2, messages)
	if err != nil {
		t.Fatalf("compile context: %v", err)
	}
	second, err := CompileContext("conversation-1", 2, []Message{messages[1], messages[2], messages[0]})
	if err != nil {
		t.Fatalf("compile reordered context: %v", err)
	}
	if first != second {
		t.Fatalf("context changed with input order:\nfirst:  %s\nsecond: %s", first, second)
	}
	if strings.Contains(first, "after") {
		t.Fatalf("context crossed the frozen boundary: %s", first)
	}
	if !strings.Contains(first, `"through_message_id":2`) || !strings.Contains(first, `"body":"first"`) || !strings.Contains(first, `"body":"second"`) {
		t.Fatalf("context omitted required history: %s", first)
	}
}

func TestCompileContextRejectsOversizeWithoutTruncation(t *testing.T) {
	_, err := CompileContext("conversation-1", 1, []Message{{
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
	contextJSON, err := CompileContext("c1", 1, []Message{{ID: 1, AuthorKind: AuthorHuman, AuthorID: "human", Body: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := CompileParticipantPrompt(contextJSON, Participant{ID: "p1", Profile: "codex:gpt", Agent: "codex", Model: "gpt", Machine: "studio", DisplayName: "Codex on Studio"})
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
