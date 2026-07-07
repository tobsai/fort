package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/fake"
)

func waitEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	rs, err := rules.Parse([]byte(ruleset))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(router.New(rs), fake.New(), st, t.TempDir()), st
}

func TestWaitBlocksThenReturnsAfterPersist(t *testing.T) {
	e, st := waitEngine(t)
	runID, _, err := e.SubmitRef(context.Background(), task.Task{ID: "t1", Title: "x"})
	if err != nil {
		t.Fatal(err)
	}
	e.Wait(runID)
	got, err := st.GetRun(runID)
	if err != nil || got.Status != "succeeded" {
		t.Fatalf("after Wait: status=%q err=%v", got.Status, err)
	}
	evs, _ := st.Events(runID)
	if len(evs) == 0 {
		t.Fatal("no events persisted after Wait")
	}
}

func TestWaitReturnsImmediatelyForFinishedOrUnknown(t *testing.T) {
	e, _ := waitEngine(t)
	done := make(chan struct{})
	go func() { e.Wait("nope"); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Wait on unknown id did not return")
	}
	runID, _, _ := e.SubmitRef(context.Background(), task.Task{ID: "t2", Title: "y"})
	e.Wait(runID)
	done2 := make(chan struct{})
	go func() { e.Wait(runID); close(done2) }()
	select {
	case <-done2:
	case <-time.After(time.Second):
		t.Fatal("Wait on finished run did not return immediately")
	}
}
