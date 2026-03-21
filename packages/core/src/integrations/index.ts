/**
 * Integration Registry — Manages all Fort integration modules
 *
 * Central registry for registering, initializing, diagnosing,
 * and shutting down all integration wrappers.
 */

import type { DiagnosticResult } from '../types.js';
import type { Integration, IntegrationStatus } from './base.js';

export interface IntegrationInfo {
  id: string;
  name: string;
  status: IntegrationStatus;
}

export class IntegrationRegistry {
  private integrations: Map<string, Integration> = new Map();

  register(integration: Integration): void {
    if (this.integrations.has(integration.id)) {
      throw new Error(`Integration "${integration.id}" is already registered.`);
    }
    this.integrations.set(integration.id, integration);
  }

  get(id: string): Integration | undefined {
    return this.integrations.get(id);
  }

  list(): IntegrationInfo[] {
    return Array.from(this.integrations.values()).map((i) => ({
      id: i.id,
      name: i.name,
      status: i.status,
    }));
  }

  async initializeAll(): Promise<Map<string, Error | null>> {
    const results = new Map<string, Error | null>();

    for (const [id, integration] of this.integrations) {
      try {
        await integration.initialize();
        results.set(id, null);
      } catch (err) {
        results.set(id, err instanceof Error ? err : new Error(String(err)));
      }
    }

    return results;
  }

  diagnoseAll(): DiagnosticResult[] {
    const results: DiagnosticResult[] = [];

    for (const [id, integration] of this.integrations) {
      try {
        results.push(integration.diagnose());
      } catch (err) {
        results.push({
          module: id,
          status: 'unhealthy',
          checks: [{
            name: 'Diagnostics execution',
            passed: false,
            message: `diagnose() failed: ${err instanceof Error ? err.message : String(err)}`,
          }],
        });
      }
    }

    return results;
  }

  async shutdownAll(): Promise<void> {
    const errors: Array<{ id: string; error: Error }> = [];

    for (const [id, integration] of this.integrations) {
      try {
        await integration.shutdown();
      } catch (err) {
        errors.push({
          id,
          error: err instanceof Error ? err : new Error(String(err)),
        });
      }
    }

    if (errors.length > 0) {
      const messages = errors.map((e) => `${e.id}: ${e.error.message}`).join('; ');
      throw new Error(`Errors during shutdown: ${messages}`);
    }
  }

  size(): number {
    return this.integrations.size;
  }
}

// Re-export everything
export { BaseIntegration } from './base.js';
export type { Integration, IntegrationStatus, IntegrationConfig } from './base.js';
export { GmailIntegration } from './gmail.js';
export type { GmailConfig, GmailMessage, GmailDraft, GmailLabel } from './gmail.js';
export { CalendarIntegration } from './calendar.js';
export type { CalendarConfig, CalendarEvent, FreeTimeSlot } from './calendar.js';
export { IMessageIntegration } from './imessage.js';
export type { IMessageConfig, IMessage, IMessageConversation } from './imessage.js';
export { BraveSearchIntegration } from './brave-search.js';
export type { BraveSearchConfig, SearchResult, SearchResponse, SearchSummary } from './brave-search.js';
export { BrowserIntegration } from './browser.js';
export type { BrowserConfig, PageContent, BrowserAction, ActionRisk } from './browser.js';
