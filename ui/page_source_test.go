package ui

import (
	"strings"
	"testing"

	"github.com/dop251/goja"
)

func boardHTMLSection(t *testing.T, startMarker, endMarker string) string {
	t.Helper()
	start := strings.Index(boardHTML, startMarker)
	if start < 0 {
		t.Fatalf("boardHTML script marker missing %q", startMarker)
	}
	end := strings.Index(boardHTML[start:], endMarker)
	if end < 0 {
		t.Fatalf("boardHTML script marker missing %q", endMarker)
	}
	return boardHTML[start : start+end]
}

func boardHTMLActivityAndDerivedScript(t *testing.T) string {
	t.Helper()
	return boardHTMLSection(t, "// ---- SSE + activity buffers", "// ---- data ----") +
		boardHTMLSection(t, "// ---- derived model ----", "// ---- render root ----")
}

// TestBoardHTMLPlaybooksContract pins the Turn 4 web surface to the published
// playbooks API while also protecting the original six dashboard views. The
// page is deliberately dependency-free, so this source contract is the narrow
// unit-test seam; browser QA covers layout and interaction fidelity.
func TestBoardHTMLPlaybooksContract(t *testing.T) {
	for _, view := range []string{"deck", "assign", "perf", "week", "today", "playbooks"} {
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

func TestBoardHTMLConversationCommandCenterContract(t *testing.T) {
	for _, want := range []string{
		`class="conversation-shell"`,
		`id="newconversation"`,
		`id="conversationlist"`,
		`id="conversationtitle"`,
		`id="conversationfeed"`,
		`id="turnintowork"`,
		`id="conversationcomposer"`,
		`id="composeragent"`,
		`id="composerprofile"`,
		`id="composermachine"`,
		`id="currentagent"`,
		`id="otheragents"`,
		`id="machinerail"`,
		`data-desktop-command-center`,
		`@media (max-width:900px)`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML missing conversation command-center contract %q", want)
		}
	}
}

func TestBoardHTMLConversationUXIsActionableAndUnambiguous(t *testing.T) {
	for _, want := range []string{
		`/fort-agent-orb.png`,
		`/api/profiles`,
		`function openNeedsYou()`,
		`Needs approval`,
		`Work is paused until you approve`,
		`Approve & continue`,
		`Request changes`,
		`profile:composerProfile`,
		`machine:composerMachine`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML missing actionable conversation contract %q", want)
		}
	}
	for _, stale := range []string{`id="givedir"`, `$('#givedir').addEventListener`} {
		if strings.Contains(boardHTML, stale) {
			t.Errorf("boardHTML still exposes duplicate conversation creation action %q", stale)
		}
	}
	if strings.Count(boardHTML, `id="newconversation"`) != 1 {
		t.Fatalf("New conversation control count = %d, want 1", strings.Count(boardHTML, `id="newconversation"`))
	}
}

func TestBoardHTMLConversationPromotionOnlyForCompletedDirectRuns(t *testing.T) {
	section := boardHTMLSection(t, "function canTurnConversationIntoWork", "function conversationFeedHTML")
	vm := goja.New()
	if _, err := vm.RunString(`function hasGate(id){return id==='gated-finished';}` + section); err != nil {
		t.Fatalf("execute conversation-promotion helper: %v", err)
	}
	value, err := vm.RunString(`JSON.stringify([
  {id:'direct-finished',status:'succeeded'},
  {id:'direct-done',status:'done'},
  {id:'direct-working',status:'running'},
  {id:'direct-paused',status:'blocked'},
  {id:'direct-failed',status:'failed'},
  {id:'direct-canceled',status:'canceled'},
  {id:'gated-finished',status:'succeeded'},
  {id:'routed-finished',status:'succeeded',flow_id:'playbook:feature-work'}
].map(function(run){return {id:run.id,offered:canTurnConversationIntoWork(run)};}));`)
	if err != nil {
		t.Fatalf("evaluate conversation-promotion eligibility: %v", err)
	}
	const want = `[{"id":"direct-finished","offered":true},{"id":"direct-done","offered":true},{"id":"direct-working","offered":false},{"id":"direct-paused","offered":false},{"id":"direct-failed","offered":false},{"id":"direct-canceled","offered":false},{"id":"gated-finished","offered":false},{"id":"routed-finished","offered":false}]`
	if got := value.String(); got != want {
		t.Fatalf("conversation-promotion eligibility = %s, want %s", got, want)
	}
	if !strings.Contains(boardHTML, `if(canTurnConversationIntoWork(run))html+='<div class="turn-work"`) {
		t.Fatal("conversation feed does not guard Turn this into work with completed-direct eligibility")
	}
}

