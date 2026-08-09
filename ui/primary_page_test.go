package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

func primaryPageSource(t *testing.T) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	result := httptest.NewRecorder()
	New(Deps{}).handlePrimaryPage(result, request)
	if result.Code != http.StatusOK {
		t.Fatalf("GET / status=%d body=%s", result.Code, result.Body.String())
	}
	if got := result.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("Content-Type=%q", got)
	}
	if got := result.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
	return result.Body.String()
}

func primaryPageFunction(t *testing.T, name string) string {
	t.Helper()
	start := strings.Index(primaryPageHTML, "function "+name+"(")
	if start < 0 {
		t.Fatalf("primary page function %q missing", name)
	}
	end := strings.Index(primaryPageHTML[start:], "\nfunction ")
	if end < 0 {
		end = strings.Index(primaryPageHTML[start:], "\n</script>")
	}
	if end < 0 {
		t.Fatalf("primary page function %q has no bounded end", name)
	}
	return primaryPageHTML[start : start+end]
}

func TestPrimaryPageContainsThePrivateChannelAndScheduleExperience(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`<title>Fort · Private Channels</title>`,
		`id="channel-rail"`,
		`id="pinned-channels"`,
		`id="recent-channels"`,
		`id="new-channel-button"`,
		`id="channel-feed"`,
		`id="channel-composer"`,
		`id="scheduled-view"`,
		`id="needs-you-dialog"`,
		`id="settings-dialog"`,
		`id="identity-dialog"`,
		`Primary Agent`,
		`Text-only chat`,
		`Choose Primary Agent`,
		`Archived Channels`,
		`Schedule inventory`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("primary page is missing %q", want)
		}
	}
}

func TestPrimaryPageUsesOnlyPhaseOneReadAndChannelAPIs(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`/api/settings/primary-agent`,
		`/api/settings/primary-agent/recheck`,
		`/api/channels`,
		`/api/needs-you`,
		`/api/schedules?state=`,
		`/api/schedules/`,
		`/occurrences?limit=50`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("primary page is missing API route %q", want)
		}
	}
	for _, forbidden := range []string{
		`/api/chat`,
		`/api/conversations`,
		`/api/playbooks`,
		`/api/route`,
		`/api/projects`,
		`/legacy`,
	} {
		if strings.Contains(source, forbidden) {
			t.Errorf("primary page calls legacy or deferred API %q", forbidden)
		}
	}
}

func TestPrimaryPageHasNoFakeTranscriptOrScheduleData(t *testing.T) {
	source := strings.ToLower(primaryPageSource(t))
	for _, forbidden := range []string{
		"trusted-agent-pilot",
		"weekly-review",
		"harness trial",
		"tax-transformation",
		"daily reflection",
		"mock data",
		"sample channel",
		"placeholder message",
	} {
		if strings.Contains(source, forbidden) {
			t.Errorf("primary page contains fake/reference content %q", forbidden)
		}
	}
}

func TestPrimaryPageThemesAreClosedAndBrowserLocalOnly(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`fort.primary.theme.v1`,
		`quiet-intelligence`,
		`private-channels`,
		`native-daylight`,
		`localStorage.getItem`,
		`localStorage.setItem`,
		`data-theme`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("primary page theme contract is missing %q", want)
		}
	}
	for _, forbidden := range []string{`/api/theme`, `document.cookie`, `theme_sync`} {
		if strings.Contains(source, forbidden) {
			t.Errorf("theme preference is not local-only: found %q", forbidden)
		}
	}
}

func TestPrimaryPageAccessibilityAndResponsiveHooks(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`class="skip-link"`,
		`aria-label="Private Channels"`,
		`aria-live="polite"`,
		`role="log"`,
		`aria-modal="true"`,
		`:focus-visible`,
		`min-height:44px`,
		`@media (max-width:860px)`,
		`@media (prefers-reduced-motion:reduce)`,
		`overflow-x:hidden`,
		`rail.inert=compact&&!open`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("primary page accessibility/responsive contract is missing %q", want)
		}
	}
}

