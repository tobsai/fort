package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
)

var _ controlapi.NonceClaimer = (*Store)(nil)

// Claim atomically consumes one service-assertion nonce for this Store's
// account. PostgreSQL's primary key is the arbitration point across all
// Vercel function instances.
func (store *Store) Claim(ctx context.Context, accountID, keyID, nonce string, expiresAt time.Time) (bool, error) {
	if strings.TrimSpace(keyID) == "" || strings.TrimSpace(nonce) == "" || expiresAt.IsZero() {
		return false, fmt.Errorf("service assertion key, nonce, and expiry are required")
	}
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return false, err
	}

	claimed := false
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		affected, err := tx.exec(ctx, `insert into fort_private.service_assertion_nonce (
  account_id, key_id, nonce, expires_at, claimed_at
) values ($1, $2, $3, $4, clock_timestamp())
on conflict (account_id, key_id, nonce) do nothing`, accountID, keyID, nonce, expiresAt.UTC())
		if err != nil {
			return err
		}
		claimed = affected == 1
		return nil
	})
	return claimed, err
}
