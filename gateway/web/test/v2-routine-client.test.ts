import { describe, expect, it, vi } from "vitest";

import {
  createFortControlRoutineClient,
  createRoutineCommandClient,
} from "@/lib/v2-routine-client";
import type { FortControlServiceClient } from "@/lib/v2-service-client";

const owner = { normalizedEmail: "owner@example.com" };
type Fetcher = (input: string | URL | Request, init?: RequestInit) => Promise<Response>;

function routineRecord(overrides: Record<string, unknown> = {}) {
  return {
    routine: {
      id: "routine:weekly",
      account_id: "account:owner",
      agent_id: "agent:researcher",
      current_revision_id: "routine-revision:weekly:1",
      state: "active",
      created_at: "2026-08-21T20:00:00Z",
    },
    current_revision: {
      id: "routine-revision:weekly:1",
      routine_id: "routine:weekly",
      revision: 1,
      agent_id: "agent:researcher",
      behavior_revision_id: "behavior:researcher:3",
      binding_revision_id: "binding:researcher:2",
      authority: "fort_cloud",
      trigger: "schedule",
      schedule: "0 0 9 * * 1",
      timezone: "America/Chicago",
      next_occurrence: "2026-08-24T14:00:00Z",
      input_source: "fort:conversation:research",
      freshness_seconds: 86_400,
      expected_result: "Weekly brief",
      result_conversation_id: "conversation:home",
      approval_boundary: "before_external_side_effect",
      missing_input_behavior: "needs_you",
      retry_policy: "once",
      catch_up_policy: "skip",
      lateness_policy: "within_90s",
      created_at: "2026-08-21T20:00:00Z",
    },
    ...overrides,
  };
}

function routineRunRecord() {
  return {
    occurrence: {
      id: "occurrence:weekly:1",
      account_id: "account:owner",
      routine_id: "routine:weekly",
      routine_revision_id: "routine-revision:weekly:1",
      kind: "scheduled",
      state: "succeeded",
      scheduled_for: "2026-08-24T14:00:00Z",
      idempotency_key: "routine:weekly@2026-08-24T14:00:00Z",
      approval_evidence_id: "approval:weekly:1",
      created_at: "2026-08-24T14:00:10Z",
      updated_at: "2026-08-24T14:00:30Z",
    },
    run: {
      id: "run:weekly:1",
      routine_id: "routine:weekly",
      routine_revision_id: "routine-revision:weekly:1",
      agent_id: "agent:researcher",
      behavior_revision_id: "behavior:researcher:3",
      binding_revision_id: "binding:researcher:2",
      occurrence_id: "occurrence:weekly:1",
      kind: "scheduled",
      state: "succeeded",
      normalized_result: "Weekly brief",
      result_message_id: "81",
      created_at: "2026-08-24T14:00:10Z",
    },
    result_conversation_id: "conversation:home",
    attempt_id: "attempt:weekly:1",
    lease_id: "lease:weekly:1",
    lease_expires_at: "2026-08-24T14:05:00Z",
    activities: [{
      sequence: 1,
      state: "succeeded",
      attempt_id: "attempt:weekly:1",
      lease_id: "lease:weekly:1",
      activity: "worker completed Routine run",
      created_at: "2026-08-24T14:00:30Z",
    }],
  };
}

