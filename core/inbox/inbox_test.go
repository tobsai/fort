package inbox

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tobsai/fort/core/task"
)

type recordingSubmitter struct {
	mu    sync.Mutex
	tasks []task.Task
}

func (r *recordingSubmitter) Submit(_ context.Context, t task.Task) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks = append(r.tasks, t)
	return "run-" + t.ID, nil
}

func writeTask(t *testing.T, dir string, tk task.Task) {
	t.Helper()
	b, _ := json.Marshal(tk)
	if err := os.WriteFile(filepath.Join(dir, tk.ID+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanSubmitsNewTaskFiles(t *testing.T) {
	dir := t.TempDir()
	sub := &recordingSubmitter{}
	in := NewDir(dir, sub)

	writeTask(t, dir, task.Task{ID: "a", Title: "first", Labels: []string{"feature"}})
	writeTask(t, dir, task.Task{ID: "b", Title: "second"})

	n, err := in.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if n != 2 {
		t.Errorf("scanned %d, want 2", n)
	}
	if len(sub.tasks) != 2 {
		t.Fatalf("submitted %d tasks", len(sub.tasks))
	}

	// processed files are moved out so a re-scan does nothing.
	n2, _ := in.Scan(context.Background())
	if n2 != 0 {
		t.Errorf("re-scan processed %d, want 0", n2)
	}
}

func TestScanIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	sub := &recordingSubmitter{}
	in := NewDir(dir, sub)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := in.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if n != 0 {
		t.Errorf("scanned %d, want 0", n)
	}
}
