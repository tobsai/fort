import type { OwnerSession } from "@/lib/v2-events";
import type { FortControlServiceClient } from "@/lib/v2-service-client";

export interface RoutineRecordWire {
  routine: {
    id: string;
    account_id: string;
    agent_id: string;
    current_revision_id: string;
    state: "active" | "paused" | "archived";
    created_at: string;
  };
  current_revision: {
    id: string;
    routine_id: string;
    revision: number;
    agent_id: string;
    behavior_revision_id: string;
    binding_revision_id: string;
    authority: "fort_cloud";
    trigger: "schedule" | "event";
    schedule?: string;
    timezone?: string;
    next_occurrence?: string;
    input_source: string;
    freshness_seconds: number;
    expected_result: string;
    result_conversation_id: string;
    approval_boundary: string;
    missing_input_behavior: "skip" | "needs_you" | "fail";
    retry_policy: string;
    catch_up_policy: string;
    lateness_policy: string;
    created_at: string;
  };
  pause_reason?: "needs_revalidation";
}

type RoutineRunState = "queued" | "working" | "needs_you" | "succeeded" | "failed" | "canceled";

export interface RoutineRunRecordWire {
  occurrence: {
    id: string;
    account_id: string;
    routine_id: string;
    routine_revision_id: string;
    kind: "scheduled" | "test";
    state: RoutineRunState;
    scheduled_for: string;
    idempotency_key: string;
    approval_evidence_id: string;
    created_at: string;
    updated_at: string;
  };
  run: {
    id: string;
    routine_id: string;
    routine_revision_id: string;
    agent_id: string;
    behavior_revision_id: string;
    binding_revision_id: string;
    occurrence_id: string;
    kind: "scheduled" | "test";
    state: RoutineRunState;
    normalized_result?: string;
    result_message_id?: string;
    created_at: string;
  };
  result_conversation_id: string;
  attempt_id?: string;
  lease_id?: string;
  lease_expires_at?: string;
  failure_code?: string;
  next_action?: string;
  activities: Array<{
    sequence: number;
    state: RoutineRunState;
    attempt_id?: string;
    lease_id?: string;
    lease_expires_at?: string;
    activity: string;
    failure_code?: string;
    next_action?: string;
    created_at: string;
  }>;
}

export interface FortControlRoutineClient {
  list(input: { owner: OwnerSession; agentID: string }): Promise<RoutineRecordWire[]>;
  listRuns(input: { owner: OwnerSession; agentID: string; routineID: string }): Promise<RoutineRunRecordWire[]>;
}

interface RoutineCreateBase {
  agentID: string;
  idempotencyKey: string;
  inputSource: string;
  freshnessSeconds: number;
  expectedResult: string;
  resultConversationID: string;
  approvalBoundary: string;
  missingInputBehavior: "skip" | "needs_you" | "fail";
  retryPolicy: string;
  catchUpPolicy: string;
  latenessPolicy: string;
}

export type RoutineCreateCommand = RoutineCreateBase & ({
  trigger: "schedule";
  schedule: string;
  timezone: string;
  nextOccurrence: string;
} | {
  trigger: "event";
  schedule?: never;
  timezone?: never;
  nextOccurrence?: never;
});

export interface RoutineIdentityCommand {
  agentID: string;
  routineID: string;
  idempotencyKey: string;
}

export interface RoutineCommandClient {
  create(input: RoutineCreateCommand): Promise<RoutineRecordWire>;
  revalidate(input: RoutineIdentityCommand): Promise<RoutineRecordWire>;
  test(input: RoutineIdentityCommand): Promise<void>;
}

type Fetcher = (input: string | URL | Request, init?: RequestInit) => Promise<Response>;

export function createFortControlRoutineClient(service: FortControlServiceClient): FortControlRoutineClient {
  return {
    async list({ owner, agentID }) {
      let payload: unknown;
      try {
        payload = await service.request({
          owner,
          path: `${agentPath(agentID)}/routines`,
          routeClass: "owner.routines.list",
          method: "GET",
        });
      } catch {
        throw readFailed();
      }
      if (!Array.isArray(payload)) throw readFailed();
      return payload.map((record) => parseRoutineRecord(record, agentID));
    },
    async listRuns({ owner, agentID, routineID }) {
      if (!pathIdentity(routineID)) throw readFailed();
      let payload: unknown;
      try {
        payload = await service.request({
          owner,
          path: `${agentPath(agentID)}/routines/${pathComponent(routineID)}/runs`,
          routeClass: "owner.routines.runs",
          method: "GET",
        });
      } catch {
        throw readFailed();
      }
      if (!Array.isArray(payload)) throw readFailed();
      return payload.map((record) => parseRoutineRunRecord(record, agentID, routineID));
    },
  };
}

