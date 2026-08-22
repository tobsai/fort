package cloudworker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobsai/fort/exec/native"
)

func TestCommandReadinessRecheckRequiresIdenticalEvidenceAndPinnedLocalDirectory(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	workdir, err := filepath.EvalSymlinks(temp)
	if err != nil {
		t.Fatal(err)
	}
	evidencePath := filepath.Join(temp, "readiness.json")
	if err := os.WriteFile(evidencePath, []byte(`{"ready":true,"source_config_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	readiness, err := NewCommandReadiness("capability:7", 7,
		[]string{"/bin/sh", "-c", `IFS= read -r value < "$1"; printf '%s' "$value"`, "fort-readiness", evidencePath},
		[]native.Provider{{Name: "openclaw"}})
	if err != nil {
		t.Fatal(err)
	}
	assignment := testAssignment(time.Now())
	assignment.Execution.Workdir = workdir

	if _, err := readiness.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if err := readiness.Recheck(context.Background(), assignment); err != nil {
		t.Fatalf("Recheck unchanged evidence: %v", err)
	}
	if err := os.WriteFile(evidencePath, []byte(`{"ready":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := readiness.Recheck(context.Background(), assignment); !errors.Is(err, ErrWorkerInvalid) {
		t.Fatalf("Recheck changed evidence error = %v, want invalid", err)
	}
}
