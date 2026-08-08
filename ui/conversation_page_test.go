package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

func TestConversationPageIsTheFocusedDefaultSurface(t *testing.T) {
	for _, want := range []string{
		"Conversations", "Computers", "Projects", "In progress", "Scheduled today",
		"/api/conversation-seats", "/api/conversations", "/api/today", "/api/machines",
		"Everyone", "participant_ids", "/targets/",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("conversation page missing %q", want)
		}
	}
	for _, stale := range []string{"Playbook builder", "Metrics dashboard", "Backlog queue"} {
		if strings.Contains(conversationPageHTML, stale) {
			t.Errorf("focused page still exposes %q", stale)
		}
	}
	if !strings.Contains(conversationPageHTML, "@media (max-width: 860px)") {
		t.Fatal("conversation page has no compact layout")
	}
}

func TestTodayUsesFortConfiguredTimezoneAcrossBrowsers(t *testing.T) {
	for _, stale := range []string{
		"Intl.DateTimeFormat().resolvedOptions().timeZone",
		"/api/today?timezone=",
	} {
		if strings.Contains(conversationPageHTML, stale) {
			t.Errorf("Today still trusts browser-local timezone via %q", stale)
		}
	}
	for _, want := range []string{
		"api('/api/today')",
		"timeZone:state.today.timezone||'UTC'",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("Today does not use Fort's configured timezone: missing %q", want)
		}
	}
}

func TestWhenFormatsWithFortsTimezoneAndFailsTruthfully(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var timezoneCalls=[];
		var state={today:{timezone:'America/Chicago'}};
		function Date(value){this.value=value}
		Date.prototype.toLocaleTimeString=function(locales,options){
			timezoneCalls.push(options.timeZone||'browser-local');
			if(options.timeZone==='Unsupported/Alias')throw new RangeError('unsupported timezone');
			if(options.timeZone==='Crash/Zone')throw new TypeError('formatter failure');
			if(options.timeZone==='America/Chicago')return '9:30 PM';
			if(options.timeZone==='UTC')return '2:30 AM';
			return '2:30 PM';
		};
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "when")); err != nil {
		t.Fatal(err)
	}

	value, err := vm.RunString(`JSON.stringify({formatted:when('2026-08-03T02:30:00Z'),timezone:timezoneCalls[0]})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"formatted":"9:30 PM","timezone":"America/Chicago"}` {
		t.Fatalf("Fort timezone formatting = %s", got)
	}

	value, err = vm.RunString(`state.today.timezone='Unsupported/Alias';when('2026-08-03T02:30:00Z')`)
	if err != nil {
		t.Fatalf("unsupported browser timezone was not handled: %v", err)
	}
	if got := value.String(); got != "2:30 AM UTC" {
		t.Fatalf("unsupported browser timezone = %q, want explicit UTC fallback", got)
	}
	value, err = vm.RunString(`JSON.stringify(timezoneCalls)`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `["America/Chicago","Unsupported/Alias","UTC"]` {
		t.Fatalf("timezone calls = %s; browser-local time must never be used", got)
	}
	if _, err := vm.RunString(`state.today.timezone='Crash/Zone';when('2026-08-03T02:30:00Z')`); err == nil {
		t.Fatal("non-timezone formatter failure was hidden by the UTC fallback")
	}
}

func TestInitialReadSurfacesSettleIndependently(t *testing.T) {
	source := conversationPageFunction(t, "loadAll")
	if !strings.Contains(source, "Promise.allSettled([api('/api/projects'),api('/api/conversations'),api('/api/conversation-seats'),api('/api/today'),api('/api/machines')])") {
		t.Fatalf("initial reads do not settle independently: %s", source)
	}
	if strings.Contains(source, "await Promise.all([") || strings.Contains(source, "catch(error){showError(error)}") {
		t.Fatalf("one failed initial read can still replace every surface: %s", source)
	}
	for _, surface := range []string{"projects", "conversations", "seats", "today", "machines"} {
		if !strings.Contains(source, "applyReadResult('"+surface+"'") {
			t.Errorf("initial load does not independently accept %s", surface)
		}
	}
	if !strings.Contains(source, "if(!readSurface('conversations').loaded)return") {
		t.Fatalf("a failed conversation list can still be mistaken for a truthful empty list: %s", source)
	}
}

func TestReadFailurePreservesLastGoodValueUntilSuccess(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var state={reads:{today:{loaded:true,error:''}},today:{fresh_at:'old'}};
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"readSurface", "applyReadResult"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		var applied=0;
		var failed=applyReadResult('today',{status:'rejected',reason:new Error('offline')},function(next){applied++;state.today=next});
		var afterFailure={accepted:failed,applied:applied,freshAt:state.today.fresh_at,loaded:state.reads.today.loaded,error:state.reads.today.error};
		var succeeded=applyReadResult('today',{status:'fulfilled',value:{fresh_at:'new'}},function(next){applied++;state.today=next});
		JSON.stringify({afterFailure:afterFailure,afterSuccess:{accepted:succeeded,applied:applied,freshAt:state.today.fresh_at,loaded:state.reads.today.loaded,error:state.reads.today.error}})
	`)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"afterFailure":{"accepted":false,"applied":0,"freshAt":"old","loaded":true,"error":"offline"},"afterSuccess":{"accepted":true,"applied":1,"freshAt":"new","loaded":true,"error":""}}`
	if got := value.String(); got != want {
		t.Fatalf("read result handling = %s, want %s", got, want)
	}
}

func TestUnknownNonemptyProjectNeverLooksLikeInbox(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`var state={projects:[],reads:{projects:{loaded:false,error:"offline"}}};`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "projectName")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`JSON.stringify({inbox:projectName(""),unknown:projectName("missing-project")})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"inbox":"Inbox","unknown":"Project unavailable"}` {
		t.Fatalf("project labels = %s", got)
	}
}

func TestSelectedUnknownProjectGetsUnavailableOption(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var state={projects:[{id:"known-project",name:"Known"}]};
		function esc(value){return String(value)}
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "projectOptionsHTML")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`projectOptionsHTML("missing-project")`)
	if err != nil {
		t.Fatal(err)
	}
	html := value.String()
	for _, want := range []string{
		`<option value="">Inbox</option>`,
		`<option value="missing-project" selected disabled>Project unavailable</option>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("unknown-project options missing %q: %s", want, html)
		}
	}
	for _, name := range []string{"renderDraftThread", "renderThread"} {
		if source := conversationPageFunction(t, name); !strings.Contains(source, "projectOptionsHTML(") {
			t.Errorf("%s does not use truthful shared project options: %s", name, source)
		}
	}
}

func TestNeverLoadedConversationNavigationDoesNotInventCountsOrEmptyState(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var elements={
			"navigation-status":{innerHTML:""},
			"projects":{innerHTML:""},
			"conversation-list":{innerHTML:""}
		};
		var document={getElementById:function(id){return elements[id]}};
		var state={
			current:"",
			project:"all",
			projects:[{id:"project-1",name:"Project one"}],
			conversations:[],
			reads:{conversations:{loaded:false,error:"offline"}},
			expandedProjects:new Set(["project-1"])
		};
		function navigationStatusHTML(){return "<div>Conversations unavailable.</div>"}
		function conversationNavigationFocusKey(){return ""}
		function restoreConversationNavigationFocus(){}
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"esc", "readSurface", "projectName", "conversationRow", "renderSidebar"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		renderSidebar();
		JSON.stringify({projects:elements.projects.innerHTML,list:elements["conversation-list"].innerHTML})
	`)
	if err != nil {
		t.Fatal(err)
	}
	html := value.String()
	if strings.Contains(html, `<small>0</small>`) {
		t.Fatalf("never-loaded conversation navigation invented zero counts: %s", html)
	}
	if strings.Contains(html, "No conversations") {
		t.Fatalf("never-loaded conversation navigation invented an empty state: %s", html)
	}
}

func TestTodayFreshnessAndRefreshFailureStayVisible(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var state={reads:{today:{loaded:true,error:''}},today:{fresh_at:'2026-08-03T21:30:00Z'}};
		function esc(value){return String(value)}
		function when(){return '4:30 PM'}
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"readSurface", "retryNotice", "todayStatusHTML"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		var fresh=todayStatusHTML();
		state.reads.today.error='offline';
		var stale=todayStatusHTML();
		state.reads.today.loaded=false;
		var unavailable=todayStatusHTML();
		JSON.stringify({fresh:fresh,stale:stale,unavailable:unavailable})
	`)
	if err != nil {
		t.Fatal(err)
	}
	var rendered struct {
		Fresh       string `json:"fresh"`
		Stale       string `json:"stale"`
		Unavailable string `json:"unavailable"`
	}
	if err := json.Unmarshal([]byte(value.String()), &rendered); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Updated", `datetime="2026-08-03T21:30:00Z"`, "4:30 PM"} {
		if !strings.Contains(rendered.Fresh, want) {
			t.Errorf("fresh Today status missing %q: %s", want, rendered.Fresh)
		}
	}
	if strings.Contains(rendered.Fresh, `role="status"`) || strings.Contains(conversationPageHTML, `id="today-status" aria-live=`) {
		t.Fatalf("routine five-second freshness updates would be announced repeatedly: %s", rendered.Fresh)
	}
	for _, want := range []string{"Showing data from 4:30 PM", `onclick="refreshToday()"`, ">Retry</button>"} {
		if !strings.Contains(rendered.Stale, want) {
			t.Errorf("stale Today status missing %q: %s", want, rendered.Stale)
		}
	}
	if !strings.Contains(rendered.Stale, `role="status"`) {
		t.Fatalf("Today refresh failure is not announced: %s", rendered.Stale)
	}
	if !strings.Contains(rendered.Unavailable, "Today unavailable") || strings.Contains(rendered.Unavailable, "Showing data from") {
		t.Fatalf("never-loaded Today status is not truthful: %s", rendered.Unavailable)
	}

	refresh := conversationPageFunction(t, "refreshToday")
	for _, want := range []string{"applyReadResult('today'", "status:'rejected'", "renderToday()"} {
		if !strings.Contains(refresh, want) {
			t.Errorf("Today refresh still hides failure state %q: %s", want, refresh)
		}
	}
}

func TestInventoryRefreshKeepsPartialSuccessAndExposesRetry(t *testing.T) {
	refresh := conversationPageFunction(t, "refreshInventory")
	for _, want := range []string{
		"Promise.allSettled([api('/api/conversation-seats'),api('/api/machines')])",
		"applyReadResult('seats'",
		"applyReadResult('machines'",
		"renderInventorySurfaces(identityChanged)",
	} {
		if !strings.Contains(refresh, want) {
			t.Errorf("inventory refresh is not independently truthful: missing %q in %s", want, refresh)
		}
	}
	if strings.Contains(refresh, "catch(error){}") {
		t.Fatalf("inventory refresh still silently swallows failures: %s", refresh)
	}

	vm := goja.New()
	if _, err := vm.RunString(`
		var state={reads:{seats:{loaded:true,error:'offline'},machines:{loaded:true,error:''}}};
		function esc(value){return String(value)}
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"readSurface", "retryNotice", "inventoryStatusHTML"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		var stale=inventoryStatusHTML('refreshInventory()','Retry');
		state.reads.seats.loaded=false;
		var incomplete=inventoryStatusHTML('refreshInventory()','Retry');
		state.reads.seats.loaded=true;state.reads.seats.error='';
		var clear=inventoryStatusHTML('refreshInventory()','Retry');
		JSON.stringify({stale:stale,incomplete:incomplete,clear:clear})
	`)
	if err != nil {
		t.Fatal(err)
	}
	got := value.String()
	for _, want := range []string{"Agent status may be out of date.", "Agent status incomplete.", `onclick=\"refreshInventory()\"`, ">Retry</button>"} {
		if !strings.Contains(got, want) {
			t.Errorf("inventory status missing %q: %s", want, got)
		}
	}
	if !strings.Contains(got, `"clear":""`) {
		t.Fatalf("successful inventory refresh leaves an error notice: %s", got)
	}
	for _, name := range []string{"draftSeatChoices", "renderParticipantManager"} {
		source := conversationPageFunction(t, name)
		if !strings.Contains(source, "inventoryStatusHTML('refreshInventory()','Retry')") {
			t.Errorf("%s cannot clear a partial machine-inventory failure: %s", name, source)
		}
		if strings.Contains(source, "inventoryStatusHTML('recheckSeats()'") {
			t.Errorf("%s retries only seats for a combined inventory failure: %s", name, source)
		}
	}
	if !strings.Contains(conversationPageHTML, `.surface-status button{flex:none;min-width:44px;min-height:44px`) {
		t.Fatal("surface retry controls do not preserve the 44-point interaction target")
	}
}

