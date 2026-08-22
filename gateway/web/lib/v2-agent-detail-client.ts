import "server-only";

import { parseAgentRecordWire, type AgentRecordWire } from "@/lib/v2-agent-client";
import type { OwnerSession } from "@/lib/v2-events";
import type { FortControlServiceClient } from "@/lib/v2-service-client";

export interface AgentConversationWire {
  conversation: {
    id: string;
    title: string;
    state: "open" | "archived";
  };
  link: {
    agent_id: string;
    conversation_id: string;
    kind: "canonical" | "secondary";
  };
  pinned: boolean;
  pinned_at?: string;
}

export type AgentDetailWire = AgentRecordWire & {
  agent: AgentRecordWire["agent"] & {
    current_profile_revision_id: string;
    current_behavior_revision_id: string;
    current_binding_revision_id: string;
  };
  profile: AgentRecordWire["profile"] & {
    id: string;
    agent_id: string;
    revision: number;
    hidden: boolean;
    pinned: boolean;
    sort_order: number;
  };
  behavior: {
    id: string;
    agent_id: string;
    revision: number;
    role: string;
    standing_instructions?: string;
    enabled_skills: string[];
    enabled_tools: string[];
    prompt_material?: string;
  };
  binding: AgentRecordWire["binding"] & {
    id: string;
    agent_id: string;
    behavior_revision_id: string;
  };
};

export interface FortControlAgentDetailClient {
  get(input: { owner: OwnerSession; agentID: string }): Promise<AgentDetailWire>;
  listConversations(input: { owner: OwnerSession; agentID: string }): Promise<AgentConversationWire[]>;
}

export function createFortControlAgentDetailClient(
  service: FortControlServiceClient,
): FortControlAgentDetailClient {
  return {
    async get({ owner, agentID }) {
      try {
        const payload = await service.request({
          owner,
          path: agentPath(agentID),
          routeClass: "owner.agents.read",
          method: "GET",
        });
        return parseAgentDetail(payload, agentID);
      } catch {
        throw readFailed();
      }
    },

    async listConversations({ owner, agentID }) {
      let payload: unknown;
      try {
        payload = await service.request({
          owner,
          path: `${agentPath(agentID)}/conversations`,
          routeClass: "owner.agent_conversations.list",
          method: "GET",
        });
      } catch {
        throw readFailed();
      }
      if (!Array.isArray(payload)) throw readFailed();
      return payload.map((item) => parseConversation(item, agentID));
    },
  };
}

function parseAgentDetail(value: unknown, agentID: string): AgentDetailWire {
  const record = parseAgentRecordWire(value);
  if (!isRecord(value) || !isRecord(value.agent) || !isRecord(value.profile) ||
      !isRecord(value.behavior) || !isRecord(value.binding) ||
      record.agent.id !== agentID || !nonempty(value.agent.current_profile_revision_id) ||
      !nonempty(value.agent.current_behavior_revision_id) || !nonempty(value.agent.current_binding_revision_id) ||
      !nonempty(value.profile.id) || value.profile.agent_id !== agentID || !positiveInteger(value.profile.revision) ||
      typeof value.profile.hidden !== "boolean" || typeof value.profile.pinned !== "boolean" ||
      !Number.isSafeInteger(value.profile.sort_order) || !optionalText(value.profile.title) || !optionalText(value.profile.avatar_url) ||
      value.profile.id !== value.agent.current_profile_revision_id ||
      !nonempty(value.behavior.id) || value.behavior.agent_id !== agentID || !positiveInteger(value.behavior.revision) ||
      !nonempty(value.behavior.role) || !optionalText(value.behavior.standing_instructions) ||
      !stringList(value.behavior.enabled_skills) || !stringList(value.behavior.enabled_tools) ||
      !optionalText(value.behavior.prompt_material) || value.behavior.id !== value.agent.current_behavior_revision_id ||
      !nonempty(value.binding.id) || value.binding.agent_id !== agentID ||
      value.binding.id !== value.agent.current_binding_revision_id ||
      value.binding.behavior_revision_id !== value.behavior.id || record.home.id !== record.agent.canonical_conversation_id) {
    throw readFailed();
  }
  return value as unknown as AgentDetailWire;
}

function agentPath(agentID: string): string {
  const normalized = agentID.trim();
  if (!normalized || normalized.length > 512 || /[\r\n\0]/.test(normalized)) throw readFailed();
  return `/api/v2/agents/${encodeURIComponent(normalized)}`;
}

function parseConversation(value: unknown, agentID: string): AgentConversationWire {
  if (!isRecord(value) || !isRecord(value.conversation) || !isRecord(value.link)) throw readFailed();
  const { conversation, link } = value;
  if (
    !nonempty(conversation.id) || !nonempty(conversation.title) ||
    (conversation.state !== "open" && conversation.state !== "archived") ||
    link.agent_id !== agentID || link.conversation_id !== conversation.id ||
    (link.kind !== "canonical" && link.kind !== "secondary") ||
    typeof value.pinned !== "boolean" || (value.pinned && !nonempty(value.pinned_at)) ||
    (link.kind === "canonical" && value.pinned)
  ) throw readFailed();
  return value as unknown as AgentConversationWire;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function nonempty(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function optionalText(value: unknown): boolean { return value === undefined || typeof value === "string"; }
function positiveInteger(value: unknown): boolean { return Number.isSafeInteger(value) && Number(value) > 0; }
function stringList(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => nonempty(item));
}

function readFailed(): Error { return new Error("fort-control Agent detail read failed"); }
