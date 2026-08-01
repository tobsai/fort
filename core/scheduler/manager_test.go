package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"
)

type memoryScheduleRepository struct {
	mu          sync.Mutex
	definitions []Definition
	occurrences map[string]Occurrence
}

func (r *memoryScheduleRepository) CreateSchedule(definition Definition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.definitions = append(r.definitions, definition)
	return nil
}
func (r *memoryScheduleRepository) ListSchedules() ([]Definition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Definition(nil), r.definitions...), nil
}
func (r *memoryScheduleRepository) UpsertScheduleOccurrence(occurrence Occurrence) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.occurrences == nil {
		r.occurrences = map[string]Occurrence{}
	}
	if old, ok := r.occurrences[occurrence.ID]; ok && occurrence.State == OccurrenceScheduled {
		occurrence = old
	}
	r.occurrences[occurrence.ID] = occurrence
	return nil
}
func (r *memoryScheduleRepository) TransitionScheduleOccurrence(id string, from, to OccurrenceState, runID, errorMessage string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.occurrences[id]
	if item.State != from {
		return false, nil
	}
	item.State, item.RunID, item.Error = to, runID, errorMessage
	r.occurrences[id] = item
	return true, nil
}
func (r *memoryScheduleRepository) UpdateScheduleFire(string, time.Time, time.Time) error { return nil }

func TestDurableSchedulerReloadsAndFiresOneShot(t *testing.T) {
	at := time.Now().Add(200 * time.Millisecond).UTC()
	repository := &memoryScheduleRepository{definitions: []Definition{{
		ID: "once", Kind: KindOnce, Expression: at.Format(time.RFC3339Nano), FlowID: "ship", Timezone: "UTC", Enabled: true,
	}}}
	fired := make(chan string, 1)
	manager := NewDurableScheduler(repository, func(_ context.Context, definition Definition, _ Occurrence) error {
		fired <- definition.FlowID
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer manager.Stop()
	select {
	case flow := <-fired:
		if flow != "ship" {
			t.Fatalf("flow = %q", flow)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reloaded one-shot did not fire")
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if len(repository.occurrences) != 1 {
		t.Fatalf("occurrences = %+v", repository.occurrences)
	}
	for _, occurrence := range repository.occurrences {
		if occurrence.State != OccurrenceSucceeded || occurrence.RunID == "" {
			t.Fatalf("occurrence = %+v", occurrence)
		}
	}
}

func TestDurableSchedulerHotRegistersNewDefinition(t *testing.T) {
	repository := &memoryScheduleRepository{}
	fired := make(chan struct{}, 1)
	manager := NewDurableScheduler(repository, func(context.Context, Definition, Occurrence) error {
		fired <- struct{}{}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	at := time.Now().Add(150 * time.Millisecond).UTC()
	if err := manager.Create(context.Background(), Definition{ID: "hot", Kind: KindOnce, Expression: at.Format(time.RFC3339Nano), FlowID: "brief", Timezone: "UTC", Enabled: true}); err != nil {
		t.Fatalf("create: %v", err)
	}
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("hot schedule did not fire")
	}
}