func TestNewConversationOpensInlineDraftWithFocusedSeatPicker(t *testing.T) {
	for _, want := range []string{
		`onclick="startConversationDraft()"`,
		"draft:null",
		"function startConversationDraft()",
		"state.current='new'",
		"history.replaceState(null,'','#new')",
		"state.events.close()",
		"state.composerFocusUntil=Date.now()+5000",
		"function renderDraftThread(draft,keepFocus)",
		"thread.scrollTop=0",
		"var readySeats=state.seats.filter(function(s){return s.state==='ready'})",
		"var setupSeats=state.seats.filter(function(s){return s.state!=='ready'})",
		"participantSeatIDs",
		"targetSeatIDs",
		"readySeats.length===1?new Set([readySeats[0].id]):new Set()",
		"function toggleDraftParticipant(id)",
		`<details class="setup-seats"`,
		"Needs setup (",
		"function seatSetupReason(reason)",
		"Sign in required",
		"Update required",
		"Model unavailable",
		"Agent unavailable",
		"Each ready seat fixes an agent profile and computer.",
		".dialog-actions{position:sticky;bottom:0",
		"padding:8px 12px;min-height:44px;cursor:pointer",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("inline new-conversation draft missing %q", want)
		}
	}
	for _, stale := range []string{`id="new-conversation-dialog"`, `id="conversation-title"`, "function createConversation(event)"} {
		if strings.Contains(conversationPageHTML, stale) {
			t.Errorf("new conversation still depends on setup modal contract %q", stale)
		}
	}
}

func TestEmptyConversationStateOffersNewConversationAction(t *testing.T) {
	source := conversationPageFunction(t, "renderThread")
	want := `<button type="button" class="primary empty-action" onclick="startConversationDraft()">New conversation</button>`
	if !strings.Contains(source, want) {
		t.Fatalf("empty conversation state has no direct New conversation action: %s", source)
	}
	if !strings.Contains(conversationPageHTML, `.empty-action{display:block;margin:16px auto 0}`) {
		t.Fatal("empty conversation action is not separated from its explanatory copy")
	}
}

func TestZeroConversationPlainRootStartsInlineDraft(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(conversationPageFunction(t, "shouldStartConversationDraft")); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path              string
		hash              string
		conversationCount int
		want              bool
	}{
		{path: "/", hash: "", conversationCount: 0, want: true},
		{path: "/", hash: "#new", conversationCount: 3, want: true},
		{path: "/", hash: "", conversationCount: 3, want: false},
		{path: "/", hash: "#conversation=known", conversationCount: 0, want: false},
		{path: "/unknown", hash: "", conversationCount: 0, want: false},
	} {
		value, err := vm.RunString(`shouldStartConversationDraft(` + strconv.Quote(test.path) + `,` + strconv.Quote(test.hash) + `,` + strconv.Itoa(test.conversationCount) + `)`)
		if err != nil {
			t.Fatal(err)
		}
		if got := value.ToBoolean(); got != test.want {
			t.Fatalf("shouldStartConversationDraft(%q, %q, %d) = %t, want %t", test.path, test.hash, test.conversationCount, got, test.want)
		}
	}
	load := conversationPageFunction(t, "loadAll")
	if !strings.Contains(load, "shouldStartConversationDraft(location.pathname,location.hash,state.conversations.length)") {
		t.Fatalf("loadAll does not enter the draft from an empty plain root: %s", load)
	}
}

func TestDraftSeatPickerUsesOneClearCompletionAndRecheckAction(t *testing.T) {
	source := conversationPageFunction(t, "renderDraftThread")
	if got := strings.Count(source, `toggleDraftAgentPicker()">Done</button>`); got != 1 {
		t.Fatalf("draft seat picker has %d completion actions, want 1: %s", got, source)
	}
	seats := conversationPageFunction(t, "draftSeatChoices")
	if got := strings.Count(seats, `onclick="recheckSeats()">Recheck</button>`); got != 1 {
		t.Fatalf("draft seat picker has %d explicit recheck actions, want 1: %s", got, seats)
	}
	if strings.Contains(seats, `>Refresh status</button>`) {
		t.Fatalf("draft seat picker exposes competing refresh copy: %s", seats)
	}
	if !strings.Contains(conversationPageHTML, `stale:'Status is stale'`) {
		t.Fatal("stale seat copy is not explicit")
	}
}

func TestFailedTargetRecoveryMatchesTheFailure(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		function esc(value){return String(value)}
		function participantTargetLabel(){return "Codex · Sol — Studio"}
		var state={seatRecheck:false};
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "renderTargetStates")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`JSON.stringify({
		unready:renderTargetStates([{id:"unready-target",participant_id:"participant-1",attempt:1,state:"failed",run_id:"run-1",error_code:"seat_unready",error:"offline"}],{"participant-1":{}}),
		provider:renderTargetStates([{id:"provider-target",participant_id:"participant-1",attempt:1,state:"failed",run_id:"run-2",error_code:"provider_failed",error:"provider exited"}],{"participant-1":{}})
	})`)
	if err != nil {
		t.Fatal(err)
	}
	var rendered struct {
		Unready  string `json:"unready"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal([]byte(value.String()), &rendered); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`onclick="recheckAndRetryTarget('unready-target')"`, `>Recheck and retry</button>`} {
		if !strings.Contains(rendered.Unready, want) {
			t.Errorf("seat readiness recovery missing %q: %s", want, rendered.Unready)
		}
	}
	for _, want := range []string{`onclick="retryTarget('provider-target')"`, `>Retry</button>`} {
		if !strings.Contains(rendered.Provider, want) {
			t.Errorf("ordinary failure recovery missing %q: %s", want, rendered.Provider)
		}
	}
	if strings.Contains(rendered.Provider, "recheckAndRetryTarget") {
		t.Fatalf("ordinary provider failure unnecessarily rechecks every seat: %s", rendered.Provider)
	}

	recovery := conversationPageFunction(t, "recheckAndRetryTarget")
	recheck := strings.Index(recovery, "await requestSeatRecheck()")
	retry := strings.Index(recovery, "'/targets/'+encodeURIComponent(id)+'/retry'")
	if recheck < 0 || retry < 0 || recheck > retry {
		t.Fatalf("failed target does not complete recheck before exact retry: %s", recovery)
	}
	for _, want := range []string{
		"var conversationID=state.current",
		"if(state.current!==conversationID)return",
		"seatRecheck:false",
		"state.seatRecheck=true",
		"'/api/conversation-seats/recheck'",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("conversation recheck contract missing %q", want)
		}
	}
}

func TestDraftSeatPickerKeepsStickyActionCompact(t *testing.T) {
	if !strings.Contains(conversationPageHTML, `.draft-picker .dialog-actions{margin:12px -16px -16px;padding:10px 16px}`) {
		t.Fatal("draft seat picker reuses the oversized dialog footer spacing")
	}
}

func TestStartingConversationDraftDoesNotPersistAnything(t *testing.T) {
	source := conversationPageFunction(t, "startConversationDraft")
	if strings.Contains(source, "api(") || strings.Contains(source, "fetch(") {
		t.Fatalf("starting a draft performs a network write: %s", source)
	}
}

func TestStartingConversationDraftAlwaysStartsFreshInSelectedProject(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var crypto={randomUUID:function(){return "new-client-turn"}};
		var history={replaceState:function(){}};
		var document={getElementById:function(){return {
			classList:{add:function(){},remove:function(){}},
			focus:function(){},
			scrollTop:99
		}}};
		function setTimeout(callback){callback()}
		function renderSidebar(){}
		function renderMain(){}
		function toggleConversationNavigation(){}
		var state={
			projects:[{id:"project-new"}],
			project:"project-new",
			seats:[
				{id:"codex@studio",state:"ready"},
				{id:"claude@mini",state:"ready"}
			],
			draft:{
				projectID:"project-old",
				participantSeatIDs:new Set(["old@seat"]),
				targetSeatIDs:new Set(["old@seat"]),
				everyoneTarget:true,
				pickerOpen:false,
				sending:true,
				error:"old error",
				pendingClientTurnID:"old-client-turn"
			},
			current:"old-conversation",
			detail:{},
			view:"computers",
			targets:new Set(["old-participant"]),
			targetAll:true,
			conversationError:"old conversation error",
			events:null,
			composerFocusUntil:0
		};
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "draftProjectID")); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "startConversationDraft")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`startConversationDraft();JSON.stringify({
		projectID:state.draft.projectID,
		participants:Array.from(state.draft.participantSeatIDs),
		targets:Array.from(state.draft.targetSeatIDs),
		everyone:state.draft.everyoneTarget,
		pickerOpen:state.draft.pickerOpen,
		sending:state.draft.sending,
		error:state.draft.error,
		clientTurnID:state.draft.pendingClientTurnID
	})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"projectID":"project-new","participants":[],"targets":[],"everyone":false,"pickerOpen":true,"sending":false,"error":"","clientTurnID":"new-client-turn"}` {
		t.Fatalf("fresh draft = %s", got)
	}
}

func TestSoleReadySeatNeverBecomesImplicitRecipient(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var crypto={randomUUID:function(){return "new-client-turn"}};
		var history={replaceState:function(){}};
		var document={getElementById:function(){return {
			classList:{add:function(){},remove:function(){}},
			focus:function(){},
			scrollTop:99
		}}};
		function setTimeout(callback){callback()}
		function renderSidebar(){}
		function renderMain(){}
		function draftProjectID(){return ""}
		function toggleConversationNavigation(){}
		var state={
			seats:[{id:"solo",state:"ready"}],
			draft:null,
			current:null,
			detail:null,
			view:"conversations",
			targets:new Set(),
			targetAll:false,
			conversationError:"",
			events:null,
			composerFocusUntil:0
		};
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "startConversationDraft")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`startConversationDraft();JSON.stringify({
		participants:Array.from(state.draft.participantSeatIDs),
		targets:Array.from(state.draft.targetSeatIDs),
		everyone:state.draft.everyoneTarget,
		pickerOpen:state.draft.pickerOpen
	})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"participants":["solo"],"targets":[],"everyone":false,"pickerOpen":false}` {
		t.Fatalf("sole-seat draft = %s; participant preselection must not silently arm a recipient", got)
	}
}

func TestConversationTitleFromFirstMessage(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(conversationPageFunction(t, "conversationTitleFromMessage")); err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		message string
		want    string
	}{
		"single line":                 {message: "hello", want: "hello"},
		"leading blank lines skipped": {message: "  \r\n\n  Ship it  \nDetails", want: "Ship it"},
		"later lines excluded":        {message: "Plan the release\nwith these details", want: "Plan the release"},
	} {
		t.Run(name, func(t *testing.T) {
			value, err := vm.RunString(`conversationTitleFromMessage(` + strconv.Quote(test.message) + `)`)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.String(); got != test.want {
				t.Fatalf("title = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDraftTargetsMapByExactSeatIdentity(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(conversationPageFunction(t, "mapDraftTargetParticipants")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`JSON.stringify(mapDraftTargetParticipants([
		{id:"p-studio",seat_id:"codex@studio"},
		{id:"p-extra",seat_id:"claude@studio"},
		{id:"p-mini",seat_id:"hermes@mini"}
	],new Set(["hermes@mini","codex@studio"])))`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `["p-mini","p-studio"]` {
		t.Fatalf("mapped participants = %s", got)
	}
	if _, err := vm.RunString(`mapDraftTargetParticipants([{id:"p-studio",seat_id:"codex@studio"}],new Set(["hermes@mini"]))`); err == nil {
		t.Fatal("missing selected seat did not fail closed")
	}
}

func TestDraftParticipantsDoNotImplicitlyBecomeRecipients(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var state={draft:{participantSeatIDs:new Set(),targetSeatIDs:new Set(),everyoneTarget:false,error:""}};
		function renderThread(){}
		function setTimeout(callback){callback()}
		var document={getElementById:function(){return null}};
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rerenderAndFocus", "toggleDraftParticipant"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`toggleDraftParticipant("codex@studio");JSON.stringify({participants:Array.from(state.draft.participantSeatIDs),targets:Array.from(state.draft.targetSeatIDs),everyone:state.draft.everyoneTarget})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"participants":["codex@studio"],"targets":[],"everyone":false}` {
		t.Fatalf("draft selection = %s; choosing a participant must not choose a recipient", got)
	}
}

func TestDraftFirstSendIsSingleFlightAndAdoptsDurableConversation(t *testing.T) {
	for _, want := range []string{
		"if(!state.draft||state.draft.sending)return",
		"state.draft.sending=true",
		"pendingClientTurnID:crypto.randomUUID()",
		"title:conversationTitleFromMessage(text)",
		"seat_ids:Array.from(draft.participantSeatIDs)",
		"mapDraftTargetParticipants(detail.participants,draft.targetSeatIDs)",
		"state.current=conversationID",
		"history.replaceState(null,'','#conversation='+encodeURIComponent(conversationID))",
		"await postConversationTurn(conversationID,text,draft.pendingClientTurnID,participantIDs)",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("draft first-send transition missing %q", want)
		}
	}
}

func TestDraftTurnFailureCannotCreateTheConversationAgain(t *testing.T) {
	source := conversationPageFunction(t, "sendDraftTurn")
	created := strings.Index(source, "state.draft=null")
	sent := strings.Index(source, "await postConversationTurn(conversationID,text,draft.pendingClientTurnID,participantIDs)")
	if created < 0 || sent < 0 || created > sent {
		t.Fatalf("draft is not retired before its first durable turn is sent: %s", source)
	}
	for _, want := range []string{
		"var pending={conversationID:conversationID,text:text,clientTurnID:draft.pendingClientTurnID,participantIDs:participantIDs,sending:true}",
		"pending.sending=false",
		"function retryPendingTurn()",
		"postConversationTurn(conversationID,pending.text,pending.clientTurnID,pending.participantIDs)",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("durable first-turn recovery missing %q", want)
		}
	}
}

func TestConversationPageRestoresExplicitTargetsWithoutImplicitEveryone(t *testing.T) {
	for _, want := range []string{
		"function targetsForConversation(detail)",
		"var latest=detail.turns[detail.turns.length-1]",
		"if(!detail.turns.length)return []",
		"state.targets=new Set(targetsForConversation(detail))",
		"function conversationIDFromLocation()",
		"history.replaceState(null,'','#conversation='+encodeURIComponent(id))",
		"await openConversation(selected)",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("conversation restoration missing %q", want)
		}
	}
	if strings.Contains(conversationPageHTML, "state.targets=new Set(state.detail.participants.filter(function(p){return p.state!=='removed'}).map(function(p){return p.id}))") {
		t.Fatal("opening a conversation still targets Everyone implicitly")
	}
}

func TestConversationStreamShowsTruthfulReconnectState(t *testing.T) {
	for _, want := range []string{
		"streamStatus:''",
		"streamRetry:false",
		`role="status"`,
		"Live updates paused. Reconnecting…",
		"This conversation may be out of date.",
		`onclick="retryConversationStream()"`,
		"source.addEventListener('error'",
		"source.addEventListener('open'",
		"reloadConversationFromStream(id,source)",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("truthful reconnect state missing %q", want)
		}
	}
}

func TestConversationStreamReconnectRebuildsAndGuardsCurrentSource(t *testing.T) {
	source := conversationPageFunction(t, "reloadConversationFromStream")
	for _, want := range []string{
		"var token={}",
		"state.streamReloadToken=token",
		"var detail=await api('/api/conversations/'+encodeURIComponent(id))",
		"if(state.current!==id||state.events!==source||state.streamReloadToken!==token)return false",
		"adoptConversationDetail(detail,true)",
		"state.streamStatus='This conversation may be out of date.'",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("guarded stream rebuild missing %q: %s", want, source)
		}
	}
}

func TestConversationStreamFrameInvalidatesAnOlderReconnectSnapshot(t *testing.T) {
	source := conversationPageFunction(t, "connectEvents")
	frame := strings.Index(source, "source.addEventListener('conversation'")
	parsed := strings.Index(source, "JSON.parse(event.data)")
	invalidated := strings.Index(source[parsed:], "state.streamReloadToken=null")
	adopted := strings.Index(source[parsed:], "adoptConversationDetail(detail,true)")
	if frame < 0 || parsed < frame || invalidated < 0 || adopted < 0 || invalidated > adopted {
		t.Fatalf("a valid SSE frame does not invalidate an older reconnect GET before adoption: %s", source)
	}
}

func TestConversationStreamFrameDoesNotRenderConversationDOMWhileComputersSelected(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var documentCalls=0;
		var refreshedToday=0;
		var reloadedLists=0;
		var source;
		var state={
			current:'conversation-1',
			view:'conversations',
			events:null,
			streamReloadToken:null,
			streamStatus:'',
			streamRetry:false,
			targetAll:false,
			targets:new Set(),
			composerFocusUntil:0,
			draft:null,
			projects:[],
			detail:null
		};
		var document={getElementById:function(){documentCalls++;return null}};
		function EventSource(){
			this.handlers={};
			this.close=function(){};
			this.addEventListener=function(name,handler){this.handlers[name]=handler};
			source=this;
		}
		function adoptConversationDetail(detail){state.detail=detail}
		function esc(value){return String(value)}
		function conversationNavigationButton(){return ''}
		function todaySheetButton(){return ''}
		function renderConversationStreamStatus(){}
		function setConversationStreamStatus(){}
		function refreshToday(){refreshedToday++}
		function reloadLists(){reloadedLists++;return {catch:function(){}}}
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"renderThread", "connectEvents"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		connectEvents('conversation-1');
		state.view='computers';
		source.handlers.conversation({data:JSON.stringify({
			conversation:{id:'conversation-1',title:'Focused',project_id:''},
			participants:[],turns:[],targets:[],messages:[]
		})});
		JSON.stringify({
			adopted:state.detail.conversation.id,
			documentCalls:documentCalls,
			refreshedToday:refreshedToday,
			reloadedLists:reloadedLists
		})
	`)
	if err != nil {
		t.Fatalf("conversation frame rendered missing conversation DOM: %v", err)
	}
	if got := value.String(); got != `{"adopted":"conversation-1","documentCalls":0,"refreshedToday":1,"reloadedLists":1}` {
		t.Fatalf("computers-view conversation frame = %s", got)
	}
}

