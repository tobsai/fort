"use client";

import { FormEvent, useState, useTransition } from "react";
import { useRouter } from "next/navigation";

import { createAgentMutationClient } from "@/lib/v2-agent-mutation-client";

const mutationClient = createAgentMutationClient();

interface AgentSettingsProps {
  agentID: string;
  profileRevisionID: string;
  behaviorRevisionID: string;
  bindingRevisionID: string;
  profile: {
    name: string;
    title: string;
    avatarURL: string;
    hidden: boolean;
    pinned: boolean;
    sortOrder: number;
  };
  behavior: {
    role: string;
    standingInstructions: string;
    enabledSkills: string[];
    enabledTools: string[];
    promptMaterial: string;
  };
}

export default function AgentSettings(props: AgentSettingsProps) {
  const [profile, setProfile] = useState(props.profile);
  const [role, setRole] = useState(props.behavior.role);
  const [standingInstructions, setStandingInstructions] = useState(props.behavior.standingInstructions);
  const [enabledSkills, setEnabledSkills] = useState(props.behavior.enabledSkills.join("\n"));
  const [enabledTools, setEnabledTools] = useState(props.behavior.enabledTools.join("\n"));
  const [promptMaterial, setPromptMaterial] = useState(props.behavior.promptMaterial);
  const [saving, setSaving] = useState<"profile" | "behavior" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState<"profile" | "behavior" | null>(null);
  const [, startTransition] = useTransition();
  const router = useRouter();

  async function saveProfile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving) return;
    setSaving("profile");
    setError(null);
    setSaved(null);
    try {
      await mutationClient.update({
        action: "profile",
        agentID: props.agentID,
        idempotencyKey: `profile:edit:${crypto.randomUUID()}`,
        expectedProfileRevisionID: props.profileRevisionID,
        profile: {
          ...profile,
          name: profile.name.trim(),
          title: profile.title.trim(),
          avatarURL: profile.avatarURL.trim(),
        },
      });
      setSaved("profile");
      startTransition(() => router.refresh());
    } catch (cause) {
      setError(errorCode(cause));
    } finally {
      setSaving(null);
    }
  }

  async function saveBehavior(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving) return;
    setSaving("behavior");
    setError(null);
    setSaved(null);
    try {
      await mutationClient.update({
        action: "behavior",
        agentID: props.agentID,
        idempotencyKey: `behavior:edit:${crypto.randomUUID()}`,
        expectedBehaviorRevisionID: props.behaviorRevisionID,
        expectedBindingRevisionID: props.bindingRevisionID,
        behavior: {
          role: role.trim(),
          standingInstructions,
          enabledSkills: normalizedLines(enabledSkills),
          enabledTools: normalizedLines(enabledTools),
          promptMaterial,
        },
      });
      setSaved("behavior");
      startTransition(() => router.refresh());
    } catch (cause) {
      setError(errorCode(cause));
    } finally {
      setSaving(null);
    }
  }

  return (
    <section className="agent-settings" aria-labelledby="agent-settings-heading">
      <div className="agent-settings-heading">
        <div>
          <span className="eyebrow">OWNER SETTINGS</span>
          <h2 id="agent-settings-heading">Agent profile and Behavior</h2>
        </div>
        {error ? <span className="err" role="alert">Save failed: {error}</span> : null}
        {saved ? <span className="agent-settings-saved" role="status">{saved === "profile" ? "Profile" : "Behavior"} saved</span> : null}
      </div>

      <div className="agent-settings-grid">
        <details className="card agent-settings-card">
          <summary>Edit profile</summary>
          <form onSubmit={saveProfile}>
            <p>Presentation only. These fields do not change how the Agent runs.</p>
            <label>
              <span>Name</span>
              <input required maxLength={120} value={profile.name} onChange={(event) => setProfile({ ...profile, name: event.target.value })} />
            </label>
            <label>
              <span>Title</span>
              <input maxLength={512} value={profile.title} onChange={(event) => setProfile({ ...profile, title: event.target.value })} />
            </label>
            <label>
              <span>Avatar URL</span>
              <input type="url" maxLength={2_048} value={profile.avatarURL} onChange={(event) => setProfile({ ...profile, avatarURL: event.target.value })} />
            </label>
            <label>
              <span>Sort order</span>
              <input type="number" step="1" value={profile.sortOrder} onChange={(event) => setProfile({ ...profile, sortOrder: Number(event.target.value) })} />
            </label>
            <div className="agent-settings-options">
              <label><input type="checkbox" checked={profile.pinned} onChange={(event) => setProfile({ ...profile, pinned: event.target.checked })} /> Pin in Agent list</label>
              <label><input type="checkbox" checked={profile.hidden} onChange={(event) => setProfile({ ...profile, hidden: event.target.checked })} /> Hide from Agent list</label>
            </div>
            <button className="btn btn-primary" type="submit" disabled={saving !== null || !profile.name.trim()}>
              {saving === "profile" ? "Saving…" : "Save profile"}
            </button>
          </form>
        </details>

        <details className="card agent-settings-card">
          <summary>Edit Behavior</summary>
          <form onSubmit={saveBehavior}>
            <p>This appends an immutable Behavior revision for future turns while retaining the current execution binding.</p>
            <label>
              <span>Role</span>
              <textarea required maxLength={4_096} value={role} onChange={(event) => setRole(event.target.value)} />
            </label>
            <label>
              <span>Standing instructions</span>
              <textarea maxLength={100_000} value={standingInstructions} onChange={(event) => setStandingInstructions(event.target.value)} />
            </label>
            <label>
              <span>Enabled skills <small>one per line</small></span>
              <textarea value={enabledSkills} onChange={(event) => setEnabledSkills(event.target.value)} />
            </label>
            <label>
              <span>Enabled tools <small>one per line</small></span>
              <textarea value={enabledTools} onChange={(event) => setEnabledTools(event.target.value)} />
            </label>
            <label>
              <span>Prompt material</span>
              <textarea maxLength={100_000} value={promptMaterial} onChange={(event) => setPromptMaterial(event.target.value)} />
            </label>
            <button className="btn btn-primary" type="submit" disabled={saving !== null || !role.trim()}>
              {saving === "behavior" ? "Saving…" : "Save Behavior"}
            </button>
          </form>
        </details>
      </div>
    </section>
  );
}

function normalizedLines(value: string): string[] {
  return [...new Set(value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean))];
}

function errorCode(value: unknown): string {
  if (value instanceof Error && /^[a-z][a-z0-9_]{0,63}$/.test(value.message)) return value.message;
  return "agent_update_failed";
}
