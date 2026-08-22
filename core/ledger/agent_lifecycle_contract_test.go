package ledger_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestSQLiteAgentLifecycleAppendsProfileAndAdvancesOnlyExactCurrentRevision(t *testing.T) {
	repository := openAgentProfileRepository(t)
	created := stableAgentCommand(t)
	if _, err := repository.CreateAgent(context.Background(), created); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	revision := created.Profile
	revision.ID = "profile:researcher:2"
	revision.Revision = 2
	revision.Name = "Principal Researcher"
	revision.CreatedAt = revision.CreatedAt.Add(time.Minute)
	command := ledger.AppendAgentProfileCommand{
		IdempotencyKey:            "profile-researcher-2",
		AccountID:                 created.Agent.AccountID,
		AgentID:                   created.Agent.ID,
		ExpectedProfileRevisionID: created.Profile.ID,
		Revision:                  revision,
		AcceptedBy:                "human:toby",
	}

	updated, err := repository.AppendAgentProfile(context.Background(), command)
	if err != nil {
		t.Fatalf("AppendAgentProfile: %v", err)
	}
	if updated.Agent.CurrentProfileRevisionID != revision.ID || updated.Profile != revision {
		t.Fatalf("current profile = %+v / %+v", updated.Agent, updated.Profile)
	}
	if updated.Agent.CurrentBehaviorRevisionID != created.Behavior.ID || updated.Agent.CurrentBindingRevisionID != created.Binding.ID {
		t.Fatalf("profile change advanced execution revisions: %+v", updated.Agent)
	}

	replayed, err := repository.AppendAgentProfile(context.Background(), command)
	if err != nil || replayed.Profile.ID != revision.ID {
		t.Fatalf("profile replay = %+v, %v", replayed, err)
	}

	stale := command
	stale.IdempotencyKey = "profile-stale"
	stale.Revision.ID = "profile:researcher:3"
	stale.Revision.Revision = 3
	stale.Revision.CreatedAt = stale.Revision.CreatedAt.Add(time.Minute)
	if _, err := repository.AppendAgentProfile(context.Background(), stale); !errors.Is(err, ledger.ErrRevisionConflict) {
		t.Fatalf("stale profile error = %v, want revision conflict", err)
	}
	stillCurrent, err := repository.GetAgent(context.Background(), created.Agent.AccountID, created.Agent.ID)
	if err != nil || stillCurrent.Profile.ID != revision.ID {
		t.Fatalf("stale command changed current profile: %+v, %v", stillCurrent.Profile, err)
	}
}

