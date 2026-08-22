package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

// postgresHumanHandoffSource is immutable ledger evidence for the message a
// human selected as the causal source of a new Handoff. Agent attribution is
// accepted only through a target binding or a completed parent Handoff.
type postgresHumanHandoffSource struct {
	messageID        int64
	turnID           string
	targetID         string
	handoffID        string
	authorAgentID    string
	targetAgentID    string
	targetBehaviorID string
	targetBindingID  string
	parent           *ledger.HandoffRecord
}

type humanHandoffCausalEvidence struct {
	sourceAgentID        string
	sourceBehaviorID     string
	sourceBindingID      string
	parentHandoffID      string
	parentStageAuthority *conversation.AuthorityGrant
	rootDelegationGrant  *conversation.AuthorityGrant
	groupTurnID          string
	maxAgentMessages     int
	maxDepth             int
	depth                int
	deadline             *time.Time
	ancestorAgentIDs     []string
}

func deriveHumanHandoffCausalEvidence(source postgresHumanHandoffSource) (humanHandoffCausalEvidence, error) {
	evidence := humanHandoffCausalEvidence{depth: 1, ancestorAgentIDs: []string{}}
	if source.handoffID != "" {
		if source.parent == nil {
			return humanHandoffCausalEvidence{}, fmt.Errorf("Handoff result source lacks its persisted parent Handoff")
		}
		parent := source.parent
		if parent.Handoff.ID != source.handoffID || parent.Handoff.State != conversation.HandoffCompleted ||
			parent.Result == nil || parent.Result.HandoffID != parent.Handoff.ID ||
			parent.Result.MessageID != strconv.FormatInt(source.messageID, 10) ||
			parent.Target.ID == "" || parent.Target.ID != source.targetID ||
			source.authorAgentID != parent.Handoff.RecipientAgentID {
			return humanHandoffCausalEvidence{}, fmt.Errorf("Handoff result source does not match one completed authoritative parent")
		}
		root := cloneAuthorityGrant(parent.Handoff.RootDelegationGrant)
		parentAuthority := cloneAuthorityGrant(parent.Handoff.EffectiveAuthority)
		evidence.sourceAgentID = parent.Handoff.RecipientAgentID
		evidence.sourceBehaviorID = parent.Handoff.RecipientBehaviorRevisionID
		evidence.sourceBindingID = parent.Handoff.RecipientBindingRevisionID
		evidence.parentHandoffID = parent.Handoff.ID
		evidence.parentStageAuthority = &parentAuthority
		evidence.rootDelegationGrant = &root
		evidence.groupTurnID = parent.Handoff.GroupTurnID
		evidence.maxAgentMessages = parent.Handoff.MaxAgentMessages
		evidence.maxDepth = parent.Handoff.MaxDepth
		evidence.depth = parent.Handoff.Depth + 1
		deadline := parent.Handoff.Deadline
		evidence.deadline = &deadline
		evidence.ancestorAgentIDs = append(append([]string{}, parent.Handoff.AncestorAgentIDs...),
			parent.Handoff.RecipientAgentID)
		return evidence, nil
	}
	if source.authorAgentID == "" {
		return evidence, nil
	}
	if source.targetID == "" || source.targetAgentID != source.authorAgentID ||
		strings.TrimSpace(source.targetBehaviorID) == "" || strings.TrimSpace(source.targetBindingID) == "" {
		return humanHandoffCausalEvidence{}, fmt.Errorf("Agent source message lacks immutable target revision evidence")
	}
	evidence.sourceAgentID = source.targetAgentID
	evidence.sourceBehaviorID = source.targetBehaviorID
	evidence.sourceBindingID = source.targetBindingID
	evidence.ancestorAgentIDs = []string{source.targetAgentID}
	return evidence, nil
}

func loadPostgresHumanHandoffSource(ctx context.Context, tx transaction, cipher collaborationBodyCipher,
	accountID, conversationID string, messageID int64) (postgresHumanHandoffSource, humanHandoffCausalEvidence, error) {
	source := postgresHumanHandoffSource{messageID: messageID}
	var foundConversationID string
	if err := tx.queryRow(ctx, `select conversation_id,coalesce(turn_id,''),coalesce(target_id,''),
  coalesce(handoff_id,''),coalesce(author_agent_id,'')
from fort_private.conversation_message
where account_id=$1 and message_id=$2`, accountID, messageID).scan(&foundConversationID,
		&source.turnID, &source.targetID, &source.handoffID, &source.authorAgentID); err != nil {
		if isNoRows(err) {
			return postgresHumanHandoffSource{}, humanHandoffCausalEvidence{},
				fmt.Errorf("%w: source message %q", ledger.ErrNotFound, strconv.FormatInt(messageID, 10))
		}
		return postgresHumanHandoffSource{}, humanHandoffCausalEvidence{}, err
	}
	if foundConversationID != conversationID {
		return postgresHumanHandoffSource{}, humanHandoffCausalEvidence{},
			fmt.Errorf("Handoff source message does not belong to its persisted source Conversation")
	}
	if source.handoffID != "" {
		parent, err := getPostgresHandoff(ctx, tx, cipher, accountID, source.handoffID)
		if err != nil {
			return postgresHumanHandoffSource{}, humanHandoffCausalEvidence{}, err
		}
		source.parent = &parent
	} else if source.authorAgentID != "" {
		if err := tx.queryRow(ctx, `select binding.agent_id,binding.behavior_revision_id,binding.binding_revision_id
from fort_private.conversation_target_binding as binding
where binding.account_id=$1 and binding.target_id=$2 and binding.agent_id=$3`, accountID,
			source.targetID, source.authorAgentID).scan(&source.targetAgentID, &source.targetBehaviorID,
			&source.targetBindingID); err != nil {
			if isNoRows(err) {
				return postgresHumanHandoffSource{}, humanHandoffCausalEvidence{},
					fmt.Errorf("Agent source message lacks immutable target revision evidence")
			}
			return postgresHumanHandoffSource{}, humanHandoffCausalEvidence{}, err
		}
	}
	evidence, err := deriveHumanHandoffCausalEvidence(source)
	return source, evidence, err
}

func cloneAuthorityGrant(grant conversation.AuthorityGrant) conversation.AuthorityGrant {
	grant.Permissions = append([]string{}, grant.Permissions...)
	grant.ContextRecordIDs = append([]string{}, grant.ContextRecordIDs...)
	return grant
}