func TestConversationOpenAdoptsOnlySuccessfulLatestRequest(t *testing.T) {
	source := conversationPageFunction(t, "openConversation")
	loaded := strings.Index(source, "var detail=await api('/api/conversations/'+encodeURIComponent(id))")
	adopted := strings.Index(source, "state.current=id")
	if loaded < 0 || adopted < 0 || adopted < loaded {
		t.Fatalf("conversation identity changes before its durable detail loads: %s", source)
	}
	for _, want := range []string{"var token={}", "state.openConversationToken=token", "if(state.openConversationToken!==token)return", "adoptConversationDetail(detail,false)"} {
		if !strings.Contains(source, want) {
			t.Errorf("guarded conversation navigation missing %q: %s", want, source)
		}
	}
}

func TestConversationSendCompletionCannotMutateAConversationOpenedLater(t *testing.T) {
	post := conversationPageFunction(t, "postConversationTurn")
	for _, want := range []string{
		"conversationID,text,clientTurnID,participantIDs",
		"/api/conversations/'+encodeURIComponent(conversationID)+'/turns",
		"return api('/api/conversations/'+encodeURIComponent(conversationID))",
	} {
		if !strings.Contains(post, want) {
			t.Errorf("conversation-bound post missing %q: %s", want, post)
		}
	}
	if strings.Contains(post, "state.current") {
		t.Fatalf("conversation-bound post reads mutable navigation state: %s", post)
	}

	for _, name := range []string{"sendDraftTurn", "sendTurn", "retryPendingTurn"} {
		source := conversationPageFunction(t, name)
		for _, want := range []string{
			"conversationID",
			"postConversationTurn(conversationID",
			"state.current!==conversationID||state.pendingTurn!==pending",
		} {
			if !strings.Contains(source, want) {
				t.Errorf("%s does not guard a late completion with %q: %s", name, want, source)
			}
		}
	}

	draft := conversationPageFunction(t, "sendDraftTurn")
	created := strings.Index(draft, "detail=await api('/api/conversations'")
	guarded := strings.LastIndex(draft, "if(state.current!=='new'||state.draft!==draft)return")
	adopted := strings.Index(draft, "state.current=conversationID")
	if created < 0 || guarded < created || adopted < guarded {
		t.Fatalf("late draft creation can still replace a conversation opened later: %s", draft)
	}
}

func TestTargetsForConversationUsesOnlyTheLatestExplicitSet(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(conversationPageFunction(t, "targetsForConversation")); err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		detail string
		want   string
	}{
		"no turn has no implicit target": {
			detail: `{participants:[{id:"solo",state:"active"}],turns:[],targets:[]}`,
			want:   `[]`,
		},
		"latest turn wins and retry is deduplicated": {
			detail: `{participants:[{id:"one",state:"active"},{id:"two",state:"active"}],turns:[{id:"old"},{id:"latest"}],targets:[{turn_id:"old",participant_id:"one"},{turn_id:"latest",participant_id:"two"},{turn_id:"latest",participant_id:"two"}]}`,
			want:   `["two"]`,
		},
		"removed participant is not restored": {
			detail: `{participants:[{id:"gone",state:"removed"}],turns:[{id:"latest"}],targets:[{turn_id:"latest",participant_id:"gone"}]}`,
			want:   `[]`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			value, err := vm.RunString(`JSON.stringify(targetsForConversation(` + test.detail + `))`)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.String(); got != test.want {
				t.Fatalf("targets = %s, want %s", got, test.want)
			}
		})
	}
}

func TestConversationComposerExposesAndEnforcesExplicitTargets(t *testing.T) {
	for _, want := range []string{
		`role="group" aria-label="Message recipients"`,
		`aria-describedby="composer-hint"`,
		`id="composer-hint" class="composer-hint" role="status" aria-live="polite"`,
		`aria-pressed="'+(everyoneSelected?'true':'false')+'"`,
		`aria-pressed="'+(state.targets.has(p.id)?'true':'false')+'"`,
		`var sendDisabled=state.targets.size&&draft.trim()&&!state.pendingTurn?'':'disabled'`,
		`oninput="updateSendButton()"`,
		`<button id="send-button" class="primary" '+sendDisabled+' onclick="sendComposer()">Send</button>`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("conversation composer does not expose explicit recipients: missing %q", want)
		}
	}
}

func TestConversationComposerExplainsWhySendNeedsARecipient(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var elements={
			"send-button":{disabled:false},
			"message-text":{value:"Ready to ship"},
			"composer-hint":{textContent:""},
			"recipient-controls":{classList:{values:new Set(),toggle:function(name,on){on?this.values.add(name):this.values.delete(name)}}}
		};
		var document={getElementById:function(id){return elements[id]||null}};
		var state={current:"conversation-1",draft:null,targets:new Set(),pendingTurn:null};
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"composerGuidance", "updateSendButton"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		updateSendButton();
		var missing={
			disabled:elements["send-button"].disabled,
			hint:elements["composer-hint"].textContent,
			attention:elements["recipient-controls"].classList.values.has("needs-target")
		};
		state.targets.add("participant-1");
		updateSendButton();
		JSON.stringify({missing:missing,chosen:{
			disabled:elements["send-button"].disabled,
			hint:elements["composer-hint"].textContent,
			attention:elements["recipient-controls"].classList.values.has("needs-target")
		}})
	`)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"missing":{"disabled":true,"hint":"Choose who should answer.","attention":true},"chosen":{"disabled":false,"hint":"","attention":false}}`
	if got := value.String(); got != want {
		t.Fatalf("composer guidance = %s, want %s", got, want)
	}
}