func TestBoardHTMLConversationPromotionPinsDefaultAssignmentRoute(t *testing.T) {
	for _, want := range []string{
		`turnConversationIntoWork(){prepareConversation('assignment',conversationSeed(),defaultAssignmentPlaybook());}`,
		`routeChoice=playbook?{id:playbook.id,revision:playbook.revision}:null`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Fatalf("boardHTML promotion does not pin its assignment route %q", want)
		}
	}

	section := boardHTMLSection(t, "function defaultAssignmentPlaybook", "function cloneData")
	vm := goja.New()
	setup := `var model={playbooks:[
  {id:'quick-answer',revision:4,is_default:false,delivery:'answer'},
  {id:'bug-fix',revision:6,is_default:false,delivery:'assignment'},
  {id:'feature-work',revision:8,is_default:true,delivery:'assignment'}
]};`
	if _, err := vm.RunString(setup + section); err != nil {
		t.Fatalf("execute promotion route helper: %v", err)
	}
	value, err := vm.RunString(`JSON.stringify(defaultAssignmentPlaybook())`)
	if err != nil {
		t.Fatalf("evaluate promotion route helper: %v", err)
	}
	const want = `{"id":"feature-work","revision":8,"is_default":true,"delivery":"assignment"}`
	if got := value.String(); got != want {
		t.Fatalf("default promotion route = %s, want %s", got, want)
	}
}

func TestBoardHTMLTranscriptUsesExplicitHumanAvatarRole(t *testing.T) {
	section := boardHTMLSection(t, "function messageAvatarHTML", "function activityTimelineHTML")
	vm := goja.New()
	setup := `
function esc(v){return String(v||'');}
function orbClass(thinking){return 'fort-orb'+(thinking?' is-thinking':'');}
`
	if _, err := vm.RunString(setup + section); err != nil {
		t.Fatalf("execute transcript-role helpers: %v", err)
	}
	value, err := vm.RunString(`({
  human:messageHTML('human','You','now','Hello','',true),
  agent:messageHTML('agent','Codex','now','Working','GPT',true)
})`)
	if err != nil {
		t.Fatalf("evaluate transcript avatars: %v", err)
	}
	var rows map[string]string
	if err := vm.ExportTo(value, &rows); err != nil {
		t.Fatalf("export transcript avatars: %v", err)
	}
	human, agent := rows["human"], rows["agent"]
	for _, want := range []string{`human-avatar`, `aria-label="You"`, `<svg`} {
		if !strings.Contains(human, want) {
			t.Errorf("human transcript avatar missing %q: %s", want, human)
		}
	}
	for _, stale := range []string{`/fort-agent-orb.png`, `fort-orb`, `is-thinking`} {
		if strings.Contains(human, stale) {
			t.Errorf("human transcript avatar still uses agent identity %q: %s", stale, human)
		}
	}
	for _, want := range []string{`/fort-agent-orb.png`, `fort-orb is-thinking`} {
		if !strings.Contains(agent, want) {
			t.Errorf("agent transcript avatar missing %q: %s", want, agent)
		}
	}
	if !strings.Contains(boardHTML, `messageHTML('human','You'`) {
		t.Fatal("human transcript row does not pass an explicit role")
	}
}

