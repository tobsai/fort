package ui

import (
	"net/http"
	"net/http/httptest"
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
		`<dialog id="project-dialog">`,
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