func TestAgentAndRecipientSelectionPreservesKeyboardFocusAfterRerender(t *testing.T) {
	for _, want := range []string{
		`id="draft-agents-button"`,
		`id="draft-participant-'+esc(s.id)+'"`,
		`id="recipient-everyone"`,
		`id="recipient-'+esc(s.id)+'"`,
		`id="recipient-'+esc(p.id)+'"`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("rerendered selection control has no stable focus target: missing %q", want)
		}
	}

	vm := goja.New()
	if _, err := vm.RunString(`
		var focused=[];
		function renderThread(){}
		function setTimeout(callback){callback()}
		var document={getElementById:function(id){return {focus:function(){focused.push(id)}}}};
		var state={
			draft:{participantSeatIDs:new Set(['seat-a']),targetSeatIDs:new Set(['seat-a']),everyoneTarget:false,error:'',pickerOpen:true},
			targetAll:false,
			targets:new Set(['participant-a']),
			detail:{participants:[{id:'participant-a',state:'active'}]}
		};
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"rerenderAndFocus", "toggleDraftParticipant", "toggleDraftAgentPicker",
		"toggleDraftTarget", "targetDraftEveryone", "toggleTarget", "targetEveryone",
	} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		toggleDraftParticipant('seat-a');
		toggleDraftParticipant('seat-a');
		toggleDraftTarget('seat-a');
		targetDraftEveryone();
		toggleTarget('participant-a');
		targetEveryone();
		toggleDraftAgentPicker();
		JSON.stringify(focused)
	`)
	if err != nil {
		t.Fatal(err)
	}
	want := `["draft-participant-seat-a","draft-participant-seat-a","recipient-seat-a","recipient-everyone","recipient-participant-a","recipient-everyone","draft-agents-button"]`
	if got := value.String(); got != want {
		t.Fatalf("restored focus = %s, want %s", got, want)
	}
}

func TestEveryoneRequiresExplicitControlActivation(t *testing.T) {
	for _, want := range []string{
		"targetAll:false",
		"everyoneTarget:false",
		"state.targetAll=false",
		"state.draft.everyoneTarget=false",
		"state.targetAll=draft.everyoneTarget",
		"function toggleTarget(id){state.targetAll=false",
		"function targetEveryone(){var active=state.detail.participants.filter(function(p){return p.state!=='removed'});if(state.targetAll)",
		"var everyoneSelected=state.targetAll",
		"var everyoneSelected=state.draft.everyoneTarget",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("explicit Everyone state missing %q", want)
		}
	}
	for _, stale := range []string{
		"var everyoneSelected=state.targets.size===active.length&&active.length",
		"var everyoneSelected=participants.length&&state.draft.targetSeatIDs.size===participants.length",
	} {
		if strings.Contains(conversationPageHTML, stale) {
			t.Errorf("Everyone still inferred from selected target count: %q", stale)
		}
	}
}

func TestEveryoneStateChangesOnlyThroughExplicitControl(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`var state={targetAll:false,targets:new Set(["one","two"]),detail:{participants:[{id:"one",state:"active"},{id:"two",state:"active"}]}};function renderThread(){};function setTimeout(callback){callback()};var document={getElementById:function(){return null}}`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rerenderAndFocus", "toggleTarget", "targetEveryone"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := vm.RunString(`targetEveryone()`); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`JSON.stringify({all:state.targetAll,targets:Array.from(state.targets)})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"all":true,"targets":["one","two"]}` {
		t.Fatalf("explicit Everyone state = %s", got)
	}
	if _, err := vm.RunString(`toggleTarget("one")`); err != nil {
		t.Fatal(err)
	}
	value, err = vm.RunString(`JSON.stringify({all:state.targetAll,targets:Array.from(state.targets)})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"all":false,"targets":["two"]}` {
		t.Fatalf("individual target state = %s", got)
	}
}

func TestConversationPageKeepsCompactThreadAndComposerInViewport(t *testing.T) {
	for _, want := range []string{
		"function machineDisplayName(machine,machines)",
		"function participantTargetLabel(p)",
		`aria-label="'+esc(participantTargetLabel(p))+'"`,
		`title="'+esc(participantTargetLabel(p))+'"`,
		"esc(participantTargetLabel(p))",
		".target-row::-webkit-scrollbar{display:none}",
		"body{overflow:hidden}.app{height:100dvh;min-height:0;display:flex;flex-direction:column;overflow:hidden}",
		".main{height:auto;min-height:0;flex:1}",
		".thread-head{height:auto;min-height:104px;flex-wrap:wrap;align-content:center;gap:7px;",
		".thread-head>div:first-child{width:100%;min-width:0}",
		".thread-tools{width:100%;gap:6px}",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("compact conversation layout missing %q", want)
		}
	}
}

func TestSmallSecondaryTextUsesTheReadableMutedToken(t *testing.T) {
	for _, want := range []string{
		`.project-row small{color:var(--muted)}`,
		`.rail-empty{color:var(--muted);font-size:12px`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("small secondary text does not use the readable muted token: missing %q", want)
		}
	}
	if strings.Contains(conversationPageHTML, `color:#657065`) {
		t.Fatal("small secondary text retains the measured 3.77:1 low-contrast color")
	}
}

func TestCompactConversationNavigationIsOnDemand(t *testing.T) {
	for _, want := range []string{
		`id="conversation-navigation" class="sidebar" aria-label="Conversation navigation" onkeydown="conversationNavigationKey(event)"`,
		`id="conversation-navigation-close"`,
		`id="conversation-navigation-button"`,
		`aria-controls="conversation-navigation"`,
		`function conversationNavigationButton()`,
		`function toggleConversationNavigation(force)`,
		`function conversationNavigationKey(event)`,
		`.sidebar.sheet{display:block;position:fixed;z-index:40;inset:12px 12px auto;max-height:calc(100dvh - 24px);overflow:auto`,
		`.sidebar.sheet .sidebar-projects{display:block}`,
		`.sidebar.sheet .conversation-list{flex-direction:column`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("compact conversation navigator missing %q", want)
		}
	}
	if strings.Contains(conversationPageHTML, `.conversation-list{flex-direction:row;overflow:auto;padding-bottom:4px}`) {
		t.Fatal("compact conversation history remains a permanent horizontal strip")
	}
	for _, name := range []string{"renderDraftThread", "renderThread", "renderMain"} {
		if got := conversationPageFunction(t, name); !strings.Contains(got, "conversationNavigationButton()") {
			t.Errorf("%s does not expose the compact conversation navigator", name)
		}
	}
}

func TestCompactConversationNavigatorUsesReachableTouchTargets(t *testing.T) {
	for _, want := range []string{
		`.sidebar.sheet .nav button{min-height:44px}`,
		`.sidebar.sheet .section-title button{min-width:44px;min-height:44px}`,
		`.sidebar.sheet .project-row,.sidebar.sheet .conversation-row{min-height:44px}`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("reachable compact navigator touch targets missing %q", want)
		}
	}
}

func TestCompactConversationNavigationMovesAndReturnsFocus(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var open=false,expanded="false",role="",ariaModal="",focused=[],prevented=0;
		var button={
			setAttribute:function(name,value){if(name==="aria-expanded")expanded=value},
			focus:function(){focused.push("button");document.activeElement=button}
		};
		var main={inert:false};
		var today={inert:false};
		var close={focus:function(){focused.push("close");document.activeElement=close}};
		var sheet={
			classList:{contains:function(){return open},toggle:function(_,next){open=next}},
			querySelector:function(selector){return selector==="#conversation-navigation-close"?close:null},
			contains:function(control){return control===close},
			setAttribute:function(name,value){if(name==="role")role=value;if(name==="aria-modal")ariaModal=value},
			removeAttribute:function(name){if(name==="role")role="";if(name==="aria-modal")ariaModal=""}
		};
		var document={
			activeElement:button,
			getElementById:function(id){if(id==="conversation-navigation")return sheet;if(id==="conversation-navigation-button")return button;if(id==="main")return main;if(id==="today-rail")return today;return null}
		};
		function setTimeout(callback){callback()}
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"toggleConversationNavigation", "conversationNavigationKey"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		toggleConversationNavigation();
		var during={ariaModal:ariaModal,mainInert:main.inert,todayInert:today.inert};
		conversationNavigationKey({key:"Escape",preventDefault:function(){prevented++}});
		JSON.stringify({during:during,open:open,expanded:expanded,role:role,ariaModal:ariaModal,mainInert:main.inert,todayInert:today.inert,focused:focused,prevented:prevented})
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"during":{"ariaModal":"true","mainInert":true,"todayInert":true},"open":false,"expanded":"false","role":"","ariaModal":"","mainInert":false,"todayInert":false,"focused":["close","button"],"prevented":1}` {
		t.Fatalf("compact conversation navigator state = %s", got)
	}
}

func TestCompactConversationNavigationTrapsTabFocus(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var focused=[],prevented=0;
		var first={focus:function(){focused.push("first");document.activeElement=first}};
		var last={focus:function(){focused.push("last");document.activeElement=last}};
		var sheet={
			classList:{contains:function(){return true}},
			querySelectorAll:function(){return [first,last]}
		};
		var document={activeElement:last,getElementById:function(){return sheet}};
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "conversationNavigationKey")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`
		conversationNavigationKey({key:"Tab",shiftKey:false,preventDefault:function(){prevented++}});
		document.activeElement=first;
		conversationNavigationKey({key:"Tab",shiftKey:true,preventDefault:function(){prevented++}});
		JSON.stringify({focused:focused,prevented:prevented})
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"focused":["first","last"],"prevented":2}` {
		t.Fatalf("compact conversation navigator tab containment = %s", got)
	}
}

func TestConversationNavigationReconcilesResponsiveBreakpoint(t *testing.T) {
	for _, want := range []string{
		`function syncConversationNavigationViewport(compact)`,
		`window.matchMedia('(max-width: 860px)')`,
		`addEventListener('change',function(event){syncConversationNavigationViewport(event.matches)})`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("conversation navigation responsive state missing %q", want)
		}
	}

	vm := goja.New()
	if _, err := vm.RunString(`
		var open=true,role="dialog",expanded="true",focused=[];
		var close={};
		var nav={focus:function(){focused.push("nav");document.activeElement=nav}};
		var button={setAttribute:function(name,value){if(name==="aria-expanded")expanded=value},focus:function(){focused.push("button");document.activeElement=button}};
		var sheet={
			classList:{contains:function(){return open},remove:function(){open=false}},
			contains:function(control){return control===close||control===nav},
			querySelector:function(){return nav},
			removeAttribute:function(name){if(name==="role")role=""}
		};
		var document={activeElement:close,getElementById:function(id){return id==="conversation-navigation"?sheet:button}};
		function setTimeout(callback){callback()}
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "syncConversationNavigationViewport")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`
		syncConversationNavigationViewport(false);
		open=false;role="";expanded="false";document.activeElement=nav;
		syncConversationNavigationViewport(true);
		JSON.stringify({open:open,role:role,expanded:expanded,focused:focused})
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"open":false,"role":"","expanded":"false","focused":["nav","button"]}` {
		t.Fatalf("responsive conversation navigation state = %s", got)
	}
}

func TestConversationNavigationRefreshPreservesFocusedDestination(t *testing.T) {
	for _, want := range []string{
		`function conversationNavigationFocusKey()`,
		`function restoreConversationNavigationFocus(key)`,
		`data-navigation-focus="scope:all"`,
		`data-navigation-focus="scope:inbox"`,
		`data-navigation-focus="conversation:`,
		`data-navigation-focus="project:`,
		`var focus=conversationNavigationFocusKey()`,
		`restoreConversationNavigationFocus(focus)`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("conversation navigation focus preservation missing %q", want)
		}
	}

	vm := goja.New()
	if _, err := vm.RunString(`
		var focused=[],controls=[];
		var oldControl={getAttribute:function(name){return name==="data-navigation-focus"?"project:alpha":""}};
		var newControl={getAttribute:function(name){return name==="data-navigation-focus"?"project:alpha":""},focus:function(){focused.push("project");document.activeElement=newControl}};
		var close={focus:function(){focused.push("close");document.activeElement=close}};
		var nav={focus:function(){focused.push("nav");document.activeElement=nav}};
		var sheet={
			classList:{contains:function(){return true}},
			contains:function(control){return control===oldControl||control===newControl||control===close||control===nav},
			querySelectorAll:function(){return controls},
			querySelector:function(selector){return selector==="#conversation-navigation-close"?close:nav}
		};
		var document={activeElement:oldControl,getElementById:function(){return sheet}};
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"conversationNavigationFocusKey", "restoreConversationNavigationFocus"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		var key=conversationNavigationFocusKey();
		document.activeElement=null;controls=[newControl];restoreConversationNavigationFocus(key);
		document.activeElement=null;controls=[];restoreConversationNavigationFocus(key);
		JSON.stringify({key:key,focused:focused})
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"key":"project:alpha","focused":["project","close"]}` {
		t.Fatalf("conversation navigation focus restoration = %s", got)
	}
}

