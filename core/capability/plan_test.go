package capability

import (
	"strings"
	"testing"
)

const validGeneratedPlan = `{
  "stages": [
    {
      "id": "read-email",
      "order": 1,
      "title": "Read the Supabase failure email",
      "prompt": "Extract bounded incident evidence.",
      "profile": "codex:gpt-5.5",
      "requires": ["email.gmail.read"],
      "input_from": [],
      "output": "incident_evidence",
      "output_format": "text",
      "max_output_bytes": 65536
    },
    {
      "id": "diagnose",
      "order": 2,
      "title": "Diagnose the Supabase failure",
      "prompt": "Use the approved incident evidence.",
      "profile": "codex:gpt-5.5",
      "requires": ["database.supabase.inspect"],
      "input_from": ["incident_evidence"],
      "output": "diagnosis",
      "output_format": "text",
      "max_output_bytes": 65536
    }
  ]
}`

func TestDecodeGeneratedPlanAcceptsStrictSequentialPlan(t *testing.T) {
	plan, err := DecodeGeneratedPlan([]byte(validGeneratedPlan), CatalogV2(), []string{"codex:gpt-5.5"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Stages) != 2 {
		t.Fatalf("stage count = %d, want 2", len(plan.Stages))
	}
	for _, stage := range plan.Stages {
		if stage.AttemptPolicy.ProviderMaxAttempts != 1 || stage.AttemptPolicy.DeadlineSeconds != 900 {
			t.Fatalf("attempt policy = %#v", stage.AttemptPolicy)
		}
	}
}

func TestDecodeGeneratedPlanRejectsModelMachineAuthority(t *testing.T) {
	raw := strings.Replace(validGeneratedPlan, `"profile": "codex:gpt-5.5",`, `"profile": "codex:gpt-5.5", "machine": "mini",`, 1)
	if _, err := DecodeGeneratedPlan([]byte(raw), CatalogV2(), []string{"codex:gpt-5.5"}); err == nil {
		t.Fatal("expected unknown machine field to fail")
	}
}

func TestDecodeGeneratedPlanRejectsNonChainInput(t *testing.T) {
	raw := strings.Replace(validGeneratedPlan, `"input_from": ["incident_evidence"]`, `"input_from": []`, 1)
	if _, err := DecodeGeneratedPlan([]byte(raw), CatalogV2(), []string{"codex:gpt-5.5"}); err == nil {
		t.Fatal("expected missing chain input to fail")
	}
}

func TestDecodeGeneratedPlanRejectsUnmappedProfile(t *testing.T) {
	raw := strings.Replace(validGeneratedPlan, `"codex:gpt-5.5"`, `"codex:made-up"`, 1)
	if _, err := DecodeGeneratedPlan([]byte(raw), CatalogV2(), nil); err == nil {
		t.Fatal("expected unknown profile to fail")
	}
}

func TestDecodeGeneratedPlanRejectsUncatalogedCoUse(t *testing.T) {
	raw := strings.Replace(validGeneratedPlan,
		`"requires": ["email.gmail.read"]`,
		`"requires": ["email.gmail.read", "database.supabase.inspect"]`, 1)
	if _, err := DecodeGeneratedPlan([]byte(raw), CatalogV2(), nil); err == nil {
		t.Fatal("expected uncataloged capability combination to fail")
	}
}
