package ui

import (
	"strings"
	"testing"

	"github.com/dop251/goja"
)

// TestBoardHTMLPlaybooksContract pins the Turn 4 web surface to the published
// playbooks API while also protecting the original six dashboard views. The
// page is deliberately dependency-free, so this source contract is the narrow
// unit-test seam; browser QA covers layout and interaction fidelity.
func TestBoardHTMLPlaybooksContract(t *testing.T) {
	for _, view := range []string{"deck", "projects", "assign", "perf", "week", "today", "playbooks"} {
		for _, want := range []string{`data-v="` + view + `"`, `id="v-` + view + `"`} {
			if !strings.Contains(boardHTML, want) {
				t.Errorf("boardHTML missing %q", want)
			}
		}
	}

	for _, want := range []string{
		`id="routepreview"`,
		`id="routepicker"`,
		`id="quickanswer"`,
		`id="playbooklist"`,
		`id="playbookeditor"`,
		`/api/playbooks`,
		`/duplicate`,
		`/api/route`,
		`playbook_id`,
		`playbook_revision`,
		`task_type`,
		`plan_gate`,
		`kind==='answer'`,
		`@media (max-width:900px)`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML missing Turn 4 contract %q", want)
		}
	}
}

func TestBoardHTMLJavaScriptParses(t *testing.T) {
	start := strings.LastIndex(boardHTML, "<script>")
	if start < 0 {
		t.Fatal("boardHTML main script missing")
	}
	start += len("<script>")
	end := strings.Index(boardHTML[start:], "</script>")
	if end < 0 {
		t.Fatal("boardHTML main script closing tag missing")
	}
	if _, err := goja.Compile("board.js", boardHTML[start:start+end], false); err != nil {
		t.Fatalf("boardHTML JavaScript does not parse: %v", err)
	}
}

func TestBoardHTMLManualTriggerCannotAutoRoute(t *testing.T) {
	const guard = `if(kind==='manual')next.trigger.enabled=false;`
	if !strings.Contains(boardHTML, guard) {
		t.Fatalf("boardHTML missing manual-trigger disable guard %q", guard)
	}
}

func TestBoardHTMLPlaybookRailUsesSourceSpecificMeta(t *testing.T) {
	for _, want := range []string{`function playbookMeta(p)`, `function branchLabel(p,a,branching)`, `function shortcutRank(p)`, `sort(function(a,b){return shortcutRank(a)-shortcutRank(b)||a.name.localeCompare(b.name);})`, `no checkpoints`, `skips design`, `delivers a doc`, `'bug fixes'`, `'features'`} {
		if !strings.Contains(boardHTML, want) {
			t.Fatalf("boardHTML missing source-specific playbook meta %q", want)
		}
	}
}

func TestBoardHTMLPlaybookComposerHasNoPlaceboAgentOrMachineOverride(t *testing.T) {
	for _, stale := range []string{`id="assignmentopts"`, `var request={text:text,agent:selAgent,machine:selMachine`} {
		if strings.Contains(boardHTML, stale) {
			t.Fatalf("boardHTML still exposes ignored playbook composer control %q", stale)
		}
	}
	if !strings.Contains(boardHTML, `var request={text:text,task_type:`) {
		t.Fatal("boardHTML missing deterministic route handoff request")
	}
}

func TestBoardHTMLAssignmentPlanGateOverridesPreviewAndHandoff(t *testing.T) {
	for _, want := range []string{
		`id="plantoggle"`,
		`let planFirst=true;`,
		`assignCtx=null;assignMode='assignment';planFirst=true;`,
		`function effectivePlanGate(){return assignMode!=='quick'&&planFirst;}`,
		`.toggle[hidden]{display:none}`,
		`$('#plantoggle').hidden=quick;`,
		`plan_gate:effectivePlanGate()`,
		`$('#plantoggle').addEventListener('click'`,
		`planFirst=!planFirst;renderAssignControls();queueRoutePreview();`,
		`plan_gate:!!resolved.plan_gate`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Fatalf("boardHTML missing per-assignment plan-gate behavior %q", want)
		}
	}
}

func TestBoardHTMLPlaybooksStackStagesAtPhoneWidths(t *testing.T) {
	for _, want := range []string{
		`@media (max-width:600px)`,
		`.pipeline{flex-direction:column;overflow-x:visible}`,
		`.stagecard{min-width:0}`,
		`.gate-label{margin-left:0}`,
		`nav::-webkit-scrollbar{display:none}`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Fatalf("boardHTML missing phone-width playbook layout %q", want)
		}
	}
}

func TestBoardHTMLQuickModePinsAnAnswerPlaybook(t *testing.T) {
	for _, want := range []string{`function availablePlaybooks()`, `body.playbook_id=answer.id`, `resolved.delivery!=='answer'`} {
		if !strings.Contains(boardHTML, want) {
			t.Fatalf("boardHTML missing Quick-mode answer invariant %q", want)
		}
	}
}

func TestBoardHTMLReloadsCatalogAfterRevisionConflict(t *testing.T) {
	if !strings.Contains(boardHTML, `if(resp.status===409){await fetchPlaybooks();`) {
		t.Fatal("boardHTML does not reload immutable catalog after a stale save conflict")
	}
}

func TestBoardHTMLAnswerPlaybookCannotEnablePlanGate(t *testing.T) {
	for _, want := range []string{`pb.delivery==='answer'`, `if(pb.delivery!=='answer')pipeline+=`, `if(!pb||pb.delivery==='answer')return;`, `No checkpoints`} {
		if !strings.Contains(boardHTML, want) {
			t.Fatalf("boardHTML missing answer-plan-gate guard %q", want)
		}
	}
}

func TestBoardHTMLQuickAnswerFailureRendersInline(t *testing.T) {
	for _, want := range []string{`quickAnswerError`, `Quick answer failed`, `if(assignMode==='quick'){`} {
		if !strings.Contains(boardHTML, want) {
			t.Fatalf("boardHTML missing inline Quick-answer failure state %q", want)
		}
	}
}
