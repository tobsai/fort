/**
 * OS Integration Manager — TypeScript-side hooks for macOS native integration
 *
 * Provides the core's side of the native macOS integrations. The Swift shell
 * calls these via the existing WebSocket IPC. Covers Spotlight queries,
 * Shortcuts intents, Finder file actions, voice input, and notification
 * policy evaluation.
 */

import type { Fort } from '../fort.js';
import type {
  DiagnosticResult,
  SpotlightResult,
  ShortcutActionResult,
  NotificationPolicy,
} from '../types.js';

const SHORTCUT_DISPATCH: Record<string, string> = {
  ask_fort: 'orchestrator',
  get_status: 'taskGraph',
  run_routine: 'routines',
  search_memory: 'memory',
  create_task: 'taskGraph',
};

export class OSIntegrationManager {
  private fort: Fort;
  private handledQueries = 0;
  private handledShortcuts = 0;
  private handledFiles = 0;
  private handledVoice = 0;

  constructor(fort: Fort) {
    this.fort = fort;
  }

  /**
   * Search tasks, threads, and memory. Return results formatted for Spotlight.
   */
  handleSpotlightQuery(query: string): SpotlightResult[] {
    this.handledQueries++;
    const results: SpotlightResult[] = [];
    const lowerQuery = query.toLowerCase();

    // Search tasks
    const allTasks = this.fort.taskGraph.getAllTasks();
    for (const task of allTasks) {
      if (
        task.title.toLowerCase().includes(lowerQuery) ||
        task.description.toLowerCase().includes(lowerQuery)
      ) {
        results.push({
          type: 'task',
          id: task.id,
          title: task.title,
          subtitle: `Task - ${task.status}`,
          relevance: task.title.toLowerCase().includes(lowerQuery) ? 1.0 : 0.5,
        });
      }
    }

    // Search threads
    const threads = this.fort.threads.listThreads();
    for (const thread of threads) {
      if (
        thread.name.toLowerCase().includes(lowerQuery) ||
        thread.description.toLowerCase().includes(lowerQuery)
      ) {
        results.push({
          type: 'thread',
          id: thread.id,
          title: thread.name,
          subtitle: `Thread - ${thread.status}`,
          relevance: thread.name.toLowerCase().includes(lowerQuery) ? 0.9 : 0.4,
        });
      }
    }

    // Search memory
    const memoryResult = this.fort.memory.search({ text: query, limit: 10 });
    for (const node of memoryResult.nodes) {
      results.push({
        type: 'memory',
        id: node.id,
        title: node.label,
        subtitle: `Memory - ${node.type}`,
        relevance: 0.7,
      });
    }

    // Sort by relevance descending
    results.sort((a, b) => b.relevance - a.relevance);

    return results;
  }

  /**
   * Route Shortcuts intents to the right Fort module.
   */
  async handleShortcutAction(
    intent: string,
    params: Record<string, unknown>,
  ): Promise<ShortcutActionResult> {
    this.handledShortcuts++;
    const target = SHORTCUT_DISPATCH[intent];

    if (!target) {
      return {
        success: false,
        error: `Unknown shortcut intent: ${intent}`,
      };
    }

    try {
      let result: unknown;

      switch (intent) {
        case 'ask_fort': {
          const message = (params.message as string) ?? '';
          const task = await this.fort.chat(message, 'shortcut');
          result = { taskId: task.id, result: task.result };
          break;
        }
        case 'get_status': {
          const tasks = this.fort.taskGraph.getAllTasks();
          const active = tasks.filter(
            (t) => t.status === 'in_progress' || t.status === 'created',
          );
          result = {
            totalTasks: tasks.length,
            activeTasks: active.length,
          };
          break;
        }
        case 'run_routine': {
          const routineId = params.routineId as string;
          result = await this.fort.routines.executeRoutine(routineId);
          break;
        }
        case 'search_memory': {
          const query = (params.query as string) ?? '';
          result = this.fort.memory.search({ text: query });
          break;
        }
        case 'create_task': {
          const title = (params.title as string) ?? 'Untitled task';
          const task = this.fort.taskGraph.createTask({
            title,
            description: (params.description as string) ?? '',
            source: 'user_chat',
          });
          result = { taskId: task.id, title: task.title };
          break;
        }
      }

      return {
        success: true,
        intent,
        target,
        data: result,
      };
    } catch (err) {
      return {
        success: false,
        error: err instanceof Error ? err.message : String(err),
      };
    }
  }

