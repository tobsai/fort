/**
 * LLMProviderStore — SQLite-backed persistence for LLM provider configuration.
 *
 * API keys are encrypted at rest with AES-256-GCM using a PBKDF2-derived key.
 * This protects against accidental log/backup exposure, not enterprise threats.
 *
 * Encrypted blobs are stored as: iv_hex:authTag_hex:ciphertext_hex
 */

import Database from 'better-sqlite3';
import { createCipheriv, createDecipheriv, pbkdf2Sync, randomBytes } from 'node:crypto';

// ─── Types ───────────────────────────────────────────────────────────

export type ProviderType = 'anthropic' | 'openai' | 'groq' | 'ollama';

export interface ProviderConfig {
  id: string;
  name: string;
  apiKey?: string;       // plaintext — encrypted before storage
  baseUrl?: string;
  defaultModel: string;
  enabled?: boolean;
  isDefault?: boolean;
}

export interface LLMProvider {
  id: string;
  name: string;
  apiKeyEncrypted: string | null;
  baseUrl: string | null;
  defaultModel: string;
  enabled: boolean;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
}

/** Provider with decrypted API key — for runtime use only, never serialise */
export interface LLMProviderRuntime extends LLMProvider {
  apiKey: string | null;
}

// ─── Static Provider Metadata ────────────────────────────────────────

export const PROVIDER_MODELS: Record<ProviderType, string[]> = {
  anthropic: ['claude-opus-4-6', 'claude-sonnet-4-5-20250929', 'claude-haiku-4-5-20251001'],
  openai: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo'],
  groq: ['llama-3.3-70b-versatile', 'llama-3.1-8b-instant'],
  ollama: [],
};

export const PROVIDER_DEFAULTS: Record<ProviderType, { name: string; defaultModel: string; baseUrl?: string }> = {
  anthropic: { name: 'Anthropic', defaultModel: 'claude-sonnet-4-5-20250929' },
  openai:    { name: 'OpenAI',    defaultModel: 'gpt-4o',                    baseUrl: 'https://api.openai.com/v1' },
  groq:      { name: 'Groq',      defaultModel: 'llama-3.3-70b-versatile',   baseUrl: 'https://api.groq.com/openai/v1' },
  ollama:    { name: 'Ollama',    defaultModel: 'llama3',                    baseUrl: 'http://localhost:11434' },
};

// ─── Store ───────────────────────────────────────────────────────────

export class LLMProviderStore {
  private db: InstanceType<typeof Database>;
  private key: Buffer;

  constructor(dbPath: string, encryptionKey: string) {
    this.db = new (Database as any)(dbPath) as InstanceType<typeof Database>;
    (this.db as any).pragma('journal_mode = WAL');
    this.key = pbkdf2Sync(encryptionKey, 'fort-llm-salt', 100000, 32, 'sha256');
    this.initSchema();
  }

