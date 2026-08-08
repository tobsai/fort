package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memoryScheduleRepository struct {
	mu          sync.Mutex
	definitions []Definition
	occurrences map[string]Occurrence
	upsertErr   error
}

func TestNewDurableSchedulerRequiresDisplayLocation(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nil display location was accepted")
		}
	}()
	NewDurableScheduler(&memoryScheduleRepository{}, func(context.Context, Definition, Occurrence) error { return nil }, nil)
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
	if r.upsertErr != nil {
		return r.upsertErr
	}
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

type blockingCreateScheduleRepository struct {
	*memoryScheduleRepository
	createEntered chan struct{}
	releaseCreate chan struct{}
}

func (r *blockingCreateScheduleRepository) CreateSchedule(definition Definition) error {
	close(r.createEntered)
	<-r.releaseCreate
	return r.memoryScheduleRepository.CreateSchedule(definition)
}

type failingStartInterleavingRepository struct {
	*memoryScheduleRepository
	mu               sync.Mutex
	listCalls        int
	startListEntered chan struct{}
	releaseStartList chan struct{}
	createPersisted  chan struct{}
	createOnce       sync.Once
}

func (r *failingStartInterleavingRepository) CreateSchedule(definition Definition) error {
	err := r.memoryScheduleRepository.CreateSchedule(definition)
	r.createOnce.Do(func() { close(r.createPersisted) })
	return err
}

func (r *failingStartInterleavingRepository) ListSchedules() ([]Definition, error) {
	r.mu.Lock()
	r.listCalls++
	call := r.listCalls
	r.mu.Unlock()
	if call == 1 {
		close(r.startListEntered)
		<-r.releaseStartList
		return nil, errors.New("startup list failed")
	}
	return r.memoryScheduleRepository.ListSchedules()
}

func TestDurableSchedulerStopCannotOvertakeCreate(t *testing.T) {
	repository := &blockingCreateScheduleRepository{
		memoryScheduleRepository: &memoryScheduleRepository{},
		createEntered:            make(chan struct{}),
		releaseCreate:            make(chan struct{}),
	}
	manager := NewDurableScheduler(repository, func(context.Context, Definition, Occurrence) error { return nil }, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}

	type createResult struct {
		definition Definition
		err        error
	}
	created := make(chan createResult, 1)
	definition := Definition{
		ID: "create-during-stop", Kind: KindOnce, Expression: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano),
		FlowID: "brief", Timezone: "UTC", Enabled: true,
	}
	go func() {
		persisted, err := manager.Create(context.Background(), definition)
		created <- createResult{definition: persisted, err: err}
	}()
	select {
	case <-repository.createEntered:
	case <-time.After(time.Second):
		t.Fatal("Create did not reach durable persistence")
	}

	stopStarted := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		close(stopStarted)
		manager.Stop()
		close(stopped)
	}()
	<-stopStarted
	stopReturnedBeforeCreate := false
	select {
	case <-stopped:
		stopReturnedBeforeCreate = true
	case <-time.After(50 * time.Millisecond):
	}
	close(repository.releaseCreate)

	var result createResult
	select {
	case result = <-created:
	case <-time.After(time.Second):
		t.Fatal("Create did not finish after persistence was released")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not finish after Create completed")
	}
	if stopReturnedBeforeCreate {
		t.Fatal("Stop overtook an in-flight Create and could strand its registration")
	}
	if result.err != nil {
		t.Fatalf("serialized Create: %v", result.err)
	}
	if result.definition.ID != definition.ID || result.definition.NextFireAt.IsZero() {
		t.Fatalf("created definition = %+v, want normalized persisted definition", result.definition)
	}
}

