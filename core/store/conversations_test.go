package store

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

func TestConversationTurnPersistsFrozenTargetsAtomically(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC)
	project := conversation.Project{ID: "p1", Name: "Fort", CreatedAt: now, UpdatedAt: now}
	if err := s.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	conv := conversation.Conversation{ID: "c1", ProjectID: project.ID, Title: "Shared thread", CreatedAt: now, UpdatedAt: now}
	participants := []conversation.Participant{
		{ID: "pc", ConversationID: conv.ID, SeatID: "codex@studio", Profile: "codex-5", Agent: "codex", Model: "gpt-5", Machine: "studio", DisplayName: "Codex on Studio", Position: 0, CreatedAt: now},
		{ID: "ph", ConversationID: conv.ID, SeatID: "hermes@mini", Profile: "hermes", Agent: "hermes", Machine: "mini", DisplayName: "Hermes on Mini", Position: 1, CreatedAt: now},
	}
	if err := s.CreateConversation(conv, participants); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	turn, targets, prompt, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn-1", ConversationID: conv.ID, HumanID: "toby", Body: "Compare your answers.",
		Targets:   []ConversationTurnTarget{{ID: "target-c", ParticipantID: "pc", RunID: "run-c"}, {ID: "target-h", ParticipantID: "ph", RunID: "run-h"}},
		CreatedAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("create turn: %v", err)
	}
	if turn.ThroughMessageID != turn.PromptMessageID || turn.PromptMessageID == 0 {
		t.Fatalf("turn boundary = %+v, want prompt message boundary", turn)
	}
	if len(targets) != 2 || targets[0].State != conversation.TargetQueued || targets[1].State != conversation.TargetQueued {
		t.Fatalf("targets = %+v, want two queued targets", targets)
	}
	if !strings.Contains(prompt, `"body":"Compare your answers."`) {
		t.Fatalf("compiled prompt omitted message: %s", prompt)
	}

	detail, err := s.GetConversation(conv.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if len(detail.Participants) != 2 || len(detail.Messages) != 1 || len(detail.Turns) != 1 || len(detail.Targets) != 2 {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.Participants[1].Machine != "mini" || detail.Participants[1].Agent != "hermes" {
		t.Fatalf("participant binding moved: %+v", detail.Participants[1])
	}
}

func TestConversationTurnRejectsOversizeWithoutPartialRows(t *testing.T) {
	s := openTemp(t)
	now := time.Now().UTC()
	conv := conversation.Conversation{ID: "c1", Title: "Thread", CreatedAt: now, UpdatedAt: now}
	p := conversation.Participant{ID: "pc", ConversationID: conv.ID, SeatID: "codex@local", Profile: "codex", Agent: "codex", Machine: "local", Position: 0, CreatedAt: now}
	if err := s.CreateConversation(conv, []conversation.Participant{p}); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn-too-large", ConversationID: conv.ID, HumanID: "toby", Body: strings.Repeat("x", conversation.MaxContextBytes),
		Targets: []ConversationTurnTarget{{ID: "target-1", ParticipantID: p.ID, RunID: "run-1"}}, CreatedAt: now,
	})
	if !errors.Is(err, conversation.ErrContextTooLarge) {
		t.Fatalf("error = %v, want ErrContextTooLarge", err)
	}
	detail, err := s.GetConversation(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Messages) != 0 || len(detail.Turns) != 0 || len(detail.Targets) != 0 {
		t.Fatalf("oversize turn left partial rows: %+v", detail)
	}
}