func TestSQLiteAgentLifecycleAcceptsBehaviorWithImmutableSuccessorBinding(t *testing.T) {
	repository := openAgentBehaviorRepository(t)
	created := stableAgentCommand(t)
	if _, err := repository.CreateAgent(context.Background(), created); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	acceptedAt := created.Binding.ActivatedAt.Add(2 * time.Minute)
	behavior := created.Behavior
	behavior.ID = "behavior:researcher:2"
	behavior.Revision = 2
	behavior.StandingInstructions = "Cite primary sources."
	behavior.CreatedAt = acceptedAt
	binding := created.Binding
	binding.ID = "binding:researcher:2"
	binding.Revision = 2
	binding.BehaviorRevisionID = behavior.ID
	binding.SupersedesRevisionID = created.Binding.ID
	binding.SeatID = "seat:researcher:2"
	binding.ActivatedAt = acceptedAt
	participant := created.Participant
	participant.ID = "participant:researcher:2"
	participant.SeatID = binding.SeatID
	participant.CreatedAt = acceptedAt
	command := ledger.AppendAgentBehaviorCommand{
		IdempotencyKey: "behavior-researcher-2", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ExpectedBehaviorRevisionID: created.Behavior.ID, ExpectedBindingRevisionID: created.Binding.ID,
		Behavior: behavior, Binding: binding, Participant: participant,
		ReadinessEvidence: []string{"ready:binding-2"}, AuthorityEvidence: []string{"authority:binding-2"},
		AcceptedBy: "human:toby", AcceptedAt: acceptedAt,
	}

	result, err := repository.AppendAgentBehavior(context.Background(), command)
	if err != nil {
		t.Fatalf("AppendAgentBehavior: %v", err)
	}
	if result.Agent.Behavior.ID != behavior.ID || result.Agent.Binding.ID != binding.ID ||
		result.Agent.Agent.CurrentBehaviorRevisionID != behavior.ID || result.Agent.Agent.CurrentBindingRevisionID != binding.ID {
		t.Fatalf("current behavior/binding = %+v", result.Agent)
	}
	if result.Transition.Kind != ledger.BindingTransitionBehavior ||
		result.Transition.PreviousBindingRevisionID != created.Binding.ID ||
		result.Transition.SuccessorBindingRevisionID != binding.ID || result.Transition.AcceptedBy != "human:toby" {
		t.Fatalf("Behavior transition evidence = %+v", result.Transition)
	}
	if result.Agent.Binding.ExecutionSourceID != created.Binding.ExecutionSourceID ||
		result.Agent.Binding.SourceAgentID != created.Binding.SourceAgentID ||
		result.Agent.Binding.SourceConfigDigest != created.Binding.SourceConfigDigest {
		t.Fatalf("Behavior change silently rebound execution: %+v", result.Agent.Binding)
	}
	if replay, err := repository.AppendAgentBehavior(context.Background(), command); err != nil || replay.Transition.SuccessorBindingRevisionID != binding.ID {
		t.Fatalf("Behavior replay = %+v, %v", replay, err)
	}

	stale := command
	stale.IdempotencyKey = "behavior-stale"
	stale.Behavior.ID = "behavior:researcher:3"
	stale.Behavior.Revision = 3
	stale.Behavior.CreatedAt = acceptedAt.Add(time.Minute)
	stale.Binding.ID = "binding:researcher:3"
	stale.Binding.Revision = 3
	stale.Binding.BehaviorRevisionID = stale.Behavior.ID
	stale.Binding.SeatID = "seat:researcher:3"
	stale.Binding.ActivatedAt = stale.Behavior.CreatedAt
	stale.Participant.ID = "participant:researcher:3"
	stale.Participant.SeatID = stale.Binding.SeatID
	stale.Participant.CreatedAt = stale.Behavior.CreatedAt
	stale.AcceptedAt = stale.Behavior.CreatedAt
	if _, err := repository.AppendAgentBehavior(context.Background(), stale); !errors.Is(err, ledger.ErrRevisionConflict) {
		t.Fatalf("stale Behavior error = %v, want revision conflict", err)
	}
}

func TestSQLiteAgentLifecycleRequiresPreviewBeforeExplicitRebind(t *testing.T) {
	repository := openAgentRebindRepository(t)
	created := stableAgentCommand(t)
	if _, err := repository.CreateAgent(context.Background(), created); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	proposal := rebindProposal(created)
	preview, err := repository.PreviewAgentRebind(context.Background(), proposal)
	if err != nil {
		t.Fatalf("PreviewAgentRebind: %v", err)
	}
	if preview.CurrentBinding.ID != created.Binding.ID || preview.ProposedBinding.ID != proposal.Binding.ID ||
		preview.CurrentSourceAgent.ID != created.SourceAgent.ID || preview.ProposedSourceAgent.ID != proposal.SourceAgent.ID ||
		preview.Digest == "" || len(preview.NonTransferableResources) != 3 {
		t.Fatalf("Rebind preview omitted old/new identity or carryover disclosure: %+v", preview)
	}

	tampered := preview
	tampered.ProposedBinding.RequestedModel = "silent-fallback"
	if _, err := repository.AcceptAgentRebind(context.Background(), ledger.AcceptAgentRebindCommand{
		IdempotencyKey: "rebind-tampered", Preview: tampered, AcceptedBy: "human:toby", AcceptedAt: proposal.Binding.ActivatedAt,
	}); err == nil {
		t.Fatal("AcceptAgentRebind accepted a preview changed after disclosure")
	}
	forged := preview
	forged.Participant.ConversationID = "conversation:foreign"
	forged.Digest, err = forged.CalculateDigest()
	if err != nil {
		t.Fatalf("calculate forged preview digest: %v", err)
	}
	if _, err := repository.AcceptAgentRebind(context.Background(), ledger.AcceptAgentRebindCommand{
		IdempotencyKey: "rebind-forged", Preview: forged, AcceptedBy: "human:toby", AcceptedAt: proposal.Binding.ActivatedAt,
	}); err == nil {
		t.Fatal("AcceptAgentRebind accepted a digest-valid preview for another Conversation")
	}

	accepted, err := repository.AcceptAgentRebind(context.Background(), ledger.AcceptAgentRebindCommand{
		IdempotencyKey: "rebind-researcher-2", Preview: preview, AcceptedBy: "human:toby", AcceptedAt: proposal.Binding.ActivatedAt,
	})
	if err != nil {
		t.Fatalf("AcceptAgentRebind: %v", err)
	}
	if accepted.Agent.Binding.ID != proposal.Binding.ID || accepted.Agent.Behavior.ID != created.Behavior.ID ||
		accepted.Agent.Profile.ID != created.Profile.ID {
		t.Fatalf("Rebind advanced the wrong current revisions: %+v", accepted.Agent)
	}
	if accepted.Transition.Kind != ledger.BindingTransitionRebind || accepted.Transition.PreviewDigest != preview.Digest ||
		accepted.Transition.PreviousBindingRevisionID != created.Binding.ID {
		t.Fatalf("Rebind transition = %+v", accepted.Transition)
	}
	if replay, err := repository.AcceptAgentRebind(context.Background(), ledger.AcceptAgentRebindCommand{
		IdempotencyKey: "rebind-researcher-2", Preview: preview, AcceptedBy: "human:toby", AcceptedAt: proposal.Binding.ActivatedAt,
	}); err != nil || replay.Transition.SuccessorBindingRevisionID != proposal.Binding.ID {
		t.Fatalf("Rebind replay = %+v, %v", replay, err)
	}

	if _, err := repository.AcceptAgentRebind(context.Background(), ledger.AcceptAgentRebindCommand{
		IdempotencyKey: "rebind-stale", Preview: preview, AcceptedBy: "human:toby", AcceptedAt: proposal.Binding.ActivatedAt,
	}); !errors.Is(err, ledger.ErrRevisionConflict) {
		t.Fatalf("stale Rebind error = %v, want revision conflict", err)
	}
	current, err := repository.GetAgent(context.Background(), created.Agent.AccountID, created.Agent.ID)
	if err != nil || current.Binding.ID != proposal.Binding.ID {
		t.Fatalf("stale Rebind caused implicit failover: %+v, %v", current.Binding, err)
	}
}

