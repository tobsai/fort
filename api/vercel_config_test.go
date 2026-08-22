package handler

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestControlDeploymentExcludesNonServerArtifacts(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	file, err := os.Open(filepath.Join(filepath.Dir(filename), "..", ".vercelignore"))
	if err != nil {
		t.Fatalf("open .vercelignore: %v", err)
	}
	defer file.Close()

	want := map[string]bool{
		".git":         false,
		".claude":      false,
		".fort-native": false,
		"build":        false,
		"dist":         false,
		"gateway":      false,
		"ui":           false,
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if _, ok := want[scanner.Text()]; ok {
			want[scanner.Text()] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read .vercelignore: %v", err)
	}
	for path, found := range want {
		if !found {
			t.Errorf(".vercelignore must exclude %q", path)
		}
	}
}

type vercelControlConfig struct {
	Regions   []string `json:"regions"`
	Functions map[string]struct {
		MaxDuration int `json:"maxDuration"`
	} `json:"functions"`
	Crons []struct {
		Path     string `json:"path"`
		Schedule string `json:"schedule"`
	} `json:"crons"`
	Rewrites []struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	} `json:"rewrites"`
}

func TestControlProjectPinsBoundedGoFunctionsToIAD(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	payload, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "vercel.json"))
	if err != nil {
		t.Fatalf("read vercel.json: %v", err)
	}
	var config vercelControlConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		t.Fatalf("decode vercel.json: %v", err)
	}
	if len(config.Regions) != 1 || config.Regions[0] != "iad1" {
		t.Fatalf("regions = %v, want [iad1]", config.Regions)
	}
	function, ok := config.Functions["api/*.go"]
	if !ok {
		t.Fatal("api/*.go function configuration is required")
	}
	if function.MaxDuration <= 0 || function.MaxDuration > 60 {
		t.Fatalf("Go maxDuration = %d, want a positive bound no greater than 60 seconds", function.MaxDuration)
	}
	nested, ok := config.Functions["api/**/*.go"]
	if !ok || nested.MaxDuration <= 0 || nested.MaxDuration > 60 {
		t.Fatalf("nested v2 Go function configuration = %+v/%t, want a positive bound no greater than 60 seconds", nested, ok)
	}
	if len(config.Crons) != 1 || config.Crons[0].Path != "/api/v2/cron/tick" || config.Crons[0].Schedule != "* * * * *" {
		t.Fatalf("Cron configuration = %+v, want one per-minute bounded tick", config.Crons)
	}
	wantRewrites := map[string]string{
		"/api/v2/worker": "/api/v2/worker/index.go",
		"/api/v2/agents/:agent_id/conversations/:conversation_id":                           "/api/v2/chat?resource=conversation&agent_id=:agent_id&conversation_id=:conversation_id",
		"/api/v2/agents/:agent_id/conversations/:conversation_id/turns":                     "/api/v2/chat?resource=turns&agent_id=:agent_id&conversation_id=:conversation_id",
		"/api/v2/agents/:agent_id/conversations/:conversation_id/targets/:target_id/retry":  "/api/v2/chat?resource=retry&agent_id=:agent_id&conversation_id=:conversation_id&target_id=:target_id",
		"/api/v2/agents/:agent_id/conversations/:conversation_id/targets/:target_id/cancel": "/api/v2/chat?resource=cancel&agent_id=:agent_id&conversation_id=:conversation_id&target_id=:target_id",
		"/api/v2/agents/:agent_id/conversations/canonical":                                  "/api/v2/owner?resource=agent_canonical_conversation&agent_id=:agent_id",
		"/api/v2/agents/:agent_id/conversations":                                            "/api/v2/owner?resource=agent_conversations&agent_id=:agent_id",
		"/api/v2/agents/:agent_id/routines/:routine_id/test":                                "/api/v2/owner?resource=routine_test&agent_id=:agent_id&routine_id=:routine_id",
		"/api/v2/agents/:agent_id/routines/:routine_id":                                     "/api/v2/owner?resource=routine_detail&agent_id=:agent_id&routine_id=:routine_id",
		"/api/v2/agents/:agent_id/routines":                                                 "/api/v2/owner?resource=routines&agent_id=:agent_id",
		"/api/v2/agents/:agent_id":                                                          "/api/v2/owner?resource=agent&agent_id=:agent_id",
		"/api/v2/groups/:group_id/turns":                                                    "/api/v2/owner?resource=group_turns&group_id=:group_id",
		"/api/v2/groups/:group_id":                                                          "/api/v2/owner?resource=group_detail&group_id=:group_id",
		"/api/v2/groups":                                                                    "/api/v2/owner?resource=groups",
		"/api/v2/handoffs/:handoff_id/cancel":                                               "/api/v2/owner?resource=handoff_cancel&handoff_id=:handoff_id",
		"/api/v2/handoffs/:handoff_id":                                                      "/api/v2/owner?resource=handoff_detail&handoff_id=:handoff_id",
		"/api/v2/handoffs":                                                                  "/api/v2/owner?resource=handoffs",
	}
	for _, rewrite := range config.Rewrites {
		if want, ok := wantRewrites[rewrite.Source]; ok && rewrite.Destination == want {
			delete(wantRewrites, rewrite.Source)
		}
	}
	if len(wantRewrites) != 0 {
		t.Fatalf("missing semantic v2 rewrites: %v", wantRewrites)
	}
}

func TestControlPreviewRetainsRoutesWithoutRegisteringPaidCron(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	root := filepath.Join(filepath.Dir(filename), "..")
	load := func(name string) vercelControlConfig {
		t.Helper()
		payload, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var config vercelControlConfig
		if err := json.Unmarshal(payload, &config); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		return config
	}

	production := load("vercel.json")
	preview := load("vercel.preview.json")
	if !reflect.DeepEqual(preview.Regions, production.Regions) ||
		!reflect.DeepEqual(preview.Functions, production.Functions) ||
		!reflect.DeepEqual(preview.Rewrites, production.Rewrites) {
		t.Fatal("preview control deployment must retain production regions, function bounds, and routes")
	}
	if len(preview.Crons) != 0 {
		t.Fatalf("preview Crons = %+v, want no registered schedule", preview.Crons)
	}
}
