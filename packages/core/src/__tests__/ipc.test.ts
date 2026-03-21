import { describe, it, expect, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync, mkdirSync } from 'node:fs';
import WebSocket from 'ws';
import { Fort } from '../fort.js';
import { IPCServer } from '../ipc/index.js';

// Use a different port per test run to avoid collisions
let portCounter = 14_100;
function nextPort(): number {
  return portCounter++;
}

function waitForMessage(ws: WebSocket): Promise<Record<string, unknown>> {
  return new Promise((resolve) => {
    ws.once('message', (raw) => {
      resolve(JSON.parse(raw.toString()));
    });
  });
}

function connectClient(port: number): Promise<WebSocket> {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(`ws://localhost:${port}/shell`);
    ws.on('open', () => resolve(ws));
    ws.on('error', reject);
  });
}

describe('IPCServer', () => {
  let tmpDir: string;
  let fort: Fort;
  let port: number;
  const openClients: WebSocket[] = [];

  function setup(): Fort {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-ipc-test-'));
    const specsDir = join(tmpDir, 'specs');
    mkdirSync(specsDir, { recursive: true });

    port = nextPort();
    fort = new Fort({
      dataDir: join(tmpDir, 'data'),
      specsDir,
    });
    // Replace the default IPC with one on our test port
    (fort as unknown as Record<string, unknown>).ipc = new IPCServer(fort, port);
    return fort;
  }

  async function client(): Promise<WebSocket> {
    const ws = await connectClient(port);
    openClients.push(ws);
    return ws;
  }

  afterEach(async () => {
    for (const ws of openClients) {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    }
    openClients.length = 0;
    if (fort) await fort.stop();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  it('should start and accept connections', async () => {
    setup();
    await fort.start();

    const ws = await client();
    expect(ws.readyState).toBe(WebSocket.OPEN);
  });

  it('should respond to get_status action', async () => {
    setup();
    await fort.start();

    const ws = await client();
    const msgPromise = waitForMessage(ws);
    ws.send(JSON.stringify({ action: 'get_status' }));

    const msg = await msgPromise;
    expect(msg.type).toBe('status');
    expect(msg.data).toHaveProperty('agents');
    expect(msg.data).toHaveProperty('health');

    const data = msg.data as Record<string, unknown>;
    expect(data.health).toBe('green');
    expect(Array.isArray(data.agents)).toBe(true);
  });

  it('should respond to get_tasks action', async () => {
    setup();
    await fort.start();

    const ws = await client();
    const msgPromise = waitForMessage(ws);
    ws.send(JSON.stringify({ action: 'get_tasks' }));

    const msg = await msgPromise;
    expect(msg.type).toBe('tasks');

    const data = msg.data as Record<string, unknown>;
    expect(data).toHaveProperty('active');
    expect(data).toHaveProperty('queued');
    expect(data).toHaveProperty('history');
  });

  it('should respond to get_agents action', async () => {
    setup();
    await fort.start();

    const ws = await client();
    const msgPromise = waitForMessage(ws);
    ws.send(JSON.stringify({ action: 'get_agents' }));

    const msg = await msgPromise;
    expect(msg.type).toBe('agents');

    const data = msg.data as Record<string, unknown>;
    expect(Array.isArray(data.agents)).toBe(true);

    const agents = data.agents as Array<Record<string, unknown>>;
    expect(agents.length).toBeGreaterThanOrEqual(4);
    expect(agents[0]).toHaveProperty('id');
    expect(agents[0]).toHaveProperty('name');
    expect(agents[0]).toHaveProperty('status');
  });

  it('should respond to run_doctor action', async () => {
    setup();
    await fort.start();

    const ws = await client();
    const msgPromise = waitForMessage(ws);
    ws.send(JSON.stringify({ action: 'run_doctor' }));

    const msg = await msgPromise;
    expect(msg.type).toBe('doctor');

    const data = msg.data as Record<string, unknown>;
    expect(Array.isArray(data.results)).toBe(true);
    expect((data.results as unknown[]).length).toBeGreaterThan(0);
  });

  it('should broadcast status when agents change', async () => {
    setup();
    await fort.start();

    const ws = await client();
    const msgPromise = waitForMessage(ws);

    // Trigger an agent event
    await fort.bus.publish('agent.started', 'test', { agent: { name: 'test' } });

    const msg = await msgPromise;
    expect(msg.type).toBe('status');
    expect(msg.data).toHaveProperty('health');
  });

  it('should broadcast to multiple clients', async () => {
    setup();
    await fort.start();

    const ws1 = await client();
    const ws2 = await client();

    const msg1Promise = waitForMessage(ws1);
    const msg2Promise = waitForMessage(ws2);

    // Trigger a task event to broadcast
    await fort.bus.publish('task.created', 'test', { title: 'Test task' });

    const msg1 = await msg1Promise;
    const msg2 = await msg2Promise;

    expect(msg1.type).toBe('tasks');
    expect(msg2.type).toBe('tasks');
  });

  it('should handle client disconnect gracefully', async () => {
    setup();
    await fort.start();

    const ws = await client();
    expect(ws.readyState).toBe(WebSocket.OPEN);

    // Close the client
    ws.close();

    // Wait for close to propagate
    await new Promise((r) => setTimeout(r, 50));

    // Server should still be running — connect a new client
    const ws2 = await client();
    expect(ws2.readyState).toBe(WebSocket.OPEN);
  });

  it('should report connected clients in diagnostics', async () => {
    setup();
    await fort.start();

    // No clients yet
    let diag = fort.ipc.diagnose();
    const clientCheck = diag.checks.find((c) => c.name === 'Connected clients');
    expect(clientCheck).toBeDefined();
    expect(clientCheck!.message).toContain('0 client');

    // Connect a client
    await client();

    diag = fort.ipc.diagnose();
    const clientCheckAfter = diag.checks.find((c) => c.name === 'Connected clients');
    expect(clientCheckAfter!.message).toContain('1 client');
  });

  it('should return error for unknown action', async () => {
    setup();
    await fort.start();

    const ws = await client();
    const msgPromise = waitForMessage(ws);
    ws.send(JSON.stringify({ action: 'nonexistent_action' }));

    const msg = await msgPromise;
    expect(msg.type).toBe('error');

    const data = msg.data as Record<string, unknown>;
    expect(data.message).toContain('Unknown action');
  });
});
