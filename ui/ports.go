package ui

import (
	"context"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/task"
	coretoday "github.com/tobsai/fort/core/today"
)

// The ui module talks to the rest of Fort only through these ports. This is
// what lets the control plane (board, chat, scheduler, all client surfaces) run
// WITHOUT the deterministic components: ui imports neither the router, the
// native runtime, nor the DAG engine. Concrete adapters live in package control
// and are wired in by cmd/fort.

// RunRef identifies the run a submitted task produced.
type RunRef struct {
	RunID   string `json:"run_id"`
	Route   string `json:"route,omitempty"`   // agent, when an execution plane routed it
	Machine string `json:"machine,omitempty"` // host it was placed on (spec 022)
	Queued  bool   `json:"queued,omitempty"`  // true when only boarded (no execution plane)
}

// Dispatcher accepts a task. With an execution plane it routes + dispatches;
// in control-only mode it simply boards the task (Queued=true).
type Dispatcher interface {
	Submit(ctx context.Context, t task.Task) (RunRef, error)
}

// AcceptedDispatcher durably boards a routed run before provider startup and
// returns without waiting for Dispatch. HTTP handlers prefer this optional seam
// so slow provider preflight cannot hold a gateway request open.
type AcceptedDispatcher interface {
	Accept(ctx context.Context, t task.Task) (RunRef, error)
}

// MachineLister reports the machine roster + reachability for the control plane
// (GET /api/machines, spec 022). It is nil in single-machine mode, in which case
// the endpoint returns an empty roster. Implemented by package control.
type MachineLister interface {
	Machines() []MachineStatus
}

// CapabilityLister returns the latest immutable, secret-free capability
// snapshot. Refresh and probing stay behind control/exec adapters.
type CapabilityLister interface {
	Capabilities() (corecap.Snapshot, uint64)
}

// ConversationSeatRechecker runs the already-bounded functional probes used
// to project shared-conversation seats. It must not install, authenticate, or
// dispatch an agent runtime.
type ConversationSeatRechecker interface {
	RecheckConversationSeats(context.Context) error
}

// ConversationDetail is the bounded conversation wire projection. Persistence
// aggregates are adapted to this type by package control before reaching ui.
type ConversationDetail struct {
	Conversation conversation.Conversation  `json:"conversation"`
	Participants []conversation.Participant `json:"participants"`
	Messages     []conversation.Message     `json:"messages"`
	Turns        []conversation.Turn        `json:"turns"`
	Targets      []conversation.Target      `json:"targets"`
}

type ConversationPort interface {
	ConversationSeats(context.Context) ([]conversation.Seat, error)
	ListProjects(context.Context) ([]conversation.Project, error)
	CreateProject(context.Context, string) (conversation.Project, error)
	RenameProject(context.Context, string, string) error
	DeleteProject(context.Context, string) error
	ListConversations(context.Context, string) ([]conversation.Conversation, error)
	GetConversation(context.Context, string) (ConversationDetail, error)
	CreateConversation(context.Context, string, string, []string) (ConversationDetail, error)
	AddConversationParticipant(context.Context, string, string) (conversation.Participant, error)
	MoveConversation(context.Context, string, string) error
	RenameConversation(context.Context, string, string) error
	SetConversationState(context.Context, string, conversation.ConversationState) error
	DeleteConversation(context.Context, string) error
	RemoveConversationParticipant(context.Context, string, string) error
	PostTurn(context.Context, string, string, string, []string) (conversation.TurnResult, error)
	RetryTarget(context.Context, string) (conversation.Target, error)
	CancelTarget(context.Context, string) error
}

// PrimaryAgentOption is one visible profile/model/computer inventory row.
// Only ready subscription-backed rows carry selectable authority; ordinary
// and unready profiles remain visible with a closed state and reason. Clients
// select only OptionID and cannot combine independent seat and policy fields.
type PrimaryAgentOption struct {
	ID          string                      `json:"option_id"`
	State       string                      `json:"state"`
	Reason      string                      `json:"reason,omitempty"`
	Seat        conversation.Seat           `json:"seat"`
	Offer       corecap.TextOnlyOptionOffer `json:"authority"`
	DisplayName string                      `json:"display_name"`
}

