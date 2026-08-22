package conversation

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/robfig/cron/v3"
)

type RoutineState string

const (
	RoutineActive   RoutineState = "active"
	RoutinePaused   RoutineState = "paused"
	RoutineArchived RoutineState = "archived"
)

type RoutineAuthority string

const RoutineAuthorityFortCloud RoutineAuthority = "fort_cloud"

type RoutineTrigger string

const (
	RoutineTriggerSchedule RoutineTrigger = "schedule"
	RoutineTriggerEvent    RoutineTrigger = "event"

	RoutineApprovalNone                     = "none"
	RoutineApprovalBeforeExternalSideEffect = "before_external_side_effect"
	RoutineLatenessWithin90Seconds          = "within_90s"
	RoutineLatenessNone                     = "none"
)

type Routine struct {
	ID                string       `json:"id"`
	AccountID         string       `json:"account_id"`
	AgentID           string       `json:"agent_id"`
	CurrentRevisionID string       `json:"current_revision_id"`
	State             RoutineState `json:"state"`
	CreatedAt         time.Time    `json:"created_at"`
}

type RoutineRevision struct {
	ID                   string           `json:"id"`
	RoutineID            string           `json:"routine_id"`
	Revision             int              `json:"revision"`
	AgentID              string           `json:"agent_id"`
	BehaviorRevisionID   string           `json:"behavior_revision_id"`
	BindingRevisionID    string           `json:"binding_revision_id"`
	Authority            RoutineAuthority `json:"authority"`
	Trigger              RoutineTrigger   `json:"trigger"`
	Schedule             string           `json:"schedule,omitempty"`
	Timezone             string           `json:"timezone,omitempty"`
	NextOccurrence       time.Time        `json:"next_occurrence,omitempty"`
	InputSource          string           `json:"input_source"`
	FreshnessSeconds     int64            `json:"freshness_seconds"`
	ExpectedResult       string           `json:"expected_result"`
	ResultConversationID string           `json:"result_conversation_id"`
	ApprovalBoundary     string           `json:"approval_boundary"`
	MissingInputBehavior string           `json:"missing_input_behavior"`
	RetryPolicy          string           `json:"retry_policy"`
	CatchUpPolicy        string           `json:"catch_up_policy"`
	LatenessPolicy       string           `json:"lateness_policy"`
	CreatedAt            time.Time        `json:"created_at"`
}

func (r Routine) Validate(revision RoutineRevision) error {
	if err := requireStrings("Routine", map[string]string{
		"id": r.ID, "account id": r.AccountID, "Agent id": r.AgentID, "current revision id": r.CurrentRevisionID,
	}); err != nil {
		return err
	}
	if r.State != RoutineActive && r.State != RoutinePaused && r.State != RoutineArchived {
		return fmt.Errorf("Routine state is invalid")
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("Routine creation time is required")
	}
	if err := revision.Validate(); err != nil {
		return err
	}
	if revision.ID != r.CurrentRevisionID || revision.RoutineID != r.ID || revision.AgentID != r.AgentID {
		return fmt.Errorf("Routine current revision must belong to its owning Agent")
	}
	return nil
}

func (r RoutineRevision) Validate() error {
	if err := requireStrings("Routine Revision", map[string]string{
		"id": r.ID, "Routine id": r.RoutineID, "Agent id": r.AgentID,
		"Behavior Revision id": r.BehaviorRevisionID, "Binding Revision id": r.BindingRevisionID,
		"input source": r.InputSource, "expected result": r.ExpectedResult,
		"result Conversation id": r.ResultConversationID, "approval boundary": r.ApprovalBoundary,
		"missing-input behavior": r.MissingInputBehavior, "retry policy": r.RetryPolicy,
		"catch-up policy": r.CatchUpPolicy, "lateness policy": r.LatenessPolicy,
	}); err != nil {
		return err
	}
	if r.Revision < 1 || r.CreatedAt.IsZero() {
		return fmt.Errorf("Routine Revision number and creation time are required")
	}
	if r.Authority != RoutineAuthorityFortCloud {
		return fmt.Errorf("first-release executable Routine authority must be fort_cloud")
	}
	if r.Trigger != RoutineTriggerSchedule && r.Trigger != RoutineTriggerEvent {
		return fmt.Errorf("Routine Revision trigger is invalid")
	}
	if r.Trigger == RoutineTriggerSchedule {
		if r.NextOccurrence.IsZero() {
			return fmt.Errorf("scheduled Routine requires schedule, timezone, and next occurrence")
		}
		if err := ValidateRoutineSchedule(r.Schedule, r.Timezone); err != nil {
			return err
		}
	} else if r.Schedule != "" || r.Timezone != "" || !r.NextOccurrence.IsZero() {
		return fmt.Errorf("event Routine cannot contain schedule semantics")
	}
	if r.FreshnessSeconds <= 0 {
		return fmt.Errorf("Routine Revision requires a positive freshness window")
	}
	if r.ApprovalBoundary != RoutineApprovalNone && r.ApprovalBoundary != RoutineApprovalBeforeExternalSideEffect {
		return fmt.Errorf("Routine Revision approval boundary is unsupported in the first release")
	}
	switch r.MissingInputBehavior {
	case "skip", "needs_you", "fail":
	default:
		return fmt.Errorf("Routine Revision missing-input behavior is invalid")
	}
	if r.Trigger == RoutineTriggerSchedule && r.LatenessPolicy != RoutineLatenessWithin90Seconds {
		return fmt.Errorf("scheduled Routine requires the first-release within_90s lateness policy")
	}
	if r.Trigger == RoutineTriggerEvent && r.LatenessPolicy != RoutineLatenessNone {
		return fmt.Errorf("event Routine requires the first-release none lateness policy")
	}
	return nil
}