func TestBoardHTMLRemovesProjectsWithoutRemovingConversations(t *testing.T) {
	for _, stale := range []string{
		`data-v="projects"`,
		`id="v-projects"`,
		`id="conversationprojects"`,
		`id="projectcount"`,
		`function projects()`,
		`function projectRooms()`,
		`function renderProjects()`,
		`aria-label="projects and conversations"`,
	} {
		if strings.Contains(boardHTML, stale) {
			t.Errorf("boardHTML still exposes Projects contract %q", stale)
		}
	}
	for _, want := range []string{
		`id="newconversation"`,
		`id="conversationlist"`,
		`id="conversationfeed"`,
		`function conversationRuns()`,
		`function renderDeck()`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML lost conversation contract %q", want)
		}
	}
}

func TestBoardHTMLConversationListRendersExplicitStatusLabels(t *testing.T) {
	if !strings.Contains(boardHTML, `class="conversation-status`) {
		t.Fatal("boardHTML conversation list does not render a visible status label")
	}
	if got := strings.Count(boardHTML, `runStatusLabel(r)`); got < 2 {
		t.Fatalf("runStatusLabel(r) occurrence count = %d, want definition plus rendered use", got)
	}
	for _, want := range []string{"Starting", "Working", "Needs approval", "Finished", "Failed"} {
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML missing visible conversation status %q", want)
		}
	}
}

func TestBoardHTMLPhoneCommandCenterKeepsNavigationStatusAndComposerUsable(t *testing.T) {
	for _, want := range []string{
		`id="conversationnav"`,
		`id="mobileconversationnav"`,
		`aria-controls="conversationnav"`,
		`aria-expanded="false"`,
		`id="closeconversationnav"`,
		`id="conversationnavscrim"`,
		`id="mobileconversationstate"`,
		`function setMobileConversationNav(open)`,
		`setMobileConversationNav(false);renderDeck();loadConversationDetail(id);`,
		`$('#mobileconversationstate').innerHTML=active?conversationStatusHTML(active):`,
		`document.body.dataset.view=v;`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML missing phone command-center contract %q", want)
		}
	}

	phoneCSS := boardHTMLSection(t, "@media (max-width:700px)", "@media (max-width:600px)")
	for _, want := range []string{
		`html,body{max-width:100%;overflow-x:hidden}`,
		`body[data-view=deck]{height:100dvh;overflow:hidden;display:flex;flex-direction:column}`,
		`nav{overscroll-behavior-x:contain;scroll-snap-type:x proximity}`,
		`nav button{min-height:44px;flex:none;scroll-snap-align:start}`,
		`.needpill{min-height:44px}`,
		`.conversation-sidebar{position:fixed`,
		`.conversation-sidebar.mobile-open{transform:translateX(0);visibility:visible}`,
		`.conversation-sidebar .side-scroll{display:block`,
		`.conversation-main{height:100%;min-height:0`,
		`.conversation-feed{min-width:0;overflow:auto`,
		`.approval-card{margin-left:0`,
		`.approval-card .approval-actions button{min-height:44px;width:100%}`,
		`.compose-actions{display:grid;grid-template-columns:repeat(2,minmax(0,1fr))`,
		`.compose-select{font-size:16px;padding:7px 25px 7px 8px;min-height:44px;max-width:none;width:100%}`,
		`.compose-button{font-size:13px;min-height:44px;width:100%}`,
	} {
		if !strings.Contains(phoneCSS, want) {
			t.Errorf("phone CSS missing responsive contract %q", want)
		}
	}
}