type PrimaryAgentView struct {
	Selection         *conversation.PrimaryAgentSetting `json:"selection"`
	State             string                            `json:"state"`
	Reason            string                            `json:"reason,omitempty"`
	Options           []PrimaryAgentOption              `json:"options"`
	ScheduleInventory *ScheduleInventory                `json:"schedule_inventory,omitempty"`
}

type PrimaryNeedsYouItem struct {
	Channel         conversation.PrimaryChannelSummary `json:"channel"`
	Target          conversation.Target                `json:"target"`
	RecoveryActions []string                           `json:"recovery_actions"`
}

// PrimaryChannelReadiness is a read-only projection of the latest capability
// inventory for a Channel's immutable stored identity. It never retargets or
// rewrites the participant or authority snapshot.
type PrimaryChannelReadiness struct {
	State      string    `json:"state"`
	Reason     string    `json:"reason,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
}

// PrimaryChannelDetail is the bounded wire projection for one marked private
// Channel. Persistence remains behind the control adapter.
type PrimaryChannelDetail struct {
	Conversation   conversation.Conversation    `json:"conversation"`
	Participants   []conversation.Participant   `json:"participants"`
	Messages       []conversation.Message       `json:"messages"`
	Turns          []conversation.Turn          `json:"turns"`
	Targets        []conversation.Target        `json:"targets"`
	PrimaryChannel *conversation.PrimaryChannel `json:"primary_identity,omitempty"`
	Readiness      PrimaryChannelReadiness      `json:"readiness"`
}

// PrimaryChannelPort deliberately has no generic participant selection or
// execution method. Its implementation resolves the one stored Primary Agent
// and sole Channel participant server-side.
type PrimaryChannelPort interface {
	PrimaryAgent(context.Context) (PrimaryAgentView, error)
	SetPrimaryAgent(context.Context, string) (PrimaryAgentView, error)
	ClearPrimaryAgent(context.Context) error
	RecheckPrimaryAgent(context.Context) (PrimaryAgentView, error)
	ListChannels(context.Context, string) ([]conversation.PrimaryChannelSummary, error)
	GetChannel(context.Context, string) (PrimaryChannelDetail, error)
	CreateChannel(context.Context, string) (PrimaryChannelDetail, error)
	RenameChannel(context.Context, string, string) error
	SetChannelState(context.Context, string, conversation.ConversationState) error
	SetChannelPinned(context.Context, string, bool) error
	PostTurn(context.Context, string, string, string) (conversation.TurnResult, error)
	RetryTarget(context.Context, string, string) (conversation.Target, error)
	RecheckAndRetryTarget(context.Context, string, string) (conversation.Target, error)
	CancelTarget(context.Context, string, string) error
	NeedsYou(context.Context) ([]PrimaryNeedsYouItem, error)
}

// AgentOption is one provider-neutral, server-resolved option for creating an
// Agent Channel. Clients submit only ID; Binding is inspectable evidence and
// cannot be reconstructed from independently selected fields.
type AgentOption struct {
	ID          string                    `json:"agent_option_id"`
	State       string                    `json:"state"`
	Reason      string                    `json:"reason,omitempty"`
	DisplayName string                    `json:"display_name"`
	Binding     conversation.AgentBinding `json:"binding"`
}

// AgentChannelSummary is sufficient for the agent-first rail: one immutable
// agent destination plus its pinned/recent Conversation shortcuts and current
// observational readiness. The conversations remain separate transcripts.
type AgentChannelSummary struct {
	Channel       conversation.AgentChannel               `json:"channel"`
	Conversations []conversation.AgentConversationSummary `json:"conversations"`
	Readiness     PrimaryChannelReadiness                 `json:"readiness"`
}

type AgentChannelDetail = AgentChannelSummary

// AgentConversationDetail is the canonical transcript projection beneath one
// owning Agent Channel. Parent-qualified service methods prevent a client from
// using a valid Conversation through the wrong agent identity.
type AgentConversationDetail struct {
	ChannelID    string                    `json:"agent_channel_id"`
	Conversation conversation.Conversation `json:"conversation"`
	Participant  conversation.Participant  `json:"participant"`
	Messages     []conversation.Message    `json:"messages"`
	Turns        []conversation.Turn       `json:"turns"`
	Targets      []conversation.Target     `json:"targets"`
	Readiness    PrimaryChannelReadiness   `json:"readiness"`
	Binding      conversation.AgentBinding `json:"binding"`
	Pinned       bool                      `json:"pinned"`
	PinnedAt     time.Time                 `json:"pinned_at,omitempty"`
}

type AgentFirstTurnResult struct {
	Conversation AgentConversationDetail `json:"conversation"`
	Turn         conversation.Turn       `json:"turn"`
	Targets      []conversation.Target   `json:"targets"`
}

type AgentNeedsYouItem struct {
	AgentChannel conversation.AgentChannel `json:"agent_channel"`
	Conversation conversation.Conversation `json:"conversation"`
	Target       conversation.Target       `json:"target"`
	Actions      []string                  `json:"recovery_actions"`
}

// AgentChannelPort is the new agent-first product seam. The legacy
// PrimaryChannelPort remains unchanged for rollback and older clients.
type AgentChannelPort interface {
	AgentOptions(context.Context) ([]AgentOption, error)
	RecheckAgentOptions(context.Context) ([]AgentOption, error)
	ListAgentChannels(context.Context, string) ([]AgentChannelSummary, error)
	GetAgentChannel(context.Context, string) (AgentChannelDetail, error)
	CreateAgentChannel(context.Context, string, string) (AgentChannelDetail, error)
	RenameAgentChannel(context.Context, string, string) error
	SetAgentChannelState(context.Context, string, conversation.AgentChannelState) error
	ListAgentConversations(context.Context, string, string) ([]conversation.AgentConversationSummary, error)
	GetAgentConversation(context.Context, string, string) (AgentConversationDetail, error)
	CreateAgentConversation(context.Context, string, string) (AgentConversationDetail, error)
	RenameAgentConversation(context.Context, string, string, string) error
	SetAgentConversationState(context.Context, string, string, conversation.ConversationState) error
	SetAgentConversationPinned(context.Context, string, string, bool) error
	PostFirstAgentTurn(context.Context, string, string, string, string) (AgentFirstTurnResult, error)
	PostAgentTurn(context.Context, string, string, string, string) (conversation.TurnResult, error)
	RetryAgentTarget(context.Context, string, string, string) (conversation.Target, error)
	CancelAgentTarget(context.Context, string, string, string) error
	AgentNeedsYou(context.Context) ([]AgentNeedsYouItem, error)
}

type TodayPort interface {
	Today(context.Context, time.Time, *time.Location) (coretoday.View, error)
}

type SchedulePort interface {
	Create(context.Context, scheduler.Definition) (scheduler.Definition, error)
}

type ScheduleFilter string

const (
	ScheduleFilterAll    ScheduleFilter = "all"
	ScheduleFilterActive ScheduleFilter = "active"
	ScheduleFilterPaused ScheduleFilter = "paused"
)

type SchedulerOwnership string

const (
	SchedulerOwnershipActive   SchedulerOwnership = "active"
	SchedulerOwnershipInactive SchedulerOwnership = "inactive"
	SchedulerOwnershipUnknown  SchedulerOwnership = "unknown"
)

type OccurrencePage struct {
	Limit    int
	Before   time.Time
	BeforeID string
}

type RelatedChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ScheduleItem struct {
	ID                 string                `json:"id"`
	Title              string                `json:"title"`
	Enabled            bool                  `json:"enabled"`
	Kind               scheduler.Kind        `json:"kind"`
	Expression         string                `json:"expression"`
	Recurrence         string                `json:"recurrence"`
	Timezone           string                `json:"timezone"`
	NextFireAt         *time.Time            `json:"next_fire_at"`
	LastFireAt         *time.Time            `json:"last_fire_at"`
	TargetKind         string                `json:"target_kind"`
	TargetID           string                `json:"target_id"`
	RelatedChannel     *RelatedChannel       `json:"related_channel,omitempty"`
	LatestOccurrence   *scheduler.Occurrence `json:"latest_occurrence,omitempty"`
	SchedulerOwnership SchedulerOwnership    `json:"scheduler_ownership"`
	ObservedAt         time.Time             `json:"observed_at"`
}

type ScheduleList struct {
	SnapshotID string         `json:"snapshot_id"`
	ObservedAt time.Time      `json:"observed_at"`
	Items      []ScheduleItem `json:"items"`
}

type ScheduleDetail struct {
	Item     ScheduleItem           `json:"item"`
	Upcoming []scheduler.Occurrence `json:"upcoming"`
	Recent   []scheduler.Occurrence `json:"recent"`
}

type ScheduleReadPort interface {
	List(context.Context, ScheduleFilter) (ScheduleList, error)
	Get(context.Context, string) (ScheduleDetail, error)
	Occurrences(context.Context, string, OccurrencePage) ([]scheduler.Occurrence, error)
}

type ScheduleInventoryState string

const (
	ScheduleInventoryAccepted   ScheduleInventoryState = "accepted"
	ScheduleInventoryUnaccepted ScheduleInventoryState = "unaccepted"
	ScheduleInventoryDrift      ScheduleInventoryState = "drift"
)

type ScheduleInventoryItem struct {
	ID         string         `json:"id"`
	Kind       scheduler.Kind `json:"kind"`
	Expression string         `json:"expression"`
	Timezone   string         `json:"timezone"`
	FlowID     string         `json:"flow_id"`
	FlowDigest string         `json:"flow_digest"`
}

type ScheduleInventory struct {
	CurrentDigest  string                  `json:"current_digest"`
	AcceptedDigest string                  `json:"accepted_digest,omitempty"`
	State          ScheduleInventoryState  `json:"state"`
	Items          []ScheduleInventoryItem `json:"items"`
}

type ScheduleInventoryPort interface {
	Inventory(context.Context, string) (ScheduleInventory, error)
}

// RunResult is a flow run's state after a Start/Resume.
type RunResult struct {
	State      string `json:"state"`
	PausedNode string `json:"paused_node,omitempty"`
}

// FlowNode is one node of a flow plan as exposed to the control plane
// (spec 033): just enough to know a run's checkpoint total.
type FlowNode struct {
	ID   string `json:"id"`
	Type string `json:"type"` // task | gate | check | transform | fanout
}

// FlowRunner runs flows by id. It is nil in control-only mode (no DAG engine);
// chat "ship X" then degrades to a boarded task and gate actions return 409.
type FlowRunner interface {
	StartFlow(ctx context.Context, flowID, runID, payload string) (RunResult, error)
	Approve(runID, nodeID, edit string) error
	Reject(runID, nodeID, note string) error
	ResumeFlow(ctx context.Context, flowID, runID string) (RunResult, error)
	// Plan returns the flow's node list (nil for an unknown id).
	Plan(flowID string) []FlowNode
}

// AcceptedFlowRunner is the asynchronous HTTP seam for flows. Start persists
// the run before returning; Resume validates the existing run before scheduling
// exactly one detached continuation.
type AcceptedFlowRunner interface {
	StartFlowAsync(ctx context.Context, flowID, runID, payload string) (RunResult, error)
	ResumeFlowAsync(ctx context.Context, flowID, runID string) error
}

// Planner decomposes a goal into backlog sub-tasks by running a planner agent
// (spec 026). It is nil in control-only mode (planning needs an execution
// plane); the /api/breakdown endpoint 409s when it is nil. Breakdown returns the
// planner run's id immediately; the sub-tasks land in the backlog asynchronously
// when that run completes.
type Planner interface {
	Breakdown(ctx context.Context, goal, agent, machine string) (runID string, err error)
}

// PlaybookCatalog owns immutable playbook revisions and deterministic route
// resolution. It is available in both full and control-only modes; Route must
// never invoke a model or dispatch runtime work.
type PlaybookCatalog interface {
	List(ctx context.Context) ([]Playbook, error)
	Save(ctx context.Context, p Playbook) (Playbook, error)
	Duplicate(ctx context.Context, id string) (Playbook, error)
	Route(ctx context.Context, req RouteRequest) (RoutePreview, error)
}

// PlaybookRunResult is a Start result. Async starts return accepted without an
// inline Answer; synchronous delivery=answer starts populate Answer. Either
// form retains inspectable event history.
type PlaybookRunResult struct {
	State      string `json:"state"`
	PausedNode string `json:"paused_node,omitempty"`
	FlowID     string `json:"flow_id"`
	Answer     string `json:"answer,omitempty"`
}

// PlaybookRunner compiles and executes an already-resolved immutable route.
// It is nil in control-only mode.
type PlaybookRunner interface {
	StartPlaybook(ctx context.Context, route RoutePreview, runID, direction string) (PlaybookRunResult, error)
}

// AcceptedPlaybookRunner persists the exact immutable playbook run and returns
// its canonical flow identity before any provider stage completes.
type AcceptedPlaybookRunner interface {
	StartPlaybookAsync(ctx context.Context, route RoutePreview, runID, direction string) (PlaybookRunResult, error)
}
