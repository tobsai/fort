/**
 * Brave Search Integration — Web search via Brave Search API
 *
 * All searches are Tier 1 (auto). API key required.
 * Provides raw search and summarization capabilities.
 */

import type { DiagnosticResult } from '../types.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { PermissionManager } from '../permissions/index.js';
import { BaseIntegration } from './base.js';

export interface BraveSearchConfig {
  enabled: boolean;
  apiKey?: string;
}

export interface SearchResult {
  title: string;
  url: string;
  description: string;
  age?: string;
}

export interface SearchResponse {
  query: string;
  results: SearchResult[];
  totalResults: number;
}

export interface SearchSummary {
  query: string;
  keyPoints: string[];
  sources: Array<{ title: string; url: string }>;
}

export class BraveSearchIntegration extends BaseIntegration {
  id = 'brave-search';
  name = 'Brave Search';

  private config: BraveSearchConfig;
  private apiReachable = false;

  private static readonly API_BASE = 'https://api.search.brave.com/res/v1';

  constructor(bus: ModuleBus, permissions: PermissionManager, config: BraveSearchConfig) {
    super(bus, permissions);
    this.config = config;
  }

  async initialize(): Promise<void> {
    if (!this.config.enabled) {
      this.status = 'disconnected';
      return;
    }

    if (!this.config.apiKey) {
      this.status = 'error';
      await this.publishEvent('integration.error', {
        integration: this.id,
        error: 'Missing Brave Search API key. Configure apiKey in config.',
      });
      return;
    }

    try {
      this.apiReachable = await this.checkApiReachability();
      this.status = this.apiReachable ? 'connected' : 'error';
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

  async search(query: string, count: number = 10, freshness?: string): Promise<SearchResponse> {
    this.ensureConfigured();

    const permission = this.checkPermission('web_search');
    if (!permission.allowed && permission.requiresApproval) {
      throw new Error(`Brave Search requires approval: ${permission.description}`);
    }

    await this.publishEvent('brave-search.search', { query, count, freshness });

    try {
      const params = new URLSearchParams({
        q: query,
        count: String(count),
      });

      if (freshness) {
        params.set('freshness', freshness);
      }

      const response = await this.apiRequest<{
        query?: { original: string };
        web?: { results: Array<{ title: string; url: string; description: string; age?: string }> };
      }>(`${BraveSearchIntegration.API_BASE}/web/search?${params}`);

      const results: SearchResult[] = (response.web?.results || []).map((r) => ({
        title: r.title,
        url: r.url,
        description: r.description,
        age: r.age,
      }));

      const searchResponse: SearchResponse = {
        query,
        results,
        totalResults: results.length,
      };

      await this.publishEvent('brave-search.search.complete', {
        query,
        resultCount: results.length,
      });

      return searchResponse;
    } catch (err) {
      await this.publishEvent('brave-search.search.error', {
        query,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  async summarize(query: string): Promise<SearchSummary> {
    this.ensureConfigured();

    await this.publishEvent('brave-search.summarize', { query });

    try {
      const searchResponse = await this.search(query, 5);

      // Extract key points from top results' descriptions
      const keyPoints = searchResponse.results
        .filter((r) => r.description)
        .map((r) => r.description)
        .slice(0, 5);

      const sources = searchResponse.results.map((r) => ({
        title: r.title,
        url: r.url,
      }));

      const summary: SearchSummary = {
        query,
        keyPoints,
        sources,
      };

      await this.publishEvent('brave-search.summarize.complete', {
        query,
        keyPointCount: keyPoints.length,
        sourceCount: sources.length,
      });

      return summary;
    } catch (err) {
      await this.publishEvent('brave-search.summarize.error', {
        query,
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  }

  diagnose(): DiagnosticResult {
    const checks = [];

    checks.push({
      name: 'Integration enabled',
      passed: this.config.enabled,
      message: this.config.enabled
        ? 'Brave Search integration is enabled'
        : 'Brave Search integration is disabled in config',
    });

    const hasApiKey = !!this.config.apiKey;
    checks.push({
      name: 'API key configured',
      passed: hasApiKey,
      message: hasApiKey
        ? 'Brave Search API key is configured'
        : 'Missing API key. Set apiKey in Brave Search config.',
    });

    checks.push({
      name: 'API reachable',
      passed: this.apiReachable,
      message: this.apiReachable
        ? 'Brave Search API is reachable'
        : 'Brave Search API has not been verified. Run initialize() first or check API key.',
    });

    checks.push({
      name: 'Connection status',
      passed: this.status === 'connected',
      message: `Current status: ${this.status}`,
    });

    const allPassed = checks.every((c) => c.passed);
    const somePassed = checks.some((c) => c.passed);

    return {
      module: 'brave-search',
      status: allPassed ? 'healthy' : somePassed ? 'degraded' : 'unhealthy',
      checks,
    };
  }

  private ensureConfigured(): void {
    if (!this.config.enabled) {
      throw new Error('Brave Search integration is not enabled. Enable it in config.');
    }
    if (!this.config.apiKey) {
      throw new Error('Brave Search API key is not configured.');
    }
  }

  private async checkApiReachability(): Promise<boolean> {
    try {
      // Simple test query to check API key and reachability
      await this.apiRequest(`${BraveSearchIntegration.API_BASE}/web/search?q=test&count=1`);
      return true;
    } catch {
      return false;
    }
  }

  private async apiRequest<T>(url: string): Promise<T> {
    const response = await fetch(url, {
      headers: {
        'Accept': 'application/json',
        'Accept-Encoding': 'gzip',
        'X-Subscription-Token': this.config.apiKey!,
      },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(`Brave Search API error (${response.status}): ${errorBody}`);
    }

    return response.json() as Promise<T>;
  }
}
