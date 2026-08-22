package postgres

import (
	"context"
	"testing"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/ledger"
)

func TestSharedPoolCreatesAccountStoresWithoutClosingSharedConnections(t *testing.T) {
	t.Parallel()

	database := &fakeDatabase{}
	shared, err := newSharedPool(database)
	if err != nil {
		t.Fatalf("newSharedPool: %v", err)
	}
	store, err := shared.ForAccount(testAccountID)
	if err != nil {
		t.Fatalf("ForAccount: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("account Store Close: %v", err)
	}
	if database.closed {
		t.Fatal("account Store closed the shared Postgres pool")
	}
	if err := shared.Close(); err != nil {
		t.Fatalf("shared pool Close: %v", err)
	}
	if !database.closed {
		t.Fatal("shared pool Close did not close Postgres")
	}
}

func TestSharedPoolRejectsMalformedAccountBeforeCreatingStore(t *testing.T) {
	t.Parallel()

	shared, err := newSharedPool(&fakeDatabase{})
	if err != nil {
		t.Fatalf("newSharedPool: %v", err)
	}
	if _, err := shared.ForAccount("account-from-request-body"); err == nil {
		t.Fatal("ForAccount accepted malformed account UUID")
	}
}

func TestSharedPoolHandoffForwardersBindOnlyTheCommandOrReadAccount(t *testing.T) {
	t.Parallel()
	database := &fakeDatabase{}
	shared, err := newSharedPool(database)
	if err != nil {
		t.Fatalf("newSharedPool: %v", err)
	}
	ctx := context.Background()
	if _, err := shared.CreateHumanHandoff(ctx, ledger.CreateHumanHandoffCommand{AccountID: "request-body-account"}); err == nil {
		t.Fatal("CreateHumanHandoff accepted a malformed command account")
	}
	if _, err := shared.ListHandoffs(ctx, "request-query-account"); err == nil {
		t.Fatal("ListHandoffs accepted a malformed read account")
	}
	if _, err := shared.GetHandoff(ctx, "request-path-account", "handoff:one"); err == nil {
		t.Fatal("GetHandoff accepted a malformed read account")
	}
	if _, err := shared.CancelHandoff(ctx, ledger.CancelHandoffCommand{AccountID: "request-body-account"}); err == nil {
		t.Fatal("CancelHandoff accepted a malformed command account")
	}
	if database.begins != 0 {
		t.Fatalf("Handoff forwarders began %d transactions for rejected accounts", database.begins)
	}
}

func TestSharedPoolWorkerArtifactForwardersBindOnlyAuthenticatedCommandAccount(t *testing.T) {
	t.Parallel()
	database := &fakeDatabase{}
	shared, err := newSharedPool(database)
	if err != nil {
		t.Fatalf("newSharedPool: %v", err)
	}
	ctx := context.Background()
	if _, err := shared.CreateWorkerArtifact(ctx, controlapi.WorkerArtifactCreateCommand{AccountID: "request-body-account"}); err == nil {
		t.Fatal("CreateWorkerArtifact accepted a malformed command account")
	}
	if _, err := shared.GetWorkerArtifactStatus(ctx, controlapi.WorkerArtifactStatusCommand{AccountID: "request-body-account"}); err == nil {
		t.Fatal("GetWorkerArtifactStatus accepted a malformed command account")
	}
	if _, err := shared.AppendWorkerArtifactChunk(ctx, controlapi.WorkerArtifactChunkCommand{AccountID: "request-body-account"}); err == nil {
		t.Fatal("AppendWorkerArtifactChunk accepted a malformed command account")
	}
	if _, err := shared.FinalizeWorkerArtifact(ctx, controlapi.WorkerArtifactFinalizeCommand{AccountID: "request-body-account"}); err == nil {
		t.Fatal("FinalizeWorkerArtifact accepted a malformed command account")
	}
	if database.begins != 0 {
		t.Fatalf("worker artifact forwarders began %d transactions for rejected accounts", database.begins)
	}
}
