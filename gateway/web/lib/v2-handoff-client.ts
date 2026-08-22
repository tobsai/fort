import "server-only";

import type { OwnerSession } from "@/lib/v2-events";
import {
  createFortControlServiceClientFromEnvironment,
  type FortControlServiceClient,
} from "@/lib/v2-service-client";

export type HandoffStateWire = "queued" | "needs_you" | "working" | "completed" | "failed" | "canceled";

export interface HandoffRecordWire {
  handoff: {
    id: string;
    account_id: string;
    state: HandoffStateWire;
    created_by_kind: "human" | "agent";
    created_by_id: string;
    group_turn_id?: string;
    source_message_id: string;
    source_agent_id?: string;
    recipient_agent_id: string;
    recipient_behavior_revision_id: string;
    recipient_binding_revision_id: string;
    source_conversation_id: string;
    output_conversation_id: string;
    requested_result: string;
    max_agent_messages: 10;
    max_depth: 3;
    depth: number;
    deadline: string;
    created_at: string;
  };
  target: {
    id: string;
    handoff_id: string;
    conversation_id: string;
    agent_id: string;
    behavior_revision_id: string;
    binding_revision_id: string;
    participant_id: string;
    state: "queued" | "working" | "answered" | "failed" | "canceled";
    created_at: string;
  };
  cancellation?: { handoff_id: string; target_id: string; state: "requested" | "canceled" };
  projections: Array<{
    handoff_id: string;
    conversation_id: string;
    output_conversation_id: string;
    authoritative_message_id: string;
    state: HandoffStateWire;
  }>;
  result?: {
    handoff_id: string;
    output_conversation_id: string;
    message_id: string;
    body: string;
  };
}

export interface FortControlHandoffClient {
  list(input: { owner: OwnerSession }): Promise<HandoffRecordWire[]>;
  read(input: { owner: OwnerSession; handoffID: string }): Promise<HandoffRecordWire>;
}

export function createFortControlHandoffClient(service: FortControlServiceClient): FortControlHandoffClient {
  return {
    async list({ owner }) {
      let payload: unknown;
      try {
        payload = await service.request({ owner, path: "/api/v2/handoffs", routeClass: "owner.handoffs.list", method: "GET" });
      } catch {
        throw readFailed();
      }
      if (!Array.isArray(payload)) throw readFailed();
      return payload.map(parseHandoff);
    },
    async read({ owner, handoffID }) {
      let payload: unknown;
      try {
        payload = await service.request({
          owner,
          path: `/api/v2/handoffs/${pathComponent(handoffID)}`,
          routeClass: "owner.handoffs.read",
          method: "GET",
        });
      } catch {
        throw readFailed();
      }
      const record = parseHandoff(payload);
      if (record.handoff.id !== handoffID) throw readFailed();
      return record;
    },
  };
}

export function createFortControlHandoffClientFromEnvironment(): FortControlHandoffClient {
  return createFortControlHandoffClient(createFortControlServiceClientFromEnvironment());
}

function parseHandoff(value: unknown): HandoffRecordWire {
  if (!isRecord(value) || !isRecord(value.handoff) || !isRecord(value.target) || !Array.isArray(value.projections)) {
    throw readFailed();
  }
  const handoff = value.handoff;
  const target = value.target;
  if (!nonempty(handoff.id) || !nonempty(handoff.account_id) || !handoffState(handoff.state) ||
      (handoff.created_by_kind !== "human" && handoff.created_by_kind !== "agent") || !nonempty(handoff.created_by_id) ||
      !nonempty(handoff.source_message_id) || !nonempty(handoff.recipient_agent_id) ||
      !nonempty(handoff.recipient_behavior_revision_id) || !nonempty(handoff.recipient_binding_revision_id) ||
      !nonempty(handoff.source_conversation_id) || !nonempty(handoff.output_conversation_id) ||
      !nonempty(handoff.requested_result) || handoff.max_agent_messages !== 10 || handoff.max_depth !== 3 ||
      !Number.isSafeInteger(handoff.depth) || Number(handoff.depth) < 1 || Number(handoff.depth) > 3 ||
      !timestamp(handoff.deadline) || !timestamp(handoff.created_at) ||
      (handoff.group_turn_id !== undefined && !nonempty(handoff.group_turn_id)) ||
      (handoff.source_agent_id !== undefined && !nonempty(handoff.source_agent_id))) throw readFailed();
  if (handoff.created_by_kind === "agent" && handoff.source_agent_id !== handoff.created_by_id) throw readFailed();
  if (handoff.group_turn_id !== undefined && handoff.output_conversation_id !== handoff.source_conversation_id) throw readFailed();
  if (!nonempty(target.id) || target.handoff_id !== handoff.id || target.conversation_id !== handoff.output_conversation_id ||
      target.agent_id !== handoff.recipient_agent_id || target.behavior_revision_id !== handoff.recipient_behavior_revision_id ||
      target.binding_revision_id !== handoff.recipient_binding_revision_id || !nonempty(target.participant_id) ||
      !["queued", "working", "answered", "failed", "canceled"].includes(String(target.state)) || !timestamp(target.created_at)) {
    throw readFailed();
  }
  for (const projection of value.projections as unknown[]) {
    if (!isRecord(projection) || projection.handoff_id !== handoff.id || !nonempty(projection.conversation_id) ||
        projection.output_conversation_id !== handoff.output_conversation_id || !handoffState(projection.state) ||
        typeof projection.authoritative_message_id !== "string" || projection.conversation_id === handoff.output_conversation_id) {
      throw readFailed();
    }
  }
  if (value.result !== undefined) {
    if (!isRecord(value.result) || value.result.handoff_id !== handoff.id ||
        value.result.output_conversation_id !== handoff.output_conversation_id || !nonempty(value.result.message_id) ||
        typeof value.result.body !== "string" || handoff.state !== "completed") throw readFailed();
  } else if (handoff.state === "completed") {
    throw readFailed();
  }
  if (value.cancellation !== undefined && (!isRecord(value.cancellation) || value.cancellation.handoff_id !== handoff.id ||
      value.cancellation.target_id !== target.id || !["requested", "canceled"].includes(String(value.cancellation.state)))) {
    throw readFailed();
  }
  return value as unknown as HandoffRecordWire;
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

function nonempty(value: unknown): value is string { return typeof value === "string" && value.trim().length > 0; }
function timestamp(value: unknown): value is string { return nonempty(value) && Number.isFinite(Date.parse(value)); }
function handoffState(value: unknown): value is HandoffStateWire {
  return ["queued", "needs_you", "working", "completed", "failed", "canceled"].includes(String(value));
}
function readFailed(): Error { return new Error("fort-control Handoff read failed"); }