func TestPrimaryPageAnnouncesOnlyChangedTargetStatus(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`id="channel-feed" role="log" aria-live="off"`,
		`id="target-status-announcer" class="visually-hidden" role="status" aria-live="polite"`,
		`targetAnnouncement:''`,
		`function announceLatestTargetStatus(targets)`,
		`if(signature===app.targetAnnouncement)return`,
		`announceLatestTargetStatus(targets)`,
		`.target-details .fact-grid div{display:grid;grid-template-columns:minmax(92px,.72fr) minmax(0,1.28fr)`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("target status announcement contract is missing %q", want)
		}
	}
	if strings.Contains(source, `card.setAttribute('aria-live','polite')`) {
		t.Fatal("target cards nest a live region inside the transcript log")
	}
}

func TestPrimaryPageRebuildsFromDurableStateAndKeepsWritesSingleFlight(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`crypto.randomUUID()`,
		`pendingTurn`,
		`app.sending`,
		`app.creating`,
		`new EventSource`,
		`addEventListener('conversation'`,
		`const seen=new Set()`,
		`Retry keeps the same client turn ID`,
		`Recheck and retry`,
		`schedule_inventory`,
		`Cancel`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("primary page durable/recovery behavior is missing %q", want)
		}
	}
}

func TestPrimaryPageProgressivelyDisclosesDurableTargetStates(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(primaryPageFunction(t, "failedRecoveryAction")); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(primaryPageFunction(t, "targetPresentation")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`JSON.stringify([
		targetPresentation({state:"answered",attempt:1},"phase1-qa"),
		targetPresentation({state:"queued",attempt:1},"phase1-qa"),
		targetPresentation({state:"working",attempt:1},"phase1-qa"),
		targetPresentation({state:"failed",attempt:1,error_code:"primary_agent_drift"},"phase1-qa"),
		targetPresentation({state:"failed",attempt:1,error_code:"daemon_interrupted"},"phase1-qa"),
		targetPresentation({state:"failed",attempt:1,error_code:"provider_failed"},"phase1-qa"),
		targetPresentation({state:"failed",attempt:1,error_code:"chat_authority_violation"},"phase1-qa"),
		targetPresentation({state:"canceled",attempt:1},"phase1-qa")
	])`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[null,{"kind":"queued","title":"Starting Primary Agent…","body":"","action":"Cancel","details":false},{"kind":"working","title":"Primary Agent is working","body":"","action":"Cancel","details":false},{"kind":"failed","title":"This didn’t start","body":"The saved Primary Agent on phase1-qa changed before Fort could begin. Fort kept your message.","action":"Recheck and retry","details":true},{"kind":"interrupted","title":"Answer interrupted","body":"Fort kept your message. Retry uses the same saved Primary Agent.","action":"Retry","details":true},{"kind":"failed","title":"Answer failed","body":"Fort couldn’t finish this answer. Fort kept your message.","action":"Retry","details":true},{"kind":"failed","title":"Answer failed","body":"Fort couldn’t finish this answer. Fort kept your message.","action":"","details":true},{"kind":"canceled","title":"Canceled by you","body":"","action":"","details":true}]`
	if got := value.String(); got != want {
		t.Fatalf("target presentation = %s\nwant %s", got, want)
	}

	source := primaryPageSource(t)
	for _, want := range []string{
		`node('details','target-details')`,
		`['Client turn ID',turn&&turn.client_turn_id]`,
		`['Error code',target.error_code]`,
		`Retry keeps this client turn ID and creates Attempt `,
		`latestTargetsForTurn(turnIDForMessage(message,turns),targets)`,
		`if(channelRefresh.status==='fulfilled')setComposerStatus('')`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("progressive target disclosure is missing %q", want)
		}
	}
	for _, forbidden := range []string{
		`Durable turn status`,
		`aria-label','Durable turn status`,
		`turn-state-heading`,
		`'Target '+String(target.id`,
	} {
		if strings.Contains(source, forbidden) {
			t.Errorf("progressive target disclosure still exposes %q", forbidden)
		}
	}
}

func TestPrimaryPageShowsOnlyTheLatestAttemptBesideItsPrompt(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(primaryPageFunction(t, "isLatestTargetAttempt")); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(primaryPageFunction(t, "latestTargetsForTurn")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`JSON.stringify(latestTargetsForTurn("turn-1",[
		{id:"failed-1",turn_id:"turn-1",participant_id:"primary",attempt:1,state:"failed"},
		{id:"answered-2",turn_id:"turn-1",participant_id:"primary",attempt:2,state:"answered"},
		{id:"other-turn",turn_id:"turn-2",participant_id:"primary",attempt:1,state:"working"}
	]))`)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := value.String(), `[{"id":"answered-2","turn_id":"turn-1","participant_id":"primary","attempt":2,"state":"answered"}]`; got != want {
		t.Fatalf("latest targets = %s, want %s", got, want)
	}
}

func TestPrimaryPageJavaScriptParses(t *testing.T) {
	start := strings.LastIndex(primaryPageHTML, "<script>")
	end := strings.LastIndex(primaryPageHTML, "</script>")
	if start < 0 || end <= start {
		t.Fatal("primary page script missing")
	}
	if _, err := goja.Compile("primary-page.js", primaryPageHTML[start+len("<script>"):end], false); err != nil {
		t.Fatalf("primary page JavaScript does not parse: %v", err)
	}
}

func TestPrimaryPageBlocksComposerForDriftedChannelAuthority(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`function channelReadinessState(detail)`,
		`function channelCanSend(detail)`,
		`const canSend=channelCanSend(app.activeDetail)`,
		`byId('composer-input').disabled=!canSend`,
		`byId('send-button').disabled=!canSend`,
		`if(!channelCanSend(app.activeDetail))`,
		`function markChannelAuthorityDrifted(detail)`,
		`markChannelAuthorityDrifted(app.activeDetail)`,
		`if(error&&error.code==='primary_agent_drift')markChannelAuthorityDrifted(app.activeDetail)`,
		`if(!failure.keepPending)app.pendingTurn=null`,
		`This Channel's saved Primary Agent authority no longer matches the current verified authority. Create a new Channel to continue.`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("primary page drift guard is missing %q", want)
		}
	}
}

