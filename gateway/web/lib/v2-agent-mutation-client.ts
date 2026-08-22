export interface AgentProfileMutation {
  action: "profile";
  agentID: string;
  idempotencyKey: string;
  expectedProfileRevisionID: string;
  profile: {
    name: string;
    title: string;
    avatarURL: string;
    hidden: boolean;
    pinned: boolean;
    sortOrder: number;
  };
}

export interface AgentBehaviorMutation {
  action: "behavior";
  agentID: string;
  idempotencyKey: string;
  expectedBehaviorRevisionID: string;
  expectedBindingRevisionID: string;
  behavior: {
    role: string;
    standingInstructions: string;
    enabledSkills: string[];
    enabledTools: string[];
    promptMaterial: string;
  };
}

export type AgentMutation = AgentProfileMutation | AgentBehaviorMutation;

export interface AgentMutationClient {
  update(input: AgentMutation): Promise<void>;
}

type Fetcher = (input: string | URL | Request, init?: RequestInit) => Promise<Response>;

export function createAgentMutationClient(fetcher: Fetcher = fetch): AgentMutationClient {
  return {
    async update(input) {
      if (!validIdentity(input.agentID) || !validIdempotencyKey(input.idempotencyKey) || !validMutation(input)) {
        throw mutationFailed();
      }
      const response = await fetcher(`/api/v2/agents/${encodeURIComponent(input.agentID)}`, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(mutationBody(input)),
      });
      if (!response.ok) throw mutationFailed(await semanticError(response));
      let payload: unknown;
      try {
        payload = await response.json() as unknown;
      } catch {
        throw mutationFailed();
      }
      if (responseAgentID(payload, input.action) !== input.agentID) {
        throw mutationFailed();
      }
    },
  };
}

function validMutation(input: AgentMutation): boolean {
  if (input.action === "profile") {
    return validIdentity(input.expectedProfileRevisionID) && validProfile(input.profile);
  }
  return validIdentity(input.expectedBehaviorRevisionID) && validIdentity(input.expectedBindingRevisionID) &&
    validBehavior(input.behavior);
}

function mutationBody(input: AgentMutation): Record<string, unknown> {
  if (input.action === "profile") {
    return {
      action: "profile",
      idempotency_key: input.idempotencyKey,
      expected_profile_revision_id: input.expectedProfileRevisionID,
      profile: {
        name: input.profile.name,
        title: input.profile.title,
        avatar_url: input.profile.avatarURL,
        hidden: input.profile.hidden,
        pinned: input.profile.pinned,
        sort_order: input.profile.sortOrder,
      },
    };
  }
  return {
    action: "behavior",
    idempotency_key: input.idempotencyKey,
    expected_behavior_revision_id: input.expectedBehaviorRevisionID,
    expected_binding_revision_id: input.expectedBindingRevisionID,
    behavior: {
      role: input.behavior.role,
      standing_instructions: input.behavior.standingInstructions,
      enabled_skills: [...input.behavior.enabledSkills],
      enabled_tools: [...input.behavior.enabledTools],
      prompt_material: input.behavior.promptMaterial,
    },
  };
}

function validProfile(profile: AgentProfileMutation["profile"]): boolean {
  return exactTrimmed(profile.name, 120, true) && exactTrimmed(profile.title, 512, false) &&
    exactTrimmed(profile.avatarURL, 2_048, false) && typeof profile.hidden === "boolean" &&
    typeof profile.pinned === "boolean" && Number.isSafeInteger(profile.sortOrder);
}

function validBehavior(behavior: AgentBehaviorMutation["behavior"]): boolean {
  return exactTrimmed(behavior.role, 4_096, true) &&
    encodedLength(behavior.standingInstructions) <= 100_000 && encodedLength(behavior.promptMaterial) <= 100_000 &&
    validNameList(behavior.enabledSkills) && validNameList(behavior.enabledTools);
}

function validNameList(values: string[]): boolean {
  if (!Array.isArray(values)) return false;
  const seen = new Set<string>();
  for (const value of values) {
    if (!exactTrimmed(value, 512, true) || seen.has(value)) return false;
    seen.add(value);
  }
  return true;
}

function validIdentity(value: string): boolean {
  return exactTrimmed(value, 512, true) && !/[\/\\\r\n\0]/.test(value);
}

function validIdempotencyKey(value: string): boolean { return exactTrimmed(value, 256, true); }

function exactTrimmed(value: string, maximumBytes: number, required: boolean): boolean {
  return typeof value === "string" && value === value.trim() && (!required || value.length > 0) &&
    encodedLength(value) <= maximumBytes;
}

function encodedLength(value: string): number {
  return typeof value === "string" ? new TextEncoder().encode(value).byteLength : Number.POSITIVE_INFINITY;
}

function responseAgentID(value: unknown, action: AgentMutation["action"]): unknown {
  if (!isRecord(value) || !isRecord(value.agent)) return undefined;
  if (action === "profile") return value.agent.id;
  return isRecord(value.agent.agent) ? value.agent.agent.id : undefined;
}

async function semanticError(response: Response): Promise<string> {
  try {
    const payload = await response.json() as unknown;
    if (isRecord(payload) && typeof payload.code === "string" && /^[a-z][a-z0-9_]{0,63}$/.test(payload.code)) {
      return payload.code;
    }
  } catch {
    // The owner proxy exposes only bounded machine-readable errors.
  }
  return "agent_update_failed";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function mutationFailed(code = "agent_update_failed"): Error { return new Error(code); }
