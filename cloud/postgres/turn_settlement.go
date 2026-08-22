package postgres

import (
	"context"
	"fmt"
	"time"
)

// settleConversationTurnIfTerminal closes a source turn only after its entire
// frozen initial wave and every accepted Handoff in that causal chain are
// terminal. Needs You and lease-expired work remain open and actionable.
func settleConversationTurnIfTerminal(ctx context.Context, tx transaction, accountID, turnID string, settledAt time.Time) error {
	if accountID == "" || turnID == "" || settledAt.IsZero() {
		return fmt.Errorf("turn settlement identity and time are required")
	}
	_, err := tx.exec(ctx, `with settled as (
  update fort_private.conversation_turn as turn_item
  set state = 'settled', updated_at = $3
  where turn_item.account_id = $1 and turn_item.turn_id = $2 and turn_item.state = 'open'
    and not exists (
      select 1 from fort_private.conversation_target as target
      where target.account_id = turn_item.account_id and target.turn_id = turn_item.turn_id
        and target.state in ('queued','claimed','working','needs_you','cancel_requested','lease_expired')
    )
    and not exists (
      select 1 from fort_private.handoff as handoff
      where handoff.account_id = turn_item.account_id and handoff.group_turn_id = turn_item.turn_id
        and handoff.state in ('requested','needs_approval','queued','working','needs_you')
    )
  returning turn_item.turn_id
)
insert into fort_private.ledger_event (
  account_id, aggregate_kind, aggregate_id, event_type, event_metadata, created_at
)
select $1, 'conversation_turn', settled.turn_id, 'conversation.turn.settled',
  '{"reason":"all_targets_and_handoffs_terminal"}'::jsonb, $3
from settled`, accountID, turnID, settledAt.UTC())
	if err != nil {
		return fmt.Errorf("settle terminal Conversation Turn: %w", err)
	}
	return nil
}
