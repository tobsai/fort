import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { LLMProviderStore } from '../llm/provider-store.js';

describe('LLMProviderStore', () => {
  let tmpDir: string;
  let store: LLMProviderStore;

  function setup(encryptionKey = 'test-key-1234') {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-llm-providers-'));
    store = new LLMProviderStore(join(tmpDir, 'llm-providers.db'), encryptionKey);
    return store;
  }

  afterEach(() => {
    store?.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  // ─── CRUD ───────────────────────────────────────────────────────────

  it('adds a provider and retrieves it', () => {
    setup();
    const provider = store.addProvider({
      id: 'anthropic',
      name: 'Anthropic',
      apiKey: 'sk-ant-test-key',
      defaultModel: 'claude-sonnet-4-5-20250929',
    });

    expect(provider.id).toBe('anthropic');
    expect(provider.name).toBe('Anthropic');
    expect(provider.defaultModel).toBe('claude-sonnet-4-5-20250929');
    expect(provider.enabled).toBe(true);
    expect(provider.isDefault).toBe(false);
    expect(provider.createdAt).toBeDefined();
    expect(provider.updatedAt).toBeDefined();
  });

  it('stores API key encrypted (not as plaintext)', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', apiKey: 'sk-ant-secret', defaultModel: 'claude-opus-4-6' });

    const provider = store.getProvider('anthropic')!;
    expect(provider.apiKeyEncrypted).not.toBeNull();
    expect(provider.apiKeyEncrypted).not.toBe('sk-ant-secret');
    expect(provider.apiKeyEncrypted).toContain(':'); // iv:tag:ciphertext format
  });

  it('decrypts API key correctly (round-trip)', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', apiKey: 'sk-ant-secret-key', defaultModel: 'claude-opus-4-6' });

    const provider = store.getProvider('anthropic')!;
    const decrypted = store.getApiKey(provider);
    expect(decrypted).toBe('sk-ant-secret-key');
  });

  it('getProviderRuntime returns decrypted key', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', apiKey: 'sk-ant-runtime-key', defaultModel: 'claude-opus-4-6' });

    const runtime = store.getProviderRuntime('anthropic')!;
    expect(runtime.apiKey).toBe('sk-ant-runtime-key');
  });

  it('returns null for non-existent provider', () => {
    setup();
    expect(store.getProvider('nonexistent')).toBeNull();
    expect(store.getProviderRuntime('nonexistent')).toBeNull();
  });

  it('lists all providers in insertion order', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', defaultModel: 'claude-opus-4-6' });
    store.addProvider({ id: 'openai', name: 'OpenAI', defaultModel: 'gpt-4o' });
    store.addProvider({ id: 'groq', name: 'Groq', defaultModel: 'llama-3.3-70b-versatile' });

    const list = store.listProviders();
    expect(list).toHaveLength(3);
    expect(list[0].id).toBe('anthropic');
    expect(list[1].id).toBe('openai');
    expect(list[2].id).toBe('groq');
  });

  it('deletes a provider', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', defaultModel: 'claude-opus-4-6' });
    store.deleteProvider('anthropic');
    expect(store.getProvider('anthropic')).toBeNull();
    expect(store.listProviders()).toHaveLength(0);
  });

  it('updates provider fields', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', defaultModel: 'claude-opus-4-6' });
    const updated = store.updateProvider('anthropic', { defaultModel: 'claude-haiku-4-5-20251001', name: 'Anthropic (updated)' });
    expect(updated.defaultModel).toBe('claude-haiku-4-5-20251001');
    expect(updated.name).toBe('Anthropic (updated)');
  });

  it('updates API key (new encrypted value replaces old)', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', apiKey: 'old-key', defaultModel: 'claude-opus-4-6' });
    store.updateProvider('anthropic', { apiKey: 'new-key' });
    const runtime = store.getProviderRuntime('anthropic')!;
    expect(runtime.apiKey).toBe('new-key');
  });

  it('throws when updating non-existent provider', () => {
    setup();
    expect(() => store.updateProvider('missing', { name: 'X' })).toThrow('Provider not found');
  });

  it('handles Ollama provider without API key', () => {
    setup();
    const provider = store.addProvider({
      id: 'ollama',
      name: 'Ollama',
      defaultModel: 'llama3',
      baseUrl: 'http://localhost:11434',
    });
    expect(provider.apiKeyEncrypted).toBeNull();
    expect(provider.baseUrl).toBe('http://localhost:11434');
    expect(store.getApiKey(provider)).toBeNull();
  });

  // ─── Default Provider ────────────────────────────────────────────────

  it('sets default provider via addProvider', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', defaultModel: 'claude-opus-4-6', isDefault: true });
    const def = store.getDefaultProvider()!;
    expect(def.id).toBe('anthropic');
    expect(def.isDefault).toBe(true);
  });

  it('only one provider is default at a time', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', defaultModel: 'claude-opus-4-6', isDefault: true });
    store.addProvider({ id: 'openai', name: 'OpenAI', defaultModel: 'gpt-4o', isDefault: true });

    const list = store.listProviders();
    const defaults = list.filter((p) => p.isDefault);
    expect(defaults).toHaveLength(1);
    expect(defaults[0].id).toBe('openai');
  });

  it('setDefault switches the default', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', defaultModel: 'claude-opus-4-6', isDefault: true });
    store.addProvider({ id: 'openai', name: 'OpenAI', defaultModel: 'gpt-4o' });

    store.setDefault('openai');

    const list = store.listProviders();
    expect(list.find((p) => p.id === 'anthropic')!.isDefault).toBe(false);
    expect(list.find((p) => p.id === 'openai')!.isDefault).toBe(true);
    expect(store.getDefaultProvider()!.id).toBe('openai');
  });

  it('setDefault throws for non-existent provider', () => {
    setup();
    expect(() => store.setDefault('missing')).toThrow('Provider not found');
  });

  it('getDefaultProvider returns null when none configured', () => {
    setup();
    expect(store.getDefaultProvider()).toBeNull();
  });

  it('getDefaultProviderRuntime returns decrypted key for default', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', apiKey: 'sk-default-key', defaultModel: 'claude-opus-4-6', isDefault: true });
    const runtime = store.getDefaultProviderRuntime()!;
    expect(runtime.id).toBe('anthropic');
    expect(runtime.apiKey).toBe('sk-default-key');
  });

  // ─── Encryption ──────────────────────────────────────────────────────

  it('different encryption keys produce different blobs', () => {
    setup('key-A');
    store.addProvider({ id: 'anthropic', name: 'Anthropic', apiKey: 'same-secret', defaultModel: 'claude-opus-4-6' });
    const blobA = store.getProvider('anthropic')!.apiKeyEncrypted;
    store.close();

    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-llm-providers-'));
    store = new LLMProviderStore(join(tmpDir, 'llm-providers.db'), 'key-B');
    store.addProvider({ id: 'anthropic', name: 'Anthropic', apiKey: 'same-secret', defaultModel: 'claude-opus-4-6' });
    const blobB = store.getProvider('anthropic')!.apiKeyEncrypted;

    expect(blobA).not.toBe(blobB);
  });

  it('each encryption of the same plaintext produces different IVs', () => {
    setup();
    store.addProvider({ id: 'anthropic', name: 'Anthropic', apiKey: 'same-key', defaultModel: 'claude-opus-4-6' });
    const blob1 = store.getProvider('anthropic')!.apiKeyEncrypted;
    store.updateProvider('anthropic', { apiKey: 'same-key' });
    const blob2 = store.getProvider('anthropic')!.apiKeyEncrypted;
    // Different IVs → different blobs even for same plaintext
    expect(blob1).not.toBe(blob2);
  });

  // ─── Static Helpers ──────────────────────────────────────────────────

  it('getModelsForProvider returns known models', () => {
    const models = LLMProviderStore.getModelsForProvider('anthropic');
    expect(models).toContain('claude-opus-4-6');
    expect(models.length).toBeGreaterThan(0);
  });

  it('getModelsForProvider returns empty array for unknown provider', () => {
    expect(LLMProviderStore.getModelsForProvider('unknown')).toEqual([]);
  });

  it('getDefaultConfig returns config for known providers', () => {
    const cfg = LLMProviderStore.getDefaultConfig('openai');
    expect(cfg.name).toBe('OpenAI');
    expect(cfg.defaultModel).toBe('gpt-4o');
    expect(cfg.baseUrl).toBeDefined();
  });
});