func TestBoardHTMLPhoneConversationNavigationOpensAndCloses(t *testing.T) {
	section := boardHTMLSection(t, "function setMobileConversationNav", "function showView")
	vm := goja.New()
	setup := `
var state={panel:false,expanded:'false',scrim:true};
var panel={classList:{toggle:function(name,on){state.panel=on;}}};
var trigger={setAttribute:function(name,value){if(name==='aria-expanded')state.expanded=value;}};
var scrim={hidden:true};
function $(id){if(id==='#conversationnav')return panel;if(id==='#mobileconversationnav')return trigger;return scrim;}
`
	if _, err := vm.RunString(setup + section); err != nil {
		t.Fatalf("execute phone conversation navigation helper: %v", err)
	}
	value, err := vm.RunString(`
setMobileConversationNav(true);
var opened={panel:state.panel,expanded:state.expanded,scrim:scrim.hidden};
setMobileConversationNav(false);
JSON.stringify({opened:opened,closed:{panel:state.panel,expanded:state.expanded,scrim:scrim.hidden}});
`)
	if err != nil {
		t.Fatalf("toggle phone conversation navigation: %v", err)
	}
	const want = `{"opened":{"panel":true,"expanded":"true","scrim":false},"closed":{"panel":false,"expanded":"false","scrim":true}}`
	if got := value.String(); got != want {
		t.Fatalf("phone conversation navigation state = %s, want %s", got, want)
	}
}

func TestBoardHTMLSSEListensForEveryActivityEventType(t *testing.T) {
	for _, eventType := range []string{
		"started", "stdout", "stderr", "message", "tool", "subagent",
		"exited", "error", "ingress", "placement", "transform", "gate",
	} {
		want := `es.addEventListener('` + eventType + `',onFortEvent);`
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML does not subscribe to named SSE event %q", eventType)
		}
	}
}

func TestBoardHTMLConversationComposerFitsNarrowWidths(t *testing.T) {
	for _, want := range []string{
		`.conversation-main{min-width:0;display:grid;grid-template-columns:minmax(0,1fr);`,
		`.compose-actions{display:grid;grid-template-columns:repeat(2,minmax(0,1fr))`,
		`.compose-select{font-size:16px;padding:7px 25px 7px 8px;min-height:44px;max-width:none;width:100%}`,
		`#composermachine{grid-column:1/-1}`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML missing narrow conversation overflow guard %q", want)
		}
	}
}

func TestBoardHTMLNarrowDeckKeepsConversationHistoryAccessible(t *testing.T) {
	for _, want := range []string{
		`.conversation-sidebar{position:fixed`,
		`.conversation-sidebar.mobile-open{transform:translateX(0);visibility:visible}`,
		`.conversation-sidebar .new-conversation{display:block`,
		`.conversation-sidebar .side-scroll{display:block`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML missing narrow conversation navigation %q", want)
		}
	}
	if strings.Contains(boardHTML, `.needpill{display:none!important}`) {
		t.Fatal("narrow layout hides the only global Needs you action")
	}
}

