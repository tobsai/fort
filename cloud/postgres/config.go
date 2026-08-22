// Package postgres implements Fort's account-scoped cloud ledger on Postgres.
package postgres

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SupavisorTransactionConfig applies transaction-pool-safe pgx settings to a
// Postgres URL. It intentionally remains usable by local integration tests;
// production Open entrypoints first enforce validateSupavisorRuntimeDatabaseURL.
// Transaction pooling can move consecutive operations between server
// connections, so Fort uses unprepared exec and disables both client caches.
func SupavisorTransactionConfig(databaseURL string) (*pgxpool.Config, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("Postgres database URL is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Postgres database URL: %w", err)
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	config.ConnConfig.StatementCacheCapacity = 0
	config.ConnConfig.DescriptionCacheCapacity = 0
	return config, nil
}

// validateSupavisorRuntimeDatabaseURL rejects a production runtime database
// URL unless it explicitly names Supabase's shared Supavisor transaction
// pooler and requires authenticated transport encryption. Local migration and
// integration tools deliberately use SupavisorTransactionConfig directly;
// every runtime Open entrypoint calls this stricter boundary first.
func validateSupavisorRuntimeDatabaseURL(databaseURL string) error {
	parsed, err := url.Parse(databaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.User == nil {
		return fmt.Errorf("Supavisor transaction pooler URL is invalid")
	}
	hostname := strings.ToLower(parsed.Hostname())
	const suffix = ".pooler.supabase.com"
	if !strings.HasSuffix(hostname, suffix) || strings.TrimSuffix(hostname, suffix) == "" ||
		parsed.Port() != strconv.Itoa(6543) {
		return fmt.Errorf("Supavisor transaction pooler host and port 6543 are required")
	}
	username := parsed.User.Username()
	separator := strings.LastIndexByte(username, '.')
	if separator < 1 || separator == len(username)-1 {
		return fmt.Errorf("Supavisor transaction pooler username must include its project reference")
	}
	sslModes, present := parsed.Query()["sslmode"]
	if !present || len(sslModes) != 1 {
		return fmt.Errorf("Supavisor transaction pooler requires an explicit SSL mode")
	}
	switch sslModes[0] {
	case "require", "verify-ca", "verify-full":
		return nil
	default:
		return fmt.Errorf("Supavisor transaction pooler requires SSL mode require, verify-ca, or verify-full")
	}
}