func TestConversationRowsKeepDistinctFocusKeysAcrossLocations(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var state={current:""};
		function esc(value){return String(value)}
		function projectName(){return "Project"}
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "conversationRow")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`
		var conversation={id:"conversation-1",title:"Focused",project_id:"project-1"};
		JSON.stringify({
			folder:conversationRow(conversation,"folder:project-1"),
			list:conversationRow(conversation,"list")
		})
	`)
	if err != nil {
		t.Fatal(err)
	}
	got := value.String()
	for _, want := range []string{
		`data-navigation-focus=\"conversation:folder:project-1:conversation-1\"`,
		`data-navigation-focus=\"conversation:list:conversation-1\"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("conversation row location does not preserve its own focus key: missing %q in %s", want, got)
		}
	}
}

func TestExpandedProjectConversationRendersOnlyOnceInSidebar(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var elements={
			"navigation-status":{innerHTML:""},
			"projects":{innerHTML:""},
			"conversation-list":{innerHTML:""}
		};
		var document={getElementById:function(id){return elements[id]}};
		var state={
			current:"",
			project:"all",
			projects:[{id:"project-1",name:"Project one"}],
			conversations:[
				{id:"project-conversation",title:"Project conversation",project_id:"project-1"},
				{id:"inbox-conversation",title:"Inbox conversation",project_id:""}
			],
			expandedProjects:new Set(["project-1"])
		};
		function navigationStatusHTML(){return ""}
		function conversationNavigationFocusKey(){return ""}
		function restoreConversationNavigationFocus(){}
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"esc", "readSurface", "projectName", "conversationRow", "renderSidebar"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		function rowCount(value){return (value.match(/class="conversation-row/g)||[]).length}
		renderSidebar();
		var expanded={projects:rowCount(elements.projects.innerHTML),list:rowCount(elements["conversation-list"].innerHTML)};
		state.expandedProjects.clear();
		renderSidebar();
		var collapsed={projects:rowCount(elements.projects.innerHTML),list:rowCount(elements["conversation-list"].innerHTML)};
		state.conversations=[state.conversations[0]];
		state.expandedProjects.add("project-1");
		renderSidebar();
		var residualListEmpty=elements["conversation-list"].innerHTML==="";
		state.project="inbox";
		renderSidebar();
		var emptyInboxVisible=elements["conversation-list"].innerHTML.indexOf("No conversations here yet.")>=0;
		state.project="all";
		state.conversations=[];
		renderSidebar();
		var emptyAllVisible=elements["conversation-list"].innerHTML.indexOf("No conversations here yet.")>=0;
		JSON.stringify({expanded:expanded,collapsed:collapsed,residualListEmpty:residualListEmpty,emptyInboxVisible:emptyInboxVisible,emptyAllVisible:emptyAllVisible})
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"expanded":{"projects":1,"list":1},"collapsed":{"projects":0,"list":2},"residualListEmpty":true,"emptyInboxVisible":true,"emptyAllVisible":true}` {
		t.Fatalf("sidebar conversation rows = %s; each conversation must have one visible control", got)
	}
}

func TestCompactConversationNavigationClosesOnDestination(t *testing.T) {
	for _, name := range []string{"newProject", "startConversationDraft", "openConversation", "showView"} {
		if got := conversationPageFunction(t, name); !strings.Contains(got, "toggleConversationNavigation(false)") {
			t.Errorf("%s can leave compact conversation navigation open", name)
		}
	}
}

func TestCompactProjectFilterKeepsConversationHistoryReachable(t *testing.T) {
	if got := conversationPageFunction(t, "filterProject"); strings.Contains(got, "toggleConversationNavigation(false)") {
		t.Fatalf("project scope closes compact navigation before its filtered history can be used: %s", got)
	}
}

func TestCompactLoadErrorKeepsConversationNavigationReachable(t *testing.T) {
	if got := conversationPageFunction(t, "showError"); !strings.Contains(got, "conversationNavigationButton()") {
		t.Fatalf("compact load failure strands navigation off-screen: %s", got)
	}
}

func TestProjectDialogReturnsFocusFromCompactNavigation(t *testing.T) {
	if !strings.Contains(conversationPageHTML, `<dialog id="project-dialog" aria-labelledby="project-dialog-title" onclose="restoreProjectDialogFocus()">`) {
		t.Fatal("project dialog does not restore compact navigation focus on close")
	}
	for _, name := range []string{"newProject", "renameProject"} {
		if got := conversationPageFunction(t, name); !strings.Contains(got, "state.projectDialogReturnFocus=document.getElementById('conversation-navigation').classList.contains('sheet')") {
			t.Errorf("%s does not remember its compact navigation origin: %s", name, got)
		}
	}

	vm := goja.New()
	if _, err := vm.RunString(`
		var focused=0;
		var state={projectDialogReturnFocus:true};
		var button={focus:function(){focused++}};
		var document={getElementById:function(){return button}};
		function setTimeout(callback){callback()}
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "restoreProjectDialogFocus")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`restoreProjectDialogFocus();JSON.stringify({returnFocus:state.projectDialogReturnFocus,focused:focused})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"returnFocus":false,"focused":1}` {
		t.Fatalf("project dialog focus return = %s", got)
	}
}

func TestCompactConversationNavigationHasNoNestedProjectSheet(t *testing.T) {
	for _, obsolete := range []string{
		`id="project-sheet-button"`,
		`project-sheet-close`,
		`compact-project-button`,
		`function toggleProjectSheet(force)`,
	} {
		if strings.Contains(conversationPageHTML, obsolete) {
			t.Errorf("compact conversation navigation retains obsolete nested project sheet %q", obsolete)
		}
	}
	for _, name := range []string{"filterProject", "newProject", "openConversation"} {
		if source := conversationPageFunction(t, name); strings.Contains(source, "toggleProjectSheet") {
			t.Errorf("%s still coordinates with the obsolete nested project sheet: %s", name, source)
		}
	}
}

func TestCompactTodayUsesContentSizedAccessibleSheet(t *testing.T) {
	for _, want := range []string{
		`id="today-rail" class="today-rail" aria-labelledby="today-title" onkeydown="todaySheetKey(event)"`,
		`id="today-sheet-button"`,
		`aria-controls="today-rail"`,
		`aria-expanded="'+String(open)+'"`,
		`id="today-sheet-close"`,
		`.today-rail.sheet{display:block;position:fixed;z-index:20;right:12px;top:12px;bottom:auto;max-height:calc(100dvh - 24px);overflow:auto`,
		`function todaySheetButton()`,
		`function todaySheetKey(event)`,
		`button.setAttribute('aria-expanded',String(open))`,
		`rail.setAttribute('role','dialog')`,
		`rail.removeAttribute('role')`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("compact Today sheet missing %q", want)
		}
	}
}

func TestTodayRefreshPreservesSheetCloseControl(t *testing.T) {
	for _, want := range []string{
		`id="today-date"`,
		`id="today-sheet-close"`,
		`document.getElementById('today-date').textContent=`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("Today refresh contract missing %q", want)
		}
	}
	if got := conversationPageFunction(t, "renderToday"); strings.Contains(got, "document.getElementById('today-title').innerHTML=") {
		t.Error("renderToday replaces the focused sheet close control")
	}
}

func TestTodayRefreshRestoresFocusedRow(t *testing.T) {
	for _, want := range []string{
		`function todayFocusKey()`,
		`function restoreTodayFocus(key)`,
		`data-today-focus="progress:`,
		`data-today-focus="scheduled:`,
		`var focus=todayFocusKey()`,
		`restoreTodayFocus(focus)`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("Today row focus preservation missing %q", want)
		}
	}

	vm := goja.New()
	if _, err := vm.RunString(`
		var focused=[],rowControls=[];
		var oldRow={id:"",getAttribute:function(name){return name==="data-today-focus"?"progress:target-1":""}};
		var newRow={getAttribute:function(name){return name==="data-today-focus"?"progress:target-1":""},focus:function(){focused.push("row");document.activeElement=newRow}};
		var close={id:"today-sheet-close",getAttribute:function(){return ""},focus:function(){focused.push("close");document.activeElement=close}};
		var rail={
			classList:{contains:function(){return true}},
			contains:function(control){return control===oldRow||control===newRow||control===close},
			querySelector:function(){return close},
			querySelectorAll:function(){return rowControls}
		};
		var document={activeElement:oldRow,getElementById:function(){return rail}};
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"todayFocusKey", "restoreTodayFocus"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		var key=todayFocusKey();
		document.activeElement=null;
		rowControls=[newRow];
		restoreTodayFocus(key);
		document.activeElement=null;
		rowControls=[];
		restoreTodayFocus(key);
		JSON.stringify({key:key,focused:focused})
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"key":"progress:target-1","focused":["row","close"]}` {
		t.Fatalf("Today row focus restoration = %s", got)
	}
}

func TestLeavingConversationViewClosesTodaySheet(t *testing.T) {
	if got := conversationPageFunction(t, "showView"); !strings.Contains(got, "toggleTodaySheet(false)") {
		t.Fatalf("showView can strand the Today sheet without its trigger: %s", got)
	}
}

