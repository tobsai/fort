package store

import (
	"database/sql"
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
	if codexAt, hermesAt := strings.Index(prompt, `"participant_id":"pc"`), strings.Index(prompt, `"participant_id":"ph"`); codexAt < 0 || hermesAt < 0 || codexAt > hermesAt {
		t.Fatalf("compiled prompt omitted canonically ordered participant identities: %s", prompt)
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

func TestConversationTurnRejectsRemovedParticipantWithBoundedCode(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	conv := conversation.Conversation{ID: "c1", Title: "Thread", CreatedAt: now, UpdatedAt: now}
	participant := conversation.Participant{
		ID: "p1", ConversationID: conv.ID, SeatID: "codex@local", Profile: "codex",
		Agent: "codex", Machine: "local", State: conversation.ParticipantActive, CreatedAt: now,
	}
	if err := s.CreateConversation(conv, []conversation.Participant{participant}); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveConversationParticipant(conv.ID, participant.ID, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn-removed", ClientTurnID: "client-removed", ConversationID: conv.ID,
		HumanID: "human", Body: "should remain atomic",
		Targets:   []ConversationTurnTarget{{ID: "target-removed", ParticipantID: participant.ID, RunID: "run-removed"}},
		CreatedAt: now.Add(2 * time.Second),
	})
	var bounded *conversation.BoundedError
	if !errors.As(err, &bounded) || bounded.Code != conversation.ErrorParticipantRemoved {
		t.Fatalf("error = %v, want bounded %q", err, conversation.ErrorParticipantRemoved)
	}
	detail, detailErr := s.GetConversation(conv.ID)
	if detailErr != nil {
		t.Fatal(detailErr)
	}
	if len(detail.Messages) != 0 || len(detail.Turns) != 0 || len(detail.Targets) != 0 {
		t.Fatalf("removed-participant rejection left partial rows: %+v", detail)
	}
}

func TestDeletingProjectKeepsItsConversationsUnfiled(t *testing.T) {
	s := openTemp(t)
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	if err := s.CreateProject(conversation.Project{ID: "p1", Name: "Fort", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateConversation(conversation.Conversation{ID: "c1", ProjectID: "p1", Title: "Thread", CreatedAt: now, UpdatedAt: now}, nil); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetConversation("c1")
	if err != nil {
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
	if !detail.Conversation.UpdatedAt.Equal(before.Conversation.UpdatedAt) {
		t.Fatalf("project deletion changed conversation activity from %s to %s", before.Conversation.UpdatedAt, detail.Conversation.UpdatedAt)
	}
}

func TestDeleteConversationRejectsQueuedAndWorkingTargetsAtomically(t *testing.T) {
	for _, state := range []conversation.TargetState{conversation.TargetQueued, conversation.TargetWorking} {
		state := state
		t.Run(string(state), func(t *testing.T) {
			s := openTemp(t)
			now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
			conv := conversation.Conversation{ID: "c1", Title: "Active thread", CreatedAt: now, UpdatedAt: now}
			participant := conversation.Participant{
				ID: "p1", ConversationID: conv.ID, SeatID: "codex@studio", Profile: "codex",
				Agent: "codex", Machine: "studio", State: conversation.ParticipantActive, CreatedAt: now,
			}
			if err := s.CreateConversation(conv, []conversation.Participant{participant}); err != nil {
				t.Fatal(err)
			}
			_, targets, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
				TurnID: "turn-1", ConversationID: conv.ID, HumanID: "human", Body: "keep working",
				Targets: []ConversationTurnTarget{{ID: "target-1", ParticipantID: participant.ID, RunID: "run-1"}}, CreatedAt: now,
			})
			if err != nil {
				t.Fatal(err)
			}
			runStatus := "queued"
			if state == conversation.TargetWorking {
				changed, err := s.TransitionConversationTarget(targets[0].ID, conversation.TargetQueued, conversation.TargetWorking, "")
				if err != nil || !changed {
					t.Fatalf("mark working: changed=%v err=%v", changed, err)
				}
				runStatus = "running"
			}
			if err := s.CreateRun(Run{ID: "run-1", Title: conv.Title, Agent: participant.Agent, Profile: participant.Profile, Machine: participant.Machine, Status: runStatus, CreatedAt: now}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.AppendEvent(Event{RunID: "run-1", Type: "started", Data: "preserve me", CreatedAt: now}); err != nil {
				t.Fatal(err)
			}

			if err := s.DeleteConversation(conv.ID); err == nil || !strings.Contains(err.Error(), "active") {
				t.Fatalf("delete %s conversation error = %v, want active conflict", state, err)
			}
			detail, err := s.GetConversation(conv.ID)
			if err != nil {
				t.Fatalf("conversation changed after rejected delete: %v", err)
			}
			if len(detail.Targets) != 1 || detail.Targets[0].State != state || len(detail.Messages) != 1 || len(detail.Turns) != 1 {
				t.Fatalf("conversation changed after rejected delete: %+v", detail)
			}
			run, err := s.GetRun("run-1")
			if err != nil || run.Status != runStatus {
				t.Fatalf("run after rejected delete = %+v err=%v", run, err)
			}
			events, err := s.Events("run-1")
			if err != nil || len(events) != 1 || events[0].Data != "preserve me" {
				t.Fatalf("events after rejected delete = %+v err=%v", events, err)
			}
		})
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

func TestAnswerConversationTargetFailsTerminallyWhenMessagePersistenceFails(t *testing.T) {
	s := openTemp(t)
	now := time.Now().UTC()
	conv := conversation.Conversation{ID: "c1", Title: "Thread", CreatedAt: now, UpdatedAt: now}
	participant := conversation.Participant{
		ID: "pc", ConversationID: conv.ID, SeatID: "codex@local", Profile: "codex:gpt",
		Agent: "codex", Model: "gpt", Machine: "local", Position: 0, CreatedAt: now,
	}
	if err := s.CreateConversation(conv, []conversation.Participant{participant}); err != nil {
		t.Fatal(err)
	}
	turn, targets, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn-1", ConversationID: conv.ID, HumanID: "toby", Body: "hello",
		Targets: []ConversationTurnTarget{{ID: "target-1", ParticipantID: participant.ID, RunID: "run-1"}}, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := s.TransitionConversationTarget(targets[0].ID, conversation.TargetQueued, conversation.TargetWorking, ""); err != nil || !changed {
		t.Fatalf("mark target working: changed=%v err=%v", changed, err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_assistant_message BEFORE INSERT ON conversation_message
		WHEN NEW.author_kind='agent' BEGIN SELECT RAISE(ABORT, 'injected assistant persistence failure'); END`); err != nil {
		t.Fatal(err)
	}

	changed, err := s.AnswerConversationTarget(targets[0].ID, conversation.Message{
		ConversationID: conv.ID, TurnID: turn.ID, TargetID: targets[0].ID,
		AuthorKind: conversation.AuthorAssistant, AuthorID: participant.ID, Body: "reply", CreatedAt: now,
	})
	if err == nil || changed {
		t.Fatalf("answer persistence: changed=%v err=%v, want terminal failure", changed, err)
	}
	detail, err := s.GetConversation(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Messages) != 1 || detail.Messages[0].AuthorKind != conversation.AuthorHuman {
		t.Fatalf("failed answer left a partial assistant message: %+v", detail.Messages)
	}
	if len(detail.Targets) != 1 || detail.Targets[0].State != conversation.TargetFailed || detail.Targets[0].ErrorCode != "answer_persist_failed" {
		t.Fatalf("failed answer target = %+v, want terminal answer_persist_failed", detail.Targets)
	}
}

func TestAnswerConversationTargetFailsTerminallyWhenCommitFails(t *testing.T) {
	s := openTemp(t)
	now := time.Now().UTC()
	conv := conversation.Conversation{ID: "c1", Title: "Thread", CreatedAt: now, UpdatedAt: now}
	participant := conversation.Participant{
		ID: "pc", ConversationID: conv.ID, SeatID: "codex@local", Profile: "codex:gpt",
		Agent: "codex", Model: "gpt", Machine: "local", Position: 0, CreatedAt: now,
	}
	if err := s.CreateConversation(conv, []conversation.Participant{participant}); err != nil {
		t.Fatal(err)
	}
	turn, targets, _, err := s.CreateConversationTurn(CreateConversationTurnParams{
		TurnID: "turn-1", ConversationID: conv.ID, HumanID: "toby", Body: "hello",
		Targets: []ConversationTurnTarget{{ID: "target-1", ParticipantID: participant.ID, RunID: "run-1"}}, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := s.TransitionConversationTarget(targets[0].ID, conversation.TargetQueued, conversation.TargetWorking, ""); err != nil || !changed {
		t.Fatalf("mark target working: changed=%v err=%v", changed, err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_answer_commit AFTER INSERT ON conversation_message
		WHEN NEW.author_kind='agent' BEGIN
			UPDATE conversation_target SET participant_id='missing-participant' WHERE id=NEW.target_id;
		END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`PRAGMA defer_foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}

	changed, err := s.AnswerConversationTarget(targets[0].ID, conversation.Message{
		ConversationID: conv.ID, TurnID: turn.ID, TargetID: targets[0].ID,
		AuthorKind: conversation.AuthorAssistant, AuthorID: participant.ID, Body: "reply", CreatedAt: now,
	})
	if err == nil || changed {
		t.Fatalf("answer commit: changed=%v err=%v, want terminal failure", changed, err)
	}
	detail, err := s.GetConversation(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Messages) != 1 || detail.Messages[0].AuthorKind != conversation.AuthorHuman {
		t.Fatalf("failed answer commit left a partial assistant message: %+v", detail.Messages)
	}
	if len(detail.Targets) != 1 || detail.Targets[0].State != conversation.TargetFailed || detail.Targets[0].ErrorCode != "answer_persist_failed" {
		t.Fatalf("failed answer commit target = %+v, want terminal answer_persist_failed", detail.Targets)
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
	first, _, firstContext, err := s.CreateConversationTurn(params)
	if err != nil {
		t.Fatal(err)
	}
	lateParticipant := conversation.Participant{ID: "p2", ConversationID: conv.ID, SeatID: "claude@mini", Profile: "claude", Agent: "claude", Model: "opus", Machine: "mini", State: conversation.ParticipantActive, CreatedAt: now.Add(time.Minute)}
	if err := s.AddConversationParticipant(lateParticipant); err != nil {
		t.Fatal(err)
	}
	params.TurnID, params.Targets[0].ID, params.Targets[0].RunID = "turn-2", "target-2", "run-2"
	second, targets, secondContext, err := s.CreateConversationTurn(params)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || second.Created || second.ID != first.ID || len(targets) != 1 || targets[0].RunID != "run-1" {
		t.Fatalf("first=%+v second=%+v targets=%+v", first, second, targets)
	}
	if secondContext != firstContext || strings.Contains(secondContext, `"participant_id":"p2"`) {
		t.Fatalf("idempotent turn context changed:\nfirst:  %s\nsecond: %s", firstContext, secondContext)
	}
	retryContext, err := s.ConversationContext(conv.ID, first.ThroughMessageID)
	if err != nil {
		t.Fatal(err)
	}
	if retryContext != firstContext {
		t.Fatalf("retry context changed:\nfirst: %s\nretry: %s", firstContext, retryContext)
	}
	detail, _ := s.GetConversation(conv.ID)
	if len(detail.Messages) != 1 || len(detail.Turns) != 1 || len(detail.Targets) != 1 || len(detail.Participants) != 2 {
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

func TestProjectNamesAreUnicodeCaseInsensitiveOnCreateAndRename(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		s := openTemp(t)
		now := time.Now().UTC()
		if err := s.CreateProject(conversation.Project{ID: "p1", Name: "Ä", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateProject(conversation.Project{ID: "p2", Name: "ä", CreatedAt: now, UpdatedAt: now}); err == nil {
			t.Fatal("Unicode case-insensitive duplicate project accepted")
		}
	})

	t.Run("rename", func(t *testing.T) {
		s := openTemp(t)
		now := time.Now().UTC()
		if err := s.CreateProject(conversation.Project{ID: "p1", Name: "Ä", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateProject(conversation.Project{ID: "p2", Name: "Other", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := s.RenameProject("p2", "ä"); err == nil {
			t.Fatal("Unicode case-insensitive duplicate project rename accepted")
		}
		projects, err := s.ListProjects()
		if err != nil {
			t.Fatal(err)
		}
		for _, project := range projects {
			if project.ID == "p2" && project.Name != "Other" {
				t.Fatalf("failed rename changed project name to %q", project.Name)
			}
		}
	})

	t.Run("missing rename prefers not found", func(t *testing.T) {
		s := openTemp(t)
		now := time.Now().UTC()
		if err := s.CreateProject(conversation.Project{ID: "p1", Name: "Occupied", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := s.RenameProject("missing", "occupied"); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("rename error = %v, want sql.ErrNoRows before name conflict", err)
		}
	})
}

func TestConversationAndProjectOrderingUsesChronologicalRFC3339NanoValues(t *testing.T) {
	s := openTemp(t)
	exactSecond := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	laterFraction := exactSecond.Add(100 * time.Millisecond)
	created := exactSecond.Add(-time.Hour)

	for _, project := range []conversation.Project{
		{ID: "p-exact", Name: "Exact second", CreatedAt: created, UpdatedAt: created},
		{ID: "p-later", Name: "Later fraction", CreatedAt: created, UpdatedAt: created},
	} {
		if err := s.CreateProject(project); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []conversation.Conversation{
		{ID: "c-exact", ProjectID: "p-exact", Title: "Exact second", CreatedAt: created, UpdatedAt: created},
		{ID: "c-later", ProjectID: "p-later", Title: "Later fraction", CreatedAt: created, UpdatedAt: created},
	} {
		if err := s.CreateConversation(item, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, message := range []conversation.Message{
		{ConversationID: "c-exact", AuthorKind: conversation.AuthorHuman, AuthorID: "human", Body: "older", CreatedAt: exactSecond},
		{ConversationID: "c-later", AuthorKind: conversation.AuthorHuman, AuthorID: "human", Body: "newer", CreatedAt: laterFraction},
	} {
		if _, err := s.AppendConversationMessage(message); err != nil {
			t.Fatal(err)
		}
	}

	items, err := s.ListConversations("")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "c-later" || items[1].ID != "c-exact" {
		t.Fatalf("conversation order = %+v, want later fractional activity first", items)
	}
	projects, err := s.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0].ID != "p-later" || projects[1].ID != "p-exact" {
		t.Fatalf("project order = %+v, want later fractional activity first", projects)
	}
}

func TestConversationStateChangeDoesNotReorderMessageActivity(t *testing.T) {
	s := openTemp(t)
	base := time.Now().UTC().Add(-time.Hour)
	for _, project := range []conversation.Project{
		{ID: "p-old", Name: "Old project", CreatedAt: base, UpdatedAt: base},
		{ID: "p-new", Name: "New project", CreatedAt: base.Add(time.Second), UpdatedAt: base.Add(time.Second)},
	} {
		if err := s.CreateProject(project); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []conversation.Conversation{
		{ID: "c-old", ProjectID: "p-old", Title: "Old activity", State: conversation.ConversationOpen, CreatedAt: base, UpdatedAt: base},
		{ID: "c-new", ProjectID: "p-new", Title: "New activity", State: conversation.ConversationOpen, CreatedAt: base.Add(time.Second), UpdatedAt: base.Add(time.Second)},
	} {
		if err := s.CreateConversation(item, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, message := range []conversation.Message{
		{ConversationID: "c-old", AuthorKind: conversation.AuthorHuman, AuthorID: "human", Body: "older", CreatedAt: base.Add(10 * time.Minute)},
		{ConversationID: "c-new", AuthorKind: conversation.AuthorHuman, AuthorID: "human", Body: "newer", CreatedAt: base.Add(20 * time.Minute)},
	} {
		if _, err := s.AppendConversationMessage(message); err != nil {
			t.Fatal(err)
		}
	}

	before, err := s.GetConversation("c-old")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetConversationState("c-old", conversation.ConversationArchived); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetConversation("c-old")
	if err != nil {
		t.Fatal(err)
	}
	if !after.Conversation.UpdatedAt.After(before.Conversation.UpdatedAt) {
		t.Fatalf("state change did not advance updated_at: before=%s after=%s", before.Conversation.UpdatedAt, after.Conversation.UpdatedAt)
	}

	items, err := s.ListConversations("")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "c-new" || items[1].ID != "c-old" {
		t.Fatalf("state change reordered conversations by metadata activity: %+v", items)
	}
	projects, err := s.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0].ID != "p-new" || projects[1].ID != "p-old" {
		t.Fatalf("state change reordered projects by metadata activity: %+v", projects)
	}
}
