/**
 * Integration Base — Standard interface for all Fort integrations
 *
 * Every integration wraps an external service with a deterministic interface,
 * safety checks, and a diagnose() method for inspectability.
 */

import type { DiagnosticResult } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { PermissionManager } from '../permissions/index.js';

export type IntegrationStatus = 'connected' | 'disconnected' | 'error';

export interface Integration {
  id: string;
  name: string;
  status: IntegrationStatus;
  initialize(): Promise<void>;
  diagnose(): DiagnosticResult;
  shutdown(): Promise<void>;
}

export interface IntegrationConfig {
  enabled: boolean;
  [key: string]: unknown;
}

export abstract class BaseIntegration implements Integration {
  abstract id: string;
  abstract name: string;
  status: IntegrationStatus = 'disconnected';

  protected bus: ModuleBus;
  protected permissions: PermissionManager;

  constructor(bus: ModuleBus, permissions: PermissionManager) {
    this.bus = bus;
    this.permissions = permissions;
  }

  abstract initialize(): Promise<void>;
  abstract diagnose(): DiagnosticResult;

  async shutdown(): Promise<void> {
    this.status = 'disconnected';
    await this.bus.publish('integration.shutdown', this.id, { integration: this.id });
  }

  protected async publishEvent(eventType: string, payload: Record<string, unknown>): Promise<void> {
    await this.bus.publish(eventType, this.id, payload);
  }

  protected checkPermission(action: string): { allowed: boolean; requiresApproval: boolean; description: string } {
    const result = this.permissions.checkAction(action);
    return { allowed: result.allowed, requiresApproval: result.requiresApproval, description: result.description };
  }
}