func TestCompactTodaySheetMovesAndReturnsFocus(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var open=false,expanded="false",role="",focused=[],prevented=0;
		var button={
			setAttribute:function(name,value){if(name==="aria-expanded")expanded=value},
			focus:function(){focused.push("button");document.activeElement=button}
		};
		var close={focus:function(){focused.push("close");document.activeElement=close}};
		var rail={
			classList:{contains:function(){return open},toggle:function(_,next){open=next}},
			querySelector:function(selector){return selector==="#today-sheet-close"?close:null},
			contains:function(control){return control===close},
			setAttribute:function(name,value){if(name==="role")role=value},
			removeAttribute:function(name){if(name==="role")role=""}
		};
		var document={
			activeElement:button,
			getElementById:function(id){return id==="today-rail"?rail:button}
		};
		function setTimeout(callback){callback()}
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"toggleTodaySheet", "todaySheetKey"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		toggleTodaySheet();
		todaySheetKey({key:"Escape",preventDefault:function(){prevented++}});
		JSON.stringify({open:open,expanded:expanded,role:role,focused:focused,prevented:prevented})
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"open":false,"expanded":"false","role":"","focused":["close","button"],"prevented":1}` {
		t.Fatalf("compact Today sheet state = %s", got)
	}
}

func TestParticipantTargetLabelKeepsIdentityReadable(t *testing.T) {
	vm := goja.New()
	loadConversationIdentityFunctions(t, vm)
	if _, err := vm.RunString(`var state={machines:[{name:"tobiass.macbook.pro.lan"},{name:"taloss.mac.mini.lan"},{name:"forge.local"}],seats:[],today:{in_progress:[]},detail:null}`); err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		participant string
		want        string
	}{
		"historical configured default on MacBook Pro": {
			participant: `{seat_id:"legacy-seat",profile:"codex:configured-default",agent:"codex",display_name:"Codex · configured default on tobiass.macbook.pro.lan",machine:"tobiass.macbook.pro.lan"}`,
			want:        "Codex · Model not recorded — MacBook Pro",
		},
		"named model on Mac mini": {
			participant: `{display_name:"Hermes · Codex GPT-5.6 Sol on taloss.mac.mini.lan",machine:"taloss.mac.mini.lan"}`,
			want:        "Hermes · Codex GPT-5.6 Sol — Mac mini",
		},
		"generic computer remains recognizable": {
			participant: `{display_name:"OpenClaw · main on forge.local",machine:"forge.local"}`,
			want:        "OpenClaw · main — Forge",
		},
	} {
		t.Run(name, func(t *testing.T) {
			value, err := vm.RunString(`participantTargetLabel(` + test.participant + `)`)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.String(); got != test.want {
				t.Fatalf("label = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMachineDisplayNameIsConciseAndCollisionSafe(t *testing.T) {
	vm := goja.New()
	loadConversationIdentityFunctions(t, vm)
	value, err := vm.RunString(`JSON.stringify([
		machineDisplayName("tobiass.macbook.pro.lan",["tobiass.macbook.pro.lan"]),
		machineDisplayName("tobiass.macbook.pro.lan",["tobiass.macbook.pro.lan","taloss.macbook.pro.lan"]),
		machineDisplayName("taloss.macbook.pro.lan",["tobiass.macbook.pro.lan","taloss.macbook.pro.lan"]),
		machineDisplayName("forge.local",["forge.local","forge.lan"]),
		machineDisplayName("forge.lan",["forge.local","forge.lan"]),
		machineDisplayName("10.0.0.9",["10.0.0.9"]),
		machineDisplayName("",[])
	])`)
	if err != nil {
		t.Fatal(err)
	}
	want := `["MacBook Pro","MacBook Pro · tobiass.macbook.pro.lan","MacBook Pro · taloss.macbook.pro.lan","Forge · forge.local","Forge · forge.lan","10.0.0.9","Local computer"]`
	if got := value.String(); got != want {
		t.Fatalf("machine labels = %s, want %s", got, want)
	}
}

func TestSeatDisplayNameUsesCuratedIdentityWithTruthfulFallbacks(t *testing.T) {
	vm := goja.New()
	loadConversationIdentityFunctions(t, vm)
	if _, err := vm.RunString(`var state={
		machines:[{name:"tobiass.macbook.pro.lan"},{name:"taloss.macbook.pro.lan"},{name:"forge.local"}],
		seats:[
			{profile:"codex:configured-default",agent:"codex",model:"gpt-5.6-sol",machine:"tobiass.macbook.pro.lan",display_name:"Codex · GPT-5.6 Sol (default) on tobiass.macbook.pro.lan"},
			{profile:"codex:gpt-5.6-sol",agent:"codex",model:"gpt-5.6-sol",machine:"tobiass.macbook.pro.lan",display_name:"Codex · GPT-5.6 Sol on tobiass.macbook.pro.lan"}
		],
		today:{in_progress:[]},detail:null
	}`); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`JSON.stringify([
		participantTargetLabel({seat_id:"legacy-seat",profile:"codex:configured-default",agent:"codex",machine:"tobiass.macbook.pro.lan"}),
		participantTargetLabel({id:"current-unresolved",profile:"codex:configured-default",agent:"codex",state:"setup_required",reason:"model_unavailable",machine:"tobiass.macbook.pro.lan",display_name:"Codex · configured default on tobiass.macbook.pro.lan"}),
		participantTargetLabel({profile:"codex:configured-default",agent:"codex",model:"gpt-5.6-sol",machine:"tobiass.macbook.pro.lan",display_name:"Codex · GPT-5.6 Sol (default) on tobiass.macbook.pro.lan"}),
		participantTargetLabel({profile:"codex:gpt-5.6-sol",agent:"codex",model:"gpt-5.6-sol",machine:"tobiass.macbook.pro.lan"}),
		participantTargetLabel({profile:"codex-sol",agent:"codex",model:"gpt-5.5",machine:"tobiass.macbook.pro.lan"}),
		participantTargetLabel({profile:"claude:opus",agent:"claude",model:"claude-opus-4",machine:"forge.local"}),
		participantTargetLabel({agent:"hermes",model:"local-model",machine:"forge.local"}),
		participantTargetLabel({agent:"codex",machine:"tobiass.macbook.pro.lan"}),
		participantInitials({display_name:"Codex · configured default on tobiass.macbook.pro.lan",agent:"codex",machine:"tobiass.macbook.pro.lan"})
	])`)
	if err != nil {
		t.Fatal(err)
	}
	want := `["Codex · Model not recorded — MacBook Pro · tobiass.macbook.pro.lan","Codex · Configured default — MacBook Pro · tobiass.macbook.pro.lan","Codex · GPT-5.6 Sol (default) — MacBook Pro · tobiass.macbook.pro.lan","Codex · GPT-5.6 Sol — MacBook Pro · tobiass.macbook.pro.lan","codex-sol · gpt-5.5 — MacBook Pro · tobiass.macbook.pro.lan","claude:opus · claude-opus-4 — Forge","hermes · local-model — Forge","codex — MacBook Pro · tobiass.macbook.pro.lan","C"]`
	if got := value.String(); got != want {
		t.Fatalf("seat labels = %s, want %s", got, want)
	}
}

func TestUnreachableSeatReasonNamesTheOfflineComputer(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(conversationPageFunction(t, "seatSetupReason")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`seatSetupReason("unreachable")`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != "Computer offline" {
		t.Fatalf("unreachable seat reason = %q, want Computer offline", got)
	}
}

func TestWebIdentitySurfacesShareOnePresentationContract(t *testing.T) {
	checks := map[string][]string{
		"groupedSeatChoices": {`class="section-title machine-section-title"`, "machineDisplayName(machine,knownMachineNames())"},
		"draftSeatChoices":   {"profileDisplayName(s)", `aria-label="'+esc(participantTargetLabel(s))+' · Ready"`, `aria-label="'+esc(participantTargetLabel(s))+' · '+esc(seatSetupReason(s.reason||s.state))+'"`},
		"renderDraftThread":  {"participantInitials(s)", `aria-label="'+esc(participantTargetLabel(s))+'"`, `title="'+esc(participantTargetLabel(s))+'"`},
		"renderThread":       {"participantInitials(p)", `aria-label="'+esc(participantTargetLabel(p))+'"`, `title="'+esc(participantTargetLabel(p))+'"`},
		"renderToday":        {"var identity=participantTargetLabel(item)"},
		"openTodayDetail":    {"profileDisplayName(identityItem)", "machineDisplayName(identityItem.machine,knownMachineNames([identityItem.machine]))"},
		"renderComputers":    {"var names=knownMachineNames(Object.keys(byName))", "machineDisplayName(name,names)", "profileDisplayName(s)"},
	}
	for name, wants := range checks {
		source := conversationPageFunction(t, name)
		for _, want := range wants {
			if !strings.Contains(source, want) {
				t.Errorf("%s does not use shared presentation identity %q: %s", name, want, source)
			}
		}
	}
	if source := conversationPageFunction(t, "renderToday"); strings.Contains(source, "item.machine||'local'") || strings.Contains(source, "[item.agent,item.model]") {
		t.Fatalf("Today still assembles a competing raw identity: %s", source)
	}
	if source := conversationPageFunction(t, "openTodayDetail"); strings.Contains(source, "[identityItem.profile||identityItem.agent,identityItem.model]") {
		t.Fatalf("run detail still duplicates profile and model: %s", source)
	}
	if !strings.Contains(conversationPageHTML, `.machine-section-title span{min-width:0;overflow-wrap:anywhere;text-transform:none;letter-spacing:0}`) {
		t.Fatal("machine group headings do not preserve readable canonical-name casing")
	}
}

func TestMachinePresentationSignatureInvalidatesNewCollisions(t *testing.T) {
	vm := goja.New()
	loadConversationIdentityFunctions(t, vm)
	if _, err := vm.RunString(`var state={machines:[{name:"tobiass.macbook.pro.lan"}],seats:[],today:{in_progress:[]},detail:null}`); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`var before=machinePresentationSignature();state.machines.push({name:"taloss.macbook.pro.lan"});JSON.stringify({changed:before!==machinePresentationSignature(),label:machineDisplayName("tobiass.macbook.pro.lan")})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"changed":true,"label":"MacBook Pro · tobiass.macbook.pro.lan"}` {
		t.Fatalf("collision invalidation = %s", got)
	}
	for name, wants := range map[string][]string{
		"refreshInventory": {"var identityBefore=machinePresentationSignature()", "identityBefore!==machinePresentationSignature()", "renderInventorySurfaces(identityChanged)"},
		"refreshToday":     {"var identityBefore=machinePresentationSignature()", "identityBefore!==machinePresentationSignature()", "renderThread()"},
	} {
		source := conversationPageFunction(t, name)
		for _, want := range wants {
			if !strings.Contains(source, want) {
				t.Errorf("%s does not invalidate stale identity presentation %q: %s", name, want, source)
			}
		}
	}
	if source := conversationPageFunction(t, "renderInventorySurfaces"); !strings.Contains(source, "renderThread()") {
		t.Fatalf("inventory presentation invalidation no longer reaches the thread: %s", source)
	}
}

func TestCompletedSeatRecheckCannotBeOverwrittenByOlderInventoryPoll(t *testing.T) {
	if !strings.Contains(conversationPageHTML, "seatInventoryVersion:0") {
		t.Fatal("conversation state has no monotonic seat-inventory version")
	}
	recheck := conversationPageFunction(t, "requestSeatRecheck")
	for _, want := range []string{
		"var seats=await api('/api/conversation-seats/recheck'",
		"state.seatInventoryVersion++",
		"state.seats=seats",
	} {
		if !strings.Contains(recheck, want) {
			t.Errorf("seat recheck lacks freshness guard %q: %s", want, recheck)
		}
	}
	poll := conversationPageFunction(t, "refreshInventory")
	for _, want := range []string{
		"if(state.seatRecheck)return",
		"var seatVersion=state.seatInventoryVersion",
		"if(seatVersion===state.seatInventoryVersion)seatsAccepted=applyReadResult('seats'",
		"function(value){state.seats=value}",
	} {
		if !strings.Contains(poll, want) {
			t.Errorf("seat inventory poll can overwrite a newer recheck; missing %q: %s", want, poll)
		}
	}
}

func TestDraftSeatRefreshDropsSelectionsThatAreNoLongerReady(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var state={
			current:'new',
			seats:[
				{id:'seat-a',state:'setup_required'},
				{id:'seat-b',state:'ready'}
			],
			draft:{
				participantSeatIDs:new Set(['seat-a','seat-b']),
				targetSeatIDs:new Set(['seat-a','seat-b']),
				everyoneTarget:true,
				pickerOpen:false,
				error:''
			}
		};
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "reconcileDraftSeatSelection")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`reconcileDraftSeatSelection();JSON.stringify({
		participants:Array.from(state.draft.participantSeatIDs),
		targets:Array.from(state.draft.targetSeatIDs),
		everyone:state.draft.everyoneTarget,
		pickerOpen:state.draft.pickerOpen,
		error:state.draft.error
	})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"participants":["seat-b"],"targets":["seat-b"],"everyone":true,"pickerOpen":false,"error":"An agent selection is no longer ready. Choose a ready agent before sending."}` {
		t.Fatalf("reconciled draft = %s", got)
	}

	value, err = vm.RunString(`state.seats=[];reconcileDraftSeatSelection();JSON.stringify({
		participants:Array.from(state.draft.participantSeatIDs),
		targets:Array.from(state.draft.targetSeatIDs),
		everyone:state.draft.everyoneTarget,
		pickerOpen:state.draft.pickerOpen
	})`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"participants":[],"targets":[],"everyone":false,"pickerOpen":true}` {
		t.Fatalf("empty refreshed draft = %s", got)
	}

	for _, name := range []string{"requestSeatRecheck", "refreshInventory"} {
		if source := conversationPageFunction(t, name); !strings.Contains(source, "reconcileDraftSeatSelection()") {
			t.Errorf("%s does not reconcile stale draft selections: %s", name, source)
		}
	}
}

func TestDraftWithNoInventoryStillOffersExplicitRecheck(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var state={seats:[],seatRecheck:false,draft:{participantSeatIDs:new Set()}};
		function groupedSeatChoices(){return ''}
		function esc(value){return String(value)}
		function participantTargetLabel(){return ''}
		function profileDisplayName(){return ''}
		function seatSetupReason(){return 'Unavailable'}
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"readSurface", "retryNotice", "inventoryStatusHTML"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := vm.RunString(conversationPageFunction(t, "draftSeatChoices")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`draftSeatChoices()`)
	if err != nil {
		t.Fatal(err)
	}
	html := value.String()
	for _, want := range []string{"No ready agent seats yet.", `onclick="recheckSeats()">Recheck</button>`} {
		if !strings.Contains(html, want) {
			t.Errorf("empty seat inventory lacks %q: %s", want, html)
		}
	}
}

