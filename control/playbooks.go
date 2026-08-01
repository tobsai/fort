package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/tobsai/fort/core/playbook"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/ui"
)

// PlaybookCatalog adapts immutable store revisions to the UI playbook port.
// Definitions remain encoded as their UI wire form so display-only fields such
// as stage descriptions survive validation and round trips unchanged.
type PlaybookCatalog struct {
	store *store.Store
	mu    sync.Mutex
	init  error
}

var _ ui.PlaybookCatalog = (*PlaybookCatalog)(nil)

// NewPlaybookCatalog builds the durable playbook catalog and atomically seeds
// it before any route preview can be served. Routing remains read-only.
func NewPlaybookCatalog(st *store.Store) *PlaybookCatalog {
	catalog := &PlaybookCatalog{store: st}
	catalog.init = catalog.initialize()
	return catalog
}

// List returns the latest immutable revision for every playbook.
func (c *PlaybookCatalog) List(ctx context.Context) ([]ui.Playbook, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if err := c.init; err != nil {
		return nil, err
	}
	return c.latestLocked()
}

// Save validates the prospective latest catalog before appending a revision.
func (c *PlaybookCatalog) Save(ctx context.Context, definition ui.Playbook) (ui.Playbook, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := contextErr(ctx); err != nil {
		return ui.Playbook{}, err
	}
	if err := c.init; err != nil {
		return ui.Playbook{}, err
	}
	return c.saveLocked(definition)
}

// Duplicate appends a disabled, non-default copy with a fresh deterministic
// slug. A duplicate cannot change automatic routing until explicitly enabled.
func (c *PlaybookCatalog) Duplicate(ctx context.Context, id string) (ui.Playbook, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := contextErr(ctx); err != nil {
		return ui.Playbook{}, err
	}
	if err := c.init; err != nil {
		return ui.Playbook{}, err
	}
	items, err := c.latestLocked()
	if err != nil {
		return ui.Playbook{}, err
	}
	var source *ui.Playbook
	used := make(map[string]bool, len(items))
	for i := range items {
		used[items[i].ID] = true
		if items[i].ID == id {
			copy := items[i]
			source = &copy
		}
	}
	if source == nil {
		return ui.Playbook{}, fmt.Errorf("control: unknown playbook %q", id)
	}
	copyID := id + "-copy"
	for n := 2; used[copyID]; n++ {
		copyID = fmt.Sprintf("%s-copy-%d", id, n)
	}
	copy := *source
	copy.ID = copyID
	copy.Name = source.Name + " copy"
	copy.Revision = 1
	copy.IsDefault = false
	copy.Trigger.Enabled = false
	return c.saveLocked(copy)
}

// Route resolves a pure deterministic preview. An explicit revision loads that
// exact immutable row; the optional plan gate override only changes the
// returned execution snapshot and never persists a definition.
func (c *PlaybookCatalog) Route(ctx context.Context, request ui.RouteRequest) (ui.RoutePreview, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := contextErr(ctx); err != nil {
		return ui.RoutePreview{}, err
	}
	if err := c.init; err != nil {
		return ui.RoutePreview{}, err
	}

	var catalog playbook.Catalog
	if request.PlaybookRevision > 0 {
		if request.PlaybookID == "" {
			return ui.RoutePreview{}, fmt.Errorf("control: playbook id is required with an explicit revision")
		}
		revision, err := c.store.PlaybookRevision(request.PlaybookID, request.PlaybookRevision)
		if err != nil {
			return ui.RoutePreview{}, fmt.Errorf("control: load playbook %s revision %d: %w", request.PlaybookID, request.PlaybookRevision, err)
		}
		definition, err := decodePlaybookRevision(revision)
		if err != nil {
			return ui.RoutePreview{}, err
		}
		selected := toCorePlaybook(definition)
		// Resolve validates a whole catalog, including exactly one default. The
		// explicit selection does not consult that flag, so a one-item immutable
		// snapshot can safely carry it solely to satisfy catalog validation.
		selected.IsDefault = true
		catalog.Playbooks = []playbook.Playbook{selected}
	} else {
		items, err := c.latestLocked()
		if err != nil {
			return ui.RoutePreview{}, err
		}
		catalog = toCoreCatalog(items)
	}

	resolved, err := catalog.Resolve(playbook.RouteRequest{
		Direction:  request.Text,
		TaskType:   playbook.TaskType(request.TaskType),
		PlaybookID: request.PlaybookID,
	})
	if err != nil {
		return ui.RoutePreview{}, err
	}
	if request.PlanGate != nil {
		if resolved.Delivery == playbook.DeliveryAnswer && *request.PlanGate {
			return ui.RoutePreview{}, fmt.Errorf("control: answer route cannot enable a plan gate")
		}
		resolved.PlanGate = *request.PlanGate
	}
	return toUIRoute(resolved), nil
}

