"use client";

import {
  displayAgent,
  nextPlaybookStageOrder,
  type DeckPlaybook,
  type DeckPlaybookAssignment,
  type DeckPlaybookStage,
} from "@/lib/command-deck";

const agents = ["hermes", "openclaw", "claude", "codex"];
const models: Record<string, string[]> = {
  hermes: ["Codex 5.6 Sol"],
  openclaw: ["Fable"],
  claude: ["Sonnet", "Opus"],
  codex: ["gpt-5.5"],
};
const profiles: Record<string, string> = {
  "claude\u0000": "claude:configured-default",
  "claude\u0000Sonnet": "claude:sonnet",
  "claude\u0000Opus": "claude:opus",
  "codex\u0000": "codex:configured-default",
  "codex\u0000gpt-5.5": "codex:gpt-5.5",
  "codex\u00005.6 Sol": "codex:gpt-5.6-sol",
  "hermes\u0000": "hermes:configured-default",
  "hermes\u0000Codex 5.6 Sol": "hermes:openai-codex/gpt-5.6-sol",
  "openclaw\u0000": "openclaw:main",
  "openclaw\u0000Fable": "openclaw:main",
};

function withExactProfile(assignment: DeckPlaybookAssignment): DeckPlaybookAssignment {
  const { profile: _stale, ...rest } = assignment;
  const profile = profiles[`${rest.agent}\u0000${rest.model ?? ""}`];
  return profile ? { ...rest, profile } : rest;
}

export function assignmentForAddedStage(prior?: DeckPlaybookAssignment): DeckPlaybookAssignment {
  return withExactProfile({
    agent: prior?.agent ?? "codex",
    model: prior ? prior.model ?? "" : "gpt-5.5",
  });
}
const triggerKinds = ["question", "bug", "feature", "research", "manual"];

