/**
 * Gmail Integration — Email access with draft-first safety model
 *
 * Composing always creates drafts. Sending requires Tier 3 approval.
 * All operations publish events on ModuleBus for full traceability.
 */

import type { DiagnosticResult } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { PermissionManager } from '../permissions/index.js';
import { BaseIntegration } from './base.js';

export interface GmailConfig {
  enabled: boolean;
  clientId?: string;
  clientSecret?: string;
  refreshToken?: string;
}

export interface GmailMessage {
  id: string;
  threadId: string;
  from: string;
  to: string[];
  subject: string;
  body: string;
  date: Date;
  labels: string[];
  snippet: string;
}

export interface GmailDraft {
  id: string;
  to: string[];
  subject: string;
  body: string;
  createdAt: Date;
}

export interface GmailLabel {
  id: string;
  name: string;
  type: string;
}

export class GmailIntegration extends BaseIntegration {
  id = 'gmail';
  name = 'Gmail';

  private config: GmailConfig;
  private tokenValid = false;

  constructor(bus: ModuleBus, permissions: PermissionManager, config: GmailConfig) {
    super(bus, permissions);
    this.config = config;
  }

  async initialize(): Promise<void> {
    if (!this.config.enabled) {
      this.status = 'disconnected';
      return;
    }

    if (!this.config.clientId || !this.config.clientSecret || !this.config.refreshToken) {
      this.status = 'error';
      await this.publishEvent('integration.error', {
        integration: this.id,
        error: 'Missing OAuth2 credentials. Configure clientId, clientSecret, and refreshToken.',
      });
      return;
    }

    try {
      // Validate token by attempting a lightweight API call
      this.tokenValid = await this.validateToken();
      this.status = this.tokenValid ? 'connected' : 'error';
      await this.publishEvent('integration.initialized', {
        integration: this.id,
        status: this.status,
      });
    } catch (err) {
      this.status = 'error';
      await this.publishEvent('integration.error', {
        integration: this.id,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async listMessages(query: string, maxResults: number = 10): Promise<GmailMessage[]> {
    this.ensureConfigured();

    const permission = this.checkPermission('read_email');
    if (!permission.allowed && permission.requiresApproval) {
      throw new Error(`Gmail listMessages requires approval: ${permission.description}`);
    }

    await this.publishEvent('gmail.listMessages', { query, maxResults });

    try {
      const response = await this.apiRequest<{ messages?: Array<{ id: string; threadId: string }> }>(
        `https://gmail.googleapis.com/gmail/v1/users/me/messages?q=${encodeURIComponent(query)}&maxResults=${maxResults}`
      );

      if (!response.messages || response.messages.length === 0) {
        return [];
      }

      const messages: GmailMessage[] = [];
      for (const msg of response.messages) {
        const full = await this.readMessage(msg.id);
        messages.push(full);
      }

      await this.publishEvent('gmail.listMessages.complete', {
        query,
        count: messages.length,
      });

      return messages;
    } catch (err) {
      await this.publishEvent('gmail.listMessages.error', {
        query,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async readMessage(messageId: string): Promise<GmailMessage> {
    this.ensureConfigured();

    const permission = this.checkPermission('read_email');
    if (!permission.allowed && permission.requiresApproval) {
      throw new Error(`Gmail readMessage requires approval: ${permission.description}`);
    }

    await this.publishEvent('gmail.readMessage', { messageId });

    try {
      const raw = await this.apiRequest<Record<string, unknown>>(
        `https://gmail.googleapis.com/gmail/v1/users/me/messages/${messageId}?format=full`
      );

      const message = this.parseMessage(raw);
      await this.publishEvent('gmail.readMessage.complete', { messageId });
      return message;
    } catch (err) {
      await this.publishEvent('gmail.readMessage.error', {
        messageId,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async createDraft(to: string[], subject: string, body: string): Promise<GmailDraft> {
    this.ensureConfigured();

    // Draft creation is Tier 2 — compose_email
    const permission = this.checkPermission('compose_email');
    if (!permission.allowed && permission.requiresApproval) {
      throw new Error(`Gmail createDraft requires approval: ${permission.description}`);
    }

    await this.publishEvent('gmail.createDraft', { to, subject });

    try {
      const raw = this.buildRawEmail(to, subject, body);
      const response = await this.apiRequest<{ id: string }>(
        'https://gmail.googleapis.com/gmail/v1/users/me/drafts',
        {
          method: 'POST',
          body: JSON.stringify({
            message: { raw },
          }),
        }
      );

      const draft: GmailDraft = {
        id: response.id,
        to,
        subject,
        body,
        createdAt: new Date(),
      };

      await this.publishEvent('gmail.createDraft.complete', {
        draftId: draft.id,
        to,
        subject,
      });

      return draft;
    } catch (err) {
      await this.publishEvent('gmail.createDraft.error', {
        to,
        subject,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async sendDraft(draftId: string): Promise<void> {
    this.ensureConfigured();

    // Sending requires Tier 3 — send_message
    const permission = this.checkPermission('send_message');
    if (permission.requiresApproval) {
      throw new Error(
        `Gmail sendDraft requires Tier 3 approval: ${permission.description}. ` +
        `Draft ${draftId} has been preserved and can be sent after approval.`
      );
    }

    await this.publishEvent('gmail.sendDraft', { draftId });

    try {
      await this.apiRequest(
        `https://gmail.googleapis.com/gmail/v1/users/me/drafts/send`,
        {
          method: 'POST',
          body: JSON.stringify({ id: draftId }),
        }
      );

      await this.publishEvent('gmail.sendDraft.complete', { draftId });
    } catch (err) {
      await this.publishEvent('gmail.sendDraft.error', {
        draftId,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async listLabels(): Promise<GmailLabel[]> {
    this.ensureConfigured();

    const permission = this.checkPermission('read_email');
    if (!permission.allowed && permission.requiresApproval) {
      throw new Error(`Gmail listLabels requires approval: ${permission.description}`);
    }

    await this.publishEvent('gmail.listLabels', {});

    try {
      const response = await this.apiRequest<{ labels: Array<{ id: string; name: string; type: string }> }>(
        'https://gmail.googleapis.com/gmail/v1/users/me/labels'
      );

      const labels: GmailLabel[] = (response.labels || []).map((l) => ({
        id: l.id,
        name: l.name,
        type: l.type,
      }));

      await this.publishEvent('gmail.listLabels.complete', { count: labels.length });
      return labels;
    } catch (err) {
      await this.publishEvent('gmail.listLabels.error', {
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  diagnose(): DiagnosticResult {
    const checks = [];

    // Check configuration
    const hasCredentials = !!(this.config.clientId && this.config.clientSecret && this.config.refreshToken);
    checks.push({
      name: 'OAuth2 credentials configured',
      passed: hasCredentials,
      message: hasCredentials
        ? 'OAuth2 credentials are present'
        : 'Missing OAuth2 credentials. Set clientId, clientSecret, and refreshToken in config.',
    });

    // Check enabled
    checks.push({
      name: 'Integration enabled',
      passed: this.config.enabled,
      message: this.config.enabled ? 'Gmail integration is enabled' : 'Gmail integration is disabled in config',
    });

    // Check token validity
    checks.push({
      name: 'OAuth token valid',
      passed: this.tokenValid,
      message: this.tokenValid
        ? 'OAuth token is valid and can access Gmail API'
        : 'OAuth token has not been validated. Run initialize() first or check credentials.',
    });

    // Check connection status
    checks.push({
      name: 'Connection status',
      passed: this.status === 'connected',
      message: `Current status: ${this.status}`,
    });

    const allPassed = checks.every((c) => c.passed);
    const somePassed = checks.some((c) => c.passed);

    return {
      module: 'gmail',
      status: allPassed ? 'healthy' : somePassed ? 'degraded' : 'unhealthy',
      checks,
    };
  }

  private ensureConfigured(): void {
    if (!this.config.enabled) {
      throw new Error('Gmail integration is not enabled. Enable it in config.');
    }
    if (!this.config.clientId || !this.config.clientSecret || !this.config.refreshToken) {
      throw new Error(
        'Gmail integration is not configured. Provide clientId, clientSecret, and refreshToken.'
      );
    }
  }

  private async validateToken(): Promise<boolean> {
    try {
      await this.apiRequest('https://gmail.googleapis.com/gmail/v1/users/me/profile');
      return true;
    } catch {
      return false;
    }
  }

  private async apiRequest<T>(url: string, options: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      'Authorization': `Bearer ${this.config.refreshToken}`,
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string> || {}),
    };

    const response = await fetch(url, { ...options, headers });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(`Gmail API error (${response.status}): ${errorBody}`);
    }

    return response.json() as Promise<T>;
  }

  private parseMessage(raw: Record<string, unknown>): GmailMessage {
    const headers = ((raw as Record<string, unknown>).payload as Record<string, unknown>)?.headers as Array<{ name: string; value: string }> || [];
    const getHeader = (name: string) => headers.find((h) => h.name.toLowerCase() === name.toLowerCase())?.value || '';

    return {
      id: raw.id as string,
      threadId: raw.threadId as string,
      from: getHeader('from'),
      to: getHeader('to').split(',').map((s: string) => s.trim()).filter(Boolean),
      subject: getHeader('subject'),
      body: this.extractBody(raw),
      date: new Date(getHeader('date')),
      labels: (raw.labelIds as string[]) || [],
      snippet: (raw.snippet as string) || '',
    };
  }

  private extractBody(raw: Record<string, unknown>): string {
    // Simplified body extraction — real implementation would handle MIME parts
    const payload = raw.payload as Record<string, unknown> | undefined;
    if (!payload) return '';

    const body = payload.body as { data?: string } | undefined;
    if (body?.data) {
      return Buffer.from(body.data, 'base64url').toString('utf-8');
    }

    const parts = payload.parts as Array<Record<string, unknown>> | undefined;
    if (parts) {
      for (const part of parts) {
        const mimeType = part.mimeType as string;
        if (mimeType === 'text/plain') {
          const partBody = part.body as { data?: string } | undefined;
          if (partBody?.data) {
            return Buffer.from(partBody.data, 'base64url').toString('utf-8');
          }
        }
      }
    }

    return '';
  }

  private buildRawEmail(to: string[], subject: string, body: string): string {
    const email = [
      `To: ${to.join(', ')}`,
      `Subject: ${subject}`,
      'Content-Type: text/plain; charset=utf-8',
      '',
      body,
    ].join('\r\n');

    return Buffer.from(email).toString('base64url');
  }
}