  initSchema(): void {
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS llm_providers (
        id                 TEXT PRIMARY KEY,
        name               TEXT NOT NULL,
        api_key_encrypted  TEXT,
        base_url           TEXT,
        default_model      TEXT NOT NULL,
        enabled            INTEGER DEFAULT 1,
        is_default         INTEGER DEFAULT 0,
        created_at         TEXT DEFAULT (datetime('now')),
        updated_at         TEXT DEFAULT (datetime('now'))
      )
    `);
  }

  addProvider(config: ProviderConfig): LLMProvider {
    const now = new Date().toISOString();
    const encrypted = config.apiKey ? this.encrypt(config.apiKey) : null;

    if (config.isDefault) {
      (this.db.prepare('UPDATE llm_providers SET is_default = 0') as any).run();
    }

    (this.db.prepare(`
      INSERT INTO llm_providers
        (id, name, api_key_encrypted, base_url, default_model, enabled, is_default, created_at, updated_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `) as any).run(
      config.id,
      config.name,
      encrypted,
      config.baseUrl ?? null,
      config.defaultModel,
      config.enabled !== false ? 1 : 0,
      config.isDefault ? 1 : 0,
      now,
      now,
    );

    return this.getProvider(config.id)!;
  }

  updateProvider(id: string, updates: Partial<ProviderConfig>): LLMProvider {
    if (!this.getProvider(id)) throw new Error(`Provider not found: ${id}`);

    const now = new Date().toISOString();
    const fields: string[] = ['updated_at = ?'];
    const values: unknown[] = [now];

    if (updates.name !== undefined)         { fields.push('name = ?');               values.push(updates.name); }
    if (updates.apiKey !== undefined)       { fields.push('api_key_encrypted = ?');  values.push(this.encrypt(updates.apiKey)); }
    if (updates.baseUrl !== undefined)      { fields.push('base_url = ?');           values.push(updates.baseUrl); }
    if (updates.defaultModel !== undefined) { fields.push('default_model = ?');      values.push(updates.defaultModel); }
    if (updates.enabled !== undefined)      { fields.push('enabled = ?');            values.push(updates.enabled ? 1 : 0); }
    if (updates.isDefault !== undefined) {
      if (updates.isDefault) {
        (this.db.prepare('UPDATE llm_providers SET is_default = 0') as any).run();
      }
      fields.push('is_default = ?');
      values.push(updates.isDefault ? 1 : 0);
    }

    values.push(id);
    (this.db.prepare(`UPDATE llm_providers SET ${fields.join(', ')} WHERE id = ?`) as any).run(...values);

    return this.getProvider(id)!;
  }

  deleteProvider(id: string): void {
    (this.db.prepare('DELETE FROM llm_providers WHERE id = ?') as any).run(id);
  }

  getProvider(id: string): LLMProvider | null {
    const row = (this.db.prepare('SELECT * FROM llm_providers WHERE id = ?') as any).get(id);
    return row ? this.rowToProvider(row as Record<string, unknown>) : null;
  }

  listProviders(): LLMProvider[] {
    const rows = (this.db.prepare(
      'SELECT * FROM llm_providers ORDER BY created_at ASC',
    ) as any).all() as unknown[];
    return rows.map((r) => this.rowToProvider(r as Record<string, unknown>));
  }

  getDefaultProvider(): LLMProvider | null {
    const row = (this.db.prepare(
      'SELECT * FROM llm_providers WHERE is_default = 1 AND enabled = 1',
    ) as any).get();
    return row ? this.rowToProvider(row as Record<string, unknown>) : null;
  }

  setDefault(id: string): void {
    if (!this.getProvider(id)) throw new Error(`Provider not found: ${id}`);
    (this.db.prepare('UPDATE llm_providers SET is_default = 0') as any).run();
    (this.db.prepare(
      'UPDATE llm_providers SET is_default = 1, updated_at = ? WHERE id = ?',
    ) as any).run(new Date().toISOString(), id);
  }

  /** Decrypt and return the API key for a provider (runtime only). */
  getApiKey(provider: LLMProvider): string | null {
    if (!provider.apiKeyEncrypted) return null;
    return this.decrypt(provider.apiKeyEncrypted);
  }

  /** Return the provider with its API key decrypted. Never serialise the result. */
  getProviderRuntime(id: string): LLMProviderRuntime | null {
    const provider = this.getProvider(id);
    if (!provider) return null;
    return {
      ...provider,
      apiKey: provider.apiKeyEncrypted ? this.decrypt(provider.apiKeyEncrypted) : null,
    };
  }

  /** Return all providers with decrypted keys (for internal routing). */
  getDefaultProviderRuntime(): LLMProviderRuntime | null {
    const provider = this.getDefaultProvider();
    if (!provider) return null;
    return {
      ...provider,
      apiKey: provider.apiKeyEncrypted ? this.decrypt(provider.apiKeyEncrypted) : null,
    };
  }

  close(): void {
    this.db.close();
  }

  /** List of model IDs known for a provider type. */
  static getModelsForProvider(providerId: string): string[] {
    return PROVIDER_MODELS[providerId as ProviderType] ?? [];
  }

  /** Default name + model + baseUrl for a given provider type. */
  static getDefaultConfig(
    providerId: ProviderType,
  ): { name: string; defaultModel: string; baseUrl?: string } {
    return PROVIDER_DEFAULTS[providerId];
  }

  // ─── Encryption ──────────────────────────────────────────────────

  private encrypt(plaintext: string): string {
    const iv = randomBytes(16);
    const cipher = createCipheriv('aes-256-gcm', this.key, iv);
    const ciphertext = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()]);
    const authTag = cipher.getAuthTag();
    return `${iv.toString('hex')}:${authTag.toString('hex')}:${ciphertext.toString('hex')}`;
  }

  private decrypt(blob: string): string {
    const parts = blob.split(':');
    if (parts.length !== 3) throw new Error('Invalid encrypted blob format');
    const [ivHex, authTagHex, ciphertextHex] = parts;
    const decipher = createDecipheriv(
      'aes-256-gcm',
      this.key,
      Buffer.from(ivHex, 'hex'),
    );
    decipher.setAuthTag(Buffer.from(authTagHex, 'hex'));
    return Buffer.concat([
      decipher.update(Buffer.from(ciphertextHex, 'hex')),
      decipher.final(),
    ]).toString('utf8');
  }

  private rowToProvider(row: Record<string, unknown>): LLMProvider {
    return {
      id:               row['id'] as string,
      name:             row['name'] as string,
      apiKeyEncrypted:  (row['api_key_encrypted'] as string | null) ?? null,
      baseUrl:          (row['base_url'] as string | null) ?? null,
      defaultModel:     row['default_model'] as string,
      enabled:          row['enabled'] === 1,
      isDefault:        row['is_default'] === 1,
      createdAt:        row['created_at'] as string,
      updatedAt:        row['updated_at'] as string,
    };
  }
}