func TestBoardHTMLConversationRunsSortByActualLatestActivity(t *testing.T) {
	vm := goja.New()
	setup := `
var agentOfRun={},actByRun={},ACT_MAX=20;
var model={gates:[{run_id:'gated',since:'2026-07-22T11:00:00Z'}],machines:[],backlog:[],playbooks:[],runs:[
  {id:'created',agent:'codex',status:'queued',title:'Created fallback',created_at:'2026-07-22T09:00:00Z'},
  {id:'failed',agent:'codex',status:'failed',title:'Failed',created_at:'2026-07-22T08:00:00Z',updated_at:'2026-07-22T10:00:00Z'},
  {id:'evented',agent:'codex',status:'running',title:'Evented',created_at:'2026-07-22T08:00:00Z',updated_at:'2026-07-22T09:00:00Z'},
  {id:'finished',agent:'codex',status:'succeeded',title:'Finished',created_at:'2026-07-22T08:00:00Z',updated_at:'2026-07-22T10:30:00Z'},
  {id:'gated',agent:'codex',status:'blocked',title:'Gated',created_at:'2026-07-22T08:00:00Z',updated_at:'2026-07-22T09:00:00Z'}
]};
Date.now=function(){return Date.parse('2026-07-22T12:00:00Z');};
function isLive(r){return r.status==='running'||r.status==='blocked';}
function runAgent(r){return r.agent;}
function dispName(a){return a||'Fort';}
function elapsed(){return '';}
function ago(){return '';}
function esc(v){return String(v||'');}
`
	if _, err := vm.RunString(setup + boardHTMLActivityAndDerivedScript(t)); err != nil {
		t.Fatalf("execute activity and derived-model helpers: %v", err)
	}
	value, err := vm.RunString(`
var latest={id:41,run_id:'evented',type:'tool',data:'{"name":"go test"}',time:'2026-07-22T12:00:00Z'};
trackEvent(latest);
trackEvent(latest);
JSON.stringify({ids:conversationRuns().map(function(r){return r.id;}),eventCount:actByRun.evented.length});`)
	if err != nil {
		t.Fatalf("evaluate conversation recency: %v", err)
	}
	const want = `{"ids":["evented","gated","finished","failed","created"],"eventCount":1}`
	if got := value.String(); got != want {
		t.Fatalf("conversation recency = %s, want %s", got, want)
	}
}

func TestBoardHTMLRunStatusRequiresPersistedWorkEvidence(t *testing.T) {
	vm := goja.New()
	setup := `
var agentOfRun={},actByRun={},ACT_MAX=20;
var model={gates:[{run_id:'gated',since:'2026-07-22T11:00:00Z'}],machines:[],backlog:[],playbooks:[],runs:[
  {id:'starting',agent:'codex',status:'running',created_at:'2026-07-22T08:00:00Z'},
  {id:'working',agent:'codex',status:'running',created_at:'2026-07-22T08:00:00Z'},
  {id:'gated',agent:'codex',status:'running',created_at:'2026-07-22T08:00:00Z'},
	{id:'paused',agent:'codex',status:'blocked',created_at:'2026-07-22T08:00:00Z'},
  {id:'finished',agent:'codex',status:'succeeded',created_at:'2026-07-22T08:00:00Z'},
  {id:'failed',agent:'codex',status:'failed',created_at:'2026-07-22T08:00:00Z'}
]};
Date.now=function(){return Date.parse('2026-07-22T12:00:00Z');};
function isLive(r){return r.status==='running'||r.status==='blocked';}
function runAgent(r){return r.agent;}
function dispName(a){return a||'Fort';}
function elapsed(){return '';}
function ago(){return '';}
function esc(v){return String(v||'');}
`
	if _, err := vm.RunString(setup + boardHTMLActivityAndDerivedScript(t)); err != nil {
		t.Fatalf("execute activity and derived-model helpers: %v", err)
	}
	value, err := vm.RunString(`
trackEvent({id:51,run_id:'working',type:'started',data:'codex',time:'2026-07-22T11:58:00Z'});
trackEvent({id:52,run_id:'gated',type:'message',data:'provider output',time:'2026-07-22T11:59:00Z'});
trackEvent({id:55,run_id:'paused',type:'message',data:'earlier provider output',time:'2026-07-22T11:55:00Z'});
trackEvent({id:53,run_id:'finished',type:'tool',data:'{"name":"go test"}',time:'2026-07-22T11:57:00Z'});
trackEvent({id:54,run_id:'failed',type:'stderr',data:'failure',time:'2026-07-22T11:56:00Z'});
JSON.stringify(model.runs.map(function(r){return {id:r.id,state:runState(r),label:runStatusLabel(r)};}));`)
	if err != nil {
		t.Fatalf("evaluate truthful run status: %v", err)
	}
	const want = `[{"id":"starting","state":"starting","label":"Starting"},{"id":"working","state":"working","label":"Working"},{"id":"gated","state":"paused-review","label":"Needs approval"},{"id":"paused","state":"paused","label":"Paused"},{"id":"finished","state":"terminal","label":"Finished"},{"id":"failed","state":"terminal","label":"Failed"}]`
	if got := value.String(); got != want {
		t.Fatalf("truthful run status = %s, want %s", got, want)
	}
}

