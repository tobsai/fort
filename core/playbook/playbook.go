// Package playbook defines Fort's reusable, deterministic agent pipelines.
// Resolution is a pure function of an explicit override, an explicit task type,
// or fixed text rules. It never calls a model.
package playbook

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tobsai/fort/core/capability"
)

// TaskType is the deterministic classification signal used by triggers and
// per-stage assignment branches.
type TaskType string

const (
	TaskQuestion TaskType = "question"
	TaskBug      TaskType = "bug"
	TaskResearch TaskType = "research"
	TaskFeature  TaskType = "feature"
	TaskManual   TaskType = "manual"
)

// Delivery controls whether the resolved pipeline creates an assignment or
// returns an answer outside the assignment board.
type Delivery string

const (
	DeliveryAssignment Delivery = "assignment"
	DeliveryAnswer     Delivery = "answer"
)

// RouteSource explains why a playbook was selected.
type RouteSource string

const (
	SourceManual  RouteSource = "manual"
	SourceTrigger RouteSource = "trigger"
	SourceDefault RouteSource = "default"
)

// Trigger binds one classified task type to a playbook. Disabled triggers are
// ignored during automatic resolution but do not prevent explicit selection.
type Trigger struct {
	Kind    TaskType `json:"kind" yaml:"kind"`
	Enabled bool     `json:"enabled" yaml:"enabled"`
}

// Assignment selects an agent and optional model for one task type. An empty
// TaskType is the required default branch for its stage.
type Assignment struct {
	TaskType TaskType `json:"task_type,omitempty" yaml:"task_type,omitempty"`
	Profile  string   `json:"profile,omitempty" yaml:"profile,omitempty"`
	Agent    string   `json:"agent" yaml:"agent"`
	Model    string   `json:"model,omitempty" yaml:"model,omitempty"`
}

