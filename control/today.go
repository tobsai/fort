package control

import (
	"context"
	"sort"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/store"
	coretoday "github.com/tobsai/fort/core/today"
)

type TodayScheduleSource interface {
	MaterializeDay(context.Context, time.Time, *time.Location) error
}

type ConversationActivity interface {
	ConversationTargetActive(string) bool
}

type TodayService struct {
	store     *store.Store
	schedules TodayScheduleSource
	activity  ConversationActivity
	startedAt time.Time
}

func NewTodayService(st *store.Store, schedules TodayScheduleSource, activity ConversationActivity) *TodayService {
	return &TodayService{store: st, schedules: schedules, activity: activity, startedAt: time.Now().UTC()}
}

func (s *TodayService) Today(ctx context.Context, now time.Time, location *time.Location) (coretoday.View, error) {
	if location == nil {
		location = time.Local
	}
	localNow := now.In(location)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	end := start.AddDate(0, 0, 1)
	zone, _ := localNow.Zone()
	view := coretoday.View{Date: start.Format("2006-01-02"), Timezone: location.String(), TimezoneAbbreviation: zone, FreshAt: now.UTC(), InProgress: []coretoday.Progress{}, Scheduled: []coretoday.Scheduled{}}

	targets, err := s.store.ListConversationTargetDispatches(conversation.TargetQueued, conversation.TargetWorking)
	if err != nil {
		return coretoday.View{}, err
	}
	seenRuns := map[string]bool{}
	projectNames := map[string]string{}
	projects, err := s.store.ListProjects()
	if err != nil {
		return coretoday.View{}, err
	}
	for _, project := range projects {
		projectNames[project.ID] = project.Name
	}
	for _, item := range targets {
		if item.Target.State == conversation.TargetWorking && (s.activity == nil || !s.activity.ConversationTargetActive(item.Target.ID)) {
			continue
		}
		seenRuns[item.Target.RunID] = true
		view.InProgress = append(view.InProgress, coretoday.Progress{
			RunID: item.Target.RunID, TargetID: item.Target.ID, ConversationID: item.Conversation.ID,
			ConversationTitle: item.Conversation.Title, ProjectID: item.Conversation.ProjectID,
			ProjectName: projectNames[item.Conversation.ProjectID], ParticipantID: item.Participant.ID,
			ParticipantName: item.Participant.DisplayName, Agent: item.Participant.Agent,
			Profile: item.Participant.Profile, Machine: item.Participant.Machine,
			State: item.Target.State, UpdatedAt: item.Target.UpdatedAt,
		})
	}

	// Queued legacy assignments are truthful durable work. A legacy running row
	// is shown only when this daemon lifetime has persisted real provider
	// activity; pre-start rows and events never count as proof.
	runs, err := s.store.ListRuns()
	if err != nil {
		return coretoday.View{}, err
	}
	for _, run := range runs {
		if seenRuns[run.ID] {
			continue
		}
		state := conversation.TargetQueued
		updatedAt := run.UpdatedAt
		if run.Status == "running" {
			events, eventErr := s.store.Events(run.ID)
			if eventErr != nil {
				return coretoday.View{}, eventErr
			}
			var activityAt time.Time
			for _, event := range events {
				if legacyProviderActivity(event.Type) && !event.CreatedAt.Before(s.startedAt) && event.CreatedAt.After(activityAt) {
					activityAt = event.CreatedAt
				}
			}
			if activityAt.IsZero() {
				continue
			}
			state, updatedAt = conversation.TargetWorking, activityAt
		} else if run.Status != "queued" {
			continue
		}
		view.InProgress = append(view.InProgress, coretoday.Progress{
			RunID: run.ID, ConversationTitle: run.Title, Agent: run.Agent, Profile: run.Profile,
			Machine: run.Machine, State: state, UpdatedAt: updatedAt,
		})
	}
	sort.SliceStable(view.InProgress, func(i, j int) bool {
		if view.InProgress[i].State != view.InProgress[j].State {
			return view.InProgress[i].State == conversation.TargetWorking
		}
		return view.InProgress[i].UpdatedAt.After(view.InProgress[j].UpdatedAt)
	})

	definitions := map[string]scheduler.Definition{}
	if s.schedules != nil {
		if err := s.schedules.MaterializeDay(ctx, now, location); err != nil {
			return coretoday.View{}, err
		}
	}
	items, err := s.store.ListSchedules()
	if err != nil {
		return coretoday.View{}, err
	}
	for _, definition := range items {
		definitions[definition.ID] = definition
	}
	occurrences, err := s.store.ScheduleOccurrencesBetween(start, end)
	if err != nil {
		return coretoday.View{}, err
	}
	for _, occurrence := range occurrences {
		definition, known := definitions[occurrence.ScheduleID]
		if !known || !definition.Enabled {
			continue
		}
		view.Scheduled = append(view.Scheduled, coretoday.Scheduled{
			OccurrenceID: occurrence.ID, ScheduleID: occurrence.ScheduleID, FlowID: definition.FlowID,
			Title: definition.Title, Recurrence: string(definition.Kind) + " · " + definition.Expression,
			ScheduledFor: occurrence.ScheduledFor, State: occurrence.State, RunID: occurrence.RunID, Error: occurrence.Error,
		})
	}
	return view, nil
}

func legacyProviderActivity(eventType string) bool {
	switch eventType {
	case "started", "stdout", "stderr", "message", "tool", "subagent":
		return true
	default:
		return false
	}
}

func (s *ConversationService) ConversationTargetActive(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.active[id]
	return run != nil && run.Status().State == runtime.StateRunning
}