func (c *PlaybookCatalog) initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	defaults := playbook.DefaultCatalog()
	if err := playbook.Validate(defaults); err != nil {
		return fmt.Errorf("control: invalid default playbook catalog: %w", err)
	}
	rows, err := c.store.LatestPlaybookRevisions()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		seed := make([]store.PlaybookRevision, 0, len(defaults.Playbooks))
		for _, definition := range defaults.Playbooks {
			wire := toUIPlaybook(definition)
			data, err := json.Marshal(wire)
			if err != nil {
				return err
			}
			seed = append(seed, store.PlaybookRevision{ID: wire.ID, Revision: wire.Revision, Data: string(data)})
		}
		if err := c.store.SeedPlaybookRevisions(seed); err != nil {
			return err
		}
	}
	items, err := c.latestLocked()
	if err != nil {
		return err
	}
	if err := playbook.Validate(toCoreCatalog(items)); err != nil {
		return fmt.Errorf("control: invalid persisted playbook catalog: %w", err)
	}
	if err := c.migrateLegacyDefaults(items); err != nil {
		return err
	}
	items, err = c.latestLocked()
	if err != nil {
		return err
	}
	if err := playbook.Validate(toCoreCatalog(items)); err != nil {
		return fmt.Errorf("control: invalid migrated playbook catalog: %w", err)
	}
	return nil
}

func (c *PlaybookCatalog) migrateLegacyDefaults(items []ui.Playbook) error {
	legacy := make(map[string]ui.Playbook)
	for _, definition := range playbook.LegacyDefaultCatalogRevision1().Playbooks {
		legacy[definition.ID] = toUIPlaybook(definition)
	}
	interim := make(map[string]ui.Playbook)
	for _, definition := range playbook.InterimConfiguredDefaultCatalog().Playbooks {
		interim[definition.ID] = toUIPlaybook(definition)
	}
	gpt55 := make(map[string]ui.Playbook)
	for _, definition := range playbook.LegacyGPT55DefaultCatalog().Playbooks {
		gpt55[definition.ID] = toUIPlaybook(definition)
	}
	current := make(map[string]ui.Playbook)
	for _, definition := range playbook.DefaultCatalog().Playbooks {
		current[definition.ID] = toUIPlaybook(definition)
	}

	for _, item := range items {
		after, currentKnown := current[item.ID]
		if !currentKnown {
			continue
		}
		currentAtLatest := after
		currentAtLatest.Revision = item.Revision
		if reflect.DeepEqual(item, currentAtLatest) {
			continue
		}

		knownPredecessor := false
		if item.Revision == 1 {
			before := legacy[item.ID]
			knownPredecessor = reflect.DeepEqual(item, before)
		}
		if !knownPredecessor && (item.Revision == 1 || item.Revision == 2) {
			before := interim[item.ID]
			before.Revision = item.Revision
			knownPredecessor = reflect.DeepEqual(item, before)
		}
		if !knownPredecessor && (item.Revision == 1 || item.Revision == 2) {
			before := gpt55[item.ID]
			before.Revision = item.Revision
			knownPredecessor = reflect.DeepEqual(item, before)
		}
		if !knownPredecessor {
			continue
		}

		after.Revision = item.Revision + 1
		data, err := json.Marshal(after)
		if err != nil {
			return err
		}
		if _, err := c.store.SavePlaybookRevisionIfLatest(item.ID, item.Revision, string(data)); err != nil {
			// Another catalog instance or an editor may have appended first. In
			// either case the immutable user/newer revision wins.
			if errors.Is(err, store.ErrPlaybookRevisionStale) {
				continue
			}
			return fmt.Errorf("control: migrate default playbook %q: %w", item.ID, err)
		}
	}
	return nil
}

