package ui_test

import (
	"testing"

	"github.com/tobsai/fort/ui"
)

func TestBacklogAddListDispatchDelete(t *testing.T) {
	s, st := newFullUI(t)

	add := do(t, s, "POST", "/api/backlog", ui.BacklogRequest{Title: "queue me", Agent: "codex", Machine: "mini"})
	if add.Code != 200 {
		t.Fatalf("add status = %d", add.Code)
	}
	item := decode[ui.BacklogItem](t, add)
	if item.ID == "" || item.Title != "queue me" || item.Source != "user" {
		t.Fatalf("added = %+v", item)
	}

	list := decode[[]ui.BacklogItem](t, do(t, s, "GET", "/api/backlog", nil))
	if len(list) != 1 || list[0].ID != item.ID {
		t.Fatalf("list = %+v", list)
	}

	disp := do(t, s, "POST", "/api/backlog/"+item.ID+"/dispatch", nil)
	if disp.Code != 200 {
		t.Fatalf("dispatch status = %d", disp.Code)
	}
	res := decode[ui.ChatResult](t, disp)
	if res.RunID == "" {
		t.Fatalf("dispatch produced no run: %+v", res)
	}
	if _, err := st.GetRun(res.RunID); err != nil {
		t.Fatalf("run not persisted: %v", err)
	}
	if remaining := decode[[]ui.BacklogItem](t, do(t, s, "GET", "/api/backlog", nil)); len(remaining) != 0 {
		t.Fatalf("backlog not emptied after dispatch: %+v", remaining)
	}
}

func TestBacklogDispatchBodylessKeepsBodyEmpty(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	item := decode[ui.BacklogItem](t, do(t, s, "POST", "/api/backlog", ui.BacklogRequest{Title: "fix it"}))
	if do(t, s, "POST", "/api/backlog/"+item.ID+"/dispatch", nil).Code != 200 {
		t.Fatal("dispatch failed")
	}
	// No fabricated body: prompt() already falls back to the title, and the run
	// row must not store a duplicate of it.
	if cd.last.Title != "fix it" || cd.last.Body != "" {
		t.Fatalf("title=%q body=%q, want body empty", cd.last.Title, cd.last.Body)
	}
}

func TestBacklogDeleteDiscards(t *testing.T) {
	s, _ := newControlUI(t)
	item := decode[ui.BacklogItem](t, do(t, s, "POST", "/api/backlog", ui.BacklogRequest{Title: "discard me"}))
	if do(t, s, "DELETE", "/api/backlog/"+item.ID, nil).Code != 200 {
		t.Fatal("delete failed")
	}
	if list := decode[[]ui.BacklogItem](t, do(t, s, "GET", "/api/backlog", nil)); len(list) != 0 {
		t.Fatalf("still present: %+v", list)
	}
}

func TestBacklogReassign(t *testing.T) {
	s, _ := newControlUI(t)
	item := decode[ui.BacklogItem](t, do(t, s, "POST", "/api/backlog", ui.BacklogRequest{Title: "move me", Agent: "claude"}))
	rec := do(t, s, "PATCH", "/api/backlog/"+item.ID, ui.BacklogPatch{Agent: "codex"})
	if rec.Code != 200 {
		t.Fatalf("patch status = %d: %s", rec.Code, rec.Body)
	}
	if got := decode[ui.BacklogItem](t, rec); got.Agent != "codex" {
		t.Fatalf("patched = %+v, want agent codex", got)
	}
	list := decode[[]ui.BacklogItem](t, do(t, s, "GET", "/api/backlog", nil))
	if len(list) != 1 || list[0].Agent != "codex" {
		t.Fatalf("list = %+v, want reassigned to codex", list)
	}
	if do(t, s, "PATCH", "/api/backlog/nope", ui.BacklogPatch{Agent: "codex"}).Code != 404 {
		t.Fatal("unknown id should 404")
	}
}

func TestBacklogAddRequiresTitle(t *testing.T) {
	s, _ := newControlUI(t)
	if do(t, s, "POST", "/api/backlog", ui.BacklogRequest{Title: "   "}).Code != 400 {
		t.Fatal("blank title should 400")
	}
}
