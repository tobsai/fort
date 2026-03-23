/**
 * Fort WebSocket Server
 *
 * Provides a WebSocket interface for the Swift menu bar app
 * and other clients to communicate with the Fort core.
 */

import { createServer, type Server, type IncomingMessage, type ServerResponse } from 'node:http';
import { writeFileSync, existsSync, readFileSync, statSync } from 'node:fs';
import { join, extname } from 'node:path';
import { stringify as stringifyYaml } from 'yaml';
import { WebSocketServer, WebSocket } from 'ws';
import type { Fort } from '../fort.js';

// Resolve dashboard dist directory relative to this file's compiled location
// At runtime: packages/core/dist/server/index.js
// Target:     packages/dashboard/dist
const DASHBOARD_DIST = join(__dirname, '..', '..', '..', 'dashboard', 'dist');

const MIME_TYPES: Record<string, string> = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript',
  '.css': 'text/css',
  '.json': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
};

interface WSMessage {
  id: string;
  type: string;
  payload?: unknown;
}

interface WSResponse {
  id: string;
  type: string;
  payload: unknown;
  error?: string;
}

export class FortServer {
  private httpServer: Server;
  private wss: WebSocketServer;
  private fort: Fort;
  private clients: Set<WebSocket> = new Set();
  private port: number;

