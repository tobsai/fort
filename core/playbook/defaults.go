package playbook

// DefaultCatalog returns a fresh, valid starter catalog matching the approved
// Feature work, Bug fix, Quick answer, and Research handoff designs.
func DefaultCatalog() Catalog {
	catalog := legacyGPT55DefaultCatalog()
	for i := range catalog.Playbooks {
		if catalog.Playbooks[i].ID != "feature-work" {
			continue
		}
		for j := range catalog.Playbooks[i].Stages {
			if catalog.Playbooks[i].Stages[j].Name != "Design" {
				continue
			}
			assignment := &catalog.Playbooks[i].Stages[j].Assignments[0]
			assignment.Profile = "codex:gpt-5.5"
			assignment.Agent = "codex"
			assignment.Model = "gpt-5.5"
		}
	}
	return catalog
}

// LegacyGPT55DefaultCatalog returns the exact defaults shipped before Feature
// work's unavailable OpenClaw design stage was replaced. It is migration input
// only; prior immutable runs continue to resolve against their stored revision.
func LegacyGPT55DefaultCatalog() Catalog {
	return legacyGPT55DefaultCatalog()
}

func legacyGPT55DefaultCatalog() Catalog {
	catalog := interimConfiguredDefaultCatalog()
	for i := range catalog.Playbooks {
		for j := range catalog.Playbooks[i].Stages {
			for k := range catalog.Playbooks[i].Stages[j].Assignments {
				assignment := &catalog.Playbooks[i].Stages[j].Assignments[k]
				if assignment.Agent == "codex" {
					assignment.Profile = "codex:gpt-5.5"
					assignment.Model = "gpt-5.5"
				}
			}
		}
	}
	return catalog
}

// LegacyDefaultCatalogRevision1 returns the exact built-in definitions that
// shipped before 5.6 work was assigned directly to Codex. It exists only so
// the control adapter can recognize untouched persisted defaults and append a
// corrected immutable revision without mistaking user edits for defaults.
func LegacyDefaultCatalogRevision1() Catalog {
	return legacyDefaultCatalogRevision1()
}

// InterimConfiguredDefaultCatalog returns the exact correction deployed before
// GPT-5.5 was explicitly approved. It is migration input only.
func InterimConfiguredDefaultCatalog() Catalog {
	return interimConfiguredDefaultCatalog()
}

func interimConfiguredDefaultCatalog() Catalog {
	catalog := legacyDefaultCatalogRevision1()
	for i := range catalog.Playbooks {
		for j := range catalog.Playbooks[i].Stages {
			for k := range catalog.Playbooks[i].Stages[j].Assignments {
				assignment := &catalog.Playbooks[i].Stages[j].Assignments[k]
				if (assignment.Agent == "hermes" && assignment.Model == "Codex 5.6 Sol") ||
					(assignment.Agent == "codex" && assignment.Model == "5.6 Sol") {
					assignment.Agent = "codex"
					assignment.Model = ""
				}
			}
		}
	}
	return catalog
}

func legacyDefaultCatalogRevision1() Catalog {
	def := func(agent, model string) []Assignment {
		return []Assignment{{Agent: agent, Model: model}}
	}
	return Catalog{Playbooks: []Playbook{
		{
			ID: "quick-answer", Name: "Quick answer", Revision: 1,
			Trigger: Trigger{Kind: TaskQuestion, Enabled: true}, Delivery: DeliveryAnswer,
			Stages: []Stage{{
				Order: 1, Name: "Answer", Prompt: "Answer the direction directly and concisely.",
				Description: "Answers inline without creating an assignment or checkpoint.", Assignments: def("hermes", "Codex 5.6 Sol"),
			}},
		},
		{
			ID: "bug-fix", Name: "Bug fix", Revision: 1,
			PlanGate: true, Trigger: Trigger{Kind: TaskBug, Enabled: true}, Delivery: DeliveryAssignment,
			Stages: []Stage{
				{Order: 1, Name: "Break down", Prompt: "Diagnose the reported bug and produce a focused repair plan.", Description: "Diagnoses the report and keeps a focused repair plan in shared memory.", Assignments: def("hermes", "Codex 5.6 Sol"), Memory: true},
				{Order: 2, Name: "Build", Prompt: "Implement and verify the approved repair.", Description: "Patches and verifies the approved repair.", Assignments: def("codex", "5.6 Sol")},
			},
		},
		{
			ID: "research", Name: "Research", Revision: 1,
			Trigger: Trigger{Kind: TaskResearch, Enabled: true}, Delivery: DeliveryAssignment,
			Stages: []Stage{
				{Order: 1, Name: "Research", Prompt: "Research the direction and preserve the evidence needed downstream.", Description: "Collects evidence and keeps it available to synthesis.", Assignments: def("hermes", ""), Memory: true},
				{Order: 2, Name: "Synthesize", Prompt: "Synthesize the research into an actionable result.", Description: "Delivers a concise, actionable research brief.", Assignments: def("claude", "")},
			},
		},
		{
			ID: "feature-work", Name: "Feature work", Revision: 1,
			IsDefault: true, PlanGate: true,
			Trigger: Trigger{Kind: TaskFeature, Enabled: true}, Delivery: DeliveryAssignment,
			Stages: []Stage{
				{Order: 1, Name: "Break down", Prompt: "Break the direction into a concrete implementation plan.", Description: "Turns your brief into a checkpoint plan and keeps it in shared memory for later stages.", Assignments: def("hermes", "Codex 5.6 Sol"), Memory: true},
				{Order: 2, Name: "Design", Prompt: "Design the solution using the approved plan.", Description: "Produces the design from the approved plan in shared memory.", Assignments: def("openclaw", "Fable")},
				{Order: 3, Name: "Build", Prompt: "Implement and verify the approved design.", Description: "Implements and verifies the approved design.", Assignments: []Assignment{
					{Agent: "claude", Model: "Sonnet"},
					{TaskType: TaskBug, Agent: "codex", Model: "5.6 Sol"},
				}},
			},
		},
	}}
}
