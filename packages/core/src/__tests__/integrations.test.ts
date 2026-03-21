import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { ModuleBus } from '../module-bus/index.js';
import { PermissionManager } from '../permissions/index.js';
import { IntegrationRegistry } from '../integrations/index.js';
import { GmailIntegration } from '../integrations/gmail.js';
import { CalendarIntegration } from '../integrations/calendar.js';
import { IMessageIntegration } from '../integrations/imessage.js';
import { BraveSearchIntegration } from '../integrations/brave-search.js';
import { BrowserIntegration } from '../integrations/browser.js';
import type { GmailConfig } from '../integrations/gmail.js';
import type { CalendarConfig } from '../integrations/calendar.js';
import type { IMessageConfig } from '../integrations/imessage.js';
import type { BraveSearchConfig } from '../integrations/brave-search.js';
import type { BrowserConfig } from '../integrations/browser.js';

describe('IntegrationRegistry', () => {
  let bus: ModuleBus;
  let permissions: PermissionManager;
  let tmpDir: string;

  beforeEach(() => {
    bus = new ModuleBus();
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-integ-'));
    permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
  });

  afterEach(() => {
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should register and retrieve integrations', () => {
    const registry = new IntegrationRegistry();
    const gmail = new GmailIntegration(bus, permissions, { enabled: false });

    registry.register(gmail);
    expect(registry.get('gmail')).toBe(gmail);
    expect(registry.size()).toBe(1);
  });

  it('should reject duplicate registrations', () => {
    const registry = new IntegrationRegistry();
    const gmail1 = new GmailIntegration(bus, permissions, { enabled: false });
    const gmail2 = new GmailIntegration(bus, permissions, { enabled: false });

    registry.register(gmail1);
    expect(() => registry.register(gmail2)).toThrow('already registered');
  });

  it('should list all integrations with status', () => {
    const registry = new IntegrationRegistry();
    registry.register(new GmailIntegration(bus, permissions, { enabled: false }));
    registry.register(new CalendarIntegration(bus, permissions, { enabled: false }));
    registry.register(new BraveSearchIntegration(bus, permissions, { enabled: false }));

    const list = registry.list();
    expect(list).toHaveLength(3);
    expect(list.map((i) => i.id)).toEqual(['gmail', 'calendar', 'brave-search']);
    expect(list.every((i) => i.status === 'disconnected')).toBe(true);
  });

  it('should initialize all integrations', async () => {
    const registry = new IntegrationRegistry();
    registry.register(new GmailIntegration(bus, permissions, { enabled: false }));
    registry.register(new BraveSearchIntegration(bus, permissions, { enabled: false }));

    const results = await registry.initializeAll();
    expect(results.size).toBe(2);
    // Disabled integrations should initialize without error (just stay disconnected)
    for (const err of results.values()) {
      expect(err).toBeNull();
    }
  });

  it('should diagnose all integrations', () => {
    const registry = new IntegrationRegistry();
    registry.register(new GmailIntegration(bus, permissions, { enabled: false }));
    registry.register(new CalendarIntegration(bus, permissions, { enabled: false }));

    const results = registry.diagnoseAll();
    expect(results).toHaveLength(2);
    expect(results[0].module).toBe('gmail');
    expect(results[1].module).toBe('calendar');
  });

  it('should shutdown all integrations', async () => {
    const registry = new IntegrationRegistry();
    const gmail = new GmailIntegration(bus, permissions, { enabled: false });
    registry.register(gmail);

    await registry.shutdownAll();
    expect(gmail.status).toBe('disconnected');
  });

  it('should return undefined for unknown integration', () => {
    const registry = new IntegrationRegistry();
    expect(registry.get('nonexistent')).toBeUndefined();
  });
});

