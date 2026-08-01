package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/scheduler"
)

func TestCmdScheduleCreatesDefinitionInRunningDaemon(t *testing.T) {
	received := make(chan scheduler.Definition, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/schedules" || r.Method != http.MethodPost {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var definition scheduler.Definition
		if err := json.NewDecoder(r.Body).Decode(&definition); err != nil {
			t.Fatal(err)
		}
		received <- definition
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	t.Setenv("FORT_ADDR", strings.TrimPrefix(server.URL, "http://"))

	if err := cmdSchedule([]string{"once", "1h", "brief"}); err != nil {
		t.Fatalf("schedule: %v", err)
	}
	definition := <-received
	if definition.Kind != scheduler.KindOnce || definition.FlowID != "brief" || definition.Expression == "" || definition.Timezone == "" {
		t.Fatalf("definition = %+v", definition)
	}
}
