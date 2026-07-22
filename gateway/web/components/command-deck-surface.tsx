"use client";

import {
  checkpointCaption,
  crewAssignments,
  crewFailureActivity,
  displayAgent,
  projectStateLabel,
  recentFailedRuns,
  relativeAge,
  runState,
  shouldShowOfflineDeck,
  sortRunsForDeck,
  type DeckBacklogItem,
  type DeckGate,
  type DeckPayload,
  type DeckRun,
  type DeckRunState,
} from "@/lib/command-deck";

export function CommandDeckSurface({
  online,
  brokerOnline,
  deck,
  loading,
  busy,
  onDecide,
  onDispatch,
}: {
  online: boolean;
  brokerOnline: boolean;
  deck: DeckPayload | null;
  loading: boolean;
  busy: boolean;
  onDecide: (gate: DeckGate, decision: "approve" | "reject") => void;
  onDispatch: (item: DeckBacklogItem) => void;
}) {
  const gates = deck?.board.gates ?? [];
  const runs = sortRunsForDeck(deck?.board.runs ?? [], gates);
  const failedRuns = recentFailedRuns(runs, gates).slice(0, 3);
  const crew = crewAssignments(runs, deck?.machines ?? [], gates);

  return (
    <div className="remote-deck">
      <div className="needs-pane">
        <div className="section-heading">
          <span className="section-label needs">NEEDS YOU</span>
          {gates.length > 0 ? <span className="needs-pill">{gates.length} waiting</span> : null}
        </div>
        {shouldShowOfflineDeck(online, deck !== null) ? (
          <EmptyCard
            text={
              brokerOnline
                ? "The broker sees this Fort. Verifying the end-to-end connection…"
                : "This Fort is offline. The deck will refresh when its relay reconnects."
            }
          />
        ) : deck === null && loading ? (
          <LoadingCards />
        ) : gates.length === 0 && failedRuns.length === 0 ? (
          <EmptyCard
            text={`That's everything — ${deck?.summary.running ?? 0} crew member${deck?.summary.running === 1 ? " is" : "s are"} working and don't need you.`}
          />
        ) : (
          <>
            {gates.map((gate) => {
              const run = runs.find((candidate) => candidate.id === gate.run_id);
              return (
                <article className="needs-card" key={`${gate.run_id}/${gate.node_id}`}>
                  <div className="card-title-row">
                    <h3>{run?.title ?? humanize(gate.node_id)}</h3>
                    <time>{relativeAge(gate.since ?? run?.updated_at)}</time>
                  </div>
                  <p>
                    {gate.input ||
                      `The crew reached ${humanize(gate.node_id)} and is waiting for your sign-off.`}
                  </p>
                  <div className="card-actions">
                    <button
                      className="btn btn-accept"
                      onClick={() => onDecide(gate, "approve")}
                      disabled={busy}
                    >
                      Accept checkpoint
                    </button>
                    <button
                      className="btn btn-secondary"
                      onClick={() => onDecide(gate, "reject")}
                      disabled={busy}
                    >
                      Redirect…
                    </button>
                  </div>
                </article>
              );
            })}
            {failedRuns.map((run) => (
              <article className="needs-card failed" key={run.id}>
                <div className="card-title-row">
                  <h3>{run.title}</h3>
                  <time>{relativeAge(run.updated_at ?? run.created_at)}</time>
                </div>
                <p>
                  {displayAgent(run.agent)} could not complete this assignment. Open the full
                  snapshot or Activity to inspect the failure.
                </p>
              </article>
            ))}
          </>
        )}
        {deck && (gates.length > 0 || failedRuns.length > 0) ? (
          <p className="calm-note">
            That&apos;s everything — {deck.summary.running} crew member
            {deck.summary.running === 1 ? " is" : "s are"} working and don&apos;t need you.
          </p>
        ) : null}
      </div>

      <aside className="overview-pane">
        <section>
          <div className="section-heading">
            <span className="section-label">PROJECTS</span>
            {deck ? <span className="count-label">{deck.summary.total} assignments</span> : null}
          </div>
          <div className="project-list">
            {runs.slice(0, 5).map((run) => (
              <ProjectRow key={run.id} run={run} gates={gates} />
            ))}
            {deck && runs.length === 0 ? <EmptyRow text="Nothing on the board yet." /> : null}
          </div>
        </section>

        <section>
          <div className="section-heading">
            <span className="section-label">CREW</span>
            {deck ? (
              <span className={`execution-pill ${deck.summary.execution ? "attached" : "control"}`}>
                {deck.summary.execution ? "execution attached" : "control only"}
              </span>
            ) : null}
          </div>
          <div className="crew-list">
            {crew.map((member) => (
              <CrewRow
                key={member.agent}
                agent={member.agent}
                run={member.run}
                attributionUnknown={member.attributionUnknown}
                gates={gates}
              />
            ))}
            {deck && crew.length === 0 ? <EmptyRow text="No crew activity yet." /> : null}
          </div>
        </section>

        <section>
          <div className="section-heading">
            <span className="section-label">UP NEXT</span>
            {deck ? <span className="count-label">{deck.backlog.length} queued</span> : null}
          </div>
          <div className="up-next-list">
            {(deck?.backlog ?? []).slice(0, 4).map((item) => (
              <div className="up-next-row" key={item.id}>
                <div>
                  <strong>{item.title}</strong>
                  <span>
                    {item.agent ? displayAgent(item.agent) : "Fort decides"}
                    {item.machine ? ` · ${item.machine}` : ""}
                  </span>
                </div>
                <button className="btn btn-secondary" onClick={() => onDispatch(item)} disabled={busy}>
                  Start
                </button>
              </div>
            ))}
            {deck && deck.backlog.length === 0 ? <EmptyRow text="Nothing queued." /> : null}
          </div>
        </section>
      </aside>
    </div>
  );
}