export function PlaybooksSurface({
  playbooks,
  selectedID,
  loading,
  busy,
  onSelect,
  onReload,
  onSave,
  onDuplicate,
}: {
  playbooks: DeckPlaybook[];
  selectedID: string;
  loading: boolean;
  busy: boolean;
  onSelect: (id: string) => void;
  onReload: () => void;
  onSave: (playbook: DeckPlaybook) => void;
  onDuplicate: (id: string) => void;
}) {
  const playbook = playbooks.find((item) => item.id === selectedID) ?? playbooks[0];

  if (loading && playbooks.length === 0) {
    return <div className="playbook-empty">Loading the sealed Playbooks catalog…</div>;
  }
  if (!playbook) {
    return (
      <div className="playbook-empty">
        <strong>No playbooks configured.</strong>
        <span>The connected Fort has not published a reusable route yet.</span>
        <button className="btn btn-secondary" onClick={onReload} disabled={busy}>
          Reload catalog
        </button>
      </div>
    );
  }

  const saveStage = (stageIndex: number, nextStage: DeckPlaybookStage) => {
    onSave({
      ...playbook,
      stages: playbook.stages.map((stage, index) => (index === stageIndex ? nextStage : stage)),
    });
  };

  const addStage = () => {
    const entered = window.prompt("Stage name", "New stage");
    const name = entered?.trim();
    if (!name) return;
    const prior = playbook.stages.at(-1)?.assignments[0];
    onSave({
      ...playbook,
      stages: [
        ...playbook.stages,
        {
          order: nextPlaybookStageOrder(playbook.stages),
          name,
          assignments: [assignmentForAddedStage(prior)],
          memory: false,
        },
      ],
    });
  };

  return (
    <section className="playbooks-view">
      <div className="playbook-layout">
        <aside className="playbook-rail" aria-label="Playbooks catalog">
          {playbooks.map((item) => (
            <button
              className={`playbook-item ${item.id === playbook.id ? "active" : ""}`}
              key={item.id}
              onClick={() => onSelect(item.id)}
              disabled={busy}
            >
              <span>
                <strong>{item.name}</strong>
                {item.is_default ? <small>default</small> : null}
              </span>
              <em>{playbookMeta(item)}</em>
            </button>
          ))}
        </aside>

        <section className="playbook-editor">
          <div className="playbook-editor-heading">
            <div>
              <div className="playbook-title-row">
                <h2>{playbook.name}</h2>
                <span>rev {playbook.revision}</span>
              </div>
              <label className="playbook-trigger">
                <span>Trigger</span>
                <select
                  value={playbook.trigger.kind}
                  disabled={busy}
                  onChange={(event) => {
                    const kind = event.target.value;
                    onSave({
                      ...playbook,
                      trigger: {
                        kind,
                        enabled: kind === "manual" ? false : playbook.trigger.enabled,
                      },
                    });
                  }}
                >
                  {triggerKinds.map((kind) => (
                    <option key={kind} value={kind}>
                      {triggerCopy(kind)}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <div className="playbook-settings">
              <span className="delivery-chip">
                {playbook.delivery === "answer" ? "Direct answer" : "Assignment"}
              </span>
              {playbook.delivery === "answer" ? (
                <span className="no-checkpoints">No checkpoints</span>
              ) : (
                <Toggle
                  label="Plan gate"
                  checked={!!playbook.plan_gate}
                  disabled={busy}
                  onChange={(checked) => onSave({ ...playbook, plan_gate: checked })}
                />
              )}
            </div>
            <div className="playbook-header-actions">
              <button className="btn btn-secondary" onClick={onReload} disabled={busy}>
                Reload
              </button>
              <button className="btn btn-secondary" onClick={() => onDuplicate(playbook.id)} disabled={busy}>
                Duplicate
              </button>
            </div>
          </div>

          <div className="playbook-pipeline" aria-label={`${playbook.name} stages`}>
            {[...playbook.stages]
              .sort((left, right) => left.order - right.order)
              .map((stage, sortedIndex) => {
                const stageIndex = playbook.stages.indexOf(stage);
                return (
                  <div className="playbook-stage-wrap" key={`${stage.order}/${stage.name}`}>
                    {sortedIndex > 0 ? <span className="stage-connector">then</span> : null}
                    <article className="playbook-stage">
                      <header>
                        <span className="stage-number">{stage.order}</span>
                        <strong>{stage.name}</strong>
                        {stage.assignments.length > 1 ? (
                          <small>by task type</small>
                        ) : (
                          <button
                            className={`memory-button ${stage.memory ? "active" : ""}`}
                            onClick={() => saveStage(stageIndex, { ...stage, memory: !stage.memory })}
                            disabled={busy}
                          >
                            {stage.memory ? "memory on" : "memory off"}
                          </button>
                        )}
                      </header>
                      <div className="stage-assignments">
                        {stage.assignments.map((assignment, assignmentIndex) => (
                          <div
                            className={stage.assignments.length > 1 ? "stage-assignment branching" : "stage-assignment"}
                            key={`${assignment.task_type ?? "default"}/${assignmentIndex}`}
                          >
                            {stage.assignments.length > 1 ? (
                              <span>{assignment.task_type || "default"}</span>
                            ) : null}
                            <select
                              aria-label={`Agent for ${stage.name}`}
                              value={assignment.agent}
                              disabled={busy}
                              onChange={(event) => {
                                const agent = event.target.value;
                                const nextAssignments = stage.assignments.map((item, index) =>
                                  index === assignmentIndex
                                    ? withExactProfile({ ...item, agent, model: models[agent]?.[0] ?? "" })
                                    : item,
                                );
                                saveStage(stageIndex, { ...stage, assignments: nextAssignments });
                              }}
                            >
                              {optionValues(agents, assignment.agent).map((agent) => (
                                <option key={agent} value={agent}>
                                  {displayAgent(agent)}
                                </option>
                              ))}
                            </select>
                            <select
                              className="model-select"
                              aria-label={`Model for ${stage.name}`}
                              value={assignment.model ?? ""}
                              disabled={busy}
                              onChange={(event) => {
                                const nextAssignments = stage.assignments.map((item, index) =>
                                  index === assignmentIndex ? withExactProfile({ ...item, model: event.target.value }) : item,
                                );
                                saveStage(stageIndex, { ...stage, assignments: nextAssignments });
                              }}
                            >
                              <option value="">Provider default</option>
                              {optionValues(models[assignment.agent] ?? [], assignment.model ?? "").map((model) => (
                                <option key={model} value={model}>
                                  {model}
                                </option>
                              ))}
                            </select>
                          </div>
                        ))}
                      </div>
                      <p>{stage.description || stage.prompt || `Runs the ${stage.name.toLowerCase()} stage.`}</p>
                      {stage.assignments.length > 1 ? (
                        <Toggle
                          label="Share output in run memory"
                          checked={!!stage.memory}
                          disabled={busy}
                          onChange={(checked) => saveStage(stageIndex, { ...stage, memory: checked })}
                        />
                      ) : null}
                    </article>
                  </div>
                );
              })}
            {playbook.delivery !== "answer" ? (
              <button className="add-stage" onClick={addStage} disabled={busy}>
                Add stage
              </button>
            ) : null}
          </div>

          <section className="playbook-shortcuts">
            <span className="section-label">SHORTCUTS — TRIGGERS THAT SKIP THE CHAIN</span>
            <div>
              {playbooks
                .filter((item) => item.id !== playbook.id && item.trigger.kind !== "manual")
                .sort((left, right) => shortcutRank(left) - shortcutRank(right) || left.name.localeCompare(right.name))
                .map((shortcut) => {
                  const assignment = shortcut.stages[0]?.assignments[0];
                  return (
                    <article className="shortcut-row" key={shortcut.id}>
                      <div>
                        <strong>
                          When {triggerCopy(shortcut.trigger.kind).toLowerCase()} → {shortcut.name}
                        </strong>
                        <span>
                          {assignment ? displayAgent(assignment.agent) : "Fort decides"}
                          {assignment?.model ? ` · ${assignment.model}` : ""}
                          {shortcut.delivery === "answer"
                            ? " · replies inline · no checkpoints"
                            : shortcut.plan_gate
                              ? " · plan gate on"
                              : " · starts directly"}
                        </span>
                      </div>
                      <Toggle
                        label={`${shortcut.name} shortcut`}
                        checked={shortcut.trigger.enabled}
                        disabled={busy}
                        hideLabel
                        onChange={(checked) =>
                          onSave({ ...shortcut, trigger: { ...shortcut.trigger, enabled: checked } })
                        }
                      />
                    </article>
                  );
                })}
            </div>
          </section>
        </section>
      </div>
    </section>
  );
}

function Toggle({
  label,
  checked,
  disabled,
  hideLabel = false,
  onChange,
}: {
  label: string;
  checked: boolean;
  disabled: boolean;
  hideLabel?: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className="playbook-toggle">
      {hideLabel ? <span className="sr-only">{label}</span> : <span>{label}</span>}
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
      />
    </label>
  );
}

function optionValues(values: string[], current: string): string[] {
  return current && !values.includes(current) ? [...values, current] : values;
}

function triggerCopy(kind: string): string {
  const copy: Record<string, string> = {
    question: "I ask a question",
    bug: "Direction is a bug report",
    feature: "Direction describes a new capability",
    research: "Direction asks for research",
    manual: "I choose it manually",
  };
  return copy[kind] ?? kind.replace(/[-_]/g, " ");
}

function playbookMeta(playbook: DeckPlaybook): string {
  const stages = `${playbook.stages.length} stage${playbook.stages.length === 1 ? "" : "s"}`;
  if (playbook.delivery === "answer") return `${stages} · no checkpoints`;
  if (playbook.trigger.kind === "bug") return `${stages} · skips design`;
  if (playbook.trigger.kind === "research") return `${stages} · delivers a brief`;
  return `${stages} · plan gate ${playbook.plan_gate ? "on" : "off"}`;
}

function shortcutRank(playbook: DeckPlaybook): number {
  if (playbook.delivery === "answer") return 0;
  if (playbook.trigger.kind === "bug") return 1;
  if (playbook.trigger.kind === "research") return 2;
  return 3;
}
