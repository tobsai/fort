import "server-only";

import type { AgentConversationWire } from "@/lib/v2-agent-detail-client";
import type { OwnerSession } from "@/lib/v2-events";
import type { FortControlServiceClient } from "@/lib/v2-service-client";

export interface AgentConversationMessageWire {
  id: number;
  conversation_id: string;
  turn_id?: string;
  target_id?: string;
  author_kind: "human" | "agent" | "system";
  author_id: string;
  author_agent_id?: string;
  body: string;
  created_at: string;
}

export interface AgentConversationTurnWire {
  id: string;
  conversation_id: string;
  client_turn_id: string;
  prompt_message_id: number;
  through_message_id: number;
  membership_revision_id: string;
  context_manifest_id: string;
  state: string;
  created_at: string;
}

export interface AgentConversationTargetWire {
  id: string;
  turn_id: string;
  conversation_id: string;
  agent_id: string;
  behavior_revision_id: string;
  binding_revision_id: string;
  participant_id: string;
  run_id: string;
  state: "queued" | "working" | "answered" | "failed" | "canceled";
  attempt_count: number;
  created_at: string;
  updated_at: string;
}

export interface AgentConversationProjectionWire {
  conversation: AgentConversationWire;
  messages: AgentConversationMessageWire[];
  turns: AgentConversationTurnWire[];
  targets: AgentConversationTargetWire[];
}

export interface FortControlAgentConversationClient {
  read(input: {
    owner: OwnerSession;
    agentID: string;
    conversationID: string;
  }): Promise<AgentConversationProjectionWire>;
}

export function createFortControlAgentConversationClient(
  service: FortControlServiceClient,
): FortControlAgentConversationClient {
  return {
    async read({ owner, agentID, conversationID }) {
      let payload: unknown;
      try {
        payload = await service.request({
          owner,
          path: `/api/v2/agents/${pathComponent(agentID)}/conversations/${pathComponent(conversationID)}`,
          routeClass: "owner.agent_conversations.read",
          method: "GET",
        });
      } catch {
        throw readFailed();
      }
      return parseProjection(payload, agentID, conversationID);
    },
  };
}

function parseProjection(value: unknown, agentID: string, conversationID: string): AgentConversationProjectionWire {
  if (!isRecord(value) || !isRecord(value.conversation) || !Array.isArray(value.messages) ||
      !Array.isArray(value.turns) || !Array.isArray(value.targets)) throw readFailed();
  const record = value.conversation;
  if (!isRecord(record.conversation) || !isRecord(record.link) ||
      record.conversation.id !== conversationID || !nonempty(record.conversation.title) ||
      (record.conversation.state !== "open" && record.conversation.state !== "archived") ||
      record.link.agent_id !== agentID || record.link.conversation_id !== conversationID ||
      (record.link.kind !== "canonical" && record.link.kind !== "secondary") || typeof record.pinned !== "boolean") {
    throw readFailed();
  }

  const messages = value.messages.map((message) => parseMessage(message, conversationID));
  const turns = value.turns.map((turn) => parseTurn(turn, conversationID));
  const turnIDs = new Set(turns.map((turn) => turn.id));
  const targets = value.targets.map((target) => parseTarget(target, agentID, conversationID, turnIDs));
  if (!strictlyIncreasing(messages.map((message) => message.id))) throw readFailed();
  return { conversation: record as unknown as AgentConversationWire, messages, turns, targets };
}

function parseMessage(value: unknown, conversationID: string): AgentConversationMessageWire {
  if (!isRecord(value) || !positiveInteger(value.id) || value.conversation_id !== conversationID ||
      (value.author_kind !== "human" && value.author_kind !== "agent" && value.author_kind !== "system") ||
      !nonempty(value.author_id) || typeof value.body !== "string" || !timestamp(value.created_at) ||
      !optionalString(value.turn_id) || !optionalString(value.target_id) || !optionalString(value.author_agent_id)) {
    throw readFailed();
  }
  return value as unknown as AgentConversationMessageWire;
}

function parseTurn(value: unknown, conversationID: string): AgentConversationTurnWire {
  if (!isRecord(value) || !nonempty(value.id) || value.conversation_id !== conversationID ||
      !nonempty(value.client_turn_id) || !positiveInteger(value.prompt_message_id) ||
      !positiveInteger(value.through_message_id) || value.through_message_id < value.prompt_message_id ||
      !nonempty(value.membership_revision_id) || !nonempty(value.context_manifest_id) ||
      !nonempty(value.state) || !timestamp(value.created_at)) throw readFailed();
  return value as unknown as AgentConversationTurnWire;
}

function parseTarget(
  value: unknown,
  agentID: string,
  conversationID: string,
  turnIDs: ReadonlySet<string>,
): AgentConversationTargetWire {
  if (!isRecord(value) || !nonempty(value.id) || !nonempty(value.turn_id) || !turnIDs.has(value.turn_id) ||
      value.conversation_id !== conversationID || value.agent_id !== agentID ||
      !nonempty(value.behavior_revision_id) || !nonempty(value.binding_revision_id) ||
      !nonempty(value.participant_id) || !nonempty(value.run_id) ||
      !["queued", "working", "answered", "failed", "canceled"].includes(String(value.state)) ||
      !Number.isSafeInteger(value.attempt_count) || Number(value.attempt_count) < 0 ||
      !timestamp(value.created_at) || !timestamp(value.updated_at)) throw readFailed();
  return value as unknown as AgentConversationTargetWire;
}

function pathComponent(value: string): string {
  const normalized = value.trim();
  if (!normalized || normalized !== value || new TextEncoder().encode(value).byteLength > 512 || /[\/\\\r\n\0]/.test(value)) {
    throw readFailed();
  }
  return encodeURIComponent(value);
}

function strictlyIncreasing(values: number[]): boolean {
  return values.every((value, index) => index === 0 || value > values[index - 1]!);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function nonempty(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function optionalString(value: unknown): boolean { return value === undefined || nonempty(value); }
function positiveInteger(value: unknown): value is number { return Number.isSafeInteger(value) && Number(value) > 0; }
function timestamp(value: unknown): value is string { return nonempty(value) && Number.isFinite(Date.parse(value)); }
function readFailed(): Error { return new Error("fort-control Agent conversation read failed"); }
