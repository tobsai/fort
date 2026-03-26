/**
 * web-search — Tier 1 (auto)
 *
 * Search the web via the Brave Search API.
 * Requires BRAVE_SEARCH_API_KEY environment variable.
 * Returns an array of { title, url, snippet } results.
 */

import type { FortTool, ToolResult } from '../types.js';

export interface WebSearchInput {
  query: string;
  count?: number;
}

export interface SearchResult {
  title: string;
  url: string;
  snippet: string;
}

const BRAVE_API_URL = 'https://api.search.brave.com/res/v1/web/search';
const DEFAULT_COUNT = 5;
const MAX_COUNT = 20;

export const webSearchTool: FortTool = {
  name: 'web-search',
  description: 'Search the web via Brave Search API. Returns title, URL, and snippet for each result.',
  inputSchema: {
    type: 'object',
    properties: {
      query: { type: 'string', description: 'Search query.' },
      count: {
        type: 'number',
        description: `Number of results to return (1–${MAX_COUNT}, default ${DEFAULT_COUNT}).`,
      },
    },
    required: ['query'],
  },
  tier: 1,

  async execute(input: unknown): Promise<ToolResult> {
    const { query, count } = input as WebSearchInput;

    if (!query || typeof query !== 'string') {
      return { success: false, output: '', error: 'Input must include a "query" string.' };
    }

    const apiKey = process.env['BRAVE_SEARCH_API_KEY'];
    if (!apiKey) {
      return {
        success: false,
        output: '',
        error: 'BRAVE_SEARCH_API_KEY environment variable is not set.',
      };
    }

    const resultCount = Math.min(Math.max(count ?? DEFAULT_COUNT, 1), MAX_COUNT);

    const url = new URL(BRAVE_API_URL);
    url.searchParams.set('q', query);
    url.searchParams.set('count', String(resultCount));

    let resp: Response;
    try {
      resp = await fetch(url.toString(), {
        headers: {
          Accept: 'application/json',
          'Accept-Encoding': 'gzip',
          'X-Subscription-Token': apiKey,
        },
      });
    } catch (err) {
      return { success: false, output: '', error: `Network error: ${(err as Error).message}` };
    }

    if (!resp.ok) {
      return {
        success: false,
        output: '',
        error: `Brave API returned ${resp.status}: ${resp.statusText}`,
      };
    }

    let data: any;
    try {
      data = await resp.json();
    } catch (err) {
      return { success: false, output: '', error: 'Failed to parse Brave API response as JSON.' };
    }

    const webResults: any[] = data?.web?.results ?? [];
    const results: SearchResult[] = webResults.slice(0, resultCount).map((r: any) => ({
      title: r.title ?? '',
      url: r.url ?? '',
      snippet: r.description ?? '',
    }));

    if (results.length === 0) {
      return { success: true, output: 'No results found.', artifacts: [] };
    }

    const lines = results.map((r, i) => `${i + 1}. ${r.title}\n   ${r.url}\n   ${r.snippet}`);
    const output = lines.join('\n\n');

    return { success: true, output, artifacts: results };
  },
};