func TestSQLiteAgentLifecycleCreatesListsArchivesAndReopensSecondaryConversations(t *testing.T) {
	repository := openAgentConversationRepository(t)
	created := stableAgentCommand(t)
	if _, err := repository.CreateAgent(context.Background(), created); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	now := created.Home.CreatedAt.Add(time.Minute)
	secondary := ledger.CreateSecondaryConversationCommand{
		IdempotencyKey: "secondary-market-map", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		Conversation: conversation.Conversation{ID: "conversation:market-map", Title: "Market map", State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now},
		Link:         conversation.AgentConversation{AgentID: created.Agent.ID, ConversationID: "conversation:market-map", Kind: conversation.AgentConversationSecondary, CreatedAt: now},
		CreatedBy:    "human:toby",
	}
	createdSecondary, err := repository.CreateSecondaryConversation(context.Background(), secondary)
	if err != nil {
		t.Fatalf("CreateSecondaryConversation: %v", err)
	}
	if createdSecondary.Link.Kind != conversation.AgentConversationSecondary {
		t.Fatalf("secondary link = %+v", createdSecondary.Link)
	}
	if _, err := repository.CreateSecondaryConversation(context.Background(), secondary); err != nil {
		t.Fatalf("secondary replay: %v", err)
	}
	renamed, err := repository.RenameAgentConversation(context.Background(), ledger.RenameAgentConversationCommand{
		IdempotencyKey: "rename-market-map", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: secondary.Conversation.ID, ExpectedTitle: "Market map", Title: "Market landscape",
		ChangedBy: "human:toby", ChangedAt: now.Add(30 * time.Second),
	})
	if err != nil || renamed.Conversation.Title != "Market landscape" {
		t.Fatalf("rename secondary = %+v, %v", renamed, err)
	}
	if replay, err := repository.RenameAgentConversation(context.Background(), ledger.RenameAgentConversationCommand{
		IdempotencyKey: "rename-market-map", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: secondary.Conversation.ID, ExpectedTitle: "Market map", Title: "Market landscape",
		ChangedBy: "human:toby", ChangedAt: now.Add(30 * time.Second),
	}); err != nil || replay.Conversation.Title != "Market landscape" {
		t.Fatalf("rename replay = %+v, %v", replay, err)
	}
	if _, err := repository.RenameAgentConversation(context.Background(), ledger.RenameAgentConversationCommand{
		IdempotencyKey: "rename-market-map-stale", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: secondary.Conversation.ID, ExpectedTitle: "Market map", Title: "Stale title",
		ChangedBy: "human:toby", ChangedAt: now.Add(45 * time.Second),
	}); !errors.Is(err, ledger.ErrRevisionConflict) {
		t.Fatalf("stale rename error = %v, want revision conflict", err)
	}
	pinned, err := repository.SetAgentConversationPin(context.Background(), ledger.SetAgentConversationPinCommand{
		IdempotencyKey: "pin-market-map", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: secondary.Conversation.ID, ExpectedPinned: false, Pinned: true,
		ChangedBy: "human:toby", ChangedAt: now.Add(time.Minute),
	})
	if err != nil || !pinned.Pinned || pinned.PinnedAt.IsZero() {
		t.Fatalf("pin secondary = %+v, %v", pinned, err)
	}
	if replay, err := repository.SetAgentConversationPin(context.Background(), ledger.SetAgentConversationPinCommand{
		IdempotencyKey: "pin-market-map", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: secondary.Conversation.ID, ExpectedPinned: false, Pinned: true,
		ChangedBy: "human:toby", ChangedAt: now.Add(time.Minute),
	}); err != nil || !replay.Pinned {
		t.Fatalf("pin replay = %+v, %v", replay, err)
	}
	listed, err := repository.ListAgentConversations(context.Background(), created.Agent.AccountID, created.Agent.ID)
	if err != nil {
		t.Fatalf("ListAgentConversations: %v", err)
	}
	if len(listed) != 2 || listed[0].Link.Kind != conversation.AgentConversationCanonical || listed[0].Pinned ||
		listed[1].Conversation.ID != secondary.Conversation.ID || !listed[1].Pinned {
		t.Fatalf("Agent Conversations = %+v", listed)
	}
	if _, err := repository.SetAgentConversationPin(context.Background(), ledger.SetAgentConversationPinCommand{
		IdempotencyKey: "unpin-market-map", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: secondary.Conversation.ID, ExpectedPinned: true, Pinned: false,
		ChangedBy: "human:toby", ChangedAt: now.Add(90 * time.Second),
	}); err != nil {
		t.Fatalf("unpin secondary: %v", err)
	}

	archived, err := repository.SetAgentConversationState(context.Background(), ledger.SetAgentConversationStateCommand{
		IdempotencyKey: "archive-market-map", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: secondary.Conversation.ID, ExpectedState: conversation.ConversationOpen,
		State: conversation.ConversationArchived, ChangedBy: "human:toby", ChangedAt: now.Add(time.Minute),
	})
	if err != nil || archived.Conversation.State != conversation.ConversationArchived {
		t.Fatalf("archive secondary = %+v, %v", archived, err)
	}
	reopened, err := repository.SetAgentConversationState(context.Background(), ledger.SetAgentConversationStateCommand{
		IdempotencyKey: "reopen-market-map", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: secondary.Conversation.ID, ExpectedState: conversation.ConversationArchived,
		State: conversation.ConversationOpen, ChangedBy: "human:toby", ChangedAt: now.Add(2 * time.Minute),
	})
	if err != nil || reopened.Conversation.State != conversation.ConversationOpen {
		t.Fatalf("reopen secondary = %+v, %v", reopened, err)
	}

	_, err = repository.SetAgentConversationState(context.Background(), ledger.SetAgentConversationStateCommand{
		IdempotencyKey: "archive-home", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: created.Home.ID, ExpectedState: conversation.ConversationOpen,
		State: conversation.ConversationArchived, ChangedBy: "human:toby", ChangedAt: now.Add(3 * time.Minute),
	})
	if !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("Home archive error = %v, want state conflict", err)
	}
	_, err = repository.RenameAgentConversation(context.Background(), ledger.RenameAgentConversationCommand{
		IdempotencyKey: "rename-home", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: created.Home.ID, ExpectedTitle: created.Home.Title, Title: "Elsewhere",
		ChangedBy: "human:toby", ChangedAt: now.Add(4 * time.Minute),
	})
	if !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("Home rename error = %v, want state conflict", err)
	}
}

