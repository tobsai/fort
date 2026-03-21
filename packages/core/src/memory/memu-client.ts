/**
 * MemU Client — HTTP client for the MemU Python sidecar service
 *
 * MemU is the graph-based memory system. This client communicates
 * with it via localhost HTTP API.
 */

import type { MemoryNode, MemoryEdge, MemoryQuery, MemorySearchResult } from '../types.js';

export interface MemUClientConfig {
  baseUrl: string;
  timeout?: number;
}

export class MemUClient {
  private baseUrl: string;
  private timeout: number;

  constructor(config: MemUClientConfig) {
    this.baseUrl = config.baseUrl.replace(/\/$/, '');
    this.timeout = config.timeout ?? 5000;
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(`${this.baseUrl}${path}`, {
        ...options,
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
          ...options.headers,
        },
      });

      if (!response.ok) {
        throw new Error(`MemU API error: ${response.status} ${response.statusText}`);
      }

      return (await response.json()) as T;
    } finally {
      clearTimeout(timer);
    }
  }

  // ─── Health ─────────────────────────────────────────────────────

  async ping(): Promise<boolean> {
    try {
      await this.request('/health');
      return true;
    } catch {
      return false;
    }
  }

  // ─── Node Operations ───────────────────────────────────────────

  async createNode(node: Omit<MemoryNode, 'id' | 'createdAt' | 'updatedAt'>): Promise<MemoryNode> {
    return this.request<MemoryNode>('/nodes', {
      method: 'POST',
      body: JSON.stringify(node),
    });
  }

  async getNode(nodeId: string): Promise<MemoryNode> {
    return this.request<MemoryNode>(`/nodes/${nodeId}`);
  }

  async updateNode(nodeId: string, updates: Partial<Pick<MemoryNode, 'label' | 'properties'>>): Promise<MemoryNode> {
    return this.request<MemoryNode>(`/nodes/${nodeId}`, {
      method: 'PATCH',
      body: JSON.stringify(updates),
    });
  }

  async deleteNode(nodeId: string): Promise<void> {
    await this.request(`/nodes/${nodeId}`, { method: 'DELETE' });
  }

  async listNodes(type?: MemoryNode['type'], limit?: number): Promise<MemoryNode[]> {
    const params = new URLSearchParams();
    if (type) params.set('type', type);
    if (limit) params.set('limit', String(limit));
    return this.request<MemoryNode[]>(`/nodes?${params}`);
  }

  // ─── Edge Operations ───────────────────────────────────────────

  async createEdge(edge: Omit<MemoryEdge, 'id' | 'createdAt'>): Promise<MemoryEdge> {
    return this.request<MemoryEdge>('/edges', {
      method: 'POST',
      body: JSON.stringify(edge),
    });
  }

  async getEdgesFrom(nodeId: string): Promise<MemoryEdge[]> {
    return this.request<MemoryEdge[]>(`/nodes/${nodeId}/edges?direction=outgoing`);
  }

  async getEdgesTo(nodeId: string): Promise<MemoryEdge[]> {
    return this.request<MemoryEdge[]>(`/nodes/${nodeId}/edges?direction=incoming`);
  }

  async deleteEdge(edgeId: string): Promise<void> {
    await this.request(`/edges/${edgeId}`, { method: 'DELETE' });
  }

  // ─── Search & Traversal ────────────────────────────────────────

  async search(query: MemoryQuery): Promise<MemorySearchResult> {
    return this.request<MemorySearchResult>('/search', {
      method: 'POST',
      body: JSON.stringify(query),
    });
  }

  async traverse(startNodeId: string, depth: number = 2): Promise<MemorySearchResult> {
    return this.request<MemorySearchResult>(
      `/traverse/${startNodeId}?depth=${depth}`
    );
  }

  // ─── Graph Operations ──────────────────────────────────────────

  async export(): Promise<{ nodes: MemoryNode[]; edges: MemoryEdge[] }> {
    return this.request('/export');
  }

  async importGraph(data: { nodes: MemoryNode[]; edges: MemoryEdge[] }): Promise<void> {
    await this.request('/import', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async stats(): Promise<{
    nodeCount: number;
    edgeCount: number;
    nodeTypes: Record<string, number>;
    edgeTypes: Record<string, number>;
  }> {
    return this.request('/stats');
  }
}