func TestBoardHTMLThinkingOrbsOnlyAnimateForEvidenceBackedWorkingState(t *testing.T) {
	for _, want := range []string{
		`@keyframes orbCoreDrift`,
		`@keyframes orbEnergyBreathe`,
		`@keyframes orbReducedEnergyPulse`,
		`.fort-orb.is-thinking{`,
		`animation:orbCoreDrift`,
		`@media (prefers-reduced-motion: reduce)`,
		`animation:orbReducedEnergyPulse 4s ease-in-out infinite!important`,
		`transform:none!important`,
		`id="brandorb"`,
		`function isThinking(r){return !!r&&runState(r)==='working';}`,
		`function orbClass(thinking){return 'fort-orb'+(thinking?' is-thinking':'');}`,
		`$('#brandorb').classList.toggle('is-thinking',model.runs.some(isThinking));`,
		`orbClass(st.state==='working')`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML missing truthful Fort orb motion contract %q", want)
		}
	}
	reducedKeyframes := boardHTMLSection(t, "@keyframes orbReducedEnergyPulse", "  .fort-orb{")
	if strings.Contains(reducedKeyframes, "transform:") {
		t.Fatal("reduced-motion Fort pulse contains spatial transform")
	}

	vm := goja.New()
	setup := `
var agentOfRun={},actByRun={},ACT_MAX=20;
var model={gates:[{run_id:'gated',since:'2026-07-22T11:00:00Z'}],machines:[],backlog:[],playbooks:[],runs:[
  {id:'starting',agent:'codex',status:'running',created_at:'2026-07-22T08:00:00Z'},
  {id:'working',agent:'codex',status:'running',created_at:'2026-07-22T08:00:00Z'},
  {id:'gated',agent:'codex',status:'running',created_at:'2026-07-22T08:00:00Z'},
  {id:'finished',agent:'codex',status:'succeeded',created_at:'2026-07-22T08:00:00Z'}
]};
Date.now=function(){return Date.parse('2026-07-22T12:00:00Z');};
function isLive(r){return r.status==='running'||r.status==='blocked';}
function runAgent(r){return r.agent;}
function dispName(a){return a||'Fort';}
function elapsed(){return '';}
function ago(){return '';}
function esc(v){return String(v||'');}
`
	if _, err := vm.RunString(setup + boardHTMLActivityAndDerivedScript(t)); err != nil {
		t.Fatalf("execute activity, derived-model, and orb helpers: %v", err)
	}
	value, err := vm.RunString(`
trackEvent({id:61,run_id:'working',type:'started',data:'codex',time:'2026-07-22T11:58:00Z'});
trackEvent({id:62,run_id:'gated',type:'message',data:'provider output',time:'2026-07-22T11:59:00Z'});
trackEvent({id:63,run_id:'finished',type:'tool',data:'{"name":"go test"}',time:'2026-07-22T11:57:00Z'});
JSON.stringify(model.runs.map(function(r){return {id:r.id,thinking:isThinking(r),cls:orbClass(isThinking(r))};}));`)
	if err != nil {
		t.Fatalf("evaluate truthful Fort orb motion: %v", err)
	}
	const want = `[{"id":"starting","thinking":false,"cls":"fort-orb"},{"id":"working","thinking":true,"cls":"fort-orb is-thinking"},{"id":"gated","thinking":false,"cls":"fort-orb"},{"id":"finished","thinking":false,"cls":"fort-orb"}]`
	if got := value.String(); got != want {
		t.Fatalf("Fort orb motion states = %s, want %s", got, want)
	}
}