  constructor(fort: Fort, port: number = 4077) {
    this.fort = fort;
    this.port = port;

    this.httpServer = createServer((req, res) => {
      if (req.url === '/health') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ status: 'ok', agents: this.fort.agents.listInfo().length }));
        return;
      }
      if (req.url === '/api/setup-status' && req.method === 'GET') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ complete: this.fort.isSetupComplete() }));
        return;
      }
      // GET /api/llm-status — validate LLM authentication
      if (req.url === '/api/llm-status') {
        const llm = this.fort.llm;
        const status = {
          configured: llm.isConfigured,
          authMethod: llm.authMethod,
          defaultTier: (llm as any).defaultTier ?? 'standard',
        };
        // Async validation
        llm.validateAuth().then((error) => {
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ ...status, valid: !error, error: error ?? undefined }));
        }).catch(() => {
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ ...status, valid: false, error: 'Validation failed' }));
        });
        return;
      }
      // Serve default avatar image
      if (req.url === '/api/default-avatar') {
        const { existsSync: exists, readFileSync: readFile } = require('node:fs');
        const { join: pathJoin } = require('node:path');
        // Check for a custom default avatar in .fort/
        const { homedir } = require('node:os');
        const customDefault = pathJoin(homedir(), '.fort', 'default-avatar.png');
        if (exists(customDefault)) {
          res.writeHead(200, { 'Content-Type': 'image/png' });
          res.end(readFile(customDefault));
          return;
        }
        // Generate a simple SVG avatar as fallback
        const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 200 200">
          <rect width="200" height="200" rx="100" fill="#1e1e28"/>
          <text x="100" y="115" font-size="80" text-anchor="middle" fill="#6c5ce7">\u{1F3F0}</text>
        </svg>`;
        res.writeHead(200, { 'Content-Type': 'image/svg+xml' });
        res.end(svg);
        return;
      }
      if (req.url === '/api/agents/create' && req.method === 'POST') {
        this.handleCreateAgent(req, res);
        return;
      }
      if (req.url === '/api/agents') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        const agents = this.fort.agents.listInfo().map((a) => ({
          ...a,
          soul: this.fort.agentFactory.getSoul(a.config.id) ?? undefined,
          emoji: this.getAgentEmoji(a.config.id),
        }));
        res.end(JSON.stringify(agents));
        return;
      }
      // /api/agents/:id/soul
      const soulMatch = req.url?.match(/^\/api\/agents\/([^/]+)\/soul$/);
      if (soulMatch) {
        const soul = this.fort.agentFactory.getSoul(soulMatch[1]);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ soul: soul ?? null }));
        return;
      }
      // GET /api/chat-history — return recent chat tasks with results
      if (req.url === '/api/chat-history') {
        const tasks = this.fort.taskGraph.getAllTasks()
          .filter((t) => t.metadata.type === 'chat' && t.result)
          .sort((a, b) => a.createdAt.getTime() - b.createdAt.getTime())
          .slice(-100)
          .map((t) => ({
            id: t.id,
            shortId: t.shortId,
            title: t.title,
            description: t.description,
            result: t.result,
            status: t.status,
            source: t.source,
            assignedAgent: t.assignedAgent,
            createdAt: t.createdAt,
          }));
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(tasks));
        return;
      }
      // Serve agent avatar images: /api/agents/:id/avatar
      const avatarMatch = req.url?.match(/^\/api\/agents\/([^/]+)\/avatar$/);
      if (avatarMatch) {
        const agentDir = this.fort.agentFactory.getAgentDir(avatarMatch[1]);
        const { existsSync: exists, readFileSync: readFile } = require('node:fs');
        const { join: pathJoin } = require('node:path');
        for (const ext of ['png', 'jpg', 'jpeg', 'webp']) {
          const avatarPath = pathJoin(agentDir, `avatar.${ext}`);
          if (exists(avatarPath)) {
            const mime = ext === 'jpg' ? 'image/jpeg' : `image/${ext}`;
            res.writeHead(200, { 'Content-Type': mime });
            res.end(readFile(avatarPath));
            return;
          }
        }
        res.writeHead(404);
        res.end();
        return;
      }
      // Static file serving from dashboard dist + SPA fallback
      this.serveStatic(req, res);
    });

    this.wss = new WebSocketServer({ server: this.httpServer });
    this.setupWebSocket();
    this.setupEventBroadcast();
  }

  private setupWebSocket(): void {
    this.wss.on('connection', (ws) => {
      this.clients.add(ws);

      ws.on('message', async (data) => {
        try {
          const msg = JSON.parse(data.toString()) as WSMessage;
          const response = await this.handleMessage(msg);
          ws.send(JSON.stringify(response));
        } catch (err) {
          ws.send(JSON.stringify({
            id: 'error',
            type: 'error',
            error: err instanceof Error ? err.message : String(err),
          }));
        }
      });

      ws.on('close', () => {
        this.clients.delete(ws);
      });

      // Send initial state
      ws.send(JSON.stringify({
        id: 'init',
        type: 'state',
        payload: this.getState(),
      }));
    });
  }

  private setupEventBroadcast(): void {
    // Broadcast task changes to all clients
    this.fort.bus.subscribe('task.created', (event) => {
      this.broadcast({ id: event.id, type: 'task.created', payload: event.payload });
    });

    this.fort.bus.subscribe('task.status_changed', (event) => {
      // The payload includes { task, previousStatus, newStatus, reason }
      // Broadcast the full task object so the portal can show shortId, result, etc.
      const payload = event.payload as Record<string, unknown>;
      const task = payload?.task ?? payload;
      this.broadcast({ id: event.id, type: 'task.status_changed', payload: task });
    });

    this.fort.bus.subscribe('agent.acknowledged', (event) => {
      this.broadcast({ id: event.id, type: 'agent.acknowledged', payload: event.payload });
    });

    this.fort.bus.subscribe('agent.started', (event) => {
      this.broadcast({ id: event.id, type: 'agent.started', payload: event.payload });
    });

    this.fort.bus.subscribe('agent.error', (event) => {
      this.broadcast({ id: event.id, type: 'agent.error', payload: event.payload });
    });

    this.fort.bus.subscribe('reflection.insight', (event) => {
      this.broadcast({ id: event.id, type: 'reflection.insight', payload: event.payload });
    });
  }

  private async handleMessage(msg: WSMessage): Promise<WSResponse> {
    switch (msg.type) {
      case 'chat':
        const chatPayload = typeof msg.payload === 'string'
          ? { text: msg.payload, agentId: undefined, hidden: false, modelTier: undefined }
          : (msg.payload as { text: string; agentId?: string; hidden?: boolean; modelTier?: string });
        const isGreeting = chatPayload.text === '__greeting__';
        const now = new Date();
        const dayName = now.toLocaleDateString('en-US', { weekday: 'long' });
        const dateFull = now.toLocaleDateString('en-US', { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' });
        const hour = now.getHours();
        const timeOfDay = hour < 12 ? 'morning' : hour < 17 ? 'afternoon' : 'evening';
        const chatText = isGreeting
          ? `Today is ${dateFull}. It is currently ${timeOfDay} (${now.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })}). Please greet me warmly for this ${dayName} ${timeOfDay} and ask what I would like to accomplish today. Be yourself — use your personality from your SOUL.md.`
          : chatPayload.text;
        const task = await this.fort.chat(
          chatText,
          isGreeting ? 'background' : 'user_chat',
          chatPayload.agentId,
          isGreeting ? 'fast' : chatPayload.modelTier,
        );
        return {
          id: msg.id,
          type: 'chat.response',
          payload: {
            taskId: task.id,
            task,
            hidden: chatPayload.hidden || isGreeting,
          },
        };

      case 'status':
        return { id: msg.id, type: 'status.response', payload: this.getState() };

      case 'tasks':
        return {
          id: msg.id,
          type: 'tasks.response',
          payload: this.fort.taskGraph.getAllTasks().slice(0, 100),
        };

      case 'tasks.active':
        return {
          id: msg.id,
          type: 'tasks.active.response',
          payload: this.fort.taskGraph.getActiveTasks(),
        };

      case 'agents':
        return {
          id: msg.id,
          type: 'agents.response',
          payload: this.fort.agents.listInfo().map((a) => ({
            ...a,
            soul: this.fort.agentFactory.getSoul(a.config.id) ?? undefined,
            emoji: this.getAgentEmoji(a.config.id),
          })),
        };

      case 'memory.search':
        return {
          id: msg.id,
          type: 'memory.search.response',
          payload: this.fort.memory.search(msg.payload as any),
        };

      case 'memory.stats':
        return {
          id: msg.id,
          type: 'memory.stats.response',
          payload: this.fort.memory.stats(),
        };

      case 'doctor':
        const results = await this.fort.runDoctor();
        return { id: msg.id, type: 'doctor.response', payload: results };

      default:
        return { id: msg.id, type: 'error', payload: null, error: `Unknown message type: ${msg.type}` };
    }
  }

  private getState(): unknown {
    return {
      agents: this.fort.agents.listInfo(),
      activeTasks: this.fort.taskGraph.getActiveTasks().length,
      totalTasks: this.fort.taskGraph.getTaskCount(),
      memoryStats: this.fort.memory.stats(),
    };
  }

  private broadcast(msg: WSResponse): void {
    const data = JSON.stringify(msg);
    for (const client of this.clients) {
      if (client.readyState === WebSocket.OPEN) {
        client.send(data);
      }
    }
  }

  private handleCreateAgent(req: IncomingMessage, res: ServerResponse): void {
    let body = '';
    req.on('data', (chunk: Buffer) => { body += chunk.toString(); });
    req.on('end', async () => {
      try {
        const { name, goals, emoji, personality, avatarDataUrl, modelTier } = JSON.parse(body) as {
          name: string;
          goals?: string;
          emoji?: string;
          personality?: string;
          avatarDataUrl?: string | null;
          modelTier?: 'fast' | 'standard' | 'powerful';
        };

        if (!name || !name.trim()) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'Agent name is required' }));
          return;
        }

        // Create the agent
        const agent = this.fort.agentFactory.create({
          name: name.trim(),
          description: goals || 'A Fort specialist agent.',
          emoji: emoji || undefined,
        });

        // Set isDefault and modelTier on the identity and re-save
        const identity = agent.identity;
        (identity as any).isDefault = true;
        if (modelTier) (identity as any).defaultModelTier = modelTier;
        // Re-save identity with isDefault flag
        const agentDir = join(
          (this.fort.agentFactory as any).agentsDir,
          identity.id,
        );
        try {
          writeFileSync(
            join(agentDir, 'identity.yaml'),
            stringifyYaml(identity),
            'utf-8',
          );
        } catch {
          // Fallback: use the factory's internal save
          (this.fort.agentFactory as any).saveIdentity(identity);
        }

        // Write SOUL.md from wizard answers
        const goalsText = goals?.trim() || 'General-purpose assistant.';
        const personalityText = personality?.trim() || 'Helpful, concise, and action-oriented.';

        const soulContent = `# ${name.trim()}