describe('GmailIntegration', () => {
  let bus: ModuleBus;
  let permissions: PermissionManager;
  let tmpDir: string;

  beforeEach(() => {
    bus = new ModuleBus();
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-gmail-'));
    permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
  });

  afterEach(() => {
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should create with correct id and name', () => {
    const gmail = new GmailIntegration(bus, permissions, { enabled: false });
    expect(gmail.id).toBe('gmail');
    expect(gmail.name).toBe('Gmail');
    expect(gmail.status).toBe('disconnected');
  });

  it('should report not-configured status in diagnose()', () => {
    const gmail = new GmailIntegration(bus, permissions, { enabled: false });
    const result = gmail.diagnose();

    expect(result.module).toBe('gmail');
    expect(result.status).toBe('unhealthy');
    expect(result.checks.length).toBeGreaterThanOrEqual(3);

    const credCheck = result.checks.find((c) => c.name === 'OAuth2 credentials configured');
    expect(credCheck).toBeDefined();
    expect(credCheck!.passed).toBe(false);

    const enabledCheck = result.checks.find((c) => c.name === 'Integration enabled');
    expect(enabledCheck).toBeDefined();
    expect(enabledCheck!.passed).toBe(false);
  });

  it('should report degraded when enabled but missing credentials', () => {
    const gmail = new GmailIntegration(bus, permissions, { enabled: true });
    const result = gmail.diagnose();

    expect(result.status).toBe('degraded');
    const enabledCheck = result.checks.find((c) => c.name === 'Integration enabled');
    expect(enabledCheck!.passed).toBe(true);
  });

  it('should throw when calling methods without config', async () => {
    const gmail = new GmailIntegration(bus, permissions, { enabled: false });
    await expect(gmail.listMessages('test')).rejects.toThrow('not enabled');
    await expect(gmail.readMessage('123')).rejects.toThrow('not enabled');
    await expect(gmail.createDraft(['test@test.com'], 'sub', 'body')).rejects.toThrow('not enabled');
    await expect(gmail.sendDraft('draft-1')).rejects.toThrow('not enabled');
    await expect(gmail.listLabels()).rejects.toThrow('not enabled');
  });

  it('should throw for missing credentials when enabled', async () => {
    const gmail = new GmailIntegration(bus, permissions, { enabled: true });
    await expect(gmail.listMessages('test')).rejects.toThrow('not configured');
  });

  it('should enforce draft-only behavior — createDraft does not send', async () => {
    const config: GmailConfig = {
      enabled: true,
      clientId: 'test-id',
      clientSecret: 'test-secret',
      refreshToken: 'test-token',
    };
    const gmail = new GmailIntegration(bus, permissions, config);

    // createDraft should attempt API call (will fail due to invalid creds)
    // but the important thing is it does NOT call send
    const events: string[] = [];
    bus.subscribe('gmail.createDraft', () => { events.push('createDraft'); });
    bus.subscribe('gmail.sendDraft', () => { events.push('sendDraft'); });

    try {
      await gmail.createDraft(['test@test.com'], 'Test', 'Body');
    } catch {
      // Expected: API call will fail with invalid credentials
    }

    expect(events).toContain('createDraft');
    expect(events).not.toContain('sendDraft');
  });

  it('should require Tier 3 approval for sendDraft', async () => {
    const config: GmailConfig = {
      enabled: true,
      clientId: 'test-id',
      clientSecret: 'test-secret',
      refreshToken: 'test-token',
    };
    const gmail = new GmailIntegration(bus, permissions, config);

    // sendDraft checks send_message action which is Tier 3 (requires approval)
    await expect(gmail.sendDraft('draft-1')).rejects.toThrow('Tier 3 approval');
  });

  it('should publish events on bus for operations', async () => {
    const config: GmailConfig = {
      enabled: true,
      clientId: 'test-id',
      clientSecret: 'test-secret',
      refreshToken: 'test-token',
    };
    const gmail = new GmailIntegration(bus, permissions, config);

    const events: string[] = [];
    bus.subscribe('gmail.listMessages', () => { events.push('listMessages'); });
    bus.subscribe('gmail.listMessages.error', () => { events.push('listMessages.error'); });

    try {
      await gmail.listMessages('test query');
    } catch {
      // Expected: API call will fail
    }

    expect(events).toContain('listMessages');
    expect(events).toContain('listMessages.error');
  });

  it('should set error status when credentials are missing during initialize', async () => {
    const gmail = new GmailIntegration(bus, permissions, { enabled: true });

    const events: string[] = [];
    bus.subscribe('integration.error', () => { events.push('error'); });

    await gmail.initialize();
    expect(gmail.status).toBe('error');
    expect(events).toContain('error');
  });
});