// Stage is one ordered step in a playbook.
type Stage struct {
	Order       int          `json:"order" yaml:"order"`
	Name        string       `json:"name" yaml:"name"`
	Prompt      string       `json:"prompt,omitempty" yaml:"prompt,omitempty"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	Assignments []Assignment `json:"assignments" yaml:"assignments"`
	Memory      bool         `json:"memory" yaml:"memory"`
}

// Playbook is an immutable revision of a reusable pipeline. Persistence owns
// revision creation; Resolve carries the selected revision into its result.
type Playbook struct {
	ID        string   `json:"id" yaml:"id"`
	Name      string   `json:"name" yaml:"name"`
	Revision  int      `json:"revision" yaml:"revision"`
	IsDefault bool     `json:"is_default" yaml:"is_default"`
	PlanGate  bool     `json:"plan_gate" yaml:"plan_gate"`
	Trigger   Trigger  `json:"trigger" yaml:"trigger"`
	Delivery  Delivery `json:"delivery" yaml:"delivery"`
	Stages    []Stage  `json:"stages" yaml:"stages"`
}

// Catalog is the currently-addressable set of immutable playbook revisions.
type Catalog struct {
	Playbooks []Playbook `json:"playbooks" yaml:"playbooks"`
}

// RouteRequest is the pure resolver input. PlaybookID has highest precedence;
// TaskType, when present, takes precedence over fixed text classification.
type RouteRequest struct {
	Direction  string   `json:"direction"`
	TaskType   TaskType `json:"task_type,omitempty"`
	PlaybookID string   `json:"playbook_id,omitempty"`
}

// ResolvedStage is a stage with its task-type branch selected.
type ResolvedStage struct {
	Order   int    `json:"order"`
	Name    string `json:"name"`
	Prompt  string `json:"prompt,omitempty"`
	Profile string `json:"profile,omitempty"`
	Agent   string `json:"agent"`
	Model   string `json:"model,omitempty"`
	Memory  bool   `json:"memory"`
}

// ResolvedRoute is an immutable execution snapshot suitable for previewing or
// compiling. PlaybookRevision prevents later catalog edits from changing it.
type ResolvedRoute struct {
	PlaybookID       string          `json:"playbook_id"`
	PlaybookRevision int             `json:"playbook_revision"`
	PlaybookName     string          `json:"playbook_name"`
	TaskType         TaskType        `json:"task_type"`
	Source           RouteSource     `json:"source"`
	Delivery         Delivery        `json:"delivery"`
	PlanGate         bool            `json:"plan_gate"`
	Stages           []ResolvedStage `json:"stages"`
}

// Validate rejects definitions whose resolution would be ambiguous or unsafe.
func Validate(c Catalog) error {
	if len(c.Playbooks) == 0 {
		return fmt.Errorf("playbook: catalog is empty")
	}
	ids := map[string]bool{}
	enabledTriggers := map[TaskType]string{}
	defaults := 0
	for i := range c.Playbooks {
		p := &c.Playbooks[i]
		who := p.ID
		if who == "" {
			return fmt.Errorf("playbook: playbooks[%d].id is required", i)
		}
		if ids[p.ID] {
			return fmt.Errorf("playbook: duplicate id %q", p.ID)
		}
		ids[p.ID] = true
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("playbook %q: name is required", who)
		}
		if p.Revision < 1 {
			return fmt.Errorf("playbook %q: revision must be at least 1", who)
		}
		if !validTriggerKind(p.Trigger.Kind) {
			return fmt.Errorf("playbook %q: invalid trigger kind %q", who, p.Trigger.Kind)
		}
		if p.Trigger.Kind == TaskManual && p.Trigger.Enabled {
			return fmt.Errorf("playbook %q: manual trigger cannot be enabled", who)
		}
		if p.Trigger.Enabled {
			if prior := enabledTriggers[p.Trigger.Kind]; prior != "" {
				return fmt.Errorf("playbook: enabled trigger %q is duplicated by %q and %q", p.Trigger.Kind, prior, who)
			}
			enabledTriggers[p.Trigger.Kind] = who
		}
		if p.Delivery != DeliveryAssignment && p.Delivery != DeliveryAnswer {
			return fmt.Errorf("playbook %q: invalid delivery %q", who, p.Delivery)
		}
		if p.Delivery == DeliveryAnswer && (p.PlanGate || len(p.Stages) != 1) {
			return fmt.Errorf("playbook %q: answer playbook must have exactly one ungated stage", who)
		}
		if p.IsDefault {
			defaults++
		}
		if len(p.Stages) == 0 {
			return fmt.Errorf("playbook %q: at least one stage is required", who)
		}
		orders := map[int]bool{}
		for j := range p.Stages {
			s := &p.Stages[j]
			if s.Order < 1 || orders[s.Order] {
				return fmt.Errorf("playbook %q: stage order %d must be positive and unique", who, s.Order)
			}
			orders[s.Order] = true
			if strings.TrimSpace(s.Name) == "" {
				return fmt.Errorf("playbook %q: stage %d name is required", who, s.Order)
			}
			defaultsInStage := 0
			branches := map[TaskType]bool{}
			for _, a := range s.Assignments {
				if strings.TrimSpace(a.Agent) == "" {
					return fmt.Errorf("playbook %q: stage %d assignment agent is required", who, s.Order)
				}
				if a.Profile != "" {
					agent, model, ok := capability.CatalogV2().RuntimeSelection(a.Profile)
					if !ok {
						return fmt.Errorf("playbook %q: stage %d assignment profile %q is unknown", who, s.Order, a.Profile)
					}
					if a.Agent != agent || a.Model != model {
						return fmt.Errorf("playbook %q: stage %d assignment profile %q does not match agent/model", who, s.Order, a.Profile)
					}
				} else {
					catalog := capability.CatalogV2()
					for _, profile := range catalog.Profiles {
						agent, model, ok := catalog.RuntimeSelection(profile.ID)
						if ok && a.Agent == agent && a.Model == model {
							if _, legacy := catalog.MapLegacyProfile(a.Agent, a.Model); !legacy {
								return fmt.Errorf("playbook %q: stage %d assignment profile is required for %s", who, s.Order, profile.ID)
							}
						}
					}
				}
				if a.TaskType == "" {
					defaultsInStage++
					continue
				}
				if !validTaskType(a.TaskType) {
					return fmt.Errorf("playbook %q: stage %d has invalid task type %q", who, s.Order, a.TaskType)
				}
				if branches[a.TaskType] {
					return fmt.Errorf("playbook %q: stage %d duplicates task type %q", who, s.Order, a.TaskType)
				}
				branches[a.TaskType] = true
			}
			if defaultsInStage != 1 {
				return fmt.Errorf("playbook %q: stage %d must have exactly one default assignment", who, s.Order)
			}
		}
	}
	if defaults != 1 {
		return fmt.Errorf("playbook: catalog must have exactly one default playbook")
	}
	return nil
}

// Resolve chooses and snapshots a playbook without performing I/O or inference.
func (c Catalog) Resolve(req RouteRequest) (ResolvedRoute, error) {
	if err := Validate(c); err != nil {
		return ResolvedRoute{}, err
	}
	typ := normalizeTaskType(req.TaskType)
	if typ == "" {
		typ = ClassifyTaskType(req.Direction)
	} else if !validTaskType(typ) {
		return ResolvedRoute{}, fmt.Errorf("playbook: invalid task type %q", req.TaskType)
	}

	var selected *Playbook
	source := SourceDefault
	if req.PlaybookID != "" {
		for i := range c.Playbooks {
			if c.Playbooks[i].ID == req.PlaybookID {
				selected = &c.Playbooks[i]
				break
			}
		}
		if selected == nil {
			return ResolvedRoute{}, fmt.Errorf("playbook: unknown playbook %q", req.PlaybookID)
		}
		source = SourceManual
	} else {
		for i := range c.Playbooks {
			p := &c.Playbooks[i]
			if p.Trigger.Enabled && p.Trigger.Kind == typ {
				selected = p
				source = SourceTrigger
				break
			}
		}
		if selected == nil {
			for i := range c.Playbooks {
				if c.Playbooks[i].IsDefault {
					selected = &c.Playbooks[i]
					break
				}
			}
		}
	}

	stages := append([]Stage(nil), selected.Stages...)
	sort.Slice(stages, func(i, j int) bool { return stages[i].Order < stages[j].Order })
	resolved := make([]ResolvedStage, 0, len(stages))
	for _, s := range stages {
		var chosen Assignment
		for _, a := range s.Assignments {
			if a.TaskType == "" {
				chosen = a
			}
		}
		for _, a := range s.Assignments {
			if a.TaskType == typ {
				chosen = a
				break
			}
		}
		resolved = append(resolved, ResolvedStage{
			Order: s.Order, Name: s.Name, Prompt: s.Prompt,
			Profile: chosen.Profile, Agent: chosen.Agent, Model: chosen.Model, Memory: s.Memory,
		})
	}
	return ResolvedRoute{
		PlaybookID: selected.ID, PlaybookRevision: selected.Revision,
		PlaybookName: selected.Name, TaskType: typ, Source: source,
		Delivery: selected.Delivery, PlanGate: selected.PlanGate, Stages: resolved,
	}, nil
}

func validTaskType(t TaskType) bool {
	switch t {
	case TaskQuestion, TaskBug, TaskResearch, TaskFeature:
		return true
	}
	return false
}

func validTriggerKind(t TaskType) bool {
	return validTaskType(t) || t == TaskManual
}

func normalizeTaskType(t TaskType) TaskType {
	return TaskType(strings.ToLower(strings.TrimSpace(string(t))))
}
