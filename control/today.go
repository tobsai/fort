package control

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/store"
	coretoday "github.com/tobsai/fort/core/today"
)

type ConversationActivity interface {
	ConversationTargetActive(string) bool
}

type TodayService struct {
	store     *store.Store
	activity  ConversationActivity
	startedAt time.Time
}

func NewTodayService(st *store.Store, activity ConversationActivity) *TodayService {
	return &TodayService{store: st, activity: activity, startedAt: time.Now().UTC()}
}

func (s *TodayService) Today(_ context.Context, now time.Time, location *time.Location) (coretoday.View, error) {
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
			Profile: item.Participant.Profile, Model: item.Participant.Model, Machine: item.Participant.Machine,
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
			Model: run.Model, Machine: run.Machine, State: state, UpdatedAt: updatedAt,
		})
	}
	sort.SliceStable(view.InProgress, func(i, j int) bool {
		if view.InProgress[i].State != view.InProgress[j].State {
			return view.InProgress[i].State == conversation.TargetWorking
		}
		return view.InProgress[i].UpdatedAt.After(view.InProgress[j].UpdatedAt)
	})

	definitions := map[string]scheduler.Definition{}
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
	selectedOccurrences := map[string]scheduler.Occurrence{}
	for _, occurrence := range occurrences {
		definition, known := definitions[occurrence.ScheduleID]
		if !known || !definition.Enabled {
			continue
		}
		selected, exists := selectedOccurrences[occurrence.ScheduleID]
		if !exists || preferTodayOccurrence(selected, occurrence, now) {
			selectedOccurrences[occurrence.ScheduleID] = occurrence
		}
	}
	for scheduleID, occurrence := range selectedOccurrences {
		definition := definitions[scheduleID]
		view.Scheduled = append(view.Scheduled, coretoday.Scheduled{
			OccurrenceID: occurrence.ID, ScheduleID: occurrence.ScheduleID, FlowID: definition.FlowID,
			Title: definition.Title, Recurrence: recurrenceSummary(definition),
			ScheduledFor: occurrence.ScheduledFor, State: occurrence.State, RunID: occurrence.RunID, Error: occurrence.Error,
		})
	}
	sort.SliceStable(view.Scheduled, func(i, j int) bool {
		return view.Scheduled[i].ScheduledFor.Before(view.Scheduled[j].ScheduledFor)
	})
	return view, nil
}

func preferTodayOccurrence(current, candidate scheduler.Occurrence, now time.Time) bool {
	currentUpcoming := !current.ScheduledFor.Before(now)
	candidateUpcoming := !candidate.ScheduledFor.Before(now)
	if currentUpcoming != candidateUpcoming {
		return candidateUpcoming
	}
	if candidateUpcoming {
		return candidate.ScheduledFor.Before(current.ScheduledFor)
	}
	return candidate.ScheduledFor.After(current.ScheduledFor)
}

func recurrenceSummary(definition scheduler.Definition) string {
	if definition.Kind == scheduler.KindOnce {
		return "Once"
	}
	fields := strings.Fields(definition.Expression)
	if definition.Kind == scheduler.KindCron && len(fields) == 6 && fields[0] == "0" {
		if fields[1] == "0" && fields[2] == "*" && fields[3] == "*" && fields[4] == "*" && fields[5] == "*" {
			return "Every hour"
		}
		minute, minuteErr := strconv.Atoi(fields[1])
		hour, hourErr := strconv.Atoi(fields[2])
		if minuteErr == nil && hourErr == nil && minute >= 0 && minute < 60 && hour >= 0 && hour < 24 && fields[3] == "*" && fields[4] == "*" && fields[5] == "*" {
			return "Daily at " + time.Date(2000, 1, 1, hour, minute, 0, 0, time.UTC).Format("3:04 PM")
		}
	}
	return string(definition.Kind) + " · " + definition.Expression
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