func TestDurableSchedulerFailedStartCannotAbsorbConcurrentCreate(t *testing.T) {
	repository := &failingStartInterleavingRepository{
		memoryScheduleRepository: &memoryScheduleRepository{},
		startListEntered:         make(chan struct{}),
		releaseStartList:         make(chan struct{}),
		createPersisted:          make(chan struct{}),
	}
	manager := NewDurableScheduler(repository, func(context.Context, Definition, Occurrence) error { return nil }, time.UTC)
	t.Cleanup(manager.Stop)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	startResult := make(chan error, 1)
	go func() { startResult <- manager.Start(ctx) }()
	select {
	case <-repository.startListEntered:
	case <-time.After(time.Second):
		t.Fatal("Start did not reach the controlled repository failure")
	}

	createStarted := make(chan struct{})
	createResult := make(chan error, 1)
	go func() {
		close(createStarted)
		_, err := manager.Create(context.Background(), Definition{
			ID: "create-during-failed-start", Kind: KindOnce, Expression: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano),
			FlowID: "brief", Timezone: "UTC", Enabled: true,
		})
		createResult <- err
	}()
	<-createStarted
	persistedBeforeStartResolved := false
	select {
	case <-repository.createPersisted:
		persistedBeforeStartResolved = true
	case <-time.After(50 * time.Millisecond):
	}
	close(repository.releaseStartList)

	select {
	case err := <-startResult:
		if err == nil {
			t.Fatal("controlled startup failure returned nil")
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return after repository release")
	}
	select {
	case err := <-createResult:
		if err == nil {
			t.Fatal("Create succeeded inside a Start that rolled back")
		}
	case <-time.After(time.Second):
		t.Fatal("Create did not resolve after failed Start rolled back")
	}
	if persistedBeforeStartResolved {
		t.Fatal("Create persisted while Start was still provisional")
	}
	repository.memoryScheduleRepository.mu.Lock()
	defer repository.memoryScheduleRepository.mu.Unlock()
	if len(repository.definitions) != 0 {
		t.Fatalf("failed Start absorbed durable definitions: %+v", repository.definitions)
	}
}

func TestDurableSchedulerFailedStartIsQuiescentAndRetryable(t *testing.T) {
	at := time.Now().Add(time.Second).UTC()
	repository := &memoryScheduleRepository{
		definitions: []Definition{{
			ID: "retry-start", Kind: KindOnce, Expression: at.Format(time.RFC3339Nano),
			FlowID: "brief", Timezone: "UTC", Enabled: true,
		}},
		upsertErr: errors.New("projection write failed"),
	}
	fired := make(chan struct{}, 1)
	manager := NewDurableScheduler(repository, func(context.Context, Definition, Occurrence) error {
		fired <- struct{}{}
		return nil
	}, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := manager.Start(ctx); err == nil {
		t.Fatal("initial materialization failure did not fail Start")
	}
	manager.mu.Lock()
	started, registered := manager.started, len(manager.registered)
	manager.mu.Unlock()
	if started || registered != 0 || len(manager.clock.c.Entries()) != 0 {
		t.Fatalf("failed Start remained live: started=%t registered=%d cron_entries=%d", started, registered, len(manager.clock.c.Entries()))
	}
	select {
	case <-fired:
		t.Fatal("failed Start dispatched a schedule")
	default:
	}

	repository.mu.Lock()
	repository.upsertErr = nil
	repository.mu.Unlock()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("retry Start: %v", err)
	}
	t.Cleanup(manager.Stop)
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("schedule was not rearmed after retrying Start")
	}
}