  /**
   * Handle "Send to Fort" from Finder. Creates a task for file analysis.
   */
  handleFileAction(filePaths: string[]): { taskId: string; fileCount: number } {
    this.handledFiles++;

    const task = this.fort.taskGraph.createTask({
      title: `Analyze ${filePaths.length} file(s)`,
      description: `Files sent from Finder:\n${filePaths.join('\n')}`,
      source: 'user_chat',
      metadata: { filePaths, source: 'finder' },
    });

    this.fort.bus.publish('os.file_action', 'os-integration', {
      taskId: task.id,
      filePaths,
    });

    return { taskId: task.id, fileCount: filePaths.length };
  }

  /**
   * Process voice transcription as a user message. Creates a task and routes
   * to the orchestrator.
   */
  async handleVoiceInput(transcript: string): Promise<{ taskId: string; response: string }> {
    this.handledVoice++;

    const task = this.fort.taskGraph.createTask({
      title: `Voice: ${transcript.slice(0, 50)}${transcript.length > 50 ? '...' : ''}`,
      description: transcript,
      source: 'user_chat',
      metadata: { source: 'voice' },
    });

    this.fort.bus.publish('os.voice_input', 'os-integration', {
      taskId: task.id,
      transcript,
    });

    const chatTask = await this.fort.chat(transcript, 'voice');

    return { taskId: chatTask.id, response: chatTask.result ?? '' };
  }

  /**
   * Returns whether a notification should be sent based on focus mode rules.
   * Default policy: suppress non-critical during DND.
   */
  getNotificationPolicy(
    category: string,
    focusMode: string | null,
  ): NotificationPolicy {
    const criticalCategories = ['error', 'security', 'critical'];
    const isCritical = criticalCategories.includes(category);

    if (focusMode === 'dnd' || focusMode === 'do_not_disturb') {
      return {
        shouldSend: isCritical,
        reason: isCritical
          ? 'Critical notifications bypass DND'
          : `Suppressed during ${focusMode}`,
        category,
        focusMode,
      };
    }

    if (focusMode === 'focus') {
      // Focus mode: allow critical + task-related
      const allowedInFocus = [...criticalCategories, 'task'];
      return {
        shouldSend: allowedInFocus.includes(category),
        reason: allowedInFocus.includes(category)
          ? 'Allowed during focus mode'
          : 'Suppressed during focus mode',
        category,
        focusMode,
      };
    }

    // No focus mode — allow everything
    return {
      shouldSend: true,
      reason: 'No focus mode active',
      category,
      focusMode,
    };
  }

  diagnose(): DiagnosticResult {
    return {
      module: 'os-integration',
      status: 'healthy',
      checks: [
        {
          name: 'Spotlight handler',
          passed: true,
          message: `${this.handledQueries} queries handled`,
        },
        {
          name: 'Shortcut handler',
          passed: true,
          message: `${this.handledShortcuts} shortcuts handled`,
        },
        {
          name: 'File action handler',
          passed: true,
          message: `${this.handledFiles} file actions handled`,
        },
        {
          name: 'Voice input handler',
          passed: true,
          message: `${this.handledVoice} voice inputs handled`,
        },
        {
          name: 'Supported intents',
          passed: true,
          message: `${Object.keys(SHORTCUT_DISPATCH).length} intents registered: ${Object.keys(SHORTCUT_DISPATCH).join(', ')}`,
        },
      ],
    };
  }
}
