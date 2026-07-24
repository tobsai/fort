package store

import (
	"strings"
	"testing"
)

func TestFailInterruptedDirectRunsReconcilesRunningRowsExactlyOnce(t *testing.T) {
	st, err := Open(t.TempDir() + "/fort.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	for _, run := range []Run{
		{ID: "running-direct", Status: "running", Agent: "codex"},
		{ID: "running-flow", Status: "running", Agent: "flow:bug-fix", FlowID: "bug-fix"},
		{ID: "blocked", Status: "blocked"},
		{ID: "succeeded", Status: "succeeded"},
	} {
		if err := st.CreateRun(run); err != nil {
			t.Fatal(err)
		}
	}

	const reason = "interrupted when the Fort daemon stopped"
	count, err := st.FailInterruptedDirectRuns(reason)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("reconciled = %d, want 1", count)
	}
	for _, id := range []string{"running-direct"} {
		run, err := st.GetRun(id)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status != "failed" || run.ExitCode != -1 || run.Error != reason {
			t.Fatalf("%s = %+v, want failed/-1 with interruption reason", id, run)
		}
		events, err := st.Events(id)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].Type != "error" || events[0].Code != -1 ||
			!strings.Contains(events[0].Data, "interrupted") {
			t.Fatalf("%s events = %+v, want one interruption error", id, events)
		}
	}
	for _, id := range []string{"blocked", "succeeded"} {
		run, err := st.GetRun(id)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status != id {
			t.Fatalf("%s status = %q, want unchanged", id, run.Status)
		}
	}
	flow, err := st.GetRun("running-flow")
	if err != nil {
		t.Fatal(err)
	}
	if flow.Status != "running" {
		t.Fatalf("running flow status = %q, want unchanged for durable resume", flow.Status)
	}
	flowEvents, err := st.Events("running-flow")
	if err != nil {
		t.Fatal(err)
	}
	if len(flowEvents) != 0 {
		t.Fatalf("running flow events = %+v, want unchanged", flowEvents)
	}

	count, err = st.FailInterruptedDirectRuns(reason)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("second reconciliation = %d, want 0", count)
	}
	events, err := st.Events("running-direct")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events after second reconciliation = %d, want 1", len(events))
	}
}