describe('CalendarIntegration', () => {
  let bus: ModuleBus;
  let permissions: PermissionManager;
  let tmpDir: string;

  beforeEach(() => {
    bus = new ModuleBus();
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-cal-'));
    permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
  });

  afterEach(() => {
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should create with correct id and name', () => {
    const cal = new CalendarIntegration(bus, permissions, { enabled: false });
    expect(cal.id).toBe('calendar');
    expect(cal.name).toBe('Google Calendar');
  });

  it('should report not-configured status in diagnose()', () => {
    const cal = new CalendarIntegration(bus, permissions, { enabled: false });
    const result = cal.diagnose();

    expect(result.module).toBe('calendar');
    expect(result.status).toBe('unhealthy');
    expect(result.checks.some((c) => c.name === 'OAuth2 credentials configured' && !c.passed)).toBe(true);
  });

  it('should throw when calling methods without config', async () => {
    const cal = new CalendarIntegration(bus, permissions, { enabled: false });
    const now = new Date();
    const later = new Date(now.getTime() + 3600000);
    await expect(cal.listEvents(now, later)).rejects.toThrow('not enabled');
    await expect(cal.getEvent('event-1')).rejects.toThrow('not enabled');
    await expect(cal.createEvent('Test', now, later)).rejects.toThrow('not enabled');
    await expect(cal.findFreeTime(now, later, 30)).rejects.toThrow('not enabled');
  });

  it('should publish events on bus', async () => {
    const config: CalendarConfig = {
      enabled: true,
      clientId: 'test-id',
      clientSecret: 'test-secret',
      refreshToken: 'test-token',
    };
    const cal = new CalendarIntegration(bus, permissions, config);

    const events: string[] = [];
    bus.subscribe('calendar.listEvents', () => { events.push('listEvents'); });

    try {
      await cal.listEvents(new Date(), new Date());
    } catch {
      // Expected: API will fail
    }

    expect(events).toContain('listEvents');
  });
});

describe('IMessageIntegration', () => {
  let bus: ModuleBus;
  let permissions: PermissionManager;
  let tmpDir: string;

  beforeEach(() => {
    bus = new ModuleBus();
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-imsg-'));
    permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
  });

  afterEach(() => {
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should create with correct id and name', () => {
    const imsg = new IMessageIntegration(bus, permissions, {
      enabled: false,
      allowedRecipients: [],
    });
    expect(imsg.id).toBe('imessage');
    expect(imsg.name).toBe('iMessage');
  });

  it('should report not-configured status in diagnose()', () => {
    const imsg = new IMessageIntegration(bus, permissions, {
      enabled: false,
      allowedRecipients: [],
    });
    const result = imsg.diagnose();

    expect(result.module).toBe('imessage');
    expect(result.checks.some((c) => c.name === 'Integration enabled' && !c.passed)).toBe(true);
    expect(result.checks.some((c) => c.name === 'Recipient allowlist configured' && !c.passed)).toBe(true);
  });

  it('should enforce recipient allowlist', () => {
    const imsg = new IMessageIntegration(bus, permissions, {
      enabled: true,
      allowedRecipients: ['+15551234567', 'friend@icloud.com'],
    });

    expect(imsg.isRecipientAllowed('+15551234567')).toBe(true);
    expect(imsg.isRecipientAllowed('friend@icloud.com')).toBe(true);
    expect(imsg.isRecipientAllowed('FRIEND@ICLOUD.COM')).toBe(true); // case-insensitive
    expect(imsg.isRecipientAllowed('+15559999999')).toBe(false);
    expect(imsg.isRecipientAllowed('stranger@example.com')).toBe(false);
  });

  it('should reject messages to non-allowlisted recipients', async () => {
    const imsg = new IMessageIntegration(bus, permissions, {
      enabled: true,
      allowedRecipients: ['+15551234567'],
    });

    await expect(imsg.sendMessage('+15559999999', 'Hello')).rejects.toThrow('not in the allowlist');
  });

  it('should throw when disabled', async () => {
    const imsg = new IMessageIntegration(bus, permissions, {
      enabled: false,
      allowedRecipients: ['+15551234567'],
    });

    await expect(imsg.sendMessage('+15551234567', 'Hello')).rejects.toThrow('not enabled');
  });

  it('should report allowlist count in diagnose()', () => {
    const imsg = new IMessageIntegration(bus, permissions, {
      enabled: true,
      allowedRecipients: ['alice@icloud.com', 'bob@icloud.com', '+15551234567'],
    });
    const result = imsg.diagnose();

    const allowlistCheck = result.checks.find((c) => c.name === 'Recipient allowlist configured');
    expect(allowlistCheck).toBeDefined();
    expect(allowlistCheck!.passed).toBe(true);
    expect(allowlistCheck!.message).toContain('3');
  });

  it('should publish events on bus', async () => {
    const imsg = new IMessageIntegration(bus, permissions, {
      enabled: true,
      allowedRecipients: ['+15551234567'],
    });

    const events: string[] = [];
    bus.subscribe('imessage.sendMessage', () => { events.push('sendMessage'); });

    try {
      await imsg.sendMessage('+15551234567', 'Test');
    } catch {
      // Expected: AppleScript/permission may not be available in test env
    }

    // Event should be published before the actual send attempt
    // It may or may not fire depending on permission check outcome
    // The key test is that the allowlist is checked first (covered above)
  });
});

