package playbook

import (
	"fmt"
	"sort"

	"github.com/tobsai/fort/core/graph"
)

// Compile converts a resolved route into the restricted graph subset used by
// playbooks: sequential task stages and, when requested, one human plan gate
// after the first stage. Resolution/validation happens before compilation.
func Compile(r ResolvedRoute) graph.Flow {
	stages := append([]ResolvedStage(nil), r.Stages...)
	sort.Slice(stages, func(i, j int) bool { return stages[i].Order < stages[j].Order })
	f := graph.Flow{
		ID:   fmt.Sprintf("playbook:%s:%d:%s", r.PlaybookID, r.PlaybookRevision, r.TaskType),
		Name: r.PlaybookName,
	}
	if f.Name == "" {
		f.Name = r.PlaybookID
	}
	if len(stages) == 0 {
		return f
	}
	f.Start = stageID(stages[0].Order)
	for i, stage := range stages {
		node := graph.Node{
			ID: stageID(stage.Order), Type: graph.Task,
			Profile: stage.Profile, Agent: stage.Agent, Model: stage.Model, Prompt: stage.Prompt,
			Context: graph.ContextPlaybook, Memory: stage.Memory,
		}
		next := ""
		if i == 0 && r.PlanGate {
			next = "plan-gate"
		} else if i+1 < len(stages) {
			next = stageID(stages[i+1].Order)
		}
		if next != "" {
			node.Edges = []graph.Edge{{On: graph.OutSuccess, To: next}}
		}
		f.Nodes = append(f.Nodes, node)
		if i == 0 && r.PlanGate {
			gate := graph.Node{ID: "plan-gate", Type: graph.Gate}
			if i+1 < len(stages) {
				gate.Edges = []graph.Edge{{On: graph.OutApprove, To: stageID(stages[i+1].Order)}}
			}
			f.Nodes = append(f.Nodes, gate)
		}
	}
	return f
}

func stageID(order int) string { return fmt.Sprintf("stage-%d", order) }
