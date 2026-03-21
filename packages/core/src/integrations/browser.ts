/**
 * Browser Integration — Playwright-based web automation with safety controls
 *
 * Enforces a site allowlist. Content parsing strips injection attempts.
 * Navigation is Tier 1 for allowlisted sites. Form submissions and
 * high-risk actions require Tier 3 approval via Action Gate.
 */

import type { DiagnosticResult } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { PermissionManager } from '../permissions/index.js';
import { BaseIntegration } from './base.js';

export interface BrowserConfig {
  enabled: boolean;
  allowedSites: string[];
  playwrightInstalled?: boolean;
}

export interface PageContent {
  url: string;
  title: string;
  text: string;
  links: Array<{ href: string; text: string }>;
  sanitized: boolean;
}

export interface BrowserAction {
  type: 'click' | 'fill' | 'select' | 'submit' | 'scroll';
  selector: string;
  value?: string;
}

export type ActionRisk = 'low' | 'medium' | 'high';

export class BrowserIntegration extends BaseIntegration {
  id = 'browser';
  name = 'Browser (Playwright)';

  private config: BrowserConfig;
  private playwrightAvailable = false;

  /**
   * Patterns that indicate prompt injection or instruction override attempts
   * in scraped web content.
   */
  private static readonly INJECTION_PATTERNS = [
    /ignore\s+(previous|all|above)\s+(instructions?|prompts?)/gi,
    /you\s+are\s+now\s+(a|an)\s+/gi,
    /system\s*:\s*/gi,
    /\[INST\]/gi,
    /\[\/INST\]/gi,
    /<\|im_start\|>/gi,
    /<\|im_end\|>/gi,
    /<<SYS>>/gi,
    /<\/SYS>>/gi,
    /ASSISTANT:\s*/gi,
    /USER:\s*/gi,
    /Human:\s*/gi,
    /Assistant:\s*/gi,
  ];

  constructor(bus: ModuleBus, permissions: PermissionManager, config: BrowserConfig) {
    super(bus, permissions);
    this.config = config;
  }

