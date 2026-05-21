/**
 * Specialist Agent — Data-driven agent created from an identity file
 *
 * Unlike core agents which are coded in TypeScript, specialist agents
 * are defined by their identity YAML and a SOUL.md file. They have
 * their own memory partition, personality, and event subscriptions.
 *
 * This is Fort's equivalent of OpenClaw's agent creation process.
 */

import { readFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { parse as parseYaml } from 'yaml';
import { BaseAgent } from './index.js';
import type { AgentConfig, SpecialistIdentity } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { MemoryManager } from '../memory/index.js';
import type { LLMClient } from '../llm/index.js';
import type { ToolRegistry } from '../tools/index.js';
import type { ToolExecutor } from '../tools/executor.js';
import { classifyAsTask, decomposeTask, formatPlan, type TriagerConfig } from './task-planner.js';
import { ModelGatedError } from '../llm/index.js';

export class SpecialistAgent extends BaseAgent {
  readonly identity: SpecialistIdentity;
  readonly agentDir: string;
  private memory: MemoryManager;
  private llm: LLMClient | null = null;
  private toolRegistry: ToolRegistry | null = null;
  private toolExecutor: ToolExecutor | null = null;
  private modelChoice: import('../services/model-choice.js').ModelChoiceService | null = null;
  private unsubscribers: Array<() => void> = [];
  private _soulCache: string | null = null;

  constructor(
    identity: SpecialistIdentity,
    bus: ModuleBus,
    taskGraph: TaskGraph,
    memory: MemoryManager,
    agentDir: string,
  ) {
    const config: AgentConfig = {
      id: identity.id,
      name: identity.name,
      type: 'specialist',
      description: identity.description,
      capabilities: identity.capabilities,
      memoryPartition: identity.memoryPartition,
    };
    super(config, bus, taskGraph);
    this.identity = identity;
    this.memory = memory;
    this.agentDir = agentDir;
  }

  protected async onStart(): Promise<void> {
    // Load soul on start
    this.refreshSoul();

    // Subscribe to configured events (skip task.created to prevent infinite loops)
    const safeEvents = this.identity.eventSubscriptions.filter(
      (e: string) => e !== 'task.created' && e !== 'task.status_changed',
    );
    for (const eventType of safeEvents) {
      const unsub = this.bus.subscribe(eventType, async (event) => {
        const task = this.taskGraph.createTask({
          title: `[${this.identity.name}] Handle ${eventType}`,
          description: JSON.stringify(event.payload),
          source: 'agent_delegation',
          assignedAgent: this.config.id,
        });
        await this.handleTask(task.id);
      });
      this.unsubscribers.push(unsub);
    }

    // Store agent activation in memory
    this.memory.createNode({
      type: 'fact',
      label: `${this.identity.name} started`,
      properties: {
        agentId: this.identity.id,
        event: 'agent_started',
        hasSoul: this._soulCache !== null,
        timestamp: new Date().toISOString(),
      },
      source: `agent:${this.identity.id}`,
    });
  }

  protected async onStop(): Promise<void> {
    for (const unsub of this.unsubscribers) {
      unsub();
    }
    this.unsubscribers = [];
  }

  /**
   * Attach an LLM client for this agent to use when processing tasks.
   */
  setLLM(llm: LLMClient): void {
    this.llm = llm;
  }

  /**
   * Attach the tool registry so this agent can check available tools.
   */
  setToolRegistry(tools: ToolRegistry): void {
    this.toolRegistry = tools;
  }

  /**
   * Attach the tool executor so this agent can run tools during LLM loops.
   */
  setToolExecutor(executor: ToolExecutor): void {
    this.toolExecutor = executor;
  }

  /**
   * Attach the model-choice service so this agent can handle gated-model prompts.
   */
  setModelChoice(svc: import('../services/model-choice.js').ModelChoiceService): void {
    this.modelChoice = svc;
  }

  protected async onTask(taskId: string): Promise<void> {
    const task = this.taskGraph.getTask(taskId);
    const isChatTask = task.metadata.type === 'chat';

    // Mark in progress and publish acknowledgment
    this.taskGraph.updateStatus(taskId, 'in_progress');

    if (isChatTask) {
      // Publish an intermediate acknowledgment so the portal can show it immediately
      this.bus.publish('agent.acknowledged', this.config.id, {
        taskId: task.id,
        shortId: task.shortId,
        title: task.title,
        agentId: this.config.id,
        agentName: this.identity.name,
        message: `Working on ${task.shortId}: ${task.title}`,
      });
    }

    // ── Classify + decompose (top-level chat tasks only) ─────────
    // If the user's message describes a multi-step task, break it down into
    // subtasks on the board before falling through to the normal reply path.
    // Subtasks (parentId !== null) skip this block entirely so we don't
    // recurse forever.
    //
    // Routes the message through the Triager agent's SOUL.md if present
    // (~/.fort/agents/triager/SOUL.md). Users can edit that file to shape
    // judgment without touching code. Falls back to the built-in classifier
    // prompt when Triager is absent (e.g. upgrade-in-progress installs).
    //
    // While the agent is un-hatched, the conversation IS the onboarding
    // interview — the system greeting and the user's answers are chat, not
    // tasks. Classifying them fires the Triager (and a classification card)
    // before the user has even replied. Skip triage entirely until hatched.
    const decomposeEnabled =
      (this.identity as any).decompose !== false
      && this.identity.hatchedAt != null
      && process.env.FORT_DISABLE_TRIAGE !== '1';
    if (
      isChatTask &&
      task.parentId === null &&
      decomposeEnabled &&
      this.llm &&
      this.llm.isConfigured
    ) {
      const userMessage = task.description;
      const triager = this.loadTriagerConfig();

      this.bus.publish('agent.classifying', this.config.id, {
        taskId: task.id, agentId: this.config.id,
      });

      const classification = await classifyAsTask(this.llm, userMessage, triager);

      // Persist the classifier's verdict on the task so the reclassify UI
      // can show "what the classifier saw" and feedback can reference it.
      this.taskGraph.updateMetadata(taskId, {
        classification: classification.isTask ? 'task' : 'question',
        classifierConfidence: classification.confidence,
        classifierSummary: classification.summary,
        // Route to the right board immediately.
        board: classification.isTask && classification.confidence >= 0.6 ? 'main' : 'questions',
      });

      this.bus.publish('agent.classified', this.config.id, {
        taskId: task.id, agentId: this.config.id,
        isTask: classification.isTask,
        confidence: classification.confidence,
        summary: classification.summary,
      });

      if (classification.isTask && classification.confidence >= 0.6) {
        this.bus.publish('agent.decomposing', this.config.id, {
          taskId: task.id, agentId: this.config.id,
        });

        const plan = await decomposeTask(this.llm, userMessage, classification, triager.soul);

        if (plan.subtasks.length > 0) {
          const children = this.taskGraph.decompose(
            taskId,
            plan.subtasks.map((s) => ({
              title: s.title,
              description: s.description + (s.expectedOutcome ? `\n\n**Expected outcome:** ${s.expectedOutcome}` : ''),
              assignedAgent: this.config.id,
            })),
          );

          // Set parent result to a markdown plan card so ChatPage renders it.
          this.taskGraph.updateStatus(
            taskId,
            'in_progress',
            'Decomposed into subtasks',
            formatPlan(classification.summary, children.map((c) => ({ shortId: c.shortId, title: c.title }))),
          );

          this.bus.publish('agent.decomposed', this.config.id, {
            taskId: task.id, agentId: this.config.id,
            subtasks: children.map((c) => ({ id: c.id, shortId: c.shortId, title: c.title })),
          });

          // Take the next step — run each subtask in order. They skip the
          // classifier because parentId !== null. Parent auto-completes when
          // all children finish via TaskGraph's checkParentCompletion.
          for (const child of children) {
            await this.onTask(child.id);
          }
          return;
        }

        // Decomposer returned empty — bail to the normal reply path.
        this.bus.publish('agent.decomposed_failed', this.config.id, {
          taskId: task.id, agentId: this.config.id, reason: 'empty_plan',
        });
      }
    }

    // ── Tool search ─────────────────────────────────────────────
    // Check available tools before responding so the LLM knows what it can use.
    // Skip for "Build tool:" subtasks to prevent infinite loops.
    const isToolBuildTask = task.title.startsWith('Build tool:');
    let toolContext: string | undefined;

    if (this.toolRegistry && isChatTask && !isToolBuildTask) {
      const toolResults = this.toolRegistry.search(task.description);
      if (toolResults.length > 0) {
        toolContext = '## Available Tools\nYou have access to these tools:\n' +
          toolResults.slice(0, 5).map(t =>
            `- **${t.name}** (${t.module}): ${t.description}\n  Capabilities: ${t.capabilities.join(', ')}`
          ).join('\n');
      } else {
        toolContext = `## Available Tools
No existing tools match this task. If this task requires an external integration, API access, or capability you don't have, you MUST respond with a JSON block proposing a tool to build:

\`\`\`json
{"needsTool": true, "toolName": "name-of-tool", "toolDescription": "what it does", "architecture": "implementation steps"}
\`\`\`

Include the JSON block in your response along with your explanation to the user. Do NOT say you cannot do something without proposing a tool to fix it.`;
      }
    }

    // ── Generate response ────────────────────────────────────────
    let responseText: string;

    if (this.llm && this.llm.isConfigured && isChatTask) {
      // LLM-powered response
      const soul = this.getSoul();
      const baseTier = (task.metadata.modelTier as string) || this.identity.defaultModelTier;
      const liveTools = this.toolRegistry ? this.toolRegistry.listLiveTools() : [];

      // Un-hatched agents run their first conversation in hatch mode:
      // the LLM gets the hatch addendum on top of the base prompt + SOUL
      // and steers toward a getting-to-know-you flow ending in a
      // proposed goals list.
      const hatchMode = this.identity.hatchedAt == null;

      // Only interactive (user_chat) requests trigger the gated-model choice
      // gate. Background work keeps the LLM client's silent auto-fallback.
      const interactive = task.source === 'user_chat';

      // Per-task routing overrides selected via the choice gate. These are
      // local-only unless the user chose "remember" (then persisted to
      // identity.yaml). They reset every task — unremembered choices never
      // mutate this.identity.
      let providerOverride: string | undefined;
      let tierOverride: string | undefined;
      const triedGated = new Set<string>(); // models we've already been told are gated
      let rounds = 0;

      // Bounded retry: on ModelGatedError, block on the user's choice, apply
      // the chosen routing, and retry. Capped at 4 rounds so a recurring gate
      // (e.g. each fallback tier is also cooled down) can't loop forever.
      while (true) {
        rounds++;
        try {
          if (this.toolExecutor && liveTools.length > 0) {
            // Use the tool loop — LLM can invoke tools mid-conversation
            const response = await this.llm.completeWithTools(
              {
                messages: [{ role: 'user', content: task.description }],
                soul: soul ?? undefined,
                taskId: task.id,
                agentId: this.identity.id,
                model: tierOverride ?? baseTier,
                providerOverride,
                injectBehaviors: true,
                injectMemory: task.description,
                context: toolContext ? [toolContext] : undefined,
                tools: liveTools,
                hatchMode,
                interactive,
              },
              this.toolExecutor,
            );
            responseText = response.content;
            // Store tool call log in task metadata for transparency
            if (response.toolCallLog.length > 0) {
              task.metadata.toolCallLog = response.toolCallLog;
              task.metadata.toolIterations = response.iterations;
            }
          } else {
            // Plain completion — no live tools registered
            const response = await this.llm.complete({
              messages: [{ role: 'user', content: task.description }],
              soul: soul ?? undefined,
              taskId: task.id,
              agentId: this.identity.id,
              model: tierOverride ?? baseTier,
              providerOverride,
              injectBehaviors: true,
              injectMemory: task.description,
              context: toolContext ? [toolContext] : undefined,
              hatchMode,
              interactive,
            });
            responseText = response.content;
          }
          break; // success
        } catch (err) {
          if (err instanceof ModelGatedError && this.modelChoice && rounds <= 4) {
            triedGated.add(err.gatedModel);

            // Block the task while we ask the user how to proceed.
            this.taskGraph.updateStatus(task.id, 'blocked', 'Model gated — awaiting your choice');

            const options = [
              ...err.viableProviders.map((p) => ({
                action: 'switch_provider' as const,
                providerId: p.id,
                label: `Switch to ${p.name}`,
              })),
              ...err.viableTiers.map((t) => ({
                action: 'lighter_model' as const,
                tier: t as 'fast' | 'standard',
                label: `Use a lighter ${err.providerId} model`,
              })),
              ...(err.canUseApiKey
                ? [{
                    action: 'use_api_key' as const,
                    providerId: err.providerId,
                    label: `Use ${err.providerId} API key instead`,
                  }]
                : []),
            ];

            const choice = await this.modelChoice.requestChoice({
              taskId: task.id,
              agentId: this.identity.id,
              gatedModel: err.gatedModel,
              options,
            });

            // Restore in_progress before retrying.
            this.taskGraph.updateStatus(task.id, 'in_progress', 'Resuming after model choice');

            if (choice.action === 'switch_provider' && choice.providerId) {
              providerOverride = choice.providerId;
              tierOverride = undefined;
              if (choice.remember) this.modelChoice.persist(this.identity.id, { provider: choice.providerId as any });
            } else if (choice.action === 'lighter_model' && choice.tier) {
              tierOverride = choice.tier;
              providerOverride = undefined;
              if (choice.remember) this.modelChoice.persist(this.identity.id, { defaultModelTier: choice.tier });
            } else if (choice.action === 'use_api_key' && choice.apiKey) {
              const { LLMClient } = await import('../llm/index.js');
              if (err.providerId === 'anthropic') {
                LLMClient.writeEnvFile(choice.apiKey);
              } else {
                const envVar = `${err.providerId.toUpperCase()}_API_KEY`;
                LLMClient.writeEnvFileValue(envVar, choice.apiKey);
              }
              this.llm.refreshAuth();
              providerOverride = err.providerId;
              tierOverride = undefined;
            } else {
              // fallback / timeout — degrade to the lowest viable tier and stop
              // re-prompting on subsequent gates.
              tierOverride = err.viableTiers[err.viableTiers.length - 1] ?? 'fast';
              providerOverride = undefined;
            }
            continue;
          }

          // Not gated, no choice service, or out of rounds — fall through to
          // the existing error mapping.
          const msg = err instanceof Error ? err.message : String(err);
          if (msg.includes('401') || msg.includes('authentication_error') || msg.includes('Invalid bearer')) {
            responseText = `Authentication error — your Claude token may be expired. Run \`fort llm setup\` or \`claude setup-token\` to re-authenticate, then restart the portal.`;
          } else if (err instanceof ModelGatedError) {
            // Gated but we can no longer prompt (no service / exhausted rounds).
            responseText = `I encountered an error: the ${err.gatedModel} model is currently gated and I couldn't switch to an alternative. Please try again.`;
          } else {
            responseText = `I encountered an error: ${msg}. Please try again.`;
          }
          break;
        }
      }
    } else if (isChatTask) {
      responseText = this.generateBasicResponse(task.description);
    } else {
      responseText = `Task completed by ${this.identity.name}.`;
    }

    // ── Check for tool-building proposals ────────────────────────
    if (isChatTask && !isToolBuildTask) {
      const toolProposal = this.extractToolProposal(responseText);
      if (toolProposal) {
        // Decompose: create a subtask for building the tool
        const subtasks = this.taskGraph.decompose(taskId, [
          {
            title: `Build tool: ${toolProposal.toolName}`,
            description: `## Tool Proposal\n\n**Name:** ${toolProposal.toolName}\n**Description:** ${toolProposal.toolDescription}\n\n## Architecture\n${toolProposal.architecture}\n\n## Original Task\n${task.title}: ${task.description}`,
            assignedAgent: this.config.id,
          },
        ]);

        // Store proposal in memory
        this.memory.createNode({
          type: 'decision',
          label: `Tool needed: ${toolProposal.toolName}`,
          properties: {
            taskId: task.id,
            subtaskId: subtasks[0].id,
            toolName: toolProposal.toolName,
            toolDescription: toolProposal.toolDescription,
            partition: this.identity.memoryPartition,
          },
          source: `agent:${this.identity.id}`,
        });

        // Publish event so portal/UI can show the proposal
        this.bus.publish('agent.tool_proposed', this.config.id, {
          taskId: task.id,
          subtaskId: subtasks[0].id,
          toolName: toolProposal.toolName,
          toolDescription: toolProposal.toolDescription,
          architecture: toolProposal.architecture,
          agentId: this.config.id,
          agentName: this.identity.name,
        });

        // Parent stays in_progress (decompose already set this).
        // Store the response text on the task but do NOT call reviewCompletion.
        this.taskGraph.updateStatus(taskId, 'in_progress', 'Waiting for tool to be built', responseText);
        return;
      }
    }

    // Store interaction in memory
    this.memory.createNode({
      type: 'fact',
      label: isChatTask ? `Chat: ${task.title}` : `Task: ${task.title}`,
      properties: {
        taskId: task.id,
        shortId: task.shortId,
        partition: this.identity.memoryPartition,
        hasResponse: true,
      },
      source: `agent:${this.identity.id}`,
    });

    // For task-classified chats and explicit tasks, run the strict reviewer
    // (it asks "did the agent actually do what was requested?"). For
    // question-classified chats the reviewer doesn't apply — the agent's
    // job was to answer, not to execute an action — so we just mark
    // completed directly.
    const isAnsweredQuestion =
      isChatTask &&
      (task.metadata as Record<string, unknown>)?.classification === 'question';
    if (isAnsweredQuestion) {
      this.taskGraph.updateStatus(taskId, 'completed', undefined, responseText);
    } else {
      await this.taskGraph.reviewCompletion(taskId, responseText);
    }
  }

  /**
   * Load the Triager agent's SOUL.md + identity from its sibling directory
   * so its prompt + model tier drive the classifier. Returns an empty config
   * when the Triager isn't installed — the planner falls back to the built-in
   * default prompt.
   *
   * Also pulls the most recent reclassification feedback nodes from memory
   * (partition: 'triager') so the classifier sees user corrections as
   * few-shot examples.
   */
  private loadTriagerConfig(): TriagerConfig {
    try {
      const triagerDir = join(this.agentDir, '..', 'triager');
      if (!existsSync(triagerDir)) return {};

      let soul: string | undefined;
      const soulPath = join(triagerDir, 'SOUL.md');
      if (existsSync(soulPath)) {
        soul = readFileSync(soulPath, 'utf-8');
      }

      let modelTier: 'fast' | 'standard' | 'powerful' | undefined;
      const identityPath = join(triagerDir, 'identity.yaml');
      if (existsSync(identityPath)) {
        try {
          const id = parseYaml(readFileSync(identityPath, 'utf-8')) as { defaultModelTier?: 'fast' | 'standard' | 'powerful' };
          modelTier = id.defaultModelTier;
        } catch {
          // ignore malformed yaml
        }
      }

      // Pull recent feedback nodes from the Triager's memory partition. We
      // store these as 'preference' nodes with properties.partition='triager'
      // since the memory schema doesn't have a dedicated 'feedback' type yet.
      const feedback = this.memory.search({
        nodeType: 'preference',
        text: 'Reclassification',
        limit: 25,
      });
      const recentFeedback = feedback.nodes
        .filter((n) => {
          const p = n.properties as Record<string, unknown>;
          return p.partition === 'triager' && typeof p.originalMessage === 'string';
        })
        .slice(0, 5)
        .map((n) => {
          const p = n.properties as Record<string, unknown>;
          const msg = typeof p.originalMessage === 'string' ? p.originalMessage : '';
          const was = p.originalClassification === 'task' ? 'task' as const : 'question' as const;
          const shouldBe = p.correctedClassification === 'task' ? 'task' as const : 'question' as const;
          return { message: msg, was, shouldBe };
        })
        .filter((f) => f.message.length > 0);

      return { soul, modelTier, recentFeedback };
    } catch {
      return {};
    }
  }

  /**
   * Generate a basic response when LLM is not available.
   * Uses the agent's personality from SOUL.md to shape tone.
   */
  private generateBasicResponse(message: string): string {
    const msg = message.toLowerCase().trim();
    const name = this.identity.name;

    // Greeting patterns
    if (/^(hi|hello|hey|howdy|yo|sup|greetings|good\s+(morning|afternoon|evening))/.test(msg)) {
      return `Hello! I'm ${name}, ready to help. What can I do for you?`;
    }

    // Thank you
    if (/^(thanks|thank you|thx|ty)/.test(msg)) {
      return `You're welcome! Let me know if you need anything else.`;
    }

    // Status check
    if (/^(how are you|status|are you there|you up)/.test(msg)) {
      return `I'm here and operational. What would you like to work on?`;
    }

    // Help request
    if (/^(help|what can you do|capabilities)/.test(msg)) {
      const soul = this.getSoul();
      if (soul) {
        const goalsMatch = soul.match(/## Goals\n([\s\S]*?)(?=\n##|$)/);
        if (goalsMatch) {
          return `I can help with: ${goalsMatch[1].trim()}\n\nWhat would you like to start with?`;
        }
      }
      return `I'm ${name}. Send me a task and I'll get to work. For full capabilities, set up an LLM connection with \`fort llm setup\`.`;
    }

    // Default — acknowledge and create a task
    return `I've noted your request. LLM is not configured yet, so I can't provide a detailed response. Run \`fort llm setup\` to enable full conversations. In the meantime, I've tracked this as a task.`;
  }

  /**
   * Get this agent's SOUL.md contents.
   * Returns null if no SOUL.md exists.
   */
  getSoul(): string | null {
    return this._soulCache;
  }

  /**
   * Re-read SOUL.md from disk. Call this after the user edits it.
   */
  refreshSoul(): void {
    const soulPath = join(this.agentDir, 'SOUL.md');
    if (existsSync(soulPath)) {
      this._soulCache = readFileSync(soulPath, 'utf-8');
    } else {
      this._soulCache = null;
    }
  }

  /**
   * Get this agent's behavioral rules (legacy, from identity.yaml).
   * SOUL.md is the preferred mechanism for personality/rules.
   */
  getBehaviors(): string[] {
    return [...this.identity.behaviors];
  }

  /**
   * Get memories scoped to this agent's partition
   */
  getMemories(limit?: number) {
    return this.memory.search({
      text: `agent:${this.identity.id}`,
      limit,
    });
  }

  /**
   * Extract a tool-building proposal from an LLM response.
   * Looks for a JSON block with `needsTool: true`.
   */
  private extractToolProposal(response: string): {
    toolName: string;
    toolDescription: string;
    architecture: string;
  } | null {
    const jsonMatch = response.match(/\{[\s\S]*?"needsTool"\s*:\s*true[\s\S]*?\}/);
    if (!jsonMatch) return null;

    try {
      const parsed = JSON.parse(jsonMatch[0]);
      if (parsed.needsTool && parsed.toolName) {
        return {
          toolName: parsed.toolName,
          toolDescription: parsed.toolDescription ?? '',
          architecture: parsed.architecture ?? '',
        };
      }
    } catch {
      // JSON parsing failed
    }
    return null;
  }

  async handleMessage(fromAgentId: string, message: unknown): Promise<void> {
    const msg = message as { type: string; data?: unknown };

    if (msg.type === 'query') {
      const memories = this.getMemories(10);
      await this.sendMessage(fromAgentId, {
        type: 'query_response',
        agent: this.identity.name,
        soul: this._soulCache ? '(has SOUL.md)' : '(no soul)',
        memories,
      });
    }
  }
}