// ValidateRoutineSchedule accepts only the exact six-field cron and IANA
// timezone contract used by the cloud scheduler. It intentionally rejects
// descriptor aliases and normalized-but-not-canonical whitespace.
func ValidateRoutineSchedule(expression, timezone string) error {
	fields := strings.Fields(expression)
	if len(fields) != 6 || strings.Join(fields, " ") != expression {
		return fmt.Errorf("scheduled Routine requires an exact six-field cron expression")
	}
	if timezone == "" || strings.TrimSpace(timezone) != timezone {
		return fmt.Errorf("scheduled Routine timezone is invalid")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return fmt.Errorf("scheduled Routine timezone is invalid")
	}
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(expression); err != nil {
		return fmt.Errorf("scheduled Routine cron expression is invalid: %w", err)
	}
	return nil
}

// NeedsRevalidation identifies a Routine whose pinned Behavior or Binding is
// no longer the owning Agent's current accepted revision.
func (r RoutineRevision) NeedsRevalidation(agent Agent) bool {
	return r.AgentID != agent.ID || r.BehaviorRevisionID != agent.CurrentBehaviorRevisionID ||
		r.BindingRevisionID != agent.CurrentBindingRevisionID
}

type RoutineRunKind string

const (
	RoutineRunScheduled RoutineRunKind = "scheduled"
	RoutineRunTest      RoutineRunKind = "test"
)

type RoutineRunState string

const (
	RoutineRunQueued    RoutineRunState = "queued"
	RoutineRunWorking   RoutineRunState = "working"
	RoutineRunNeedsYou  RoutineRunState = "needs_you"
	RoutineRunSucceeded RoutineRunState = "succeeded"
	RoutineRunFailed    RoutineRunState = "failed"
	RoutineRunCanceled  RoutineRunState = "canceled"
)

type RoutineRun struct {
	ID                 string          `json:"id"`
	RoutineID          string          `json:"routine_id"`
	RoutineRevisionID  string          `json:"routine_revision_id"`
	AgentID            string          `json:"agent_id"`
	BehaviorRevisionID string          `json:"behavior_revision_id"`
	BindingRevisionID  string          `json:"binding_revision_id"`
	OccurrenceID       string          `json:"occurrence_id"`
	Kind               RoutineRunKind  `json:"kind"`
	State              RoutineRunState `json:"state"`
	NormalizedResult   string          `json:"normalized_result,omitempty"`
	ResultMessageID    string          `json:"result_message_id,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

func (r RoutineRun) ValidateFor(routine Routine, revision RoutineRevision) error {
	if err := routine.Validate(revision); err != nil {
		return err
	}
	if err := requireStrings("Routine run", map[string]string{
		"id": r.ID, "Routine id": r.RoutineID, "Routine Revision id": r.RoutineRevisionID,
		"Agent id": r.AgentID, "Behavior Revision id": r.BehaviorRevisionID,
		"Binding Revision id": r.BindingRevisionID, "occurrence id": r.OccurrenceID,
	}); err != nil {
		return err
	}
	if r.RoutineID != routine.ID || r.RoutineRevisionID != revision.ID || r.AgentID != routine.AgentID ||
		r.BehaviorRevisionID != revision.BehaviorRevisionID || r.BindingRevisionID != revision.BindingRevisionID {
		return fmt.Errorf("Routine run must pin its owner, Behavior Revision, and Binding Revision")
	}
	if r.Kind != RoutineRunScheduled && r.Kind != RoutineRunTest {
		return fmt.Errorf("Routine run kind is invalid")
	}
	if !r.State.Valid() {
		return fmt.Errorf("Routine run state is invalid")
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("Routine run creation time is required")
	}
	result := strings.TrimSpace(r.NormalizedResult)
	messageID := strings.TrimSpace(r.ResultMessageID)
	if r.State == RoutineRunSucceeded {
		if result == "" || messageID == "" {
			return fmt.Errorf("successful Routine run requires one normalized result message")
		}
	} else if result != "" || messageID != "" {
		return fmt.Errorf("only a successful Routine run may create a result message")
	}
	return nil
}

func (s RoutineRunState) Valid() bool {
	switch s {
	case RoutineRunQueued, RoutineRunWorking, RoutineRunNeedsYou, RoutineRunSucceeded, RoutineRunFailed, RoutineRunCanceled:
		return true
	default:
		return false
	}
}