func TestDurableSchedulerCreateKeepsSuccessAfterProjectionFailure(t *testing.T) {
	repository := &memoryScheduleRepository{}
	manager := NewDurableScheduler(repository, func(context.Context, Definition, Occurrence) error { return nil }, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Stop)

	repository.mu.Lock()
	repository.upsertErr = errors.New("projection write failed")
	repository.mu.Unlock()
	at := time.Now().Add(time.Hour).UTC()
	definition := Definition{
		ID: "durable-despite-projection", Kind: KindOnce, Expression: at.Format(time.RFC3339Nano),
		FlowID: "brief", Timezone: "UTC", Enabled: true,
	}
	created, err := manager.Create(context.Background(), definition)
	if err != nil {
		t.Fatalf("durable registered schedule reported projection failure: %v", err)
	}
	if created.ID != definition.ID || created.NextFireAt.IsZero() || created.CreatedAt.IsZero() {
		t.Fatalf("created schedule = %+v, want persisted projection", created)
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if len(repository.definitions) != 1 || repository.definitions[0].ID != definition.ID {
		t.Fatalf("durable definitions = %+v", repository.definitions)
	}
	manager.mu.Lock()
	registered := manager.registered[definition.ID]
	manager.mu.Unlock()
	if !registered {
		t.Fatal("successfully created schedule was not registered")
	}
}

func TestDurableSchedulerIsTerminalAfterStop(t *testing.T) {
	repository := &memoryScheduleRepository{}
	manager := NewDurableScheduler(repository, func(context.Context, Definition, Occurrence) error { return nil }, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	manager.Stop()

	if err := manager.Start(ctx); err == nil {
		manager.Stop()
		t.Fatal("stopped scheduler restarted and could duplicate registered jobs")
	}
	definition := Definition{
		ID: "after-stop", Kind: KindOnce, Expression: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano),
		FlowID: "brief", Timezone: "UTC", Enabled: true,
	}
	if _, err := manager.Create(context.Background(), definition); err == nil {
		t.Fatal("stopped scheduler accepted a schedule it cannot own")
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if len(repository.definitions) != 0 {
		t.Fatalf("stopped scheduler persisted definitions: %+v", repository.definitions)
	}
}

func TestDurableSchedulerReloadsAndFiresOneShot(t *testing.T) {
	at := time.Now().Add(200 * time.Millisecond).UTC()
	repository := &memoryScheduleRepository{definitions: []Definition{{
		ID: "once", Kind: KindOnce, Expression: at.Format(time.RFC3339Nano), FlowID: "ship", Timezone: "UTC", Enabled: true,
	}}}
	fired := make(chan string, 1)
	manager := NewDurableScheduler(repository, func(_ context.Context, definition Definition, _ Occurrence) error {
		fired <- definition.FlowID
		return nil
	}, time.UTC)
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
	deadline := time.Now().Add(time.Second)
	for {
		repository.mu.Lock()
		occurrences := make([]Occurrence, 0, len(repository.occurrences))
		for _, occurrence := range repository.occurrences {
			occurrences = append(occurrences, occurrence)
		}
		repository.mu.Unlock()
		if len(occurrences) == 1 && occurrences[0].State == OccurrenceSucceeded && occurrences[0].RunID != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("occurrences did not reach succeeded state: %+v", occurrences)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestDurableSchedulerHotRegistersNewDefinition(t *testing.T) {
	repository := &memoryScheduleRepository{}
	fired := make(chan struct{}, 1)
	manager := NewDurableScheduler(repository, func(context.Context, Definition, Occurrence) error {
		fired <- struct{}{}
		return nil
	}, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	at := time.Now().Add(150 * time.Millisecond).UTC()
	if _, err := manager.Create(context.Background(), Definition{ID: "hot", Kind: KindOnce, Expression: at.Format(time.RFC3339Nano), FlowID: "brief", Timezone: "UTC", Enabled: true}); err != nil {
		t.Fatalf("create: %v", err)
	}
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("hot schedule did not fire")
	}
}

func TestDurableSchedulerOverlappingCronCallbacksClaimCapturedOccurrencesOnce(t *testing.T) {
	firstPlanned := time.Date(2026, 8, 3, 3, 0, 0, 0, time.UTC)
	secondPlanned := firstPlanned.Add(time.Minute)
	repository := &memoryScheduleRepository{}
	definition := Definition{
		ID: "overlapping-cron", Kind: KindCron, Expression: "0 * * * * *",
		FlowID: "minute-brief", Timezone: "UTC", Enabled: true,
	}
	for _, scheduledFor := range []time.Time{firstPlanned, secondPlanned} {
		if err := repository.UpsertScheduleOccurrence(Occurrence{
			ID: OccurrenceID(definition.ID, scheduledFor), ScheduleID: definition.ID,
			ScheduledFor: scheduledFor, State: OccurrenceScheduled,
		}); err != nil {
			t.Fatal(err)
		}
	}

	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	abort := make(chan struct{})
	started := make(chan Occurrence, 2)
	finished := make(chan Occurrence, 2)
	type launchRequest struct{ start chan struct{} }
	launches := make(chan launchRequest, 2)
	manager := NewDurableScheduler(repository, func(_ context.Context, _ Definition, occurrence Occurrence) error {
		var release <-chan struct{}
		switch {
		case occurrence.ScheduledFor.Equal(firstPlanned):
			release = releaseFirst
		case occurrence.ScheduledFor.Equal(secondPlanned):
			release = releaseSecond
		default:
			return errors.New("unexpected planned occurrence")
		}
		select {
		case started <- occurrence:
		case <-abort:
			return nil
		}
		select {
		case <-release:
		case <-abort:
			return nil
		}
		select {
		case finished <- occurrence:
		case <-abort:
		}
		return nil
	}, time.UTC)
	wait := newControlledCronWait()
	manager.clock.cronNow = func() time.Time { return firstPlanned.Add(-time.Second) }
	manager.clock.cronWait = wait.wait
	manager.now = func() time.Time { return secondPlanned.Add(2 * time.Second) }
	t.Cleanup(func() {
		close(abort)
		manager.clock.Stop()
	})
	if err := manager.register(definition); err != nil {
		t.Fatal(err)
	}
	if len(manager.clock.plannedCron) != 1 {
		t.Fatalf("planned cron jobs = %d, want one", len(manager.clock.plannedCron))
	}
	manager.clock.plannedCron[0].launch = func(callback func()) {
		request := launchRequest{start: make(chan struct{})}
		launches <- request
		go func() {
			select {
			case <-request.start:
			case <-abort:
			}
			callback()
		}()
	}
	if entries := manager.clock.c.Entries(); len(entries) != 0 {
		t.Fatalf("durable planned cron leaked through robfig inspection: %+v", entries)
	}
	manager.clock.Start()

	firstWait := awaitCronWait(t, wait)
	if !firstWait.plannedFor.Equal(firstPlanned) {
		t.Fatalf("first planned time = %s, want %s", firstWait.plannedFor, firstPlanned)
	}
	// The callback is observed two seconds late, but it must retain the planned
	// occurrence identity while the loop advances to the next cron occurrence.
	firstWait.observedAt <- firstPlanned.Add(2 * time.Second)
	secondWait := awaitCronWait(t, wait)
	if !secondWait.plannedFor.Equal(secondPlanned) {
		t.Fatalf("second planned time = %s, want %s", secondWait.plannedFor, secondPlanned)
	}
	secondWait.observedAt <- secondPlanned
	awaitLaunch := func() launchRequest {
		t.Helper()
		select {
		case request := <-launches:
			return request
		case <-time.After(time.Second):
			t.Fatal("planned callback was not launched")
			return launchRequest{}
		}
	}
	firstLaunch := awaitLaunch()
	secondLaunch := awaitLaunch()

	// Invoke the second callback before the first, exactly as out-of-order
	// goroutine scheduling can. Each closure must retain its own planned time.
	close(secondLaunch.start)
	select {
	case occurrence := <-started:
		if !occurrence.ScheduledFor.Equal(secondPlanned) || occurrence.ID != OccurrenceID(definition.ID, secondPlanned) {
			t.Fatalf("first observed callback = %+v, want captured second occurrence", occurrence)
		}
	case <-time.After(time.Second):
		t.Fatal("second callback did not overtake the delayed first callback")
	}
	close(firstLaunch.start)
	select {
	case occurrence := <-started:
		if !occurrence.ScheduledFor.Equal(firstPlanned) || occurrence.ID != OccurrenceID(definition.ID, firstPlanned) {
			t.Fatalf("second observed callback = %+v, want captured first occurrence", occurrence)
		}
	case <-time.After(time.Second):
		t.Fatal("first callback did not start after release")
	}

	close(releaseSecond)
	select {
	case occurrence := <-finished:
		if !occurrence.ScheduledFor.Equal(secondPlanned) {
			t.Fatalf("first completed callback = %+v, want second occurrence", occurrence)
		}
	case <-time.After(time.Second):
		t.Fatal("second callback did not complete")
	}
	close(releaseFirst)
	select {
	case occurrence := <-finished:
		if !occurrence.ScheduledFor.Equal(firstPlanned) {
			t.Fatalf("second completed callback = %+v, want first occurrence", occurrence)
		}
	case <-time.After(time.Second):
		t.Fatal("first callback did not complete")
	}

	deadline := time.Now().Add(time.Second)
	for {
		repository.mu.Lock()
		first := repository.occurrences[OccurrenceID(definition.ID, firstPlanned)]
		second := repository.occurrences[OccurrenceID(definition.ID, secondPlanned)]
		repository.mu.Unlock()
		if first.State == OccurrenceSucceeded && second.State == OccurrenceSucceeded {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("captured occurrences did not finish: first=%+v second=%+v", first, second)
		}
		time.Sleep(time.Millisecond)
	}

	repository.mu.Lock()
	defer repository.mu.Unlock()
	if len(repository.occurrences) != 2 {
		t.Fatalf("occurrences = %+v, want exactly two planned claims", repository.occurrences)
	}
	for _, scheduledFor := range []time.Time{firstPlanned, secondPlanned} {
		id := OccurrenceID(definition.ID, scheduledFor)
		occurrence := repository.occurrences[id]
		if occurrence.ID != id || !occurrence.ScheduledFor.Equal(scheduledFor) || occurrence.RunID != "schedule-"+id || occurrence.State != OccurrenceSucceeded {
			t.Fatalf("captured occurrence = %+v, want one succeeded claim for %s", occurrence, scheduledFor)
		}
	}
}

func TestDurableSchedulerDoesNotOverwriteFiredStateFromLinkedRun(t *testing.T) {
	repository := &memoryScheduleRepository{}
	manager := NewDurableScheduler(repository, func(_ context.Context, _ Definition, occurrence Occurrence) error {
		changed, err := repository.TransitionScheduleOccurrence(occurrence.ID, OccurrenceRunning, OccurrenceFired, occurrence.RunID, "")
		if err != nil || !changed {
			t.Fatalf("linked run did not mark occurrence fired: changed=%v err=%v", changed, err)
		}
		return nil
	}, time.UTC)
	scheduledFor := time.Date(2026, 8, 3, 3, 0, 0, 0, time.UTC)
	manager.fire(context.Background(), Definition{ID: "gated", Kind: KindOnce, FlowID: "ship-feature", Timezone: "UTC"}, scheduledFor)

	repository.mu.Lock()
	defer repository.mu.Unlock()
	occurrence := repository.occurrences[OccurrenceID("gated", scheduledFor)]
	if occurrence.State != OccurrenceFired {
		t.Fatalf("occurrence = %+v, want fired after gated run paused", occurrence)
	}
}

func TestDurableSchedulerClaimsSharedOccurrenceExactlyOnceAcrossInstances(t *testing.T) {
	repository := &memoryScheduleRepository{}
	triggered := make(chan Occurrence, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseTrigger := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseTrigger)
	trigger := func(_ context.Context, _ Definition, occurrence Occurrence) error {
		triggered <- occurrence
		<-release
		return nil
	}
	first := NewDurableScheduler(repository, trigger, time.UTC)
	second := NewDurableScheduler(repository, trigger, time.UTC)
	now := time.Date(2026, 8, 3, 3, 0, 1, 0, time.UTC)
	first.now = func() time.Time { return now }
	second.now = func() time.Time { return now }
	scheduledFor := now.Add(-time.Second)
	definition := Definition{
		ID: "restart-shared", Kind: KindOnce, Expression: scheduledFor.Format(time.RFC3339),
		FlowID: "ship-once", Timezone: "UTC", Enabled: true,
	}

	start := make(chan struct{})
	done := make(chan string, 2)
	for _, item := range []struct {
		name    string
		manager *DurableScheduler
	}{{"first", first}, {"second", second}} {
		item := item
		go func() {
			<-start
			item.manager.fire(context.Background(), definition, scheduledFor)
			done <- item.name
		}()
	}
	close(start)

	var claimed Occurrence
	select {
	case claimed = <-triggered:
	case <-time.After(time.Second):
		t.Fatal("neither scheduler instance claimed the shared occurrence")
	}
	select {
	case <-done:
		// The unblocked fire call lost the Scheduled -> Running claim. The
		// winning trigger remains behind release until that is proven.
	case <-time.After(time.Second):
		t.Fatal("duplicate fire did not return after losing the shared claim")
	}
	select {
	case duplicate := <-triggered:
		t.Fatalf("shared occurrence triggered twice: first=%+v duplicate=%+v", claimed, duplicate)
	default:
	}

	releaseTrigger()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("claimed fire did not finish after trigger release")
	}
	select {
	case duplicate := <-triggered:
		t.Fatalf("shared occurrence triggered again after completion: first=%+v duplicate=%+v", claimed, duplicate)
	default:
	}

	repository.mu.Lock()
	defer repository.mu.Unlock()
	if len(repository.occurrences) != 1 {
		t.Fatalf("occurrences = %+v, want one shared durable occurrence", repository.occurrences)
	}
	persisted := repository.occurrences[OccurrenceID(definition.ID, scheduledFor)]
	if claimed.ID != persisted.ID || claimed.RunID != persisted.RunID || persisted.State != OccurrenceSucceeded {
		t.Fatalf("claimed=%+v persisted=%+v, want one succeeded claim", claimed, persisted)
	}
}

func TestDurableSchedulerOwnsConfiguredDisplayDayMaterialization(t *testing.T) {
	location, err := time.LoadLocation("Pacific/Honolulu")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2099, 8, 2, 23, 0, 0, 0, time.UTC)
	preexisting := Definition{
		ID: "preexisting", Kind: KindOnce, Expression: "2099-08-02T20:00:00",
		FlowID: "brief", Timezone: "Pacific/Honolulu", Enabled: true,
	}
	tomorrow := Definition{
		ID: "tomorrow", Kind: KindOnce, Expression: "2099-08-03T20:00:00",
		FlowID: "tomorrow-brief", Timezone: "Pacific/Honolulu", Enabled: true,
	}
	repository := &memoryScheduleRepository{definitions: []Definition{preexisting, tomorrow}}
	manager := NewDurableScheduler(repository, func(context.Context, Definition, Occurrence) error { return nil }, location)
	manager.now = func() time.Time { return now }
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Stop)

	preexistingAt := time.Date(2099, 8, 3, 6, 0, 0, 0, time.UTC)
	repository.mu.Lock()
	_, foundAtStart := repository.occurrences[OccurrenceID(preexisting.ID, preexistingAt)]
	repository.mu.Unlock()
	if !foundAtStart {
		t.Fatalf("startup did not materialize %s in configured display day", preexistingAt)
	}
	tomorrowAt := time.Date(2099, 8, 4, 6, 0, 0, 0, time.UTC)
	repository.mu.Lock()
	_, foundTomorrow := repository.occurrences[OccurrenceID(tomorrow.ID, tomorrowAt)]
	repository.mu.Unlock()
	if !foundTomorrow {
		t.Fatalf("startup did not prepare following display day occurrence %s", tomorrowAt)
	}

	hot := Definition{
		ID: "hot", Kind: KindOnce, Expression: "2099-08-02T21:00:00",
		FlowID: "summary", Timezone: "Pacific/Honolulu", Enabled: true,
	}
	if _, err := manager.Create(context.Background(), hot); err != nil {
		t.Fatal(err)
	}
	hotAt := time.Date(2099, 8, 3, 7, 0, 0, 0, time.UTC)
	repository.mu.Lock()
	_, foundAfterCreate := repository.occurrences[OccurrenceID(hot.ID, hotAt)]
	repository.mu.Unlock()
	if !foundAfterCreate {
		t.Fatalf("hot create did not materialize %s in configured display day", hotAt)
	}
}

func TestDayMaterializationCronUsesConfiguredLocalMidnight(t *testing.T) {
	location, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := dayMaterializationCron(location), "CRON_TZ=America/Chicago 0 0 0 * * *"; got != want {
		t.Fatalf("day materialization cron = %q, want %q", got, want)
	}
	manager := NewDurableScheduler(&memoryScheduleRepository{}, func(context.Context, Definition, Occurrence) error { return nil }, location)
	manager.now = func() time.Time { return time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC) }
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Stop)
	entries := manager.clock.c.Entries()
	if len(entries) != 1 {
		t.Fatalf("registered cron entries = %d, want one day-boundary owner", len(entries))
	}
	after := time.Date(2026, 8, 2, 4, 59, 59, 0, time.UTC)
	wantNext := time.Date(2026, 8, 2, 5, 0, 0, 0, time.UTC)
	if next := entries[0].Schedule.Next(after); !next.Equal(wantNext) {
		t.Fatalf("next configured midnight = %s, want %s", next, wantNext)
	}
}