func TestConversationMessagesUseOneConciseCompleteSeatIdentity(t *testing.T) {
	for _, want := range []string{
		"var name=human?'You':(p?participantTargetLabel(p):'Agent')",
		`var title=human||!p?'':(' title="'+esc(participantTargetLabel(p))+'"')`,
		"esc(p?participantTargetLabel(p):'Agent')+' · '+esc(stateLabel)",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("conversation message identity missing %q", want)
		}
	}
	if strings.Contains(conversationPageHTML, "p.profile+(p.model?' · '+p.model:'')+' · '+p.machine") {
		t.Fatal("conversation answer still duplicates a raw profile and machine identity")
	}
}

func TestConversationPageJavaScriptParses(t *testing.T) {
	start := strings.LastIndex(conversationPageHTML, "<script>")
	end := strings.LastIndex(conversationPageHTML, "</script>")
	if start < 0 || end <= start {
		t.Fatal("conversation page script missing")
	}
	if _, err := goja.Compile("conversation.js", conversationPageHTML[start+len("<script>"):end], false); err != nil {
		t.Fatalf("conversation page JavaScript does not parse: %v", err)
	}
}

func conversationPageFunction(t *testing.T, name string) string {
	t.Helper()
	start := strings.Index(conversationPageHTML, "function "+name+"(")
	if start < 0 {
		t.Fatalf("conversation page function %q missing", name)
	}
	end := strings.IndexByte(conversationPageHTML[start:], '\n')
	if end < 0 {
		t.Fatalf("conversation page function %q is not line-delimited", name)
	}
	return conversationPageHTML[start : start+end]
}

func loadConversationIdentityFunctions(t *testing.T, vm *goja.Runtime) {
	t.Helper()
	for _, name := range []string{"initials", "machineFriendlyName", "knownMachineNames", "machineDisplayName", "machinePresentationSignature", "profileDisplayName", "participantTargetLabel", "participantInitials"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConversationPagePreservesComposerAcrossLiveUpdates(t *testing.T) {
	for _, want := range []string{
		"var draft=oldComposer?oldComposer.value:''",
		"Date.now()<state.composerFocusUntil",
		"state.composerFocusUntil=Date.now()+5000",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("conversation page does not preserve composer state: missing %q", want)
		}
	}
}

func TestConversationPageUsesAnAccessibleProjectDialog(t *testing.T) {
	for _, want := range []string{
		`<dialog id="project-dialog" aria-labelledby="project-dialog-title"`,
		`onclose="restoreProjectDialogFocus()"`,
		`<h2 id="project-dialog-title">`,
		`<input id="project-name"`,
		`function saveProject(event)`,
		`document.getElementById('project-name').focus()`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("conversation page project dialog missing %q", want)
		}
	}
	if strings.Contains(conversationPageHTML, "prompt('Project name')") || strings.Contains(conversationPageHTML, "prompt('Rename project'") {
		t.Fatal("project actions still depend on browser prompts")
	}
}

func TestProjectRenameDoesNotInterpolateNamesIntoInlineJavaScript(t *testing.T) {
	source := conversationPageFunction(t, "renderSidebar")
	for _, want := range []string{
		`data-project-id="'+esc(p.id)+'"`,
		`data-project-name="'+esc(p.name)+'"`,
		`onclick="renameProject(this.dataset.projectId,this.dataset.projectName)"`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("project rename boundary missing %q: %s", want, source)
		}
	}
	if strings.Contains(source, `renameProject(\''+esc(p.id)+'\',\''+esc(p.name)+'\')`) {
		t.Fatalf("project name is interpolated into inline JavaScript: %s", source)
	}
}

func TestProjectRenameIsExplicitKeyboardAndTouchAction(t *testing.T) {
	source := conversationPageFunction(t, "renderSidebar")
	for _, want := range []string{
		`type="button" class="folder-rename"`,
		`aria-label="Rename `,
		`>Rename</button>`,
		`.folder-rename{`,
		`min-height:44px`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("explicit project rename control missing %q", want)
		}
	}
	if strings.Contains(source, "ondblclick=") {
		t.Fatalf("project rename remains double-click-only: %s", source)
	}
}

func TestConversationPageProjectsAreCollapsibleFolders(t *testing.T) {
	for _, want := range []string{
		"expandedProjects:new Set()",
		"folder-conversations",
		"aria-expanded=",
		"function toggleProjectFolder(id)",
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("conversation page project folders missing %q", want)
		}
	}
}

func TestProjectFolderToggleDoesNotChangeSelectedScopeOrDraftDestination(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var renders=0;
		function renderSidebar(){renders++}
		var state={
			projects:[{id:"project-1"},{id:"project-2"}],
			project:"project-1",
			expandedProjects:new Set(["project-1"])
		};
	`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"draftProjectID", "toggleProjectFolder"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		var before=draftProjectID();
		toggleProjectFolder("project-1");
		var afterCollapse={scope:state.project,draftProject:draftProjectID()};
		toggleProjectFolder("project-2");
		JSON.stringify({before:before,afterCollapse:afterCollapse,afterExpand:{scope:state.project,draftProject:draftProjectID()},renders:renders})
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"before":"project-1","afterCollapse":{"scope":"project-1","draftProject":"project-1"},"afterExpand":{"scope":"project-1","draftProject":"project-1"},"renders":2}` {
		t.Fatalf("folder toggle changed selected scope or draft destination: %s", got)
	}
}

func TestTranscriptRerenderPreservesReadingPositionUnlessFollowingOrExplicit(t *testing.T) {
	vm := goja.New()
	for _, name := range []string{"captureTranscriptScroll", "restoreTranscriptScroll"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := vm.RunString(`
		var reading={scrollHeight:1000,scrollTop:300,clientHeight:400};
		var readingState=captureTranscriptScroll(reading,false);
		reading.scrollHeight=1200;
		restoreTranscriptScroll(reading,readingState);
		var following={scrollHeight:1000,scrollTop:570,clientHeight:400};
		var followingState=captureTranscriptScroll(following,false);
		following.scrollHeight=1200;
		restoreTranscriptScroll(following,followingState);
		var forced={scrollHeight:1000,scrollTop:300,clientHeight:400};
		var forcedState=captureTranscriptScroll(forced,true);
		forced.scrollHeight=1200;
		restoreTranscriptScroll(forced,forcedState);
		JSON.stringify({reading:reading.scrollTop,following:following.scrollTop,forced:forced.scrollTop})
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.String(); got != `{"reading":300,"following":1200,"forced":1200}` {
		t.Fatalf("restored transcript scroll = %s", got)
	}

	render := conversationPageFunction(t, "renderThread")
	for _, want := range []string{"captureTranscriptScroll(thread,forceBottom)", "restoreTranscriptScroll(thread,transcriptScroll)"} {
		if !strings.Contains(render, want) {
			t.Errorf("transcript rerender missing %q", want)
		}
	}
	if source := conversationPageFunction(t, "openConversation"); !strings.Contains(source, "renderMain(true)") {
		t.Errorf("opening a conversation does not explicitly follow its newest message: %s", source)
	}
	if source := conversationPageFunction(t, "connectEvents"); strings.Contains(source, "renderThread(true)") {
		t.Errorf("a passive stream update forcibly follows the transcript bottom: %s", source)
	}
	for _, name := range []string{"sendTurn", "retryPendingTurn"} {
		if source := conversationPageFunction(t, name); !strings.Contains(source, "renderThread(true)") {
			t.Errorf("%s does not explicitly follow the sent turn: %s", name, source)
		}
	}
}

func TestCompactTodaySelectionClosesSheetBeforeNavigation(t *testing.T) {
	for _, name := range []string{"openConversation", "openTodayDetail"} {
		if source := conversationPageFunction(t, name); !strings.Contains(source, "toggleTodaySheet(false)") {
			t.Errorf("%s does not close the compact Today sheet", name)
		}
	}
}

func TestDialogHeaderCloseControlsRetainFortyFourPointHitArea(t *testing.T) {
	if !strings.Contains(conversationPageHTML, `.run-detail-head>button{min-width:44px;min-height:44px}`) {
		t.Fatal("dialog header Close controls do not retain a 44-point hit area")
	}
}

func TestInlineTargetActionsRetainFortyFourPointHitArea(t *testing.T) {
	if !strings.Contains(conversationPageHTML, `.status button{`) || !strings.Contains(conversationPageHTML, `min-width:44px;min-height:44px`) {
		t.Fatal("inline target actions do not retain a 44-point hit area")
	}
}

func TestConversationRetryKeepsOneCurrentResponseSlot(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(conversationPageFunction(t, "esc")); err != nil {
		t.Fatal(err)
	}
	loadConversationIdentityFunctions(t, vm)
	for _, name := range []string{"renderTargetStates"} {
		if _, err := vm.RunString(conversationPageFunction(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := vm.RunString(`var state={machines:[{name:"studio.local"}],seats:[],today:{in_progress:[]},detail:null}`); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`renderTargetStates([
		{id:"target-1",participant_id:"seat-1",attempt:1,state:"failed",run_id:"run-old",error_code:"offline",error:"transport lost"},
		{id:"target-2",participant_id:"seat-1",attempt:2,state:"answered",run_id:"run-new"}
	],{"seat-1":{display_name:"Codex · Sol on studio.local",machine:"studio.local"}})`)
	if err != nil {
		t.Fatal(err)
	}
	html := value.String()
	if got := strings.Count(html, `class="status `); got != 1 {
		t.Fatalf("rendered %d response slots, want one current slot: %s", got, html)
	}
	for _, want := range []string{"answered", "run-old", "transport lost", "run-new"} {
		if !strings.Contains(html, want) {
			t.Errorf("response slot lost %q: %s", want, html)
		}
	}
	if strings.Contains(html, `>Retry</button>`) {
		t.Fatalf("obsolete failed attempt remains actionable: %s", html)
	}
}

func TestTargetStateLabelsAreHumanReadable(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(conversationPageFunction(t, "esc")); err != nil {
		t.Fatal(err)
	}
	loadConversationIdentityFunctions(t, vm)
	if _, err := vm.RunString(conversationPageFunction(t, "renderTargetStates")); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(`var state={seatRecheck:false,machines:[{name:"studio.local"}],seats:[],today:{in_progress:[]},detail:null}`); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`renderTargetStates([
		{id:"queued",participant_id:"queued",attempt:1,state:"queued",run_id:"run-queued"},
		{id:"working",participant_id:"working",attempt:1,state:"working",run_id:"run-working"},
		{id:"answered",participant_id:"answered",attempt:1,state:"answered",run_id:"run-answered"},
		{id:"failed",participant_id:"failed",attempt:1,state:"failed",run_id:"run-failed",error_code:"provider_failed",error:"failed"},
		{id:"canceled",participant_id:"canceled",attempt:1,state:"canceled",run_id:"run-canceled"}
	],{
		queued:{display_name:"Codex",machine:"studio.local"},
		working:{display_name:"Codex",machine:"studio.local"},
		answered:{display_name:"Codex",machine:"studio.local"},
		failed:{display_name:"Codex",machine:"studio.local"},
		canceled:{display_name:"Codex",machine:"studio.local"}
	})`)
	if err != nil {
		t.Fatal(err)
	}
	html := value.String()
	for _, state := range []string{"Queued", "Working", "Answered", "Failed", "Canceled"} {
		if !strings.Contains(html, " · "+state) {
			t.Errorf("target state is not human-readable %q: %s", state, html)
		}
		if strings.Contains(html, " · "+strings.ToLower(state)) {
			t.Errorf("target state still exposes raw enum %q: %s", strings.ToLower(state), html)
		}
	}
}

func TestUnavailableAgentSeatsAreSemanticallyDisabled(t *testing.T) {
	if source := conversationPageFunction(t, "draftSeatChoices"); !strings.Contains(source, `<button type="button" class="setup-seat" disabled`) {
		t.Fatalf("unavailable agent seats are not disabled controls: %s", source)
	}
}

func TestConversationParticipantManagerAddsAndRemovesFutureParticipants(t *testing.T) {
	for _, want := range []string{
		`<dialog id="participants-dialog"`,
		`id="participant-manager-list"`,
		`onclick="openParticipantManager()">Agents</button>`,
		`function renderParticipantManager()`,
		`function addConversationParticipant(seatID)`,
		`function removeConversationParticipant(participantID)`,
		`'/participants',{method:'POST'`,
		`'/participants/'+encodeURIComponent(participantID),{method:'DELETE'`,
		`participantMutation:false`,
		`.participant-manager-row button{min-height:44px`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("participant manager missing %q", want)
		}
	}
	if strings.Contains(conversationPageHTML, "prompt('Choose an agent seat:") {
		t.Fatal("participant management still depends on a numbered browser prompt")
	}
}

