// Package today defines the truthful right-rail projection shared by Fort's
// presentation surfaces.
package today

import (
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/scheduler"
)

type Progress struct {
	RunID             string                   `json:"run_id"`
	TargetID          string                   `json:"target_id,omitempty"`
	ConversationID    string                   `json:"conversation_id,omitempty"`
	ConversationTitle string                   `json:"conversation_title"`
	ProjectID         string                   `json:"project_id,omitempty"`
	ProjectName       string                   `json:"project_name,omitempty"`
	ParticipantID     string                   `json:"participant_id,omitempty"`
	ParticipantName   string                   `json:"participant_name,omitempty"`
	Agent             string                   `json:"agent"`
	Profile           string                   `json:"profile,omitempty"`
	Machine           string                   `json:"machine,omitempty"`
	State             conversation.TargetState `json:"state"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

type Scheduled struct {
	OccurrenceID string                    `json:"occurrence_id"`
	ScheduleID   string                    `json:"schedule_id"`
	FlowID       string                    `json:"flow_id"`
	Title        string                    `json:"title"`
	Recurrence   string                    `json:"recurrence"`
	ScheduledFor time.Time                 `json:"scheduled_for"`
	State        scheduler.OccurrenceState `json:"state"`
	RunID        string                    `json:"run_id,omitempty"`
	Error        string                    `json:"error,omitempty"`
}

type View struct {
	Date                 string      `json:"date"`
	Timezone             string      `json:"timezone"`
	TimezoneAbbreviation string      `json:"timezone_abbreviation"`
	FreshAt              time.Time   `json:"fresh_at"`
	InProgress           []Progress  `json:"in_progress"`
	Scheduled            []Scheduled `json:"scheduled"`
}
