/**
 * Chat → Task classifier and decomposer.
 *
 * Two LLM-driven helpers used by `SpecialistAgent.onTask` to decide whether
 * an incoming chat message describes a multi-step task and, if so, break it
 * into subtasks.
 *
 * Both helpers are conservative: any parse failure or low-confidence signal
 * falls back to "not a task" so the agent's default chat-reply path runs.
 */
import type { LLMClient, ModelTier } from '../llm/index.js';

/**
 * Tunable inputs to the classifier/decomposer. When a Triager agent is
 * installed at `~/.fort/agents/triager/`, the caller passes its SOUL.md
 * here and the planner uses it as the classifier's system prompt instead
 * of the built-in default.
 *
 * `recentFeedback` is a small set of prior user corrections (from the
 * Triager's memory partition) injected as few-shot examples so the model
 * learns from the user over time.
 */
export interface TriagerConfig {
  /** Contents of the Triager's SOUL.md (used as classifier system prompt). */
  soul?: string;
  /** Model tier for the classifier. Defaults to 'fast' so this stays cheap. */
  modelTier?: ModelTier;
  /** Up to ~5 recent reclassification corrections from memory. */
  recentFeedback?: FeedbackExample[];
}

export interface FeedbackExample {
  /** The original user message that was classified. */
  message: string;
  /** What the classifier originally said ('task' or 'question'). */
  was: 'task' | 'question';
  /** What the user corrected it to. */
  shouldBe: 'task' | 'question';
}

export interface TaskClassification {
  isTask: boolean;
  /** 0..1 — the classifier's confidence that this message is a multi-step task. */
  confidence: number;
  /** One-sentence summary of what the user wants, used as the parent task's title narrative. */
  summary: string;
}

export interface SubtaskSpec {
  title: string;
  description: string;
  expectedOutcome?: string;
}

export interface DecompositionResult {
  subtasks: SubtaskSpec[];
}

const MAX_SUBTASKS = 6;

const CLASSIFIER_SYSTEM = `You decide whether a user's chat message describes a multi-step **task** that an agent should plan and execute, or whether it's a casual message / single-turn question that just needs a reply.

A **task** is something like:
- "Plan a 3-day trip to Lisbon"
- "Refactor the auth middleware to use JWTs"
- "Find me 5 vendors for office furniture and email me a comparison"
- "Schedule a haircut for Tuesday and add it to my calendar"

**Not a task:**
- "Hi"
- "What's the capital of France?"
- "How do I install Node?"
- "Thanks!"
- "Can you explain how OAuth works?"

Output strict JSON only (no markdown fences, no commentary):
{"isTask": true|false, "confidence": 0..1, "summary": "one-sentence summary of what the user wants"}

Be conservative — if unsure, prefer isTask=false.`;

const DECOMPOSER_SYSTEM = (max: number) => `You break a user's task into at most ${max} concrete subtasks an agent can execute one at a time.

Each subtask should:
- Have a short imperative title (e.g. "Find 3 flights to LIS").
- Have a description that's specific enough to act on without re-asking the user.
- Optionally state the expected outcome.

Output strict JSON only (no markdown fences, no commentary):
{"subtasks": [{"title": "...", "description": "...", "expectedOutcome": "..."}, ...]}

If the task is too vague to break down meaningfully, return {"subtasks": []}.`;

/** Strip code fences and locate the first {...} JSON object in a string. */
function extractJson(raw: string): unknown {
  if (!raw) return null;
  // Strip ```json ... ``` fences if the model added them.
  const fenced = raw.match(/```(?:json)?\s*([\s\S]*?)```/);
  const candidate = fenced ? fenced[1] : raw;
  // Find the first { and last } to tolerate trailing prose.
  const start = candidate.indexOf('{');
  const end = candidate.lastIndexOf('}');
  if (start < 0 || end <= start) return null;
  try {
    return JSON.parse(candidate.slice(start, end + 1));
  } catch {
    return null;
  }
}

/**
 * Decide whether `message` is a task.
 *
 * The 3rd argument is a TriagerConfig (preferred) — the SOUL.md becomes the
 * classifier system prompt, `modelTier` controls cost, and `recentFeedback`
 * injects prior user corrections as few-shot examples.
 *
 * Legacy callers can still pass a plain SOUL string; we'll wrap it.
 */
