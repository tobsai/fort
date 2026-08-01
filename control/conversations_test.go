package control

import (
	"context"
	"strings"
	"testing"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/cluster"
	"github.com/tobsai/fort/exec/fake"
)

type fixedConversationSeats []conversation.Seat

func (s fixedConversationSeats) ConversationSeats() []conversation.Seat {
	return append([]conversation.Seat(nil), s...)
}

type fixedCapabilitySnapshot struct{}

func (fixedCapabilitySnapshot) Capabilities() (corecap.Snapshot, uint64) {
	return corecap.Snapshot{Machines: []corecap.MachineInventory{{
		Name: "studio", Reachable: true,
		Profiles: []corecap.ProfileOffer{{ID: "codex:gpt-5.6-sol", Agent: "codex", State: corecap.OfferReady}},
	}}}, 1
}

func TestConversationServiceFansOneFrozenTurnAcrossExactSeats(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	seats := fixedConversationSeats{
		{ID: "codex-5@studio", Profile: "codex-5", Agent: "codex", Model: "gpt-5", Machine: "studio", DisplayName: "Codex on Studio", State: "ready"},
		{ID: "hermes@mini", Profile: "hermes", Agent: "hermes", Machine: "mini", DisplayName: "Hermes on Mini", State: "ready"},
	}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Shared thread", []string{"codex-5@studio", "hermes@mini"})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	result, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-1", "Compare your answers.", []string{detail.Participants[0].ID, detail.Participants[1].ID})
	if err != nil {
		t.Fatalf("post turn: %v", err)
	}
	if len(result.Targets) != 2 {
		t.Fatalf("targets = %+v", result.Targets)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, getErr := service.GetConversation(context.Background(), detail.Conversation.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if len(got.Targets) == 2 && got.Targets[0].State == conversation.TargetAnswered && got.Targets[1].State == conversation.TargetAnswered {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Targets[0].State != conversation.TargetAnswered || got.Targets[1].State != conversation.TargetAnswered {
		t.Fatalf("target states = %+v", got.Targets)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("messages = %+v, want one human and two attributed replies", got.Messages)
	}

	specs := rt.Dispatched()
	if len(specs) != 2 {
		t.Fatalf("dispatched = %+v", specs)
	}
	for _, spec := range specs {
		if !strings.Contains(spec.Prompt, `"conversation_id":"`+detail.Conversation.ID+`"`) || !strings.Contains(spec.Prompt, `"body":"Compare your answers."`) {
			t.Fatalf("target did not receive the shared snapshot: %s", spec.Prompt)
		}
	}
	byMachine := map[string]bool{}
	for _, spec := range specs {
		byMachine[spec.Machine] = true
		if spec.Profile == "" || spec.Agent == "" {
			t.Fatalf("dispatch lost exact seat identity: %+v", spec)
		}
	}
	if !byMachine["studio"] || !byMachine["mini"] {
		t.Fatalf("machines = %+v, want studio and mini", byMachine)
	}
}

func TestConversationServiceRejectsUnreadyTargetBeforePersistingTurn(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	seats := fixedConversationSeats{{ID: "codex@mini", Profile: "codex", Agent: "codex", Machine: "mini", DisplayName: "Codex on Mini", State: "ready"}}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Thread", []string{"codex@mini"})
	if err != nil {
		t.Fatal(err)
	}
	seats[0].State = "unavailable"
	if _, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-1", "hello", []string{detail.Participants[0].ID}); err == nil {
		t.Fatal("expected unavailable seat error")
	}
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 0 || len(got.Turns) != 0 || len(rt.Dispatched()) != 0 {
		t.Fatalf("rejected turn left work behind: detail=%+v dispatched=%+v", got, rt.Dispatched())
	}
}

func TestCapabilityConversationSeatsAreProfileMachinePairs(t *testing.T) {
	source := SnapshotConversationSeats{Source: fixedCapabilitySnapshot{}}
	seats := source.ConversationSeats()
	if len(seats) != 1 {
		t.Fatalf("seats = %+v", seats)
	}
	if seats[0].ID != "codex:gpt-5.6-sol@studio" || seats[0].Model != "gpt-5.6-sol" || seats[0].State != "ready" {
		t.Fatalf("seat = %+v", seats[0])
	}
}

func TestConversationRetryUsesOriginalTurnBoundary(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	rt.ExitCode = 2
	seats := fixedConversationSeats{{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", State: "ready"}}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Thread", []string{"codex@studio"})
	if err != nil {
		t.Fatal(err)
	}
	posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-1", "original", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, posted.Targets[0].ID, conversation.TargetFailed)

	rt.ExitCode = 0
	retried, err := service.RetryTarget(context.Background(), posted.Targets[0].ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, retried.ID, conversation.TargetAnswered)
	specs := rt.Dispatched()
	if len(specs) != 2 || specs[0].Prompt != specs[1].Prompt {
		t.Fatalf("retry changed frozen prompt: %+v", specs)
	}
}

func waitForConversationTargetState(t *testing.T, service *ConversationService, conversationID, targetID string, want conversation.TargetState) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := service.GetConversation(context.Background(), conversationID)
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range detail.Targets {
			if target.ID == targetID && target.State == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("target %s did not reach %s", targetID, want)
}

func TestConversationClientTurnRetryIsIdempotentAtServiceBoundary(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	service := NewConversationService(st, rt, fixedConversationSeats{{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready"}}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Thread", []string{"codex@studio"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.PostTurn(context.Background(), detail.Conversation.ID, "same-client-id", "hello", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, first.Targets[0].ID, conversation.TargetAnswered)
	second, err := service.PostTurn(context.Background(), detail.Conversation.ID, "same-client-id", "hello", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if second.Turn.ID != first.Turn.ID || len(rt.Dispatched()) != 1 {
		t.Fatalf("duplicate turn dispatched: first=%+v second=%+v specs=%+v", first, second, rt.Dispatched())
	}
}

func TestConversationClusterCombinesLocalAndRemoteAnswers(t *testing.T) {
	st := newStore(t)
	local := fake.New()
	remote := fake.New()
	rt := cluster.New("studio", local, map[string]runtime.Runtime{"mini": remote})
	seats := fixedConversationSeats{
		{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", State: "ready"},
		{ID: "hermes@mini", Profile: "hermes", Agent: "hermes", Machine: "mini", DisplayName: "Hermes on Mini", State: "ready"},
	}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Two computers", []string{"codex@studio", "hermes@mini"})
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-two", "answer together", []string{detail.Participants[0].ID, detail.Participants[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range turn.Targets {
		waitForConversationTargetState(t, service, detail.Conversation.ID, target.ID, conversation.TargetAnswered)
	}
	got, _ := service.GetConversation(context.Background(), detail.Conversation.ID)
	if len(got.Messages) != 3 || len(local.Dispatched()) != 1 || len(remote.Dispatched()) != 1 {
		t.Fatalf("detail=%+v local=%+v remote=%+v", got, local.Dispatched(), remote.Dispatched())
	}
	if got.Messages[1].AuthorID == got.Messages[2].AuthorID {
		t.Fatalf("answers lost participant attribution: %+v", got.Messages)
	}
}

func TestCancelingOneConversationTargetLeavesPeerWorking(t *testing.T) {
	st := newStore(t)
	local, remote := fake.New(), fake.New()
	local.Block, remote.Block = true, true
	rt := cluster.New("studio", local, map[string]runtime.Runtime{"mini": remote})
	service := NewConversationService(st, rt, fixedConversationSeats{
		{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready"},
		{ID: "hermes@mini", Profile: "hermes", Agent: "hermes", Machine: "mini", State: "ready"},
	}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Cancelable", []string{"codex@studio", "hermes@mini"})
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-cancel", "keep one going", []string{detail.Participants[0].ID, detail.Participants[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range turn.Targets {
		waitForConversationTargetState(t, service, detail.Conversation.ID, target.ID, conversation.TargetWorking)
	}
	if err := service.CancelTarget(context.Background(), turn.Targets[0].ID); err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, turn.Targets[0].ID, conversation.TargetCanceled)
	got, _ := service.GetConversation(context.Background(), detail.Conversation.ID)
	var peer conversation.Target
	for _, target := range got.Targets {
		if target.ID == turn.Targets[1].ID {
			peer = target
			break
		}
	}
	if peer.ID == "" || peer.State != conversation.TargetWorking || !service.ConversationTargetActive(turn.Targets[1].ID) {
		t.Fatalf("peer stopped with canceled target: %+v", got.Targets)
	}
	if err := service.CancelTarget(context.Background(), turn.Targets[1].ID); err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, turn.Targets[1].ID, conversation.TargetCanceled)
}
