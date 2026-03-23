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
import { BaseAgent } from './index.js';
import type { AgentConfig, SpecialistIdentity } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { MemoryManager } from '../memory/index.js';
import type { LLMClient } from '../llm/index.js';
import type { ToolRegistry } from '../tools/index.js';

export class SpecialistAgent extends BaseAgent {
  readonly identity: SpecialistIdentity;
  readonly agentDir: string;
  private memory: MemoryManager;
  private llm: LLMClient | null = null;
  private toolRegistry: ToolRegistry | null = null;
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
      try {
        const soul = this.getSoul();
        const modelTier = (task.metadata.modelTier as string) || this.identity.defaultModelTier;
        const response = await this.llm.complete({
          messages: [{ role: 'user', content: task.description }],
          soul: soul ?? undefined,
          taskId: task.id,
          agentId: this.identity.id,
          model: modelTier,
          injectBehaviors: true,
          injectMemory: task.description,
          context: toolContext ? [toolContext] : undefined,
        });
        responseText = response.content;
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        if (msg.includes('401') || msg.includes('authentication_error') || msg.includes('Invalid bearer')) {
          responseText = `Authentication error — your Claude token may be expired. Run \`fort llm setup\` or \`claude setup-token\` to re-authenticate, then restart the portal.`;
        } else {
          responseText = `I encountered an error: ${msg}. Please try again.`;
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

    // Review completion with LLM before marking done
    await this.taskGraph.reviewCompletion(taskId, responseText);
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