func (c *PlaybookCatalog) latestLocked() ([]ui.Playbook, error) {
	rows, err := c.store.LatestPlaybookRevisions()
	if err != nil {
		return nil, err
	}
	items := make([]ui.Playbook, 0, len(rows))
	for _, row := range rows {
		item, err := decodePlaybookRevision(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDefault != items[j].IsDefault {
			return items[i].IsDefault
		}
		left, right := strings.ToLower(items[i].Name), strings.ToLower(items[j].Name)
		if left != right {
			return left < right
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (c *PlaybookCatalog) saveLocked(definition ui.Playbook) (ui.Playbook, error) {
	items, err := c.latestLocked()
	if err != nil {
		return ui.Playbook{}, err
	}
	nextRevision := 1
	expectedRevision := 0
	replaced := false
	for i := range items {
		if items[i].ID == definition.ID {
			if definition.Revision != items[i].Revision {
				return ui.Playbook{}, fmt.Errorf("control: stale revision for playbook %q: latest %d, submitted %d: %w", definition.ID, items[i].Revision, definition.Revision, store.ErrPlaybookRevisionStale)
			}
			expectedRevision = items[i].Revision
			nextRevision = items[i].Revision + 1
			replaced = true
			break
		}
	}
	definition.Revision = nextRevision
	if replaced {
		for i := range items {
			if items[i].ID == definition.ID {
				items[i] = definition
			}
		}
	} else {
		items = append(items, definition)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if err := playbook.Validate(toCoreCatalog(items)); err != nil {
		return ui.Playbook{}, err
	}
	data, err := json.Marshal(definition)
	if err != nil {
		return ui.Playbook{}, err
	}
	revision, err := c.store.SavePlaybookRevisionIfLatest(definition.ID, expectedRevision, string(data))
	if err != nil {
		return ui.Playbook{}, err
	}
	if revision.Revision != definition.Revision {
		return ui.Playbook{}, fmt.Errorf("control: saved playbook %q as revision %d, want %d", definition.ID, revision.Revision, definition.Revision)
	}
	return definition, nil
}

func decodePlaybookRevision(revision store.PlaybookRevision) (ui.Playbook, error) {
	var definition ui.Playbook
	if err := json.Unmarshal([]byte(revision.Data), &definition); err != nil {
		return ui.Playbook{}, fmt.Errorf("control: decode playbook %s revision %d: %w", revision.ID, revision.Revision, err)
	}
	definition.ID = revision.ID
	definition.Revision = revision.Revision
	return definition, nil
}

func toCoreCatalog(items []ui.Playbook) playbook.Catalog {
	catalog := playbook.Catalog{Playbooks: make([]playbook.Playbook, 0, len(items))}
	for _, item := range items {
		catalog.Playbooks = append(catalog.Playbooks, toCorePlaybook(item))
	}
	return catalog
}

func toCorePlaybook(in ui.Playbook) playbook.Playbook {
	out := playbook.Playbook{
		ID: in.ID, Name: in.Name, Revision: in.Revision,
		IsDefault: in.IsDefault, PlanGate: in.PlanGate,
		Trigger:  playbook.Trigger{Kind: playbook.TaskType(in.Trigger.Kind), Enabled: in.Trigger.Enabled},
		Delivery: playbook.Delivery(in.Delivery),
		Stages:   make([]playbook.Stage, 0, len(in.Stages)),
	}
	for _, stage := range in.Stages {
		converted := playbook.Stage{
			Order: stage.Order, Name: stage.Name, Prompt: stage.Prompt, Description: stage.Description, Memory: stage.Memory,
			Assignments: make([]playbook.Assignment, 0, len(stage.Assignments)),
		}
		for _, assignment := range stage.Assignments {
			converted.Assignments = append(converted.Assignments, playbook.Assignment{
				TaskType: playbook.TaskType(assignment.TaskType), Profile: assignment.Profile, Agent: assignment.Agent, Model: assignment.Model,
			})
		}
		out.Stages = append(out.Stages, converted)
	}
	return out
}

func toUIPlaybook(in playbook.Playbook) ui.Playbook {
	out := ui.Playbook{
		ID: in.ID, Name: in.Name, Revision: in.Revision,
		IsDefault: in.IsDefault, PlanGate: in.PlanGate,
		Trigger:  ui.PlaybookTrigger{Kind: string(in.Trigger.Kind), Enabled: in.Trigger.Enabled},
		Delivery: string(in.Delivery),
		Stages:   make([]ui.PlaybookStage, 0, len(in.Stages)),
	}
	for _, stage := range in.Stages {
		converted := ui.PlaybookStage{
			Order: stage.Order, Name: stage.Name, Prompt: stage.Prompt, Description: stage.Description, Memory: stage.Memory,
			Assignments: make([]ui.PlaybookAssignment, 0, len(stage.Assignments)),
		}
		for _, assignment := range stage.Assignments {
			converted.Assignments = append(converted.Assignments, ui.PlaybookAssignment{
				TaskType: string(assignment.TaskType), Profile: assignment.Profile, Agent: assignment.Agent, Model: assignment.Model,
			})
		}
		out.Stages = append(out.Stages, converted)
	}
	return out
}

func toUIRoute(in playbook.ResolvedRoute) ui.RoutePreview {
	out := ui.RoutePreview{
		PlaybookID: in.PlaybookID, PlaybookRevision: in.PlaybookRevision,
		PlaybookName: in.PlaybookName, TaskType: string(in.TaskType),
		Source: string(in.Source), PlanGate: in.PlanGate, Delivery: string(in.Delivery),
		Stages: make([]ui.ResolvedPlaybookStage, 0, len(in.Stages)),
	}
	for _, stage := range in.Stages {
		out.Stages = append(out.Stages, ui.ResolvedPlaybookStage{
			Order: stage.Order, Name: stage.Name, Prompt: stage.Prompt,
			Profile: stage.Profile, Agent: stage.Agent, Model: stage.Model, Memory: stage.Memory,
		})
	}
	return out
}

func contextErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func firstNonemptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
