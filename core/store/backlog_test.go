package store

import (
	"path/filepath"
	"testing"
)

func openBacklogTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestBacklogCRUD(t *testing.T) {
	s := openBacklogTest(t)
	if items, err := s.ListBacklog(); err != nil || len(items) != 0 {
		t.Fatalf("empty backlog: %v %v", items, err)
	}
	a := BacklogItem{ID: "b1", Title: "refactor loader", Body: "do it", Agent: "codex", Machine: "mini", Labels: []string{"dev"}, Source: "user"}
	if err := s.CreateBacklogItem(a); err != nil {
		t.Fatal(err)
	}
	b := BacklogItem{ID: "b2", Title: "write docs", Source: "agent"}
	if err := s.CreateBacklogItem(b); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListBacklog()
	if err != nil || len(items) != 2 {
		t.Fatalf("list: %v %v", items, err)
	}
	if items[0].ID != "b2" || items[1].ID != "b1" {
		t.Fatalf("order = %s,%s; want b2,b1", items[0].ID, items[1].ID)
	}
	got, err := s.GetBacklogItem("b1")
	if err != nil || got.Title != "refactor loader" || got.Agent != "codex" || got.Machine != "mini" || got.Source != "user" || len(got.Labels) != 1 || got.Labels[0] != "dev" {
		t.Fatalf("get b1 = %+v (%v)", got, err)
	}
	if err := s.DeleteBacklogItem("b1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetBacklogItem("b1"); err == nil {
		t.Fatal("b1 should be gone")
	}
	items, _ = s.ListBacklog()
	if len(items) != 1 || items[0].ID != "b2" {
		t.Fatalf("after delete = %+v", items)
	}
}