func TestAgentConversationLifecycleDigestsExcludeControlAllocatedIdentityAndTime(t *testing.T) {
	base := stableAgentCommand(t)
	firstAt := base.Home.CreatedAt.Add(time.Minute)
	first := ledger.CreateSecondaryConversationCommand{
		IdempotencyKey: "secondary-market-map", AccountID: base.Agent.AccountID, AgentID: base.Agent.ID,
		Conversation: conversation.Conversation{ID: "conversation:first", Title: "Market map", State: conversation.ConversationOpen, CreatedAt: firstAt, UpdatedAt: firstAt},
		Link:         conversation.AgentConversation{AgentID: base.Agent.ID, ConversationID: "conversation:first", Kind: conversation.AgentConversationSecondary, CreatedAt: firstAt},
		CreatedBy:    "human:toby",
	}
	second := first
	second.Conversation.ID = "conversation:second"
	second.Conversation.CreatedAt = firstAt.Add(time.Hour)
	second.Conversation.UpdatedAt = firstAt.Add(time.Hour)
	second.Link.ConversationID = second.Conversation.ID
	second.Link.CreatedAt = firstAt.Add(time.Hour)
	firstDigest, err := first.Digest()
	if err != nil {
		t.Fatalf("first create digest: %v", err)
	}
	secondDigest, err := second.Digest()
	if err != nil {
		t.Fatalf("second create digest: %v", err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("server allocation changed create digest: %s != %s", firstDigest, secondDigest)
	}
	second.Conversation.Title = "Another topic"
	if changed, err := second.Digest(); err != nil || changed == firstDigest {
		t.Fatalf("semantic create change digest = %q, %v", changed, err)
	}

	rename := ledger.RenameAgentConversationCommand{IdempotencyKey: "rename", AccountID: base.Agent.AccountID,
		AgentID: base.Agent.ID, ConversationID: first.Conversation.ID, ExpectedTitle: "Market map", Title: "Market landscape",
		ChangedBy: "human:toby", ChangedAt: firstAt}
	state := ledger.SetAgentConversationStateCommand{IdempotencyKey: "archive", AccountID: base.Agent.AccountID,
		AgentID: base.Agent.ID, ConversationID: first.Conversation.ID, ExpectedState: conversation.ConversationOpen,
		State: conversation.ConversationArchived, ChangedBy: "human:toby", ChangedAt: firstAt}
	pin := ledger.SetAgentConversationPinCommand{IdempotencyKey: "pin", AccountID: base.Agent.AccountID,
		AgentID: base.Agent.ID, ConversationID: first.Conversation.ID, ExpectedPinned: false, Pinned: true,
		ChangedBy: "human:toby", ChangedAt: firstAt}
	for name, pair := range map[string][2]func() (string, error){
		"rename": {
			rename.Digest,
			func() (string, error) {
				later := rename
				later.ChangedAt = firstAt.Add(time.Hour)
				return later.Digest()
			},
		},
		"state": {
			state.Digest,
			func() (string, error) {
				later := state
				later.ChangedAt = firstAt.Add(time.Hour)
				return later.Digest()
			},
		},
		"pin": {
			pin.Digest,
			func() (string, error) { later := pin; later.ChangedAt = firstAt.Add(time.Hour); return later.Digest() },
		},
	} {
		a, err := pair[0]()
		if err != nil {
			t.Fatalf("%s first digest: %v", name, err)
		}
		b, err := pair[1]()
		if err != nil || a != b {
			t.Fatalf("%s time changed digest: %q != %q, %v", name, a, b, err)
		}
	}
}

type agentProfileRepository interface {
	ledger.AgentRepository
	AppendAgentProfile(context.Context, ledger.AppendAgentProfileCommand) (ledger.AgentRecord, error)
}

type agentBehaviorRepository interface {
	ledger.AgentRepository
	AppendAgentBehavior(context.Context, ledger.AppendAgentBehaviorCommand) (ledger.AgentBindingAdvanceResult, error)
}

type agentRebindRepository interface {
	ledger.AgentRepository
	PreviewAgentRebind(context.Context, ledger.PreviewAgentRebindCommand) (ledger.AgentRebindPreview, error)
	AcceptAgentRebind(context.Context, ledger.AcceptAgentRebindCommand) (ledger.AgentBindingAdvanceResult, error)
}

type agentConversationRepository interface {
	ledger.AgentRepository
	CreateSecondaryConversation(context.Context, ledger.CreateSecondaryConversationCommand) (ledger.AgentConversationRecord, error)
	ListAgentConversations(context.Context, string, string) ([]ledger.AgentConversationRecord, error)
	RenameAgentConversation(context.Context, ledger.RenameAgentConversationCommand) (ledger.AgentConversationRecord, error)
	SetAgentConversationState(context.Context, ledger.SetAgentConversationStateCommand) (ledger.AgentConversationRecord, error)
	SetAgentConversationPin(context.Context, ledger.SetAgentConversationPinCommand) (ledger.AgentConversationRecord, error)
}

func openAgentProfileRepository(t *testing.T) agentProfileRepository {
	t.Helper()
	repository := openAgentRepository(t)
	lifecycle, ok := repository.(agentProfileRepository)
	if !ok {
		t.Fatal("Store does not implement profile lifecycle")
	}
	return lifecycle
}

func openAgentBehaviorRepository(t *testing.T) agentBehaviorRepository {
	t.Helper()
	repository := openAgentRepository(t)
	lifecycle, ok := repository.(agentBehaviorRepository)
	if !ok {
		t.Fatal("Store does not implement Behavior lifecycle")
	}
	return lifecycle
}

func openAgentRebindRepository(t *testing.T) agentRebindRepository {
	t.Helper()
	repository := openAgentRepository(t)
	lifecycle, ok := repository.(agentRebindRepository)
	if !ok {
		t.Fatal("Store does not implement Rebind lifecycle")
	}
	return lifecycle
}

func openAgentConversationRepository(t *testing.T) agentConversationRepository {
	t.Helper()
	repository := openAgentRepository(t)
	lifecycle, ok := repository.(agentConversationRepository)
	if !ok {
		t.Fatal("Store does not implement Agent Conversation lifecycle")
	}
	return lifecycle
}

func rebindProposal(created ledger.CreateAgentCommand) ledger.PreviewAgentRebindCommand {
	acceptedAt := created.Binding.ActivatedAt.Add(5 * time.Minute)
	binding := created.Binding
	binding.ID = "binding:researcher:2"
	binding.Revision = 2
	binding.ExecutionSourceID = "source:mini"
	binding.SourceAgentID = "source-agent:mini:researcher"
	binding.SeatID = "seat:researcher:mini"
	binding.FortProfile = "hermes:researcher"
	binding.Provider = "hermes"
	binding.RequestedModel = "hermes-main"
	binding.ResolvedModel = "hermes-main"
	binding.ComputerID = "computer:mini"
	binding.AdapterID = "model.chat.hermes"
	binding.AdapterRevision = "adapter:2"
	binding.SourceConfigDigest = strings.Repeat("b", 64)
	binding.SessionBehavior = "new_session"
	binding.MemoryBehavior = "source_managed"
	binding.SupersedesRevisionID = created.Binding.ID
	binding.ActivatedAt = acceptedAt
	source := created.ExecutionSource
	source.ID = binding.ExecutionSourceID
	source.Framework = "hermes"
	source.InstanceID = "instance:mini"
	source.GatewayID = "gateway:mini"
	source.DisplayName = "Hermes · Mac mini"
	source.LastSeenAt = acceptedAt
	source.ResourceSharing.SourceMemory = conversation.ResourceMachineShared
	sourceAgent := created.SourceAgent
	sourceAgent.ID = binding.SourceAgentID
	sourceAgent.ExecutionSourceID = source.ID
	sourceAgent.DisplayName = "Researcher · Mac mini"
	sourceAgent.LastSeenAt = acceptedAt
	participant := created.Participant
	participant.ID = "participant:researcher:mini"
	participant.SeatID = binding.SeatID
	participant.Profile = binding.FortProfile
	participant.Agent = binding.Provider
	participant.Model = binding.RequestedModel
	participant.Machine = binding.ComputerID
	participant.CreatedAt = acceptedAt
	return ledger.PreviewAgentRebindCommand{
		AccountID: created.Agent.AccountID, AgentID: created.Agent.ID, ExpectedBindingRevisionID: created.Binding.ID,
		Binding: binding, ExecutionSource: source, SourceAgent: sourceAgent, Participant: participant,
		NonTransferableResources: []ledger.RebindResource{ledger.RebindResourceFiles, ledger.RebindResourceSessions, ledger.RebindResourceSourceMemory},
		ReadinessEvidence:        []string{"ready:hermes-mini"}, AuthorityEvidence: []string{"authority:chat-revalidated"},
		GeneratedAt: acceptedAt.Add(-time.Minute),
	}
}
