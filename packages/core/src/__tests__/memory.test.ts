import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { ModuleBus } from '../module-bus/index.js';
import { MemoryManager } from '../memory/index.js';

describe('MemoryManager', () => {
  let tmpDir: string;
  let memory: MemoryManager;
  let bus: ModuleBus;

  function setup() {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-test-'));
    bus = new ModuleBus();
    memory = new MemoryManager(join(tmpDir, 'memory.db'), bus);
    return { memory, bus };
  }

  afterEach(() => {
    memory?.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should create and retrieve nodes', () => {
    setup();
    const node = memory.createNode({
      type: 'person',
      label: 'Sarah',
      properties: { relationship: 'wife' },
      source: 'test',
    });

    expect(node.id).toBeTruthy();
    expect(node.type).toBe('person');
    expect(node.label).toBe('Sarah');

    const retrieved = memory.getNode(node.id);
    expect(retrieved).not.toBeNull();
    expect(retrieved!.label).toBe('Sarah');
  });

  it('should create and traverse edges', () => {
    setup();
    const toby = memory.createNode({ type: 'person', label: 'Toby' });
    const sarah = memory.createNode({ type: 'person', label: 'Sarah' });

    memory.createEdge({
      sourceId: toby.id,
      targetId: sarah.id,
      type: 'married_to',
    });

    const edges = memory.getEdgesFrom(toby.id);
    expect(edges).toHaveLength(1);
    expect(edges[0].type).toBe('married_to');
    expect(edges[0].targetId).toBe(sarah.id);
  });

  it('should search nodes by text', () => {
    setup();
    memory.createNode({ type: 'person', label: 'Sarah' });
    memory.createNode({ type: 'project', label: 'Fort Project' });
    memory.createNode({ type: 'preference', label: 'concise responses' });

    const results = memory.search({ text: 'Sarah' });
    expect(results.nodes).toHaveLength(1);
    expect(results.nodes[0].label).toBe('Sarah');
  });

  it('should search by node type', () => {
    setup();
    memory.createNode({ type: 'person', label: 'Toby' });
    memory.createNode({ type: 'person', label: 'Sarah' });
    memory.createNode({ type: 'project', label: 'Fort' });

    const results = memory.search({ nodeType: 'person' });
    expect(results.nodes).toHaveLength(2);
  });

  it('should traverse the graph', () => {
    setup();
    const toby = memory.createNode({ type: 'person', label: 'Toby' });
    const fort = memory.createNode({ type: 'project', label: 'Fort' });
    const ts = memory.createNode({ type: 'decision', label: 'Use TypeScript' });

    memory.createEdge({ sourceId: toby.id, targetId: fort.id, type: 'works_on' });
    memory.createEdge({ sourceId: fort.id, targetId: ts.id, type: 'decision' });

    const result = memory.traverse(toby.id, 2);
    expect(result.nodes.length).toBeGreaterThanOrEqual(2);
  });

  it('should update nodes', () => {
    setup();
    const node = memory.createNode({ type: 'fact', label: 'Old fact' });
    memory.updateNode(node.id, { label: 'Updated fact' });
    expect(memory.getNode(node.id)!.label).toBe('Updated fact');
  });

  it('should delete nodes and cascade edges', () => {
    setup();
    const a = memory.createNode({ type: 'person', label: 'A' });
    const b = memory.createNode({ type: 'person', label: 'B' });
    memory.createEdge({ sourceId: a.id, targetId: b.id, type: 'knows' });

    memory.deleteNode(a.id);
    expect(memory.getNode(a.id)).toBeNull();
    expect(memory.getEdgesTo(b.id)).toHaveLength(0);
  });

  it('should export the graph', () => {
    setup();
    memory.createNode({ type: 'fact', label: 'Test' });
    const exported = memory.exportGraph();
    expect(exported.nodes).toHaveLength(1);
  });

  it('should report stats', () => {
    setup();
    memory.createNode({ type: 'person', label: 'A' });
    memory.createNode({ type: 'project', label: 'B' });

    const stats = memory.stats();
    expect(stats.nodeCount).toBe(2);
    expect(stats.nodeTypes.person).toBe(1);
    expect(stats.nodeTypes.project).toBe(1);
  });
});
