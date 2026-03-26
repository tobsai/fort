/**
 * UsageTracker — subscribes to usage.recorded events and persists to UsageStore.
 *
 * Passive subscriber: disabling this has zero effect on other functionality.
 * Feature flag: FORT_USAGE_TRACKING=false disables it.
 */

import type { ModuleBus } from '../module-bus/index.js';
import { UsageStore } from './store.js';
import type { DiagnosticResult } from '../types.js';

export class UsageTracker {
  private store: UsageStore;
  private bus: ModuleBus;
  private enabled: boolean;
  private unsubscribe: (() => void) | null = null;
  private recordedCount = 0;
  private errorCount = 0;

  constructor(store: UsageStore, bus: ModuleBus) {
    this.store = store;
    this.bus = bus;
    this.enabled = process.env.FORT_USAGE_TRACKING !== 'false';
  }

  start(): void {
    if (!this.enabled) return;

    this.unsubscribe = this.bus.subscribe('usage.recorded', (event) => {
      try {
        const payload = event.payload as {
          taskId: string;
          agentId: string;
          model: string;
          inputTokens: number;
          outputTokens: number;
          cacheReadTokens?: number;
          cacheWriteTokens?: number;
        };
        this.store.recordUsage({
          taskId: payload.taskId,
          agentId: payload.agentId,
          model: payload.model,
          inputTokens: payload.inputTokens ?? 0,
          outputTokens: payload.outputTokens ?? 0,
          cacheReadTokens: payload.cacheReadTokens ?? 0,
          cacheWriteTokens: payload.cacheWriteTokens ?? 0,
        });
        this.recordedCount++;
      } catch {
        this.errorCount++;
      }
    });
  }

  stop(): void {
    if (this.unsubscribe) {
      this.unsubscribe();
      this.unsubscribe = null;
    }
  }

  getStore(): UsageStore {
    return this.store;
  }

  diagnose(): DiagnosticResult {
    return {
      module: 'usage-tracker',
      status: 'healthy',
      checks: [
        {
          name: 'Enabled',
          passed: this.enabled,
          message: this.enabled ? 'Usage tracking active' : 'Disabled via FORT_USAGE_TRACKING=false',
        },
        {
          name: 'Events recorded',
          passed: true,
          message: `${this.recordedCount} usage events persisted (${this.errorCount} errors)`,
        },
      ],
    };
  }
}