describe("fort-control Routine client", () => {
  it("lists only exact fort_cloud Routines for the Agent over the signed service seam", async () => {
    const request = vi.fn(async () => [routineRecord()]);
    const client = createFortControlRoutineClient({ request } as FortControlServiceClient);

    const records = await client.list({ owner, agentID: "agent:researcher" });

    expect(records[0]?.current_revision).toMatchObject({
      authority: "fort_cloud",
      behavior_revision_id: "behavior:researcher:3",
      binding_revision_id: "binding:researcher:2",
      result_conversation_id: "conversation:home",
    });
    expect(request).toHaveBeenCalledWith({
      owner,
      path: "/api/v2/agents/agent%3Aresearcher/routines",
      routeClass: "owner.routines.list",
      method: "GET",
    });
  });

  it("rejects a Routine whose authority or Agent parent is not exact", async () => {
    for (const record of [
      routineRecord({ current_revision: { ...routineRecord().current_revision, authority: "source_native" } }),
      routineRecord({ routine: { ...routineRecord().routine, agent_id: "agent:other" } }),
    ]) {
      const service = { request: async () => [record] } as FortControlServiceClient;
      await expect(createFortControlRoutineClient(service).list({ owner, agentID: "agent:researcher" }))
        .rejects.toThrow("fort-control Routine read failed");
    }
  });

  it("lists exact durable run history for one Routine", async () => {
    const request = vi.fn(async () => [routineRunRecord()]);
    const client = createFortControlRoutineClient({ request } as FortControlServiceClient);

    const runs = await client.listRuns({
      owner,
      agentID: "agent:researcher",
      routineID: "routine:weekly",
    });

    expect(runs[0]).toMatchObject({
      run: { id: "run:weekly:1", state: "succeeded", normalized_result: "Weekly brief" },
      activities: [{ activity: "worker completed Routine run" }],
    });
    expect(request).toHaveBeenCalledWith({
      owner,
      path: "/api/v2/agents/agent%3Aresearcher/routines/routine%3Aweekly/runs",
      routeClass: "owner.routines.runs",
      method: "GET",
    });
  });

  it("creates with only approved Routine semantics and client idempotency", async () => {
    const fetcher = vi.fn<Fetcher>(async () => new Response(JSON.stringify(routineRecord()), {
      status: 201,
      headers: { "content-type": "application/json" },
    }));
    const client = createRoutineCommandClient(fetcher);

    await client.create({
      agentID: "agent:researcher",
      idempotencyKey: "routine:create:one",
      trigger: "schedule",
      schedule: "0 0 9 * * 1",
      timezone: "America/Chicago",
      nextOccurrence: "2026-08-24T14:00:00Z",
      inputSource: "fort:conversation:research",
      freshnessSeconds: 86_400,
      expectedResult: "Weekly brief",
      resultConversationID: "conversation:home",
      approvalBoundary: "before_external_side_effect",
      missingInputBehavior: "needs_you",
      retryPolicy: "once",
      catchUpPolicy: "skip",
      latenessPolicy: "within_90s",
      provider: "forged",
      model: "forged",
      machine: "forged",
      authority: "source_native",
    } as Parameters<typeof client.create>[0] & Record<"provider" | "model" | "machine" | "authority", string>);

    const [path, init] = fetcher.mock.calls[0]!;
    expect(path).toBe("/api/v2/agents/agent%3Aresearcher/routines");
    expect(init).toMatchObject({ method: "POST", headers: { "content-type": "application/json" } });
    expect(JSON.parse(String(init?.body))).toEqual({
      idempotency_key: "routine:create:one",
      trigger: "schedule",
      schedule: "0 0 9 * * 1",
      timezone: "America/Chicago",
      next_occurrence: "2026-08-24T14:00:00Z",
      input_source: "fort:conversation:research",
      freshness_seconds: 86_400,
      expected_result: "Weekly brief",
      result_conversation_id: "conversation:home",
      approval_boundary: "before_external_side_effect",
      missing_input_behavior: "needs_you",
      retry_policy: "once",
      catch_up_policy: "skip",
      lateness_policy: "within_90s",
    });
  });

  it("rejects a five-field scheduled Routine before network access", async () => {
    const fetcher = vi.fn<Fetcher>();
    const client = createRoutineCommandClient(fetcher);
    await expect(client.create({
      agentID: "agent:researcher",
      idempotencyKey: "routine:create:bad-cron",
      trigger: "schedule",
      schedule: "0 9 * * 1",
      timezone: "America/Chicago",
      nextOccurrence: "2026-08-24T14:00:00Z",
      inputSource: "none",
      freshnessSeconds: 86_400,
      expectedResult: "Weekly brief",
      resultConversationID: "conversation:home",
      approvalBoundary: "none",
      missingInputBehavior: "needs_you",
      retryPolicy: "once",
      catchUpPolicy: "skip",
      latenessPolicy: "within_90s",
    })).rejects.toThrow("routine_command_failed");
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("revalidates and queues Test Routine with bounded command bodies", async () => {
    const fetcher = vi.fn<Fetcher>(async (input: string | URL | Request) => {
      const path = String(input);
      if (path.endsWith("/test")) {
        return new Response(JSON.stringify({
          run: { routine_id: "routine:weekly", kind: "test", state: "queued" },
        }), { status: 202, headers: { "content-type": "application/json" } });
      }
      return new Response(JSON.stringify(routineRecord({
        routine: { ...routineRecord().routine, current_revision_id: "routine-revision:weekly:2", state: "active" },
        current_revision: { ...routineRecord().current_revision, id: "routine-revision:weekly:2", revision: 2 },
      })), { status: 200, headers: { "content-type": "application/json" } });
    });
    const client = createRoutineCommandClient(fetcher);

    await client.revalidate({
      agentID: "agent:researcher",
      routineID: "routine:weekly",
      idempotencyKey: "routine:revalidate:two",
    });
    await client.test({
      agentID: "agent:researcher",
      routineID: "routine:weekly",
      idempotencyKey: "routine:test:one",
    });

    expect(JSON.parse(String(fetcher.mock.calls[0]?.[1]?.body))).toEqual({
      idempotency_key: "routine:revalidate:two",
      action: "revalidate",
    });
    expect(JSON.parse(String(fetcher.mock.calls[1]?.[1]?.body))).toEqual({
      idempotency_key: "routine:test:one",
    });
  });
});
