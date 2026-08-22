package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tobsai/fort/cloud/controlapi"
	coreruntime "github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/cloudworker"
)

const (
	testAccountID   = "4af424a4-d81a-47d5-a495-400868883b86"
	testWorkerToken = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY"
)

func TestLoadWorkerModeConfigRequiresExplicitToken(t *testing.T) {
	config := workerModeFile{
		Endpoint: "https://fort-control.example/api/v2/worker", AccountID: testAccountID,
		WorkerID: "worker:studio", MachineID: "machine:studio", TokenEnv: "FORT_TEST_WORKER_TOKEN",
		CapabilityRevisionID: "capability:7", CapabilityRevision: 7,
		ReadinessCommand: []string{"/usr/bin/true"},
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "worker.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadWorkerModeConfig(path, func(string) string { return "" }); err == nil {
		t.Fatal("worker config accepted a missing machine token")
	}
	loaded, err := loadWorkerModeConfig(path, func(name string) string {
		if name == "FORT_TEST_WORKER_TOKEN" {
			return testWorkerToken
		}
		return ""
	})
	if err != nil {
		t.Fatalf("loadWorkerModeConfig: %v", err)
	}
	if loaded.file.MachineID != config.MachineID || loaded.token != testWorkerToken {
		t.Fatalf("loaded worker config = %#v", loaded)
	}
}

func TestWorkerModeHasZeroAuthorizedRealAdaptersAndCannotStartProvider(t *testing.T) {
	runtime, adapters := workerModeExecution()
	if _, err := adapters.Prepare(cloudworkerAssignmentForAuthorizationTest(), cloudworker.ExecutionContext{}); !errors.Is(err, cloudworker.ErrAdapterNotApproved) {
		t.Fatalf("adapter Prepare error = %v, want not approved", err)
	}
	if _, err := runtime.Dispatch(context.Background(), coreruntime.RunSpec{Agent: "openclaw"}); !errors.Is(err, cloudworker.ErrAdapterNotApproved) {
		t.Fatalf("runtime Dispatch error = %v, want not approved", err)
	}
}

func TestLoadWorkerModeConfigRejectsOperatorSuppliedAdapterAuthorization(t *testing.T) {
	config := map[string]any{
		"endpoint": "https://fort-control.example/api/v2/worker", "account_id": testAccountID,
		"worker_id": "worker:studio", "machine_id": "machine:studio", "token_env": "FORT_TEST_WORKER_TOKEN",
		"capability_revision_id": "capability:7", "capability_revision": 7,
		"readiness_command": []string{"/usr/bin/true"},
		"approved_bindings": []any{map[string]any{"provider": "openclaw"}},
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "worker.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadWorkerModeConfig(path, func(string) string { return testWorkerToken }); err == nil {
		t.Fatal("worker accepted operator-supplied provider adapter authorization")
	}
}

func cloudworkerAssignmentForAuthorizationTest() controlapi.WorkerAssignment {
	return controlapi.WorkerAssignment{}
}