function ProjectRow({ run, gates }: { run: DeckRun; gates: DeckGate[] }) {
  const state = runState(run, gates);
  const label = projectStateLabel(run, gates);
  return (
    <article className="project-row">
      <span className={`project-state state-${state}`} aria-label={label} />
      <div>
        <strong>{run.title}</strong>
        <span>{checkpointCaption(run.checkpoints)}</span>
      </div>
      <span className={`status-pill state-${state}`}>{label}</span>
    </article>
  );
}

function CrewRow({
  agent,
  run,
  attributionUnknown,
  gates,
}: {
  agent: string;
  run?: DeckRun;
  attributionUnknown: boolean;
  gates: DeckGate[];
}) {
  const state = attributionUnknown ? "idle" : run ? runState(run, gates) : "idle";
  const activity: Record<DeckRunState, string> = {
    "needs-you": run
      ? `waiting on your sign-off · ${relativeAge(run.updated_at ?? run.created_at)}`
      : "waiting on your sign-off",
    working: run
      ? `${run.title} · ${relativeAge(run.updated_at ?? run.created_at)}`
      : "working",
    delivered: run ? `delivered ${run.title}` : "delivered",
    failed: run ? crewFailureActivity(run, gates) : "failed",
    idle: "open capacity — ready for an assignment",
  };
  return (
    <div className="crew-row">
      <span className={`status-dot state-${state}`} />
      <strong>{displayAgent(agent)}</strong>
      <span>
        {attributionUnknown ? "playbook active — agent attribution unavailable" : activity[state]}
      </span>
    </div>
  );
}

function LoadingCards() {
  return (
    <div className="loading-stack" role="status" aria-label="Loading Command Deck">
      <span />
      <span />
    </div>
  );
}

function EmptyCard({ text }: { text: string }) {
  return <div className="empty-card">{text}</div>;
}

function EmptyRow({ text }: { text: string }) {
  return <div className="empty-row">{text}</div>;
}

function humanize(value: string): string {
  const text = value.replace(/[-_]+/g, " ").trim();
  return text ? text.charAt(0).toUpperCase() + text.slice(1) : "Checkpoint review";
}