  async initialize(): Promise<void> {
    if (!this.config.enabled) {
      this.status = 'disconnected';
      return;
    }

    try {
      this.playwrightAvailable = await this.checkPlaywright();
      this.status = this.playwrightAvailable ? 'connected' : 'error';
      await this.publishEvent('integration.initialized', {
        integration: this.id,
        status: this.status,
        playwrightAvailable: this.playwrightAvailable,
      });
    } catch (err) {
      this.status = 'error';
      await this.publishEvent('integration.error', {
        integration: this.id,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async navigate(url: string): Promise<PageContent> {
    this.ensureConfigured();
    this.enforceAllowlist(url);

    await this.publishEvent('browser.navigate', { url });

    try {
      const playwright = await this.getPlaywright();
      const browser = await playwright.chromium.launch({ headless: true });
      const page = await browser.newPage();

      await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });

      const title = await page.title();
      /* eslint-disable no-eval -- page.evaluate runs in browser context */
      const text: string = await page.evaluate('document.body.innerText');
      const links: Array<{ href: string; text: string }> = await page.evaluate(`
        Array.from(document.querySelectorAll('a[href]')).map(function(a) {
          return { href: a.href, text: a.innerText.trim() };
        })
      `);

      await browser.close();

      const content: PageContent = {
        url,
        title,
        text: this.sanitizeContent(text),
        links: links.slice(0, 100), // Cap at 100 links
        sanitized: true,
      };

      await this.publishEvent('browser.navigate.complete', {
        url,
        title,
        textLength: content.text.length,
      });

      return content;
    } catch (err) {
      await this.publishEvent('browser.navigate.error', {
        url,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async getContent(url: string): Promise<PageContent> {
    this.ensureConfigured();
    this.enforceAllowlist(url);

    await this.publishEvent('browser.getContent', { url });

    try {
      // Use fetch for simple content retrieval (faster than full browser)
      const response = await fetch(url, {
        headers: {
          'User-Agent': 'Fort/1.0 (Personal AI Agent)',
        },
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      const html = await response.text();
      const content = this.parseHtml(url, html);

      await this.publishEvent('browser.getContent.complete', {
        url,
        title: content.title,
        textLength: content.text.length,
      });

      return content;
    } catch (err) {
      await this.publishEvent('browser.getContent.error', {
        url,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async screenshot(url: string): Promise<Buffer> {
    this.ensureConfigured();
    this.enforceAllowlist(url);

    await this.publishEvent('browser.screenshot', { url });

    try {
      const playwright = await this.getPlaywright();
      const browser = await playwright.chromium.launch({ headless: true });
      const page = await browser.newPage();

      await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });
      const buffer = await page.screenshot({ fullPage: false });
      await browser.close();

      await this.publishEvent('browser.screenshot.complete', {
        url,
        size: buffer.length,
      });

      return buffer;
    } catch (err) {
      await this.publishEvent('browser.screenshot.error', {
        url,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async executeAction(url: string, action: BrowserAction): Promise<void> {
    this.ensureConfigured();
    this.enforceAllowlist(url);

    const risk = this.assessActionRisk(action);

    // High-risk actions (form submissions, purchases) require Tier 3 approval
    if (risk === 'high') {
      const permission = this.checkPermission('browser_action');
      if (permission.requiresApproval) {
        throw new Error(
          `Browser action "${action.type}" on ${url} requires Tier 3 approval: ${permission.description}. ` +
          `Action details: ${JSON.stringify(action)}`
        );
      }
    }

    await this.publishEvent('browser.executeAction', { url, action, risk });

    try {
      const playwright = await this.getPlaywright();
      const browser = await playwright.chromium.launch({ headless: true });
      const page = await browser.newPage();

      await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });

      switch (action.type) {
        case 'click':
          await page.click(action.selector);
          break;
        case 'fill':
          await page.fill(action.selector, action.value || '');
          break;
        case 'select':
          await page.selectOption(action.selector, action.value || '');
          break;
        case 'submit':
          await page.click(action.selector);
          break;
        case 'scroll':
          await page.evaluate(`document.querySelector('${action.selector.replace(/'/g, "\\'")}')?.scrollIntoView()`);
          break;
      }

      await browser.close();

      await this.publishEvent('browser.executeAction.complete', {
        url,
        action: action.type,
        selector: action.selector,
      });
    } catch (err) {
      await this.publishEvent('browser.executeAction.error', {
        url,
        action: action.type,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  isUrlAllowed(url: string): boolean {
    try {
      const parsed = new URL(url);
      return this.config.allowedSites.some((allowed) => {
        // Support wildcard subdomains: *.example.com
        if (allowed.startsWith('*.')) {
          const domain = allowed.slice(2);
          return parsed.hostname === domain || parsed.hostname.endsWith('.' + domain);
        }
        return parsed.hostname === allowed || parsed.hostname.endsWith('.' + allowed);
      });
    } catch {
      return false;
    }
  }

  diagnose(): DiagnosticResult {
    const checks = [];

    checks.push({
      name: 'Integration enabled',
      passed: this.config.enabled,
      message: this.config.enabled
        ? 'Browser integration is enabled'
        : 'Browser integration is disabled in config',
    });

    checks.push({
      name: 'Playwright installed',
      passed: this.playwrightAvailable,
      message: this.playwrightAvailable
        ? 'Playwright is installed and available'
        : 'Playwright is not installed. Run: npx playwright install chromium',
    });

    const hasAllowlist = this.config.allowedSites.length > 0;
    checks.push({
      name: 'Site allowlist configured',
      passed: hasAllowlist,
      message: hasAllowlist
        ? `${this.config.allowedSites.length} allowed site(s) configured`
        : 'No allowed sites configured. Add sites to the allowlist.',
      details: { allowedSites: this.config.allowedSites },
    });

    checks.push({
      name: 'Connection status',
      passed: this.status === 'connected',
      message: `Current status: ${this.status}`,
    });

    const allPassed = checks.every((c) => c.passed);
    const somePassed = checks.some((c) => c.passed);

    return {
      module: 'browser',
      status: allPassed ? 'healthy' : somePassed ? 'degraded' : 'unhealthy',
      checks,
    };
  }

  private ensureConfigured(): void {
    if (!this.config.enabled) {
      throw new Error('Browser integration is not enabled. Enable it in config.');
    }
  }

  private enforceAllowlist(url: string): void {
    if (!this.isUrlAllowed(url)) {
      throw new Error(
        `URL "${url}" is not in the allowlist. ` +
        `Allowed sites: ${this.config.allowedSites.join(', ')}`
      );
    }
  }

  private assessActionRisk(action: BrowserAction): ActionRisk {
    switch (action.type) {
      case 'submit':
        return 'high';
      case 'fill':
        return 'medium';
      case 'click':
      case 'select':
      case 'scroll':
        return 'low';
      default:
        return 'high';
    }
  }

  private sanitizeContent(text: string): string {
    let sanitized = text;
    for (const pattern of BrowserIntegration.INJECTION_PATTERNS) {
      sanitized = sanitized.replace(pattern, '[FILTERED]');
    }
    return sanitized;
  }

  private parseHtml(url: string, html: string): PageContent {
    // Lightweight HTML parsing without a full DOM parser
    const titleMatch = html.match(/<title[^>]*>(.*?)<\/title>/i);
    const title = titleMatch ? titleMatch[1].trim() : '';

    // Strip HTML tags to get text content
    let text = html
      .replace(/<script[\s\S]*?<\/script>/gi, '')
      .replace(/<style[\s\S]*?<\/style>/gi, '')
      .replace(/<[^>]+>/g, ' ')
      .replace(/&nbsp;/g, ' ')
      .replace(/&amp;/g, '&')
      .replace(/&lt;/g, '<')
      .replace(/&gt;/g, '>')
      .replace(/&quot;/g, '"')
      .replace(/\s+/g, ' ')
      .trim();

    text = this.sanitizeContent(text);

    // Extract links
    const linkRegex = /<a[^>]+href=["']([^"']+)["'][^>]*>(.*?)<\/a>/gi;
    const links: Array<{ href: string; text: string }> = [];
    let match;
    while ((match = linkRegex.exec(html)) !== null && links.length < 100) {
      links.push({
        href: match[1],
        text: match[2].replace(/<[^>]+>/g, '').trim(),
      });
    }

    return { url, title, text, links, sanitized: true };
  }

  private async checkPlaywright(): Promise<boolean> {
    try {
      const moduleName = 'playwright';
      await import(moduleName);
      return true;
    } catch {
      return false;
    }
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private async getPlaywright(): Promise<any> {
    try {
      const moduleName = 'playwright';
      return await import(moduleName);
    } catch {
      throw new Error(
        'Playwright is not installed. Install it with: npm install playwright && npx playwright install chromium'
      );
    }
  }
}