func TestDeletingProjectKeepsItsConversationsUnfiled(t *testing.T) {
	s := openTemp(t)
	now := time.Now().UTC()
	if err := s.CreateProject(conversation.Project{ID: "p1", Name: "Fort", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateConversation(conversation.Conversation{ID: "c1", ProjectID: "p1", Title: "Thread", CreatedAt: now, UpdatedAt: now}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProject("p1"); err != nil {
		t.Fatal(err)
	}
	detail, err := s.GetConversation("c1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Conversation.ProjectID != "" {
		t.Fatalf("project id = %q, want unfiled", detail.Conversation.ProjectID)
	}
}

func TestConversationTargetStateUpdateIsCompareAndSwap(t *testing.T) {
	s := openTemp(t)
	now := time.Now().UTC()
	conv := conversation.Conversation{ID: "c1", Title: "Thread", CreatedAt: now, UpdatedAt: now}
	p := conversation.Participant{ID: "pc", ConversationID: conv.ID, SeatID: "codex@local", Profile: "codex", Agent: "codex", Machine: "local", Position: 0, CreatedAt: now}
	if err := s.CreateConversation(conv, []conversation.Participant{p}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.CreateConversationTurn(CreateConversationTurnParams{TurnID: "turn-1", ConversationID: conv.ID, HumanID: "toby", Body: "hello", Targets: []ConversationTurnTarget{{ID: "target-1", ParticipantID: p.ID, RunID: "run-1"}}, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	changed, err := s.TransitionConversationTarget("target-1", conversation.TargetQueued, conversation.TargetWorking, "")
	if err != nil || !changed {
		t.Fatalf("queued -> working: changed=%v err=%v", changed, err)
	}
	changed, err = s.TransitionConversationTarget("target-1", conversation.TargetQueued, conversation.TargetFailed, "late failure")
	if err != nil || changed {
		t.Fatalf("stale transition: changed=%v err=%v", changed, err)
	}
}

func TestAnswerConversationTargetPersistsStateAndMessageTogether(t *testing.T) {
	s := openTemp(t)
	now := time.Now().UTC()
	conv := conversation.Conversation{ID: "c1", Title: "Thread", CreatedAt: now, UpdatedAt: now}
	p := conversation.Participant{ID: "pc", ConversationID: conv.ID, SeatID: "codex@local", Profile: "codex", Agent: "codex", Machine: "local", Position: 0, CreatedAt: now}
	if err := s.CreateConversation(conv, []conversation.Participant{p}); err != nil {
		t.Fatal(err)
	}
	turn, targets, _, err := s.CreateConversationTurn(CreateConversationTurnParams{TurnID: "turn-1", ConversationID: conv.ID, HumanID: "toby", Body: "hello", Targets: []ConversationTurnTarget{{ID: "target-1", ParticipantID: p.ID, RunID: "run-1"}}, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TransitionConversationTarget(targets[0].ID, conversation.TargetQueued, conversation.TargetWorking, ""); err != nil {
		t.Fatal(err)
	}
	changed, err := s.AnswerConversationTarget(targets[0].ID, conversation.Message{ConversationID: conv.ID, TurnID: turn.ID, TargetID: targets[0].ID, AuthorKind: conversation.AuthorAssistant, AuthorID: p.ID, Body: "reply", CreatedAt: now})
	if err != nil || !changed {
		t.Fatalf("answer: changed=%v err=%v", changed, err)
	}
	detail, err := s.GetConversation(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Targets[0].State != conversation.TargetAnswered || len(detail.Messages) != 2 || detail.Messages[1].Body != "reply" {
		t.Fatalf("detail = %+v", detail)
	}
}

func TestFailInterruptedConversationTargetsClearsQueuedAndWorking(t *testing.T) {
	s := openTemp(t)
	now := time.Now().UTC()
	conv := conversation.Conversation{ID: "c1", Title: "Thread", CreatedAt: now, UpdatedAt: now}
	p := conversation.Participant{ID: "pc", ConversationID: conv.ID, SeatID: "codex@local", Profile: "codex", Agent: "codex", Machine: "local", Position: 0, CreatedAt: now}
	if err := s.CreateConversation(conv, []conversation.Participant{p}); err != nil {
		t.Fatal(err)
	}
	_, targets, _, err := s.CreateConversationTurn(CreateConversationTurnParams{TurnID: "turn-1", ConversationID: conv.ID, HumanID: "toby", Body: "hello", Targets: []ConversationTurnTarget{{ID: "target-1", ParticipantID: p.ID, RunID: "run-1"}}, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TransitionConversationTarget(targets[0].ID, conversation.TargetQueued, conversation.TargetWorking, ""); err != nil {
		t.Fatal(err)
	}
	count, err := s.FailInterruptedConversationTargets("daemon stopped")
	if err != nil || count != 1 {
		t.Fatalf("reconcile count=%d err=%v", count, err)
	}
	detail, _ := s.GetConversation(conv.ID)
	if detail.Targets[0].State != conversation.TargetFailed || detail.Targets[0].Error != "daemon stopped" {
		t.Fatalf("target = %+v", detail.Targets[0])
	}
}

func TestConversationClientTurnIDIsIdempotent(t *testing.T) {
	s := openTemp(t)
	now := time.Now().UTC()
	conv := conversation.Conversation{ID: "c1", Title: "Thread", State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now}
	p := conversation.Participant{ID: "p1", ConversationID: conv.ID, SeatID: "codex@local", Profile: "codex", Agent: "codex", Machine: "local", State: conversation.ParticipantActive, CreatedAt: now}
	if err := s.CreateConversation(conv, []conversation.Participant{p}); err != nil {
		t.Fatal(err)
	}
	params := CreateConversationTurnParams{TurnID: "turn-1", ClientTurnID: "client-1", ConversationID: conv.ID, HumanID: "human", Body: "hello", Targets: []ConversationTurnTarget{{ID: "target-1", ParticipantID: p.ID, RunID: "run-1"}}, CreatedAt: now}
	first, _, _, err := s.CreateConversationTurn(params)
	if err != nil {
		t.Fatal(err)
	}
	params.TurnID, params.Targets[0].ID, params.Targets[0].RunID = "turn-2", "target-2", "run-2"
	second, targets, _, err := s.CreateConversationTurn(params)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || second.Created || second.ID != first.ID || len(targets) != 1 || targets[0].RunID != "run-1" {
		t.Fatalf("first=%+v second=%+v targets=%+v", first, second, targets)
	}
	detail, _ := s.GetConversation(conv.ID)
	if len(detail.Messages) != 1 || len(detail.Turns) != 1 || len(detail.Targets) != 1 {
		t.Fatalf("duplicate client turn created rows: %+v", detail)
	}
}

func TestProjectNamesAreCaseInsensitiveAndMoveDoesNotReorderConversation(t *testing.T) {
	s := openTemp(t)
	now := time.Now().UTC()
	if err := s.CreateProject(conversation.Project{ID: "p1", Name: "Fort", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProject(conversation.Project{ID: "p2", Name: "fort", CreatedAt: now, UpdatedAt: now}); err == nil {
		t.Fatal("case-insensitive duplicate project accepted")
	}
	conv := conversation.Conversation{ID: "c1", Title: "Thread", State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateConversation(conv, nil); err != nil {
		t.Fatal(err)
	}
	before, _ := s.GetConversation(conv.ID)
	if err := s.MoveConversation(conv.ID, "p1"); err != nil {
		t.Fatal(err)
	}
	after, _ := s.GetConversation(conv.ID)
	if !after.Conversation.UpdatedAt.Equal(before.Conversation.UpdatedAt) {
		t.Fatalf("move changed activity from %s to %s", before.Conversation.UpdatedAt, after.Conversation.UpdatedAt)
	}
}
