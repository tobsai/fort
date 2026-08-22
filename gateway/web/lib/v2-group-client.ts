import "server-only";

import type { OwnerSession } from "@/lib/v2-events";
import {
  createFortControlServiceClientFromEnvironment,
  type FortControlServiceClient,
} from "@/lib/v2-service-client";

export interface GroupRecordWire {
  group: {
    id: string;
    account_id: string;
    conversation_id: string;
    state: "open" | "archived";
    current_membership_revision_id: string;
    created_at: string;
  };
  conversation: {
    id: string;
    title: string;
    state: "open" | "archived";
  };
  membership: {
    id: string;
    group_id: string;
    revision: number;
    members: Array<{ agent_id: string; position: number }>;
  };
  member_bindings: Array<{
    agent_id: string;
    behavior_revision_id: string;
    binding_revision_id: string;
    participant_id: string;
  }>;
}

export interface GroupTurnWire {
  message: {
    id: number;
    conversation_id: string;
    turn_id: string;
    author_kind: "human";
    author_id: string;
    body: string;
    created_at: string;
  };
  envelope: {
    id: string;
    group_id: string;
    conversation_id: string;
    client_turn_id: string;
    membership_revision_id: string;
    selection: "explicit" | "everyone";
    concurrency_policy: "sequential" | "concurrent";
    max_agent_messages: 10;
    max_handoff_depth: 3;
    deadline: string;
    created_at: string;
  };
  recipients: GroupRecordWire["member_bindings"];
  initial_targets: Array<GroupRecordWire["member_bindings"][number] & {
    id: string;
    group_turn_id: string;
    wave: 0;
    state: "queued" | "working" | "answered" | "failed" | "canceled";
    created_at: string;
  }>;
}