export async function classifyAsTask(
  llm: LLMClient,
  message: string,
  triagerOrSoul?: TriagerConfig | string,
): Promise<TaskClassification> {
  if (!llm.isConfigured) {
    return { isTask: false, confidence: 0, summary: '' };
  }

  const triager: TriagerConfig = typeof triagerOrSoul === 'string'
    ? { soul: triagerOrSoul }
    : (triagerOrSoul ?? {});

  // System prompt: prefer the Triager's SOUL.md when present.
  const system = triager.soul && triager.soul.trim().length > 0
    ? triager.soul + '\n\n## Output\nStrict JSON only — no commentary, no markdown fences:\n{"isTask": true|false, "confidence": 0..1, "summary": "one-sentence summary"}'
    : CLASSIFIER_SYSTEM;

  // Build the user prompt — optionally prepend recent feedback as few-shot.
  const feedbackBlock = triager.recentFeedback && triager.recentFeedback.length > 0
    ? '## Recent user corrections\nThese are messages where the human flipped your previous answer. Treat them as the strongest signal for similar future cases.\n\n'
      + triager.recentFeedback
          .slice(0, 5)
          .map((f) => `- "${f.message.replace(/\s+/g, ' ').slice(0, 140)}" → was ${f.was.toUpperCase()}, should be ${f.shouldBe.toUpperCase()}`)
          .join('\n')
      + '\n\n## Classify\n'
    : '';
  const userContent = feedbackBlock + message;

  try {
    const response = await llm.complete({
      messages: [{ role: 'user', content: userContent }],
      system,
      model: triager.modelTier ?? 'fast',
      maxTokens: 200,
      temperature: 0.1,
      injectBehaviors: false,
    });
    const parsed = extractJson(response.content);
    if (!parsed || typeof parsed !== 'object') {
      return { isTask: false, confidence: 0, summary: '' };
    }
    const obj = parsed as Record<string, unknown>;
    const isTask = obj.isTask === true;
    const confidenceRaw = obj.confidence;
    const confidence = typeof confidenceRaw === 'number' ? Math.max(0, Math.min(1, confidenceRaw)) : 0;
    const summary = typeof obj.summary === 'string' ? obj.summary : '';
    return { isTask, confidence, summary };
  } catch {
    return { isTask: false, confidence: 0, summary: '' };
  }
}

/**
 * Break the task into subtasks. Runs on the `standard` tier. Returns an empty
 * subtask list on any error — caller should treat that as "decompose failed,
 * fall back to a normal reply".
 */
export async function decomposeTask(
  llm: LLMClient,
  message: string,
  classification: Pick<TaskClassification, 'summary'>,
  agentSoul?: string,
): Promise<DecompositionResult> {
  if (!llm.isConfigured) return { subtasks: [] };
  try {
    const response = await llm.complete({
      messages: [{
        role: 'user',
        content: `User request: ${message}\n\nClassifier summary: ${classification.summary}\n\nBreak this into actionable subtasks.`,
      }],
      system: DECOMPOSER_SYSTEM(MAX_SUBTASKS),
      soul: agentSoul,
      model: 'standard',
      maxTokens: 1500,
      temperature: 0.2,
      injectBehaviors: false,
    });
    const parsed = extractJson(response.content);
    if (!parsed || typeof parsed !== 'object') return { subtasks: [] };
    const arr = (parsed as Record<string, unknown>).subtasks;
    if (!Array.isArray(arr)) return { subtasks: [] };
    const subtasks: SubtaskSpec[] = [];
    for (const entry of arr) {
      if (!entry || typeof entry !== 'object') continue;
      const e = entry as Record<string, unknown>;
      const title = typeof e.title === 'string' ? e.title.trim() : '';
      const description = typeof e.description === 'string' ? e.description.trim() : '';
      const expectedOutcome = typeof e.expectedOutcome === 'string' ? e.expectedOutcome.trim() : undefined;
      if (!title || !description) continue;
      subtasks.push({ title, description, expectedOutcome });
      if (subtasks.length >= MAX_SUBTASKS) break;
    }
    return { subtasks };
  } catch {
    return { subtasks: [] };
  }
}

/**
 * Render the parent task's `result` as a markdown plan card so ChatPage can
 * display it under the user's message. Kept as a pure formatter so the shape
 * is testable without spinning up a chat session.
 */
export function formatPlan(
  summary: string,
  subtasks: Array<{ shortId: string; title: string }>,
): string {
  const lines = subtasks.map((s, i) => `${i + 1}. \`${s.shortId}\` ${s.title}`);
  return `> ${summary}\n\n**Plan:**\n${lines.join('\n')}\n\n_Working through these now — each step will appear on the board as it completes._`;
}