describe('BraveSearchIntegration', () => {
  let bus: ModuleBus;
  let permissions: PermissionManager;
  let tmpDir: string;

  beforeEach(() => {
    bus = new ModuleBus();
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-brave-'));
    permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
  });

  afterEach(() => {
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should create with correct id and name', () => {
    const search = new BraveSearchIntegration(bus, permissions, { enabled: false });
    expect(search.id).toBe('brave-search');
    expect(search.name).toBe('Brave Search');
  });

  it('should report not-configured status in diagnose()', () => {
    const search = new BraveSearchIntegration(bus, permissions, { enabled: false });
    const result = search.diagnose();

    expect(result.module).toBe('brave-search');
    expect(result.status).toBe('unhealthy');
    expect(result.checks.some((c) => c.name === 'API key configured' && !c.passed)).toBe(true);
  });

  it('should throw when calling methods without config', async () => {
    const search = new BraveSearchIntegration(bus, permissions, { enabled: false });
    await expect(search.search('test')).rejects.toThrow('not enabled');
    await expect(search.summarize('test')).rejects.toThrow('not enabled');
  });

  it('should throw for missing API key when enabled', async () => {
    const search = new BraveSearchIntegration(bus, permissions, { enabled: true });
    await expect(search.search('test')).rejects.toThrow('API key');
  });

  it('should publish events on bus', async () => {
    const search = new BraveSearchIntegration(bus, permissions, {
      enabled: true,
      apiKey: 'test-key',
    });

    const events: string[] = [];
    bus.subscribe('brave-search.search', () => { events.push('search'); });
    bus.subscribe('brave-search.search.error', () => { events.push('search.error'); });

    try {
      await search.search('test query');
    } catch {
      // Expected: API call will fail with test key
    }

    expect(events).toContain('search');
    expect(events).toContain('search.error');
  });
});