export interface GroupMessageWire {
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

export interface GroupProjectionWire {
  group: GroupRecordWire;
  turns: GroupTurnWire[];
  messages: GroupMessageWire[];
}

export interface FortControlGroupClient {
  list(input: { owner: OwnerSession }): Promise<GroupRecordWire[]>;
  read(input: { owner: OwnerSession; groupID: string }): Promise<GroupProjectionWire>;
}

export function createFortControlGroupClient(service: FortControlServiceClient): FortControlGroupClient {
  return {
    async list({ owner }) {
      let payload: unknown;
      try {
        payload = await service.request({
          owner,
          path: "/api/v2/groups?state=open",
          routeClass: "owner.groups.list",
          method: "GET",
        });
      } catch {
        throw readFailed();
      }
      if (!Array.isArray(payload)) throw readFailed();
      return payload.map(parseGroup);
    },
    async read({ owner, groupID }) {
      let payload: unknown;
      try {
        payload = await service.request({
          owner,
          path: `/api/v2/groups/${pathComponent(groupID)}`,
          routeClass: "owner.groups.read",
          method: "GET",
        });
      } catch {
        throw readFailed();
      }
      if (!isRecord(payload) || !Array.isArray(payload.turns) || !Array.isArray(payload.messages)) throw readFailed();
      const group = parseGroup(payload.group);
      if (group.group.id !== groupID) throw readFailed();
      let previousMessageID = 0;
      const messages = payload.messages.map((message) => {
        const parsed = parseMessage(message, group);
        if (parsed.id <= previousMessageID) throw readFailed();
        previousMessageID = parsed.id;
        return parsed;
      });
      return { group, turns: payload.turns.map((turn) => parseTurn(turn, group)), messages };
    },
  };
}

function parseMessage(value: unknown, group: GroupRecordWire): GroupMessageWire {
  if (!isRecord(value) || !positiveInteger(value.id) || value.conversation_id !== group.group.conversation_id ||
      !["human", "agent", "system"].includes(String(value.author_kind)) || !nonempty(value.author_id) ||
      typeof value.body !== "string" || !timestamp(value.created_at) ||
      (value.turn_id !== undefined && !nonempty(value.turn_id)) ||
      (value.target_id !== undefined && !nonempty(value.target_id))) throw readFailed();

  if (value.author_kind === "agent") {
    if (!nonempty(value.author_agent_id)) throw readFailed();
  } else if (value.author_agent_id !== undefined) {
    throw readFailed();
  }
  return value as unknown as GroupMessageWire;
}

export function createFortControlGroupClientFromEnvironment(): FortControlGroupClient {
  return createFortControlGroupClient(createFortControlServiceClientFromEnvironment());
}

function parseGroup(value: unknown): GroupRecordWire {
  if (!isRecord(value) || !isRecord(value.group) || !isRecord(value.conversation) ||
      !isRecord(value.membership) || !Array.isArray(value.membership.members) ||
      !Array.isArray(value.member_bindings)) throw readFailed();
  const { group, conversation, membership } = value;
  const members = membership.members as unknown[];
  if (
    !nonempty(group.id) || !nonempty(group.account_id) || !nonempty(group.conversation_id) ||
    (group.state !== "open" && group.state !== "archived") ||
    !nonempty(group.current_membership_revision_id) || !nonempty(group.created_at) ||
    !nonempty(conversation.id) || !nonempty(conversation.title) || conversation.id !== group.conversation_id ||
    conversation.state !== group.state || !nonempty(membership.id) || membership.id !== group.current_membership_revision_id ||
    !nonempty(membership.group_id) || membership.group_id !== group.id ||
    !Number.isSafeInteger(membership.revision) || Number(membership.revision) < 1 ||
    members.length < 2 || members.length > 6
  ) throw readFailed();
  const seen = new Set<string>();
  for (const [position, member] of members.entries()) {
    if (!isRecord(member) || !nonempty(member.agent_id) || member.position !== position || seen.has(member.agent_id)) {
      throw readFailed();
    }
    seen.add(member.agent_id);
  }
  const bindings = value.member_bindings as unknown[];
  if (bindings.length !== members.length) throw readFailed();
  for (const [position, binding] of bindings.entries()) {
    const member = members[position];
    if (!isRecord(binding) || !isRecord(member) || binding.agent_id !== member.agent_id ||
        !nonempty(binding.behavior_revision_id) || !nonempty(binding.binding_revision_id) ||
        !nonempty(binding.participant_id)) throw readFailed();
  }
  return value as unknown as GroupRecordWire;
}

function parseTurn(value: unknown, group: GroupRecordWire): GroupTurnWire {
  if (!isRecord(value) || !isRecord(value.message) || !isRecord(value.envelope) ||
      !Array.isArray(value.recipients) || !Array.isArray(value.initial_targets)) throw readFailed();
  const message = value.message;
  const envelope = value.envelope;
  if (!positiveInteger(message.id) || message.conversation_id !== group.group.conversation_id ||
      !nonempty(message.turn_id) || message.author_kind !== "human" || !nonempty(message.author_id) ||
      typeof message.body !== "string" || !timestamp(message.created_at) ||
      envelope.id !== message.turn_id || envelope.group_id !== group.group.id ||
      envelope.conversation_id !== group.group.conversation_id || !nonempty(envelope.client_turn_id) ||
      !nonempty(envelope.membership_revision_id) ||
      (envelope.selection !== "explicit" && envelope.selection !== "everyone") ||
      (envelope.concurrency_policy !== "sequential" && envelope.concurrency_policy !== "concurrent") ||
      envelope.max_agent_messages !== 10 || envelope.max_handoff_depth !== 3 ||
      !timestamp(envelope.deadline) || !timestamp(envelope.created_at) ||
      value.recipients.length === 0 || value.recipients.length !== value.initial_targets.length) throw readFailed();
  const seenRecipients = new Set<string>();
  for (const [position, rawRecipient] of value.recipients.entries()) {
    const recipient = parseBinding(rawRecipient);
    if (seenRecipients.has(recipient.agent_id)) throw readFailed();
    seenRecipients.add(recipient.agent_id);
    const target = value.initial_targets[position];
    if (!isRecord(target) || !nonempty(target.id) || target.group_turn_id !== envelope.id || target.wave !== 0 ||
        !sameBinding(parseBinding(target), recipient) ||
        !["queued", "working", "answered", "failed", "canceled"].includes(String(target.state)) ||
        !timestamp(target.created_at)) throw readFailed();
  }
  return value as unknown as GroupTurnWire;
}

function parseBinding(value: unknown): GroupRecordWire["member_bindings"][number] {
  if (!isRecord(value) || !nonempty(value.agent_id) || !nonempty(value.behavior_revision_id) ||
      !nonempty(value.binding_revision_id) || !nonempty(value.participant_id)) throw readFailed();
  return value as unknown as GroupRecordWire["member_bindings"][number];
}

function sameBinding(
  left: GroupRecordWire["member_bindings"][number],
  right: GroupRecordWire["member_bindings"][number],
): boolean {
  return left.agent_id === right.agent_id && left.behavior_revision_id === right.behavior_revision_id &&
    left.binding_revision_id === right.binding_revision_id && left.participant_id === right.participant_id;
}

function pathComponent(value: string): string {
  if (!nonempty(value) || value !== value.trim() || new TextEncoder().encode(value).byteLength > 512 || /[\/\\\r\n\0]/.test(value)) {
    throw readFailed();
  }
  return encodeURIComponent(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function nonempty(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function positiveInteger(value: unknown): value is number { return Number.isSafeInteger(value) && Number(value) > 0; }
function timestamp(value: unknown): value is string { return nonempty(value) && Number.isFinite(Date.parse(value)); }

function readFailed(): Error { return new Error("fort-control Group read failed"); }
