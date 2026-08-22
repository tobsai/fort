package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
)

const maximumCursorEvents = 256

var (
	_ controlapi.CursorReader = (*Store)(nil)
	_ controlapi.CursorReader = (*SharedPool)(nil)
)

type cursorEventData struct {
	AggregateKind      string    `json:"aggregate_kind"`
	AggregateID        string    `json:"aggregate_id"`
	TurnID             string    `json:"turn_id,omitempty"`
	TargetID           string    `json:"target_id,omitempty"`
	ExecutionAttemptID string    `json:"execution_attempt_id,omitempty"`
	WorkerID           string    `json:"worker_id,omitempty"`
	Metadata           any       `json:"metadata"`
	CreatedAt          time.Time `json:"created_at"`
}

// ReadCursorPage reads a bounded ascending page from the durable ledger. The
// returned data intentionally excludes encrypted sensitive event fields.
func (store *Store) ReadCursorPage(ctx context.Context, accountID, afterCursor string) (controlapi.CursorPage, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return controlapi.CursorPage{}, err
	}
	afterEventID, err := parseEventCursor(afterCursor)
	if err != nil {
		return controlapi.CursorPage{}, err
	}
	page := controlapi.CursorPage{Events: make([]controlapi.CursorEvent, 0), NextCursor: afterCursor}
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		result, err := tx.query(ctx, `select event_id, aggregate_kind, aggregate_id,
  event_type, coalesce(turn_id, ''), coalesce(target_id, ''),
  coalesce(execution_attempt_id, ''), coalesce(worker_id, ''),
  event_metadata::text, created_at
from fort_private.ledger_event
where account_id = $1 and event_id > $2
order by event_id asc
limit $3`, accountID, afterEventID, maximumCursorEvents)
		if err != nil {
			return err
		}
		defer result.close()

		lastEventID := afterEventID
		for result.next() {
			var eventID int64
			var kind, metadataJSON string
			var data cursorEventData
			if err := result.scan(&eventID, &data.AggregateKind, &data.AggregateID,
				&kind, &data.TurnID, &data.TargetID, &data.ExecutionAttemptID,
				&data.WorkerID, &metadataJSON, &data.CreatedAt); err != nil {
				return err
			}
			if eventID <= lastEventID {
				return fmt.Errorf("ledger cursor is not strictly ascending")
			}
			if strings.TrimSpace(kind) == "" || kind != strings.TrimSpace(kind) {
				return fmt.Errorf("ledger event kind is invalid")
			}
			if err := json.Unmarshal([]byte(metadataJSON), &data.Metadata); err != nil {
				return fmt.Errorf("decode ledger event metadata: %w", err)
			}
			cursor := formatEventCursor(eventID)
			candidate := page
			candidate.Events = append(append([]controlapi.CursorEvent{}, page.Events...), controlapi.CursorEvent{
				Cursor: cursor, Kind: kind, Data: data,
			})
			candidate.NextCursor = cursor
			payload, err := json.Marshal(candidate)
			if err != nil {
				return fmt.Errorf("encode ledger cursor page: %w", err)
			}
			if len(payload)+1 > controlapi.MaximumCursorPageBytes {
				if len(page.Events) == 0 {
					return fmt.Errorf("ledger event %d exceeds cursor page limit", eventID)
				}
				break
			}
			page = candidate
			lastEventID = eventID
		}
		return result.errResult()
	})
	return page, err
}

// ReadCursorPage implements controlapi.CursorReader directly on a shared
// Vercel pool by creating a non-owning account-bound Store for the request.
func (pool *SharedPool) ReadCursorPage(ctx context.Context, accountID, afterCursor string) (controlapi.CursorPage, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return controlapi.CursorPage{}, err
	}
	return store.ReadCursorPage(ctx, accountID, afterCursor)
}

func parseEventCursor(cursor string) (int64, error) {
	const prefix = "cursor-"
	if !strings.HasPrefix(cursor, prefix) {
		return 0, fmt.Errorf("Fort event cursor must start with %q", prefix)
	}
	value := strings.TrimPrefix(cursor, prefix)
	eventID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || eventID < 0 || formatEventCursor(eventID) != cursor {
		return 0, fmt.Errorf("Fort event cursor is invalid")
	}
	return eventID, nil
}

func formatEventCursor(eventID int64) string {
	return "cursor-" + strconv.FormatInt(eventID, 10)
}