func TestBoardHTMLEventsUseWireTimeWithoutPerpetualWorkingCopy(t *testing.T) {
	if !strings.Contains(boardHTML, `e.time`) {
		t.Fatal("boardHTML does not use the persisted Event.time wire field")
	}
	for _, stale := range []string{`e.created_at||e.at`, `is working…`} {
		if strings.Contains(boardHTML, stale) {
			t.Errorf("boardHTML still uses misleading activity contract %q", stale)
		}
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

func TestBoardHTMLHandoffShowsImmediateStateAndSuppressesDuplicateClicks(t *testing.T) {
	for _, want := range []string{
		`id="handoffstatus"`,
		`role="status"`,
		`aria-live="polite"`,
		`let handoffPending=false;`,
		`if(handoffPending)return;`,
		`setHandoffPending(true`,
		`button.disabled=handoffPending;`,
		`button.setAttribute('aria-busy',handoffPending?'true':'false');`,
		`if(result.kind==='answer'&&r.status!==202)`,
		`catch(err)`,
		`finally{setHandoffPending(false`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Errorf("boardHTML missing truthful single-flight handoff contract %q", want)
		}
	}

	handoff := boardHTMLSection(t, `async function handoff(){`, `$('#handoff').addEventListener`)
	pending := strings.Index(handoff, `setHandoffPending(true`)
	preview := strings.Index(handoff, `await previewRoute()`)
	chat := strings.Index(handoff, `await fetch('/api/chat'`)
	if pending < 0 || preview < 0 || chat < 0 || pending > preview || pending > chat {
		t.Fatalf("handoff busy state must be visible before route/chat awaits: pending=%d preview=%d chat=%d", pending, preview, chat)
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

func TestBoardHTMLAddedStageInheritsProviderDefault(t *testing.T) {
	profileStart := strings.Index(boardHTML, "function assignmentProfilesFor")
	stageStart := strings.Index(boardHTML, "function addPlaybookStage(){")
	if profileStart < 0 || stageStart < 0 {
		t.Fatal("boardHTML playbook assignment helpers missing")
	}
	profileEnd := strings.Index(boardHTML[profileStart:], "function playbookByID")
	stageEnd := strings.Index(boardHTML[stageStart:], "function toggleShortcut")
	if profileEnd < 0 || stageEnd < 0 {
		t.Fatal("boardHTML playbook assignment helper boundaries missing")
	}

	vm := goja.New()
	script := `
var model={profiles:[
  {id:'claude:configured-default',agent:'claude',model:'',state:'ready'},
  {id:'codex:configured-default',agent:'codex',model:'',state:'ready'}
],playbooks:[{id:'inherited',delivery:'assignment',stages:[
  {order:1,name:'Prior',assignments:[{agent:'claude',model:''}]}
]}]};
var selectedPlaybook='inherited',saved=null;
function profileSelectable(p){return p&&p.state!=='unavailable'&&p.state!=='setup_required';}
function playbookByID(id){return model.playbooks[0];}
function cloneData(v){return JSON.parse(JSON.stringify(v));}
function stageAssignments(st){return st.assignments||[];}
function prompt(){return 'Inherited';}
function savePlaybook(next){saved=next;}
` + boardHTML[profileStart:profileStart+profileEnd] +
		boardHTML[stageStart:stageStart+stageEnd] +
		`addPlaybookStage();JSON.stringify(saved.stages[1].assignments[0]);`
	value, err := vm.RunString(script)
	if err != nil {
		t.Fatalf("execute add-stage helper: %v", err)
	}
	const want = `{"agent":"claude","model":"","profile":"claude:configured-default"}`
	if got := value.String(); got != want {
		t.Fatalf("inherited assignment = %s, want %s", got, want)
	}
}

func TestBoardHTMLPlaybookModelsComeFromProfileCatalog(t *testing.T) {
	for _, stale := range []string{`const PB_MODELS=`, `const PB_PROFILE_IDS=`} {
		if strings.Contains(boardHTML, stale) {
			t.Fatalf("boardHTML still duplicates the profile catalog in %q", stale)
		}
	}
	for _, want := range []string{
		`function assignmentProfilesFor(agent)`,
		`(model.profiles||[])`,
		`function assignmentProfileFor(agent,modelName)`,
		`function syncAssignmentProfile(a)`,
		`function modelOptionList(agent,current)`,
		`profileSelectable(profile)?'':' disabled'`,
	} {
		if !strings.Contains(boardHTML, want) {
			t.Fatalf("boardHTML missing profile-backed playbook model contract %q", want)
		}
	}

	section := boardHTMLSection(t, "function assignmentProfilesFor", "function playbookByID")
	vm := goja.New()
	setup := `
var model={profiles:[
  {id:'codex:configured-default',agent:'codex',model:'',display_name:'Codex configured default',state:'ready'},
  {id:'codex:gpt-5.6-sol',agent:'codex',model:'gpt-5.6-sol',display_name:'Codex Sol',state:'ready'},
  {id:'codex:gpt-5.6-terra',agent:'codex',model:'gpt-5.6-terra',display_name:'Codex Terra',state:'ready'},
  {id:'codex:gpt-5.6-luna',agent:'codex',model:'gpt-5.6-luna',display_name:'Codex Luna',state:'setup_required'},
  {id:'claude:configured-default',agent:'claude',model:'',display_name:'Claude configured default',state:'ready'}
]};
function profileSelectable(p){return p&&p.state!=='unavailable'&&p.state!=='setup_required';}
`
	if _, err := vm.RunString(setup + section); err != nil {
		t.Fatalf("execute profile-backed playbook helpers: %v", err)
	}
	value, err := vm.RunString(`
var assignment={agent:'codex',model:'gpt-5.6-terra'};
syncAssignmentProfile(assignment);
JSON.stringify({
  models:assignmentProfilesFor('codex').map(function(p){return p.model;}),
  assignment:assignment,
  missing:assignmentProfileFor('codex','invented')
});`)
	if err != nil {
		t.Fatalf("evaluate profile-backed playbook helpers: %v", err)
	}
	const want = `{"models":["","gpt-5.6-sol","gpt-5.6-terra","gpt-5.6-luna"],"assignment":{"agent":"codex","model":"gpt-5.6-terra","profile":"codex:gpt-5.6-terra"},"missing":null}`
	if got := value.String(); got != want {
		t.Fatalf("profile-backed playbook helpers = %s, want %s", got, want)
	}
}

func TestBoardHTMLQuickAnswerFailureRendersInline(t *testing.T) {
	for _, want := range []string{`quickAnswerError`, `Quick answer failed`, `if(assignMode==='quick'){`} {
		if !strings.Contains(boardHTML, want) {
			t.Fatalf("boardHTML missing inline Quick-answer failure state %q", want)
		}
	}
}

func TestBoardHTMLNeedsYouCountsOnlyGatesAndRecentFailures(t *testing.T) {
	vm := goja.New()
	setup := `
var agentOfRun={},actByRun={},ACT_MAX=20;
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
function dispName(a){return a||'Fort';}
function elapsed(){return '';}
function ago(){return '';}
function esc(v){return String(v||'');}
`
	if _, err := vm.RunString(setup + boardHTMLActivityAndDerivedScript(t)); err != nil {
		t.Fatalf("execute derived-model helpers: %v", err)
	}
	value, err := vm.RunString(`JSON.stringify({
  needCount:needCount(),
  recentFailureIDs:recentFailed().map(function(r){return r.id;})
})`)
	if err != nil {
		t.Fatalf("evaluate attention model: %v", err)
	}
	const want = `{"needCount":3,"recentFailureIDs":["recent","boundary"]}`
	if got := value.String(); got != want {
		t.Fatalf("attention model = %s, want %s", got, want)
	}
}
