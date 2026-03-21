/**
 * iMessage Integration — macOS native messaging via AppleScript
 *
 * Sends messages via osascript, reads via chat.db SQLite access.
 * Enforces a configurable recipient allowlist for safety.
 * Sending requires Tier 2 permission (compose_imessage).
 */

import { execSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import type { DiagnosticResult } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { PermissionManager } from '../permissions/index.js';
import { BaseIntegration } from './base.js';

export interface IMessageConfig {
  enabled: boolean;
  allowedRecipients: string[];
  chatDbPath?: string;
}

export interface IMessage {
  id: string;
  text: string;
  sender: string;
  recipient: string;
  date: Date;
  isFromMe: boolean;
}

export interface IMessageConversation {
  chatId: string;
  recipient: string;
  displayName: string;
  lastMessageDate: Date;
  messageCount: number;
}

const DEFAULT_CHAT_DB = `${process.env.HOME}/Library/Messages/chat.db`;

export class IMessageIntegration extends BaseIntegration {
  id = 'imessage';
  name = 'iMessage';

  private config: IMessageConfig;
  private applescriptAvailable = false;
  private chatDbAccessible = false;

  constructor(bus: ModuleBus, permissions: PermissionManager, config: IMessageConfig) {
    super(bus, permissions);
    this.config = config;
  }

  async initialize(): Promise<void> {
    if (!this.config.enabled) {
      this.status = 'disconnected';
      return;
    }

    try {
      // Check AppleScript availability
      this.applescriptAvailable = this.checkAppleScript();

      // Check chat.db access
      const dbPath = this.config.chatDbPath || DEFAULT_CHAT_DB;
      this.chatDbAccessible = existsSync(dbPath) && this.checkChatDbAccess(dbPath);

      if (this.applescriptAvailable || this.chatDbAccessible) {
        this.status = 'connected';
      } else {
        this.status = 'error';
      }

      await this.publishEvent('integration.initialized', {
        integration: this.id,
        status: this.status,
        applescriptAvailable: this.applescriptAvailable,
        chatDbAccessible: this.chatDbAccessible,
      });
    } catch (err) {
      this.status = 'error';
      await this.publishEvent('integration.error', {
        integration: this.id,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async sendMessage(recipient: string, text: string): Promise<void> {
    this.ensureConfigured();

    // Check allowlist
    if (!this.isRecipientAllowed(recipient)) {
      throw new Error(
        `Recipient "${recipient}" is not in the allowlist. ` +
        `Allowed recipients: ${this.config.allowedRecipients.join(', ')}`
      );
    }

    // Tier 2 — compose_imessage
    const permission = this.checkPermission('compose_imessage');
    if (permission.requiresApproval) {
      throw new Error(
        `iMessage sendMessage requires approval: ${permission.description}. ` +
        `Message to "${recipient}" has been logged for review.`
      );
    }

    if (!this.applescriptAvailable) {
      throw new Error('AppleScript is not available. Cannot send iMessages on this system.');
    }

    await this.publishEvent('imessage.sendMessage', { recipient, textLength: text.length });

    try {
      // Sanitize input to prevent AppleScript injection
      const safeRecipient = this.sanitizeForAppleScript(recipient);
      const safeText = this.sanitizeForAppleScript(text);

      const script = `
        tell application "Messages"
          set targetService to 1st account whose service type = iMessage
          set targetBuddy to participant "${safeRecipient}" of targetService
          send "${safeText}" to targetBuddy
        end tell
      `;

      execSync(`osascript -e '${script.replace(/'/g, "'\\''")}'`, {
        timeout: 10000,
        encoding: 'utf-8',
      });

      await this.publishEvent('imessage.sendMessage.complete', {
        recipient,
        textLength: text.length,
      });
    } catch (err) {
      await this.publishEvent('imessage.sendMessage.error', {
        recipient,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async readRecent(recipient?: string, limit: number = 20): Promise<IMessage[]> {
    this.ensureConfigured();

    const permission = this.checkPermission('read_email'); // read_email covers message reading
    if (!permission.allowed && permission.requiresApproval) {
      throw new Error(`iMessage readRecent requires approval: ${permission.description}`);
    }

    if (!this.chatDbAccessible) {
      throw new Error(
        'Cannot access chat.db. Ensure Full Disk Access is granted to the running process.'
      );
    }

    await this.publishEvent('imessage.readRecent', { recipient, limit });

    try {
      const dbPath = this.config.chatDbPath || DEFAULT_CHAT_DB;
      let query = `
        SELECT
          m.ROWID as id,
          m.text,
          h.id as handle_id,
          m.date as msg_date,
          m.is_from_me
        FROM message m
        LEFT JOIN handle h ON m.handle_id = h.ROWID
      `;

      if (recipient) {
        const safeRecipient = recipient.replace(/'/g, "''");
        query += ` WHERE h.id = '${safeRecipient}'`;
      }

      query += ` ORDER BY m.date DESC LIMIT ${limit}`;

      const result = execSync(
        `sqlite3 -json "${dbPath}" "${query.replace(/"/g, '\\"')}"`,
        { timeout: 10000, encoding: 'utf-8' }
      );

      let rows: Array<Record<string, unknown>> = [];
      try {
        rows = JSON.parse(result || '[]');
      } catch {
        rows = [];
      }

      const messages: IMessage[] = rows.map((row) => ({
        id: String(row.id),
        text: (row.text as string) || '',
        sender: row.is_from_me ? 'me' : (row.handle_id as string) || 'unknown',
        recipient: row.is_from_me ? (row.handle_id as string) || 'unknown' : 'me',
        date: this.parseCoreDataTimestamp(row.msg_date as number),
        isFromMe: !!(row.is_from_me),
      }));

      await this.publishEvent('imessage.readRecent.complete', {
        count: messages.length,
        recipient,
      });

      return messages;
    } catch (err) {
      await this.publishEvent('imessage.readRecent.error', {
        recipient,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async getConversations(): Promise<IMessageConversation[]> {
    this.ensureConfigured();

    if (!this.chatDbAccessible) {
      throw new Error(
        'Cannot access chat.db. Ensure Full Disk Access is granted to the running process.'
      );
    }

    await this.publishEvent('imessage.getConversations', {});

    try {
      const dbPath = this.config.chatDbPath || DEFAULT_CHAT_DB;
      const query = `
        SELECT
          c.ROWID as chat_id,
          c.chat_identifier as recipient,
          c.display_name,
          MAX(m.date) as last_date,
          COUNT(m.ROWID) as msg_count
        FROM chat c
        LEFT JOIN chat_message_join cmj ON c.ROWID = cmj.chat_id
        LEFT JOIN message m ON cmj.message_id = m.ROWID
        GROUP BY c.ROWID
        ORDER BY last_date DESC
      `;

      const result = execSync(
        `sqlite3 -json "${dbPath}" "${query.replace(/"/g, '\\"')}"`,
        { timeout: 10000, encoding: 'utf-8' }
      );

      let rows: Array<Record<string, unknown>> = [];
      try {
        rows = JSON.parse(result || '[]');
      } catch {
        rows = [];
      }

      const conversations: IMessageConversation[] = rows.map((row) => ({
        chatId: String(row.chat_id),
        recipient: (row.recipient as string) || '',
        displayName: (row.display_name as string) || (row.recipient as string) || '',
        lastMessageDate: this.parseCoreDataTimestamp(row.last_date as number),
        messageCount: (row.msg_count as number) || 0,
      }));

      await this.publishEvent('imessage.getConversations.complete', {
        count: conversations.length,
      });

      return conversations;
    } catch (err) {
      await this.publishEvent('imessage.getConversations.error', {
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  isRecipientAllowed(recipient: string): boolean {
    return this.config.allowedRecipients.some(
      (allowed) => allowed.toLowerCase() === recipient.toLowerCase()
    );
  }

  diagnose(): DiagnosticResult {
    const checks = [];

    checks.push({
      name: 'Integration enabled',
      passed: this.config.enabled,
      message: this.config.enabled ? 'iMessage integration is enabled' : 'iMessage integration is disabled in config',
    });

    checks.push({
      name: 'macOS platform',
      passed: process.platform === 'darwin',
      message: process.platform === 'darwin'
        ? 'Running on macOS'
        : `Running on ${process.platform} — iMessage requires macOS`,
    });

    checks.push({
      name: 'AppleScript available',
      passed: this.applescriptAvailable,
      message: this.applescriptAvailable
        ? 'osascript is available for sending messages'
        : 'osascript is not available. Ensure macOS and Messages.app are configured.',
    });

    const dbPath = this.config.chatDbPath || DEFAULT_CHAT_DB;
    checks.push({
      name: 'chat.db accessible',
      passed: this.chatDbAccessible,
      message: this.chatDbAccessible
        ? `chat.db at ${dbPath} is accessible`
        : `Cannot access ${dbPath}. Grant Full Disk Access to read message history.`,
    });

    checks.push({
      name: 'Recipient allowlist configured',
      passed: this.config.allowedRecipients.length > 0,
      message: this.config.allowedRecipients.length > 0
        ? `${this.config.allowedRecipients.length} allowed recipient(s) configured`
        : 'No allowed recipients configured. Add recipients to the allowlist.',
      details: { allowedRecipients: this.config.allowedRecipients },
    });

    const allPassed = checks.every((c) => c.passed);
    const somePassed = checks.some((c) => c.passed);

    return {
      module: 'imessage',
      status: allPassed ? 'healthy' : somePassed ? 'degraded' : 'unhealthy',
      checks,
    };
  }

  private ensureConfigured(): void {
    if (!this.config.enabled) {
      throw new Error('iMessage integration is not enabled. Enable it in config.');
    }
  }

  private checkAppleScript(): boolean {
    try {
      execSync('which osascript', { timeout: 5000, encoding: 'utf-8' });
      return true;
    } catch {
      return false;
    }
  }

  private checkChatDbAccess(dbPath: string): boolean {
    try {
      execSync(`sqlite3 "${dbPath}" "SELECT 1 FROM message LIMIT 1"`, {
        timeout: 5000,
        encoding: 'utf-8',
      });
      return true;
    } catch {
      return false;
    }
  }

  private sanitizeForAppleScript(input: string): string {
    // Escape backslashes, double quotes, and prevent AppleScript injection
    return input
      .replace(/\\/g, '\\\\')
      .replace(/"/g, '\\"')
      .replace(/\n/g, '\\n')
      .replace(/\r/g, '\\r');
  }

  /**
   * macOS Messages stores dates as Core Data timestamps:
   * seconds since 2001-01-01 00:00:00 UTC, sometimes in nanoseconds.
   */
  private parseCoreDataTimestamp(timestamp: number): Date {
    if (!timestamp) return new Date(0);
    // Core Data epoch: 2001-01-01T00:00:00Z
    const coreDataEpoch = 978307200;
    // If timestamp > 1e15, it's in nanoseconds
    const seconds = timestamp > 1e15 ? timestamp / 1e9 : timestamp;
    return new Date((seconds + coreDataEpoch) * 1000);
  }
}