func TestPrimaryPageDriftSubmissionFailureDoesNotOfferRetry(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`function errorText(error){return error&&error.message||"Request failed"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(primaryPageFunction(t, "submissionFailurePresentation")); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`JSON.stringify({
		drift:submissionFailurePresentation({code:"primary_agent_drift",message:"Primary Agent authority drifted on phase1-qa"},"This Channel's saved Primary Agent authority no longer matches the current verified authority. Create a new Channel to continue."),
		retryable:submissionFailurePresentation({code:"network",message:"Network unavailable"},"")
	})`)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"drift":{"authorityDrift":true,"keepPending":false,"message":"This Channel's saved Primary Agent authority no longer matches the current verified authority. Create a new Channel to continue."},"retryable":{"authorityDrift":false,"keepPending":true,"message":"Network unavailable Retry keeps the same client turn ID."}}`
	if got := value.String(); got != want {
		t.Fatalf("submission failure presentation = %s\nwant %s", got, want)
	}
	if !strings.Contains(primaryPageSource(t), `const failure=submissionFailurePresentation(error,channelComposerBlockMessage(app.activeDetail))`) {
		t.Fatal("sendMessage does not use the tested drift failure presentation")
	}
}

func TestPrimaryPageUsesRealFortAssetsAndNoForbiddenDrawnAssets(t *testing.T) {
	source := strings.ToLower(primaryPageSource(t))
	if !strings.Contains(source, `src="/fort-agent-orb.png"`) || !strings.Contains(source, `href="/fort-icon.png"`) {
		t.Fatal("primary page does not use the supplied Fort raster assets")
	}
	for _, forbidden := range []string{"<svg", "linear-gradient", "radial-gradient", "data:image/svg", "&#x"} {
		if strings.Contains(source, forbidden) {
			t.Errorf("primary page contains forbidden drawn asset %q", forbidden)
		}
	}
}

