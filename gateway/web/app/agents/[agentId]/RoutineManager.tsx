"use client";

import { FormEvent, useState, useTransition } from "react";
import { useRouter } from "next/navigation";

import {
  createRoutineCommandClient,
  type RoutineRecordWire,
  type RoutineRunRecordWire,
} from "@/lib/v2-routine-client";

const commandClient = createRoutineCommandClient();

interface ResultConversationOption {
  id: string;
  title: string;
  kind: "canonical" | "secondary";
}

export default function RoutineManager({
  agentID,
  routines,
  runsByRoutine,
  resultConversations,
}: {
  agentID: string;
  routines: RoutineRecordWire[];
  runsByRoutine: Record<string, RoutineRunRecordWire[]>;
  resultConversations: ResultConversationOption[];
}) {
  const router = useRouter();
  const [refreshing, startTransition] = useTransition();
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [trigger, setTrigger] = useState<"schedule" | "event">("schedule");
  const [schedule, setSchedule] = useState("");
  const [timezone, setTimezone] = useState("");
  const [nextOccurrence, setNextOccurrence] = useState("");
  const [inputSource, setInputSource] = useState("");
  const [freshnessSeconds, setFreshnessSeconds] = useState(3_600);
  const [expectedResult, setExpectedResult] = useState("");
  const [resultConversationID, setResultConversationID] = useState(resultConversations[0]?.id ?? "");
  const [approvalBoundary, setApprovalBoundary] = useState("none");
  const [missingInputBehavior, setMissingInputBehavior] = useState<"skip" | "needs_you" | "fail">("needs_you");
  const [retryPolicy, setRetryPolicy] = useState("once");
  const [catchUpPolicy, setCatchUpPolicy] = useState("skip");
  const [latenessPolicy, setLatenessPolicy] = useState("within_90s");
  const busy = pending !== null || refreshing;

  async function createRoutine(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || !canCreate()) return;
    setPending("create");
    setError(null);
    setNotice(null);
    try {
      const base = {
        agentID,
        idempotencyKey: `routine:create:${crypto.randomUUID()}`,
        inputSource: inputSource.trim(),
        freshnessSeconds,
        expectedResult: expectedResult.trim(),
        resultConversationID,
        approvalBoundary: approvalBoundary.trim(),
        missingInputBehavior,
        retryPolicy: retryPolicy.trim(),
        catchUpPolicy: catchUpPolicy.trim(),
        latenessPolicy: latenessPolicy.trim(),
      };
      if (trigger === "schedule") {
        await commandClient.create({
          ...base,
          trigger,
          schedule: schedule.trim(),
          timezone: timezone.trim(),
          nextOccurrence: nextOccurrence.trim(),
        });
      } else {
        await commandClient.create({ ...base, trigger });
      }
      setNotice("Routine created with fort_cloud scheduling authority.");
      setExpectedResult("");
      startTransition(() => router.refresh());
    } catch (cause) {
      setError(errorCode(cause));
    } finally {
      setPending(null);
    }
  }

  async function revalidate(routineID: string) {
    if (busy) return;
    setPending(`revalidate:${routineID}`);
    setError(null);
    setNotice(null);
    try {
      await commandClient.revalidate({
        agentID,
        routineID,
        idempotencyKey: `routine:revalidate:${crypto.randomUUID()}`,
      });
      setNotice("Routine revalidated against the Agent's current Behavior and Binding Revisions.");
      startTransition(() => router.refresh());
    } catch (cause) {
      setError(errorCode(cause));
    } finally {
      setPending(null);
    }
  }

  async function runTest(routineID: string) {
    if (busy) return;
    setPending(`test:${routineID}`);
    setError(null);
    setNotice(null);
    try {
      await commandClient.test({
        agentID,
        routineID,
        idempotencyKey: `routine:test:${crypto.randomUUID()}`,
      });
      setNotice("Test Routine queued through the normal occurrence path.");
      startTransition(() => router.refresh());
    } catch (cause) {
      setError(errorCode(cause));
    } finally {
      setPending(null);
    }
  }

  function canCreate(): boolean {
    return Boolean(
      resultConversationID && inputSource.trim() && expectedResult.trim() && approvalBoundary.trim() &&
      retryPolicy.trim() && catchUpPolicy.trim() && latenessPolicy.trim() &&
      Number.isSafeInteger(freshnessSeconds) && freshnessSeconds > 0 &&
      (trigger === "event" || (exactSixFieldCron(schedule.trim()) && timezone.trim() && validTimestamp(nextOccurrence.trim()))),
    );
  }

  const conversationNames = new Map(resultConversations.map((conversation) => [conversation.id, conversation.title]));

  return (
    <section className="routine-manager" aria-labelledby="routine-manager-heading">
      <header className="routine-manager-heading">
        <div>
          <span className="eyebrow">AUTOMATION</span>
          <h2 id="routine-manager-heading">Agent-owned Routines</h2>
          <p>Every first-release Routine is scheduled by <code>fort_cloud</code> and reports to one exact Conversation.</p>
        </div>
        {error ? <span className="err" role="alert">Routine command failed: {error}</span> : null}
        {notice ? <span className="routine-notice" role="status">{notice}</span> : null}
      </header>

      <div className="routine-list">
        {routines.length === 0 ? <div className="card routine-empty">No Routines yet.</div> : null}
        {routines.map((record) => {
          const revision = record.current_revision;
          const history = runsByRoutine[record.routine.id] ?? [];
          const testing = pending === `test:${record.routine.id}`;
          const revalidating = pending === `revalidate:${record.routine.id}`;
          return (
            <article className="card routine-card" key={record.routine.id}>
              <div className="routine-card-heading">
                <div>
                  <span className={`routine-state routine-state-${record.routine.state}`}>Routine state · {record.routine.state}</span>
                  <h3>{revision.expected_result}</h3>
                  <code>{record.routine.id}</code>
                </div>
                <div className="routine-actions">
                  {record.routine.state === "paused" && record.pause_reason === "needs_revalidation" ? (
                    <button className="btn" disabled={busy} onClick={() => revalidate(record.routine.id)} type="button">
                      {revalidating ? "Revalidating…" : "Revalidate"}
                    </button>
                  ) : null}
                  <button
                    className="btn btn-primary"
                    disabled={busy || record.routine.state !== "active"}
                    onClick={() => runTest(record.routine.id)}
                    type="button"
                  >
                    {testing ? "Queuing…" : "Test Routine"}
                  </button>
                </div>
              </div>
              <dl className="routine-evidence">
                <div><dt>Behavior Revision</dt><dd><code>{revision.behavior_revision_id}</code></dd></div>
                <div><dt>Binding Revision</dt><dd><code>{revision.binding_revision_id}</code></dd></div>
                <div><dt>Schedule</dt><dd>{revision.trigger === "schedule" ? revision.schedule : "Event triggered"}</dd></div>
                <div><dt>Timezone</dt><dd>{revision.trigger === "schedule" ? revision.timezone : "Not scheduled"}</dd></div>
                <div><dt>Next occurrence</dt><dd>{revision.trigger === "schedule" ? <time dateTime={revision.next_occurrence}>{revision.next_occurrence}</time> : "Awaiting event"}</dd></div>
                <div><dt>Result Conversation</dt><dd>{conversationNames.get(revision.result_conversation_id) ?? revision.result_conversation_id}</dd></div>
              </dl>
              <section className="routine-run-history" aria-label={`${revision.expected_result} Run history`}>
                <h4>Run history</h4>
                {history.length === 0 ? <p>No runs yet.</p> : (
                  <ol>
                    {history.map((item) => (
                      <li key={item.run.id}>
                        <div className="routine-run-heading">
                          <strong>{item.run.state.replace("_", " ")}</strong>
                          <span>{item.run.kind}</span>
                          <time dateTime={item.run.created_at}>{item.run.created_at}</time>
                        </div>
                        {item.failure_code ? <p><b>Failure</b> · {item.failure_code}</p> : null}
                        {item.next_action ? <p><b>Next action</b> · {item.next_action}</p> : null}
                        {item.run.normalized_result ? <p className="routine-run-result">{item.run.normalized_result}</p> : null}
                        <details>
                          <summary>{item.activities.length} activit{item.activities.length === 1 ? "y" : "ies"}</summary>
                          <ul>
                            {item.activities.map((activity) => (
                              <li key={activity.sequence}>
                                <time dateTime={activity.created_at}>{activity.created_at}</time> · {activity.activity}
                              </li>
                            ))}
                          </ul>
                        </details>
                      </li>
                    ))}
                  </ol>
                )}
              </section>
            </article>
          );
        })}
      </div>

      <details className="card routine-create-card">
        <summary>Create Routine</summary>
        <form onSubmit={createRoutine}>
          <p>Creation records only Routine semantics. Fort derives the Agent&apos;s current immutable revisions.</p>
          <div className="routine-form-grid">
            <label>
              <span>Trigger</span>
              <select value={trigger} disabled={busy} onChange={(event) => {
                const value = event.target.value as "schedule" | "event";
                setTrigger(value);
                setLatenessPolicy(value === "schedule" ? "within_90s" : "none");
              }}>
                <option value="schedule">Schedule</option>
                <option value="event">Event</option>
              </select>
            </label>
            {trigger === "schedule" ? <>
              <label><span>Schedule</span><input required maxLength={512} placeholder="0 0 9 * * 1" value={schedule} onChange={(event) => setSchedule(event.target.value)} /></label>
              <label><span>Timezone</span><input required maxLength={128} placeholder="America/Chicago" value={timezone} onChange={(event) => setTimezone(event.target.value)} /></label>
              <label><span>Next occurrence <small>RFC 3339</small></span><input required maxLength={64} placeholder="2026-08-24T14:00:00Z" value={nextOccurrence} onChange={(event) => setNextOccurrence(event.target.value)} /></label>
            </> : null}
            <label><span>Input source</span><input required maxLength={4_096} placeholder="none or fort:conversation:&lt;id&gt;" value={inputSource} onChange={(event) => setInputSource(event.target.value)} /></label>
            <label><span>Freshness window <small>seconds</small></span><input required type="number" min="1" max={365 * 24 * 60 * 60} step="1" value={freshnessSeconds} onChange={(event) => setFreshnessSeconds(Number(event.target.value))} /></label>
            <label className="routine-form-wide"><span>Expected result</span><textarea required maxLength={4_096} value={expectedResult} onChange={(event) => setExpectedResult(event.target.value)} /></label>
            <label>
              <span>Result Conversation</span>
              <select required value={resultConversationID} onChange={(event) => setResultConversationID(event.target.value)}>
                {resultConversations.map((conversation) => (
                  <option value={conversation.id} key={conversation.id}>{conversation.title} · {conversation.kind === "canonical" ? "Home" : "Conversation"}</option>
                ))}
              </select>
            </label>
            <label>
              <span>Approval boundary</span>
              <select value={approvalBoundary} onChange={(event) => setApprovalBoundary(event.target.value)}>
                <option value="none">None</option>
                <option value="before_external_side_effect">Before external side effect</option>
              </select>
            </label>
            <label>
              <span>Missing or stale input</span>
              <select value={missingInputBehavior} onChange={(event) => setMissingInputBehavior(event.target.value as "skip" | "needs_you" | "fail")}>
                <option value="needs_you">Needs You</option>
                <option value="skip">Skip</option>
                <option value="fail">Fail</option>
              </select>
            </label>
            <label><span>Retry policy</span><input required maxLength={512} value={retryPolicy} onChange={(event) => setRetryPolicy(event.target.value)} /></label>
            <label><span>Catch-up policy</span><input required maxLength={512} value={catchUpPolicy} onChange={(event) => setCatchUpPolicy(event.target.value)} /></label>
            <label><span>Lateness policy</span><input required readOnly value={latenessPolicy} /></label>
          </div>
          {resultConversations.length === 0 ? <p className="err" role="alert">Open Home or a secondary Conversation before creating a Routine.</p> : null}
          <button className="btn btn-primary" disabled={busy || !canCreate()} type="submit">
            {pending === "create" ? "Creating…" : "Create Routine"}
          </button>
        </form>
      </details>
    </section>
  );
}

function validTimestamp(value: string): boolean { return value.length > 0 && Number.isFinite(Date.parse(value)); }

function exactSixFieldCron(value: string): boolean {
  return value.split(" ").length === 6 && value.split(" ").every((field) => field.length > 0);
}

function errorCode(value: unknown): string {
  if (value instanceof Error && /^[a-z][a-z0-9_]{0,63}$/.test(value.message)) return value.message;
  return "routine_command_failed";
}
