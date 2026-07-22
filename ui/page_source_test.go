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

func TestBoardHTMLUsesTimeBoundedAttentionAcrossRunStates(t *testing.T) {
	start := strings.Index(boardHTML, "// ---- derived model ----")
	end := strings.Index(boardHTML, "// ---- render root ----")
	if start < 0 || end <= start {
		t.Fatal("boardHTML derived-model script markers missing")
	}

	vm := goja.New()
	setup := `
var model={
  gates:[{run_id:'gated'}], machines:[], backlog:[], playbooks:[],
  runs:[
    {id:'gated',agent:'flow:gated',status:'failed',title:'Gated',created_at:'2026-07-22T11:55:00Z'},
    {id:'recent',agent:'flow:recent',status:'failed',title:'Recent',created_at:'2026-07-22T11:00:00Z'},
    {id:'boundary',agent:'flow:boundary',status:'error',title:'Boundary',created_at:'2026-07-20T12:00:00Z'},
    {id:'working',agent:'flow:working',status:'running',title:'Working',created_at:'2026-07-22T10:00:00Z'},
    {id:'delivered',agent:'flow:delivered',status:'succeeded',title:'Delivered',created_at:'2026-07-22T09:00:00Z'},
    {id:'old',agent:'flow:old',status:'failed',title:'Old',created_at:'2026-07-19T11:00:00Z'}
  ]
};
Date.now=function(){return Date.parse('2026-07-22T12:00:00Z');};
function isLive(r){return r.status==='running'||r.status==='blocked';}
function runAgent(r){return r.agent;}
`
	if _, err := vm.RunString(setup + boardHTML[start:end]); err != nil {
		t.Fatalf("execute derived-model helpers: %v", err)
	}
	value, err := vm.RunString(`JSON.stringify({
  needCount:needCount(),
  gatedState:projectState(model.runs[0]),
  projectIDs:projects().map(function(p){return p.run.id;})
})`)
	if err != nil {
		t.Fatalf("evaluate attention model: %v", err)
	}
	const want = `{"needCount":3,"gatedState":"need","projectIDs":["gated","recent","boundary","working","delivered","old"]}`
	if got := value.String(); got != want {
		t.Fatalf("attention model = %s, want %s", got, want)
	}
}