func TestPrimaryPageEventRefreshDoesNotReconnectTheSameChannel(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`eventChannelID`,
		`if(app.eventSource&&app.eventChannelID===channelID)return`,
		`refreshChannelDetail(channelID)`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("same-channel EventSource contract is missing %q", want)
		}
	}
	refresh := source[strings.Index(source, `function scheduleChannelRefresh`):strings.Index(source, `async function refreshChannelListOnly`)]
	if strings.Contains(refresh, `loadChannel(channelID)`) {
		t.Fatal("an EventSource event reloads through loadChannel and reconnects the stream")
	}
}

func TestPrimaryPagePreservesRecoveryFocusAndReadingPositionAcrossRefresh(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`function focusedTargetControl()`,
		`const focused=focusedTargetControl();const detail=await updateChannelDetail(id,false,false);if(focused.targetID)focusTarget(focused.targetID,false,focused.control)`,
		`cancel.dataset.targetFocus='cancel'`,
		`retry.dataset.targetFocus='recovery'`,
		`summary.dataset.targetFocus='details'`,
		`const wasAtBottom=feed.scrollHeight-feed.scrollTop-feed.clientHeight<=48`,
		`const previousScrollTop=feed.scrollTop`,
		`feed.scrollTop=wasAtBottom?feed.scrollHeight:previousScrollTop`,
		`.target-details summary{`,
		`color:var(--text)`,
		`.target-link{color:var(--body)`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("SSE recovery focus/scroll contract is missing %q", want)
		}
	}
}

func TestPrimaryPageSeparatesStoredIdentityFromLiveReadiness(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{`function channelReadinessLabel`, `Stored identity`, `primary_state`, `readiness&&detail.readiness.state`} {
		if !strings.Contains(source, want) {
			t.Errorf("truthful Channel identity status is missing %q", want)
		}
	}
	render := source[strings.Index(source, `function renderActiveChannel`):strings.Index(source, `function renderMessages`)]
	if strings.Contains(render, `participant.state`) {
		t.Fatal("Channel header presents durable participant membership as live readiness")
	}
}

func TestPrimaryPageTargetsAreDeepLinkedAndAnswersUseAttemptAuthority(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`function targetAnchorID`,
		`card.dataset.targetId=`,
		`link.href='#'+card.id`,
		`function answerTarget(message)`,
		`target.authority||{}`,
		`target.receipt||{}`,
		`message.target_id`,
		`function isLatestTargetAttempt`,
		`function focusedTargetControl`,
		`targetActions(activeChannelID(),target,latest)`,
		`value.dataset.targetId===String(targetID)`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("durable target/attribution contract is missing %q", want)
		}
	}
	actions := source[strings.Index(source, `function targetActions`):strings.Index(source, `function factGrid`)]
	if !strings.Contains(actions, `if(!latest)return null`) {
		t.Fatal("historical failed attempts can still expose Retry controls")
	}
}

func TestPrimaryPageRendersCompleteScheduleInventoryEvidence(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`list(inventory.items).forEach`,
		`inventory.current_digest`,
		`inventory.accepted_digest`,
		`item.id`,
		`item.kind`,
		`item.expression`,
		`item.timezone`,
		`item.flow_id`,
		`item.flow_digest`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("schedule inventory evidence is missing %q", want)
		}
	}
}

func TestPrimaryPageScheduleRowsExposeReadOnlyOperationalEvidence(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`item.scheduler_ownership`,
		`item.observed_at`,
		`occurrence.error`,
		`occurrence.run_id`,
		`View evidence`,
		`Open Channel`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("schedule row evidence/action is missing %q", want)
		}
	}
	if strings.Contains(source, `/legacy`) {
		t.Fatal("read-only schedule actions link to the legacy surface")
	}
}

func TestPrimaryPageScheduleRowsKeepLongEvidenceLegible(t *testing.T) {
	source := primaryPageSource(t)
	for _, want := range []string{
		`.main-surface{width:100%;min-width:0;display:grid;grid-template-columns:minmax(0,1fr)`,
		`.channel-view{width:100%;height:100%;display:grid;grid-template-columns:minmax(0,1fr)`,
		`.schedule-state span{display:block`,
		`text-transform:none`,
		`-webkit-line-clamp:2`,
		`grid-template-columns:minmax(205px,1.2fr)`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("schedule row legibility rule is missing %q", want)
		}
	}
}
