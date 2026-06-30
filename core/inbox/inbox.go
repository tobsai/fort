// Package inbox sources new tasks from a watched directory and submits them to
// the engine (backlog AO-015: "watched file/dir" task source). Each *.json file
// is a task.Task; processed files are moved into .processed/ so a restart never
// re-runs them.
package inbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tobsai/fort/core/task"
)

// Submitter routes and dispatches a task (implemented by *engine.Engine).
type Submitter interface {
	Submit(ctx context.Context, t task.Task) (string, error)
}

// Dir watches a directory for task files.
type Dir struct {
	path string
	sub  Submitter
}

// NewDir builds an inbox over path.
func NewDir(path string, sub Submitter) *Dir {
	return &Dir{path: path, sub: sub}
}

func (d *Dir) processedDir() string { return filepath.Join(d.path, ".processed") }

// Scan submits every new *.json task file and moves it to .processed/. It
// returns the number of tasks submitted.
func (d *Dir) Scan(ctx context.Context) (int, error) {
	if err := os.MkdirAll(d.path, 0o755); err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(d.path)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		full := filepath.Join(d.path, e.Name())
		t, err := readTask(full)
		if err != nil {
			return count, fmt.Errorf("inbox: %s: %w", e.Name(), err)
		}
		if _, err := d.sub.Submit(ctx, t); err != nil {
			return count, fmt.Errorf("inbox: submit %s: %w", e.Name(), err)
		}
		if err := d.markProcessed(full, e.Name()); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// Watch polls Scan on the given interval until ctx is canceled.
func (d *Dir) Watch(ctx context.Context, interval time.Duration) error {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if _, err := d.Scan(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func (d *Dir) markProcessed(full, name string) error {
	if err := os.MkdirAll(d.processedDir(), 0o755); err != nil {
		return err
	}
	return os.Rename(full, filepath.Join(d.processedDir(), name))
}

func readTask(path string) (task.Task, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return task.Task{}, err
	}
	var t task.Task
	if err := json.Unmarshal(b, &t); err != nil {
		return task.Task{}, err
	}
	if t.ID == "" {
		t.ID = strings.TrimSuffix(filepath.Base(path), ".json")
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	return t, nil
}
