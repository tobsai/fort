package capability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	MaxDirectionBytes       = 65_536
	MaxGeneratedPlanBytes   = 131_072
	MaxStageOutputBytes     = 262_144
	MaxHandoffReservedBytes = 1 << 20
)

var stageIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type AttemptPolicy struct {
	ProviderMaxAttempts int `json:"provider_max_attempts"`
	DeadlineSeconds     int `json:"deadline_seconds"`
}

type Stage struct {
	ID             string        `json:"id"`
	Order          int           `json:"order"`
	Title          string        `json:"title"`
	Prompt         string        `json:"prompt"`
	Profile        string        `json:"profile"`
	Requires       []string      `json:"requires"`
	InputFrom      []string      `json:"input_from"`
	Output         string        `json:"output"`
	OutputFormat   string        `json:"output_format"`
	MaxOutputBytes int           `json:"max_output_bytes"`
	AttemptPolicy  AttemptPolicy `json:"attempt_policy"`
}

type Plan struct {
	Stages []Stage `json:"stages"`
}

type generatedStage struct {
	ID             string   `json:"id"`
	Order          int      `json:"order"`
	Title          string   `json:"title"`
	Prompt         string   `json:"prompt"`
	Profile        string   `json:"profile"`
	Requires       []string `json:"requires"`
	InputFrom      []string `json:"input_from"`
	Output         string   `json:"output"`
	OutputFormat   string   `json:"output_format"`
	MaxOutputBytes int      `json:"max_output_bytes"`
}

type generatedPlan struct {
	Stages []generatedStage `json:"stages"`
}

// DecodeGeneratedPlan strictly decodes, validates, and normalizes the one
// model-produced plan object. It supplies the immutable one-attempt stage
// policy; the model cannot choose execution retry or deadline behavior.
func DecodeGeneratedPlan(raw []byte, catalog Catalog, permittedProfiles []string) (Plan, error) {
	if len(raw) == 0 || len(raw) > MaxGeneratedPlanBytes || !utf8.Valid(raw) {
		return Plan{}, fmt.Errorf("capability: generated plan size/UTF-8 is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var generated generatedPlan
	if err := decoder.Decode(&generated); err != nil {
		return Plan{}, fmt.Errorf("capability: invalid generated plan: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Plan{}, fmt.Errorf("capability: generated plan must contain exactly one JSON object")
	}
	if len(generated.Stages) < 1 || len(generated.Stages) > 16 {
		return Plan{}, fmt.Errorf("capability: plan must contain 1 to 16 stages")
	}
	allowed := make(map[string]bool, len(permittedProfiles))
	for _, profile := range permittedProfiles {
		if _, ok := catalog.profile(profile); !ok || allowed[profile] {
			return Plan{}, fmt.Errorf("capability: invalid permitted profile %q", profile)
		}
		allowed[profile] = true
	}

	plan := Plan{Stages: make([]Stage, len(generated.Stages))}
	stageIDs := map[string]bool{}
	outputs := map[string]bool{}
	for i, source := range generated.Stages {
		if !stageIDPattern.MatchString(source.ID) || stageIDs[source.ID] {
			return Plan{}, fmt.Errorf("capability: stage %d has invalid or duplicate id", i+1)
		}
		stageIDs[source.ID] = true
		if source.Order != i+1 {
			return Plan{}, fmt.Errorf("capability: stage %q order must be %d", source.ID, i+1)
		}
		if strings.TrimSpace(source.Title) == "" || len([]byte(source.Title)) > 160 ||
			len([]byte(source.Prompt)) > 8*1024 {
			return Plan{}, fmt.Errorf("capability: stage %q title/prompt is invalid", source.ID)
		}
		if _, ok := catalog.profile(source.Profile); !ok {
			return Plan{}, fmt.Errorf("capability: stage %q profile %q is not cataloged", source.ID, source.Profile)
		}
		if len(allowed) > 0 && !allowed[source.Profile] {
			return Plan{}, fmt.Errorf("capability: stage %q profile %q is not permitted", source.ID, source.Profile)
		}
		if source.Requires == nil || len(source.Requires) > 8 || !uniqueStrings(source.Requires) {
			return Plan{}, fmt.Errorf("capability: stage %q requirements are invalid", source.ID)
		}
		for _, requirement := range source.Requires {
			if _, ok := catalog.capability(requirement); !ok {
				return Plan{}, fmt.Errorf("capability: stage %q requirement %q is not cataloged", source.ID, requirement)
			}
		}
		sort.Slice(source.Requires, func(i, j int) bool {
			return catalog.capabilityRank(source.Requires[i]) < catalog.capabilityRank(source.Requires[j])
		})
		if _, ok := catalog.bindingFor(source.Profile, source.Requires); !ok {
			return Plan{}, fmt.Errorf("capability: stage %q has no cataloged execution binding", source.ID)
		}
		if source.InputFrom == nil || len(source.InputFrom) > 1 {
			return Plan{}, fmt.Errorf("capability: stage %q input_from is invalid", source.ID)
		}
		if i == 0 {
			if len(source.InputFrom) != 0 {
				return Plan{}, fmt.Errorf("capability: first stage cannot have input")
			}
		} else if len(source.InputFrom) != 1 || source.InputFrom[0] != generated.Stages[i-1].Output {
			return Plan{}, fmt.Errorf("capability: stage %q must consume the preceding output", source.ID)
		}
		if !stageIDPattern.MatchString(source.Output) || outputs[source.Output] {
			return Plan{}, fmt.Errorf("capability: stage %q output is invalid or duplicated", source.ID)
		}
		outputs[source.Output] = true
		if source.OutputFormat != "text" && source.OutputFormat != "json" {
			return Plan{}, fmt.Errorf("capability: stage %q output_format is invalid", source.ID)
		}
		if source.MaxOutputBytes < 1 || source.MaxOutputBytes > MaxStageOutputBytes {
			return Plan{}, fmt.Errorf("capability: stage %q max_output_bytes is invalid", source.ID)
		}
		plan.Stages[i] = Stage{
			ID: source.ID, Order: source.Order, Title: source.Title, Prompt: source.Prompt,
			Profile: source.Profile, Requires: source.Requires, InputFrom: source.InputFrom,
			Output: source.Output, OutputFormat: source.OutputFormat,
			MaxOutputBytes: source.MaxOutputBytes,
			AttemptPolicy:  AttemptPolicy{ProviderMaxAttempts: 1, DeadlineSeconds: 900},
		}
	}
	if canonical, err := canonicalJSON(generated); err != nil || len(canonical) > MaxGeneratedPlanBytes {
		return Plan{}, fmt.Errorf("capability: canonical generated plan exceeds the bound")
	}
	return plan, nil
}