export function createRoutineCommandClient(fetcher: Fetcher = fetch): RoutineCommandClient {
  return {
    async create(input) {
      if (!validCreate(input)) throw commandFailed();
      const body: Record<string, unknown> = {
        idempotency_key: input.idempotencyKey,
        trigger: input.trigger,
        input_source: input.inputSource,
        freshness_seconds: input.freshnessSeconds,
        expected_result: input.expectedResult,
        result_conversation_id: input.resultConversationID,
        approval_boundary: input.approvalBoundary,
        missing_input_behavior: input.missingInputBehavior,
        retry_policy: input.retryPolicy,
        catch_up_policy: input.catchUpPolicy,
        lateness_policy: input.latenessPolicy,
      };
      if (input.trigger === "schedule") {
        body.schedule = input.schedule;
        body.timezone = input.timezone;
        body.next_occurrence = input.nextOccurrence;
      }
      const payload = await command(fetcher, `${agentPath(input.agentID)}/routines`, "POST", body);
      return parseRoutineRecord(payload, input.agentID);
    },

    async revalidate(input) {
      if (!validIdentityCommand(input)) throw commandFailed();
      const payload = await command(
        fetcher,
        `${agentPath(input.agentID)}/routines/${pathComponent(input.routineID)}`,
        "PATCH",
        { idempotency_key: input.idempotencyKey, action: "revalidate" },
      );
      const record = parseRoutineRecord(payload, input.agentID);
      if (record.routine.id !== input.routineID) throw commandFailed();
      return record;
    },

    async test(input) {
      if (!validIdentityCommand(input)) throw commandFailed();
      const payload = await command(
        fetcher,
        `${agentPath(input.agentID)}/routines/${pathComponent(input.routineID)}/test`,
        "POST",
        { idempotency_key: input.idempotencyKey },
      );
      if (!isRecord(payload) || !isRecord(payload.run) || payload.run.routine_id !== input.routineID ||
          payload.run.kind !== "test" || !["queued", "working", "needs_you", "succeeded", "failed", "canceled"]
            .includes(String(payload.run.state))) throw commandFailed();
    },
  };
}

