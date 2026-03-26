import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync } from 'node:fs';
import { Fort } from '../fort.js';

describe('Introspector', () => {
  let tmpDir: string;
  let fort: Fort;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-introspect-'));
    const specsDir = join(tmpDir, 'specs');
    mkdirSync(specsDir, { recursive: true });

    fort = new Fort({
      dataDir: join(tmpDir, 'data'),
      specsDir,
    });
    return fort;
  }

  afterEach(async () => {
    if (fort) await fort.stop();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should generate module manifest with all registered modules', async () => {
    setup();
    await fort.start();

    const manifest = fort.introspect.generateModuleManifest();

    expect(manifest.name).toBe('fort');
    expect(manifest.modules.length).toBeGreaterThan(0);
    expect(manifest.generatedAt).toBeTruthy();

    // Doctor-registered modules should appear
    const moduleNames = manifest.modules.map((m) => m.name);
    expect(moduleNames).toContain('memory');
    expect(moduleNames).toContain('tools');
    expect(moduleNames).toContain('agents');
    expect(moduleNames).toContain('tasks');
    expect(moduleNames).toContain('introspect');
  });

  it('should generate capability map with tools and agents', async () => {
    setup();
    await fort.start();

    // Register a tool to show up in capabilities
    fort.tools.register({
      name: 'test-tool',
      description: 'A test tool',
      capabilities: ['test_cap'],
      inputTypes: ['string'],
      outputTypes: ['string'],
      tags: ['test'],
      module: 'test',
      version: '1.0.0',
    });

    const capabilities = fort.introspect.generateCapabilityMap();

    expect(capabilities.length).toBeGreaterThan(0);

    // Tool capability should be present
    const toolCap = capabilities.find(
      (c) => c.capability === 'test_cap' && c.providerType === 'tool',
    );
    expect(toolCap).toBeDefined();
    expect(toolCap!.provider).toBe('test-tool');

    // No agent capabilities by default (core agents are now services)
    const agentCaps = capabilities.filter((c) => c.providerType === 'agent');
    expect(agentCaps.length).toBe(0);

    // Module capabilities should be present
    const moduleCaps = capabilities.filter((c) => c.providerType === 'module');
    expect(moduleCaps.length).toBeGreaterThan(0);
  });

  it('should generate event catalog capturing bus event types', async () => {
    setup();
    await fort.start();

    // Publish some events to populate history
    await fort.bus.publish('test.event', 'test-source', { data: 'hello' });

    const catalog = fort.introspect.generateEventCatalog();

    // Should have events from fort.start() and our test event in history
    expect(catalog.length).toBeGreaterThan(0);

    // Look for events with subscribers (agent.message is subscribed by AgentRegistry)
    const subscribedEvents = catalog.filter((e) => e.subscriberCount > 0);
    expect(subscribedEvents.length).toBeGreaterThan(0);
  });

  it('should generate system profile with correct counts', async () => {
    setup();
    await fort.start();

    // Register a tool
    fort.tools.register({
      name: 'profile-tool',
      description: 'For profile test',
      capabilities: ['test'],
      inputTypes: [],
      outputTypes: [],
      tags: [],
      module: 'test',
      version: '1.0.0',
    });

    const profile = await fort.introspect.generateSystemProfile();

    expect(profile.version).toBe('0.1.0');
    expect(profile.uptime).toBeGreaterThanOrEqual(0);
    expect(profile.moduleCount).toBeGreaterThan(0);
    expect(profile.agentCount).toBe(0); // no agents by default (core agents are now services)
    expect(profile.toolCount).toBeGreaterThanOrEqual(1);
    expect(profile.generatedAt).toBeTruthy();

    // Diagnostic summary should be present
    expect(profile.diagnosticSummary).not.toBeNull();
    expect(profile.diagnosticSummary!.totalChecks).toBeGreaterThan(0);
  });

  it('should search capabilities and find relevant tools', async () => {
    setup();
    await fort.start();

    fort.tools.register({
      name: 'email-reader',
      description: 'Read email messages',
      capabilities: ['email_read', 'email_search'],
      inputTypes: [],
      outputTypes: [],
      tags: ['email'],
      module: 'email',
      version: '1.0.0',
    });

    const results = fort.introspect.searchCapabilities('email');
    expect(results.length).toBeGreaterThan(0);
    expect(results.some((r) => r.provider === 'email-reader')).toBe(true);

    // Non-matching query should return empty
    const noResults = fort.introspect.searchCapabilities('nonexistent_xyz_abc');
    expect(noResults).toHaveLength(0);
  });

  it('should export valid JSON documentation', async () => {
    setup();
    await fort.start();

    const jsonStr = await fort.introspect.exportDocumentation('json');

    // Should be valid JSON
    const parsed = JSON.parse(jsonStr);
    expect(parsed).toHaveProperty('manifest');
    expect(parsed).toHaveProperty('capabilities');
    expect(parsed).toHaveProperty('events');
    expect(parsed).toHaveProperty('profile');

    expect(parsed.manifest.name).toBe('fort');
    expect(Array.isArray(parsed.capabilities)).toBe(true);
    expect(Array.isArray(parsed.events)).toBe(true);
    expect(parsed.profile.version).toBe('0.1.0');
  });

  it('should export valid markdown documentation', async () => {
    setup();
    await fort.start();

    const markdown = await fort.introspect.exportDocumentation('markdown');

    // Should contain markdown headings
    expect(markdown).toContain('# Fort System Documentation');
    expect(markdown).toContain('## System Profile');
    expect(markdown).toContain('## Modules');
    expect(markdown).toContain('## Capabilities');

    // Should contain actual data
    expect(markdown).toContain('**Version**');
    expect(markdown).toContain('**Agents**');
  });

  it('should return healthy diagnostics', async () => {
    setup();
    await fort.start();

    const result = fort.introspect.diagnose();

    expect(result.module).toBe('introspect');
    expect(result.status).toBe('healthy');
    expect(result.checks.length).toBe(3);
    expect(result.checks.every((c) => c.passed)).toBe(true);
  });
});
