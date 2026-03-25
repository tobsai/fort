import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { webSearchTool } from '../../tools/builtins/web-search.js';

// ─── Mock fetch ──────────────────────────────────────────────────────

function makeBraveResponse(results: Array<{ title: string; url: string; description: string }>) {
  return {
    ok: true,
    status: 200,
    statusText: 'OK',
    json: async () => ({ web: { results } }),
  } as unknown as Response;
}

describe('web-search tool', () => {
  const originalFetch = global.fetch;
  const originalEnv = process.env['BRAVE_SEARCH_API_KEY'];

  beforeEach(() => {
    process.env['BRAVE_SEARCH_API_KEY'] = 'test-api-key';
  });

  afterEach(() => {
    global.fetch = originalFetch;
    if (originalEnv === undefined) {
      delete process.env['BRAVE_SEARCH_API_KEY'];
    } else {
      process.env['BRAVE_SEARCH_API_KEY'] = originalEnv;
    }
    vi.restoreAllMocks();
  });

  it('has correct metadata', () => {
    expect(webSearchTool.name).toBe('web-search');
    expect(webSearchTool.tier).toBe(1);
    expect(webSearchTool.description).toBeTruthy();
  });

  it('returns results from Brave API', async () => {
    global.fetch = vi.fn().mockResolvedValueOnce(
      makeBraveResponse([
        { title: 'Fort Project', url: 'https://example.com/fort', description: 'A cool project' },
      ]),
    );

    const result = await webSearchTool.execute({ query: 'fort project' });

    expect(result.success).toBe(true);
    expect(result.output).toContain('Fort Project');
    expect(result.output).toContain('https://example.com/fort');
    expect(result.output).toContain('A cool project');
    expect(result.artifacts).toHaveLength(1);
  });

  it('passes the query and count to Brave API', async () => {
    let capturedUrl = '';
    global.fetch = vi.fn().mockImplementationOnce((url: string) => {
      capturedUrl = url;
      return Promise.resolve(makeBraveResponse([]));
    });

    await webSearchTool.execute({ query: 'hello world', count: 3 });

    expect(capturedUrl).toContain('q=hello+world');
    expect(capturedUrl).toContain('count=3');
  });

  it('sends the API key as X-Subscription-Token header', async () => {
    let capturedHeaders: any = {};
    global.fetch = vi.fn().mockImplementationOnce((_url: string, init: RequestInit) => {
      capturedHeaders = init.headers;
      return Promise.resolve(makeBraveResponse([]));
    });

    await webSearchTool.execute({ query: 'test' });

    expect((capturedHeaders as any)['X-Subscription-Token']).toBe('test-api-key');
  });

  it('returns error when BRAVE_SEARCH_API_KEY is not set', async () => {
    delete process.env['BRAVE_SEARCH_API_KEY'];

    const result = await webSearchTool.execute({ query: 'test' });

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/BRAVE_SEARCH_API_KEY/i);
  });

  it('returns error when query is missing', async () => {
    const result = await webSearchTool.execute({});
    expect(result.success).toBe(false);
    expect(result.error).toMatch(/query/i);
  });

  it('returns "No results found." when API returns empty results', async () => {
    global.fetch = vi.fn().mockResolvedValueOnce(makeBraveResponse([]));

    const result = await webSearchTool.execute({ query: 'obscure query' });

    expect(result.success).toBe(true);
    expect(result.output).toBe('No results found.');
    expect(result.artifacts).toEqual([]);
  });

  it('returns error when Brave API returns non-200', async () => {
    global.fetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      status: 429,
      statusText: 'Too Many Requests',
      json: async () => ({}),
    } as unknown as Response);

    const result = await webSearchTool.execute({ query: 'test' });

    expect(result.success).toBe(false);
    expect(result.error).toContain('429');
  });

  it('returns error on network failure', async () => {
    global.fetch = vi.fn().mockRejectedValueOnce(new Error('ECONNREFUSED'));

    const result = await webSearchTool.execute({ query: 'test' });

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/network error/i);
  });

  it('caps count at MAX_COUNT (20)', async () => {
    let capturedUrl = '';
    global.fetch = vi.fn().mockImplementationOnce((url: string) => {
      capturedUrl = url;
      return Promise.resolve(makeBraveResponse([]));
    });

    await webSearchTool.execute({ query: 'test', count: 9999 });

    expect(capturedUrl).toContain('count=20');
  });

  it('returns multiple results with numbered formatting', async () => {
    global.fetch = vi.fn().mockResolvedValueOnce(
      makeBraveResponse([
        { title: 'Result One', url: 'https://one.com', description: 'First result' },
        { title: 'Result Two', url: 'https://two.com', description: 'Second result' },
      ]),
    );

    const result = await webSearchTool.execute({ query: 'multi' });

    expect(result.success).toBe(true);
    expect(result.output).toContain('1. Result One');
    expect(result.output).toContain('2. Result Two');
    expect(result.artifacts).toHaveLength(2);
  });
});