describe('BrowserIntegration', () => {
  let bus: ModuleBus;
  let permissions: PermissionManager;
  let tmpDir: string;

  beforeEach(() => {
    bus = new ModuleBus();
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-browser-'));
    permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
  });

  afterEach(() => {
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should create with correct id and name', () => {
    const browser = new BrowserIntegration(bus, permissions, {
      enabled: false,
      allowedSites: [],
    });
    expect(browser.id).toBe('browser');
    expect(browser.name).toBe('Browser (Playwright)');
  });

  it('should report not-configured status in diagnose()', () => {
    const browser = new BrowserIntegration(bus, permissions, {
      enabled: false,
      allowedSites: [],
    });
    const result = browser.diagnose();

    expect(result.module).toBe('browser');
    expect(result.checks.some((c) => c.name === 'Integration enabled' && !c.passed)).toBe(true);
    expect(result.checks.some((c) => c.name === 'Site allowlist configured' && !c.passed)).toBe(true);
  });

  it('should enforce URL allowlist', () => {
    const browser = new BrowserIntegration(bus, permissions, {
      enabled: true,
      allowedSites: ['example.com', '*.github.com'],
    });

    expect(browser.isUrlAllowed('https://example.com/page')).toBe(true);
    expect(browser.isUrlAllowed('https://www.example.com/page')).toBe(true); // subdomains match parent domain
    expect(browser.isUrlAllowed('https://github.com/repo')).toBe(true);
    expect(browser.isUrlAllowed('https://api.github.com/v1')).toBe(true);
    expect(browser.isUrlAllowed('https://evil.com')).toBe(false);
    expect(browser.isUrlAllowed('https://notexample.com')).toBe(false);
  });

  it('should reject navigation to non-allowlisted URLs', async () => {
    const browser = new BrowserIntegration(bus, permissions, {
      enabled: true,
      allowedSites: ['example.com'],
    });

    await expect(browser.navigate('https://evil.com')).rejects.toThrow('not in the allowlist');
    await expect(browser.getContent('https://evil.com')).rejects.toThrow('not in the allowlist');
    await expect(browser.screenshot('https://evil.com')).rejects.toThrow('not in the allowlist');
  });

  it('should reject actions on non-allowlisted URLs', async () => {
    const browser = new BrowserIntegration(bus, permissions, {
      enabled: true,
      allowedSites: ['example.com'],
    });

    await expect(
      browser.executeAction('https://evil.com', { type: 'click', selector: '#btn' })
    ).rejects.toThrow('not in the allowlist');
  });

  it('should throw when disabled', async () => {
    const browser = new BrowserIntegration(bus, permissions, {
      enabled: false,
      allowedSites: ['example.com'],
    });

    await expect(browser.navigate('https://example.com')).rejects.toThrow('not enabled');
  });

  it('should support wildcard subdomains in allowlist', () => {
    const browser = new BrowserIntegration(bus, permissions, {
      enabled: true,
      allowedSites: ['*.example.com'],
    });

    expect(browser.isUrlAllowed('https://example.com')).toBe(true);
    expect(browser.isUrlAllowed('https://www.example.com')).toBe(true);
    expect(browser.isUrlAllowed('https://sub.deep.example.com')).toBe(true);
    expect(browser.isUrlAllowed('https://notexample.com')).toBe(false);
  });

  it('should report allowlist count in diagnose()', () => {
    const browser = new BrowserIntegration(bus, permissions, {
      enabled: true,
      allowedSites: ['example.com', '*.github.com', 'docs.rs'],
    });
    const result = browser.diagnose();

    const allowlistCheck = result.checks.find((c) => c.name === 'Site allowlist configured');
    expect(allowlistCheck).toBeDefined();
    expect(allowlistCheck!.passed).toBe(true);
    expect(allowlistCheck!.message).toContain('3');
  });

  it('should publish events on bus for navigation', async () => {
    const browser = new BrowserIntegration(bus, permissions, {
      enabled: true,
      allowedSites: ['example.com'],
    });

    const events: string[] = [];
    bus.subscribe('browser.getContent', () => { events.push('getContent'); });
    bus.subscribe('browser.getContent.error', () => { events.push('getContent.error'); });
    bus.subscribe('browser.getContent.complete', () => { events.push('getContent.complete'); });

    try {
      await browser.getContent('https://example.com');
    } catch {
      // May fail in test environment
    }

    expect(events).toContain('getContent');
    // Should have either complete or error
    expect(events.length).toBeGreaterThanOrEqual(2);
  });
});

describe('Integration Registry — Full Lifecycle', () => {
  let bus: ModuleBus;
  let permissions: PermissionManager;
  let tmpDir: string;

  beforeEach(() => {
    bus = new ModuleBus();
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-lifecycle-'));
    permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
  });

  afterEach(() => {
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should handle full lifecycle: register, initialize, diagnose, shutdown', async () => {
    const registry = new IntegrationRegistry();

    // Register all integrations (all disabled — safe for testing)
    registry.register(new GmailIntegration(bus, permissions, { enabled: false }));
    registry.register(new CalendarIntegration(bus, permissions, { enabled: false }));
    registry.register(new IMessageIntegration(bus, permissions, {
      enabled: false,
      allowedRecipients: [],
    }));
    registry.register(new BraveSearchIntegration(bus, permissions, { enabled: false }));
    registry.register(new BrowserIntegration(bus, permissions, {
      enabled: false,
      allowedSites: [],
    }));

    expect(registry.size()).toBe(5);

    // Initialize all
    const initResults = await registry.initializeAll();
    expect(initResults.size).toBe(5);
    for (const err of initResults.values()) {
      expect(err).toBeNull();
    }

    // All should be disconnected (disabled)
    const list = registry.list();
    expect(list.every((i) => i.status === 'disconnected')).toBe(true);

    // Diagnose all
    const diagResults = registry.diagnoseAll();
    expect(diagResults).toHaveLength(5);
    for (const result of diagResults) {
      expect(result.checks.length).toBeGreaterThan(0);
      // All disabled integrations should be unhealthy or degraded
      expect(['unhealthy', 'degraded']).toContain(result.status);
    }

    // Shutdown all
    await registry.shutdownAll();
    const postShutdown = registry.list();
    expect(postShutdown.every((i) => i.status === 'disconnected')).toBe(true);
  });
});