${goalsText}

## Goals
${goalsText}

## Personality
${personalityText}

## Rules
- Every request should result in a clear action or response
- Always acknowledge the user before working on complex tasks
- Be transparent about limitations

## Boundaries
- Never send messages to contacts without explicit approval
- Never make purchases or financial decisions
- Never modify system settings without confirmation
`;
        writeFileSync(join(agentDir, 'SOUL.md'), soulContent, 'utf-8');

        // Reload the soul into the agent's cache and start it
        agent.refreshSoul();
        await agent.start();

        // Save avatar image if uploaded
        if (avatarDataUrl) {
          try {
            // avatarDataUrl is "data:image/png;base64,..."
            const matches = avatarDataUrl.match(/^data:image\/(png|jpeg|jpg|webp);base64,(.+)$/);
            if (matches) {
              const ext = matches[1] === 'jpeg' ? 'jpg' : matches[1];
              const buffer = Buffer.from(matches[2], 'base64');
              writeFileSync(join(agentDir, `avatar.${ext}`), buffer);
            }
          } catch {
            // Avatar save failed — non-fatal, agent still created
          }
        } else {
          // Copy default avatar if one exists in ~/.fort/
          try {
            const { homedir } = require('node:os');
            const defaultAvatar = join(homedir(), '.fort', 'default-avatar.png');
            if (existsSync(defaultAvatar)) {
              const { copyFileSync } = require('node:fs');
              copyFileSync(defaultAvatar, join(agentDir, 'avatar.png'));
            }
          } catch {
            // Non-fatal
          }
        }

        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          id: identity.id,
          name: identity.name,
          emoji: identity.emoji,
          isDefault: true,
        }));
      } catch (err) {
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          error: err instanceof Error ? err.message : String(err),
        }));
      }
    });
  }

  private serveStatic(req: IncomingMessage, res: ServerResponse): void {
    const url = req.url?.split('?')[0] || '/';
    const filePath = join(DASHBOARD_DIST, url === '/' ? 'index.html' : url);

    // Try to serve the requested file
    if (existsSync(filePath) && statSync(filePath).isFile()) {
      const ext = extname(filePath);
      const mime = MIME_TYPES[ext] || 'application/octet-stream';
      res.writeHead(200, { 'Content-Type': mime });
      res.end(readFileSync(filePath));
      return;
    }

    // SPA fallback: serve index.html for all non-file routes (client-side routing)
    const indexPath = join(DASHBOARD_DIST, 'index.html');
    if (existsSync(indexPath)) {
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
      res.end(readFileSync(indexPath));
      return;
    }

    // Dashboard not built yet
    res.writeHead(503, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end('<h1>Dashboard not built</h1><p>Run <code>npm run build</code> in packages/dashboard first.</p>');
  }

  private getAgentEmoji(agentId: string): string | undefined {
    try {
      const identities = this.fort.agentFactory.listIdentities();
      const identity = identities.find((i) => i.id === agentId);
      return identity?.emoji;
    } catch {
      return undefined;
    }
  }

  async start(): Promise<void> {
    // Validate LLM auth and auto-downgrade tier if needed (e.g. OAuth with limited model access)
    // Must complete before accepting connections so the default tier is correct
    if (this.fort.llm.isConfigured) {
      await this.fort.llm.validateAuth().catch(() => {});
    }

    return new Promise((resolve) => {
      this.httpServer.listen(this.port, () => {
        resolve();
      });
    });
  }

  async stop(): Promise<void> {
    for (const client of this.clients) {
      client.close();
    }
    this.clients.clear();

    return new Promise((resolve, reject) => {
      this.wss.close(() => {
        this.httpServer.close((err) => {
          if (err) reject(err);
          else resolve();
        });
      });
    });
  }

  get address(): string {
    return `ws://localhost:${this.port}`;
  }
}