async function command(
  fetcher: Fetcher,
  path: string,
  method: "POST" | "PATCH",
  body: Record<string, unknown>,
): Promise<unknown> {
  let response: Response;
  try {
    response = await fetcher(path, {
      method,
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch {
    throw commandFailed();
  }
  if (!response.ok) throw commandFailed(await semanticError(response));
  try {
    return await response.json() as unknown;
  } catch {
    throw commandFailed();
  }
}

function parseRoutineRecord(value: unknown, agentID: string): RoutineRecordWire {
  if (!isRecord(value) || !isRecord(value.routine) || !isRecord(value.current_revision)) throw readFailed();
  const routine = value.routine;
  const revision = value.current_revision;
  if (!nonempty(routine.id) || !nonempty(routine.account_id) || routine.agent_id !== agentID ||
      !nonempty(routine.current_revision_id) || !["active", "paused", "archived"].includes(String(routine.state)) ||
      !timestamp(routine.created_at) || !nonempty(revision.id) || revision.id !== routine.current_revision_id ||
      revision.routine_id !== routine.id || revision.agent_id !== agentID || !positiveInteger(revision.revision) ||
      !nonempty(revision.behavior_revision_id) || !nonempty(revision.binding_revision_id) ||
      revision.authority !== "fort_cloud" || (revision.trigger !== "schedule" && revision.trigger !== "event") ||
      !nonempty(revision.input_source) || !positiveInteger(revision.freshness_seconds) ||
      !nonempty(revision.expected_result) || !nonempty(revision.result_conversation_id) ||
      !nonempty(revision.approval_boundary) || !["skip", "needs_you", "fail"].includes(String(revision.missing_input_behavior)) ||
      !nonempty(revision.retry_policy) || !nonempty(revision.catch_up_policy) || !nonempty(revision.lateness_policy) ||
      !timestamp(revision.created_at)) throw readFailed();
  if (revision.trigger === "schedule" &&
      (!exactSixFieldCron(revision.schedule) || !nonempty(revision.timezone) ||
       revision.lateness_policy !== "within_90s" || !timestamp(revision.next_occurrence))) throw readFailed();
  if (revision.trigger === "event" && revision.lateness_policy !== "none") throw readFailed();
  if (revision.approval_boundary !== "none" && revision.approval_boundary !== "before_external_side_effect") throw readFailed();
  if (routine.state === "paused") {
    if (value.pause_reason !== "needs_revalidation") throw readFailed();
  } else if (value.pause_reason !== undefined) {
    throw readFailed();
  }
  return value as unknown as RoutineRecordWire;
}

function parseRoutineRunRecord(value: unknown, agentID: string, routineID: string): RoutineRunRecordWire {
  if (!isRecord(value) || !isRecord(value.occurrence) || !isRecord(value.run) ||
      !Array.isArray(value.activities)) throw readFailed();
  const occurrence = value.occurrence;
  const run = value.run;
  if (!nonempty(occurrence.id) || !nonempty(occurrence.account_id) || occurrence.routine_id !== routineID ||
      !nonempty(occurrence.routine_revision_id) || !routineKind(occurrence.kind) || !routineRunState(occurrence.state) ||
      !timestamp(occurrence.scheduled_for) || !nonempty(occurrence.idempotency_key) ||
      typeof occurrence.approval_evidence_id !== "string" || !timestamp(occurrence.created_at) ||
      !timestamp(occurrence.updated_at) || !nonempty(run.id) || run.routine_id !== routineID ||
      run.routine_revision_id !== occurrence.routine_revision_id || run.agent_id !== agentID ||
      !nonempty(run.behavior_revision_id) || !nonempty(run.binding_revision_id) ||
      run.occurrence_id !== occurrence.id || run.kind !== occurrence.kind || !routineRunState(run.state) ||
      run.state !== occurrence.state || !timestamp(run.created_at) || !nonempty(value.result_conversation_id)) throw readFailed();
  if ((run.state === "succeeded") !== (nonempty(run.normalized_result) && nonempty(run.result_message_id))) throw readFailed();
  for (const activity of value.activities) {
    if (!isRecord(activity) || !positiveInteger(activity.sequence) || !routineRunState(activity.state) ||
        !nonempty(activity.activity) || !timestamp(activity.created_at) ||
        (activity.lease_expires_at !== undefined && !timestamp(activity.lease_expires_at))) throw readFailed();
  }
  if (value.lease_expires_at !== undefined && !timestamp(value.lease_expires_at)) throw readFailed();
  return value as unknown as RoutineRunRecordWire;
}

function validCreate(input: RoutineCreateCommand): boolean {
  if (!pathIdentity(input.agentID) || !intent(input.idempotencyKey, 512) || !intent(input.inputSource, 4_096) ||
      !positiveInteger(input.freshnessSeconds) || input.freshnessSeconds > 365 * 24 * 60 * 60 ||
      !intent(input.expectedResult, 4_096) || !pathIdentity(input.resultConversationID) ||
      !["none", "before_external_side_effect"].includes(input.approvalBoundary) ||
      !["skip", "needs_you", "fail"].includes(input.missingInputBehavior) ||
      !intent(input.retryPolicy, 512) || !intent(input.catchUpPolicy, 512) || !intent(input.latenessPolicy, 512)) return false;
  return input.trigger === "event" ? input.latenessPolicy === "none" :
    input.latenessPolicy === "within_90s" && exactSixFieldCron(input.schedule) &&
    intent(input.timezone, 128) && timestamp(input.nextOccurrence);
}

function validIdentityCommand(input: RoutineIdentityCommand): boolean {
  return pathIdentity(input.agentID) && pathIdentity(input.routineID) && intent(input.idempotencyKey, 512);
}

function agentPath(agentID: string): string { return `/api/v2/agents/${pathComponent(agentID)}`; }

function pathComponent(value: string): string {
  if (!pathIdentity(value)) throw readFailed();
  return encodeURIComponent(value);
}

function pathIdentity(value: unknown): value is string {
  return nonempty(value) && value === value.trim() && new TextEncoder().encode(value).byteLength <= 512 &&
    !/[\/\\\r\n\0]/.test(value);
}

function intent(value: unknown, maximum: number): value is string {
  return nonempty(value) && value === value.trim() && new TextEncoder().encode(value).byteLength <= maximum &&
    !/[\r\n\0]/.test(value);
}

async function semanticError(response: Response): Promise<string> {
  try {
    const payload = await response.json() as unknown;
    if (isRecord(payload) && typeof payload.code === "string" && /^[a-z][a-z0-9_]{0,63}$/.test(payload.code)) {
      return payload.code;
    }
  } catch {
    // Only bounded machine-readable error codes cross the owner proxy.
  }
  return "routine_command_failed";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function nonempty(value: unknown): value is string { return typeof value === "string" && value.trim().length > 0; }
function positiveInteger(value: unknown): value is number { return Number.isSafeInteger(value) && Number(value) > 0; }
function routineKind(value: unknown): value is "scheduled" | "test" { return value === "scheduled" || value === "test"; }
function routineRunState(value: unknown): value is RoutineRunState {
  return ["queued", "working", "needs_you", "succeeded", "failed", "canceled"].includes(String(value));
}
function exactSixFieldCron(value: unknown): value is string {
  return typeof value === "string" && value === value.trim() && value.split(" ").length === 6 &&
    value.split(" ").every((field) => field.length > 0) && value.length <= 512;
}
function timestamp(value: unknown): value is string {
  return nonempty(value) && value === value.trim() && value.length <= 64 && Number.isFinite(Date.parse(value));
}
function readFailed(): Error { return new Error("fort-control Routine read failed"); }
function commandFailed(code = "routine_command_failed"): Error { return new Error(code); }