func TestParticipantManagerDoesNotOfferRemovedHistoricalSeatAgain(t *testing.T) {
	source := conversationPageFunction(t, "renderParticipantManager")
	if !strings.Contains(source, `var used=new Set(state.detail.participants.map(function(p){return p.seat_id}))`) {
		t.Fatalf("participant manager does not reserve historical seat identities: %s", source)
	}
}

func TestParticipantMutationsCannotApplyToAnotherConversation(t *testing.T) {
	refresh := conversationPageFunction(t, "refreshConversationParticipants")
	if !strings.Contains(refresh, "function refreshConversationParticipants(conversationID)") || !strings.Contains(refresh, "if(state.current!==conversationID)return false") {
		t.Fatalf("participant refresh is not guarded by its conversation: %s", refresh)
	}
	for _, name := range []string{"addConversationParticipant", "removeConversationParticipant"} {
		source := conversationPageFunction(t, name)
		for _, want := range []string{"var conversationID=state.current", "refreshConversationParticipants(conversationID)", "state.participantMutation===mutation"} {
			if !strings.Contains(source, want) {
				t.Errorf("%s missing %q: %s", name, want, source)
			}
		}
	}
}

func TestParticipantManagerNamesDialogAndSeatActions(t *testing.T) {
	for _, want := range []string{
		`<dialog id="participants-dialog" aria-labelledby="participants-title" aria-describedby="participants-description">`,
		`id="participants-title"`,
		`id="participants-description"`,
		`aria-label="Remove '+esc(participantTargetLabel(p))+'"`,
		`aria-label="Add '+esc(participantTargetLabel(s))+'"`,
		`aria-label="Unavailable '+esc(participantTargetLabel(s))+' · '+esc(seatSetupReason(s.reason||s.state))+'"`,
		`aria-busy`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("participant manager accessibility missing %q", want)
		}
	}
}

func TestTodayRowsShowTruthfulActivityAndBoundedNavigation(t *testing.T) {
	for _, want := range []string{
		`<time datetime="'+esc(item.updated_at)+'">`,
		`item.state==='working'?'active ':'accepted '`,
		`participantTargetLabel(item)`,
		`item.run_id?'button':'div'`,
		`data-today-kind="progress"`,
		`data-today-kind="scheduled"`,
		`openTodayDetail(this.dataset.todayKind,this.dataset.todayKey)`,
		`id="run-detail-dialog"`,
		`id="run-detail-status"`,
		`id="run-detail-metadata"`,
		`id="run-detail-error"`,
		`function openTodayDetail(kind,key)`,
		`document.getElementById('run-detail-dialog').showModal()`,
	} {
		if !strings.Contains(conversationPageHTML, want) {
			t.Errorf("truthful Today detail missing %q", want)
		}
	}
	if strings.Contains(conversationPageHTML, "/api/runs/") {
		t.Fatal("the default conversation surface fetches the legacy run-detail contract")
	}
	source := conversationPageFunction(t, "openTodayDetail")
	for _, forbidden := range []string{"api(", ".nodes", ".events", ".body", "No task body"} {
		if strings.Contains(source, forbidden) {
			t.Errorf("bounded Today detail references %q: %s", forbidden, source)
		}
	}
	if strings.Contains(conversationPageHTML, `id="run-detail-body"`) {
		t.Fatal("bounded Today detail claims to expose a task body")
	}
}

func TestTodayRunDetailsRenderOnlyFromCurrentProjection(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var apiCalls=0;
		function api(){apiCalls++;throw new Error('unexpected API call')}
		var sheetCloses=[];
		function toggleTodaySheet(value){sheetCloses.push(value)}
		function esc(value){return String(value==null?'':value)}
		function when(value){return value==='2026-08-03T14:15:00Z'?'9:15 AM':'10:30 AM'}
		function profileDisplayName(item){return [item.agent,item.profile,item.model].filter(Boolean).join(' / ')}
		function knownMachineNames(){return []}
		function machineDisplayName(machine){return 'Machine '+machine}
		var shown=0;
		var elements={
			'run-detail-dialog':{showModal:function(){shown++}},
			'run-detail-title':{textContent:''},
			'run-detail-status':{innerHTML:''},
			'run-detail-identity-card':{hidden:false},
			'run-detail-identity':{textContent:''},
			'run-detail-machine':{textContent:''},
			'run-detail-metadata':{innerHTML:''},
			'run-detail-error':{textContent:'',hidden:false}
		};
		var document={getElementById:function(id){return elements[id]}};
		var state={today:{in_progress:[{
			run_id:'run-legacy',conversation_title:'Nightly audit',project_name:'Operations',
			agent:'codex',profile:'sol',model:'gpt-5.6',machine:'forge.local',
			state:'working',updated_at:'2026-08-03T14:15:00Z'
		}],scheduled:[{
			occurrence_id:'occurrence-42',schedule_id:'schedule-7',flow_id:'flow-maintenance',
			run_id:'run-scheduled',title:'Scheduled cleanup',recurrence:'weekdays',
			scheduled_for:'2026-08-03T15:30:00Z',state:'failed',error:'agent exited'
		}]}};
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "openTodayDetail")); err != nil {
		t.Fatal(err)
	}

	legacyValue, err := vm.RunString(`openTodayDetail('progress','run-legacy');JSON.stringify({
		apiCalls:apiCalls,sheetCloses:sheetCloses,shown:shown,
		title:elements['run-detail-title'].textContent,
		status:elements['run-detail-status'].innerHTML,
		identityHidden:elements['run-detail-identity-card'].hidden,
		identity:elements['run-detail-identity'].textContent,
		machine:elements['run-detail-machine'].textContent,
		metadata:elements['run-detail-metadata'].innerHTML,
		errorHidden:elements['run-detail-error'].hidden,
		error:elements['run-detail-error'].textContent
	})`)
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		APICalls       int    `json:"apiCalls"`
		SheetCloses    []bool `json:"sheetCloses"`
		Shown          int    `json:"shown"`
		Title          string `json:"title"`
		Status         string `json:"status"`
		IdentityHidden bool   `json:"identityHidden"`
		Identity       string `json:"identity"`
		Machine        string `json:"machine"`
		Metadata       string `json:"metadata"`
		ErrorHidden    bool   `json:"errorHidden"`
		Error          string `json:"error"`
	}
	if err := json.Unmarshal([]byte(legacyValue.String()), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.APICalls != 0 || legacy.Shown != 1 || len(legacy.SheetCloses) != 1 || legacy.SheetCloses[0] {
		t.Fatalf("legacy detail escaped the local Today interaction boundary: %+v", legacy)
	}
	if legacy.Title != "Nightly audit" || !strings.Contains(legacy.Status, "Working") || !strings.Contains(legacy.Status, "9:15 AM") {
		t.Errorf("legacy detail lost title or truthful activity: %+v", legacy)
	}
	if legacy.IdentityHidden || legacy.Identity != "codex / sol / gpt-5.6" || legacy.Machine != "Machine forge.local" {
		t.Errorf("legacy detail lost bounded execution identity: %+v", legacy)
	}
	for _, want := range []string{"Run ID", "run-legacy", "Project", "Operations"} {
		if !strings.Contains(legacy.Metadata, want) {
			t.Errorf("legacy metadata missing %q: %s", want, legacy.Metadata)
		}
	}
	if !legacy.ErrorHidden || legacy.Error != "" {
		t.Errorf("legacy detail invented an error: %+v", legacy)
	}

	scheduledValue, err := vm.RunString(`openTodayDetail('scheduled','occurrence-42');JSON.stringify({
		apiCalls:apiCalls,sheetCloses:sheetCloses,shown:shown,
		title:elements['run-detail-title'].textContent,
		status:elements['run-detail-status'].innerHTML,
		identityHidden:elements['run-detail-identity-card'].hidden,
		metadata:elements['run-detail-metadata'].innerHTML,
		errorHidden:elements['run-detail-error'].hidden,
		error:elements['run-detail-error'].textContent
	})`)
	if err != nil {
		t.Fatal(err)
	}
	var scheduled struct {
		APICalls       int    `json:"apiCalls"`
		SheetCloses    []bool `json:"sheetCloses"`
		Shown          int    `json:"shown"`
		Title          string `json:"title"`
		Status         string `json:"status"`
		IdentityHidden bool   `json:"identityHidden"`
		Metadata       string `json:"metadata"`
		ErrorHidden    bool   `json:"errorHidden"`
		Error          string `json:"error"`
	}
	if err := json.Unmarshal([]byte(scheduledValue.String()), &scheduled); err != nil {
		t.Fatal(err)
	}
	if scheduled.APICalls != 0 || scheduled.Shown != 2 || len(scheduled.SheetCloses) != 2 || scheduled.SheetCloses[1] {
		t.Fatalf("scheduled detail escaped the local Today interaction boundary: %+v", scheduled)
	}
	if scheduled.Title != "Scheduled cleanup" || !strings.Contains(scheduled.Status, "Failed") || !strings.Contains(scheduled.Status, "10:30 AM") {
		t.Errorf("scheduled detail lost occurrence status or scheduled time: %+v", scheduled)
	}
	if !scheduled.IdentityHidden {
		t.Errorf("scheduled detail invented execution identity absent from Today: %+v", scheduled)
	}
	for _, want := range []string{"Run ID", "run-scheduled", "Occurrence ID", "occurrence-42", "Schedule ID", "schedule-7", "Flow ID", "flow-maintenance", "Recurrence", "weekdays"} {
		if !strings.Contains(scheduled.Metadata, want) {
			t.Errorf("scheduled metadata missing %q: %s", want, scheduled.Metadata)
		}
	}
	if scheduled.ErrorHidden || scheduled.Error != "agent exited" {
		t.Errorf("scheduled detail lost occurrence error: %+v", scheduled)
	}
}

func TestTodayProgressUsesHumanReadableStateLabels(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`
		var state={today:{date:'2026-08-03',timezone_abbreviation:'CDT',in_progress:[
			{run_id:'queued-run',conversation_title:'Queued work',agent:'Codex',state:'queued',updated_at:'2026-08-03T14:00:00Z'},
			{run_id:'working-run',conversation_title:'Working work',agent:'Codex',state:'working',updated_at:'2026-08-03T14:01:00Z'}
		],scheduled:[]}};
		var elements={
			'today-date':{textContent:''},'today-status':{innerHTML:''},
			'in-progress':{innerHTML:''},'scheduled':{innerHTML:''}
		};
		var document={getElementById:function(id){return elements[id]}};
		function readSurface(){return {loaded:true}}
		function todayFocusKey(){return ''}
		function todayStatusHTML(){return ''}
		function participantTargetLabel(item){return item.agent}
		function esc(value){return String(value)}
		function when(){return '9:00 AM'}
		function restoreTodayFocus(){}
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(conversationPageFunction(t, "renderToday")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`renderToday();elements['in-progress'].innerHTML`)
	if err != nil {
		t.Fatal(err)
	}
	html := value.String()
	for _, state := range []string{"Queued", "Working"} {
		if !strings.Contains(html, " · "+state+" · ") {
			t.Errorf("Today state is not human-readable %q: %s", state, html)
		}
		if strings.Contains(html, " · "+strings.ToLower(state)+" · ") {
			t.Errorf("Today state still exposes raw enum %q: %s", strings.ToLower(state), html)
		}
	}
}

func TestTodayLabelsBlockedScheduleOccurrenceAsFired(t *testing.T) {
	if !strings.Contains(conversationPageFunction(t, "renderToday"), "fired:'Fired'") {
		t.Fatal("Today cannot distinguish a fired schedule waiting at a human gate from a completed occurrence")
	}
}

func TestRootServesConversationPageAndLegacyDeckRemainsReachable(t *testing.T) {
	s := &Server{}
	root := httptest.NewRecorder()
	s.handlePage(root, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(root.Body.String(), "Shared conversations") {
		t.Fatalf("root did not serve conversation page: %s", root.Body.String())
	}
	legacy := httptest.NewRecorder()
	s.handleLegacyPage(legacy, httptest.NewRequest(http.MethodGet, "/legacy", nil))
	if legacy.Body.String() != boardHTML {
		t.Fatal("legacy deck changed while installing focused default")
	}
}
