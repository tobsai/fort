package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/securebody"
)

func TestSupavisorTransactionConfigDisablesPreparedStatements(t *testing.T) {
	t.Parallel()

	config, err := SupavisorTransactionConfig("postgres://postgres.projectref:secret@aws-0-us-east-1.pooler.supabase.com:6543/postgres?sslmode=require")
	if err != nil {
		t.Fatalf("SupavisorTransactionConfig: %v", err)
	}
	if config.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeExec {
		t.Fatalf("query mode = %v, want unprepared exec", config.ConnConfig.DefaultQueryExecMode)
	}
	if config.ConnConfig.StatementCacheCapacity != 0 {
		t.Fatalf("statement cache capacity = %d, want 0", config.ConnConfig.StatementCacheCapacity)
	}
	if config.ConnConfig.DescriptionCacheCapacity != 0 {
		t.Fatalf("description cache capacity = %d, want 0", config.ConnConfig.DescriptionCacheCapacity)
	}
}

func TestRuntimeOpenEntryPointsRejectUnsafeDatabaseURLBeforeDial(t *testing.T) {
	t.Parallel()

	unsafe := "postgres://fort_gateway:secret@db.projectref.supabase.co:5432/postgres?sslmode=require"
	openers := map[string]func() error{
		"account Store": func() error {
			_, err := Open(context.Background(), unsafe, testAccountID)
			return err
		},
		"encrypted account Store": func() error {
			_, err := OpenWithKeyRing(context.Background(), unsafe, testAccountID, securebody.KeyRing{})
			return err
		},
		"shared pool": func() error {
			_, err := OpenSharedPool(context.Background(), unsafe)
			return err
		},
		"encrypted shared pool": func() error {
			_, err := OpenSharedPoolWithKeyRing(context.Background(), unsafe, securebody.KeyRing{})
			return err
		},
	}
	for name, open := range openers {
		t.Run(name, func(t *testing.T) {
			err := open()
			if err == nil || !strings.Contains(err.Error(), "Supavisor transaction pooler") {
				t.Fatalf("runtime open error = %v, want fail-closed database URL rejection", err)
			}
		})
	}
}

func TestValidateSupavisorRuntimeDatabaseURLRequiresExplicitTLSAndSharedTransactionPooler(t *testing.T) {
	t.Parallel()

	valid := []string{
		"postgres://postgres.projectref:secret@aws-0-us-east-1.pooler.supabase.com:6543/postgres?sslmode=require",
		"postgresql://postgres.projectref:secret@aws-0-us-east-1.pooler.supabase.com:6543/postgres?sslmode=verify-ca",
		"postgresql://postgres.projectref:secret@aws-0-us-east-1.pooler.supabase.com:6543/postgres?sslmode=verify-full",
	}
	for _, databaseURL := range valid {
		t.Run(databaseURL, func(t *testing.T) {
			if err := validateSupavisorRuntimeDatabaseURL(databaseURL); err != nil {
				t.Fatalf("validateSupavisorRuntimeDatabaseURL: %v", err)
			}
		})
	}

	invalid := map[string]string{
		"missing TLS mode":          "postgres://postgres.projectref:secret@aws-0-us-east-1.pooler.supabase.com:6543/postgres",
		"opportunistic TLS":         "postgres://postgres.projectref:secret@aws-0-us-east-1.pooler.supabase.com:6543/postgres?sslmode=prefer",
		"disabled TLS":              "postgres://postgres.projectref:secret@aws-0-us-east-1.pooler.supabase.com:6543/postgres?sslmode=disable",
		"direct database":           "postgres://postgres:secret@db.projectref.supabase.co:5432/postgres?sslmode=require",
		"shared session pooler":     "postgres://postgres.projectref:secret@aws-0-us-east-1.pooler.supabase.com:5432/postgres?sslmode=require",
		"dedicated pooler":          "postgres://postgres:secret@db.projectref.supabase.co:6543/postgres?sslmode=require",
		"lookalike pooler hostname": "postgres://postgres.projectref:secret@pooler.supabase.com.attacker.invalid:6543/postgres?sslmode=require",
		"unqualified username":      "postgres://postgres:secret@aws-0-us-east-1.pooler.supabase.com:6543/postgres?sslmode=require",
	}
	for name, databaseURL := range invalid {
		t.Run(name, func(t *testing.T) {
			err := validateSupavisorRuntimeDatabaseURL(databaseURL)
			if err == nil || !strings.Contains(err.Error(), "Supavisor transaction pooler") {
				t.Fatalf("validation error = %v, want fail-closed Supavisor contract error", err)
			}
		})
	}
}
