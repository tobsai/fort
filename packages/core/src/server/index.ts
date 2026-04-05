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
import {
  loadAuthConfig,
  buildGoogleAuthUrl,
  exchangeCodeForEmail,
  getSessionEmail,
  isEmailAllowed,
  buildSessionCookieHeader,
  buildClearCookieHeader,
  generateOAuthState,
  buildStateCookieHeader,
  buildClearStateCookieHeader,
  getOAuthState,
  type GoogleAuthConfig,
} from './auth.js';

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
  private authConfig: GoogleAuthConfig;
  // Maps in-flight task IDs → thread IDs so agent responses can be persisted
  private taskToThread = new Map<string, string>();

  constructor(fort: Fort, port: number = 4077) {
    this.fort = fort;
    this.port = port;
    this.authConfig = loadAuthConfig();

    this.httpServer = createServer((req, res) => {
      // Always allow health check without auth
      if (req.url === '/health') {
        this.handleHealthCheck(res);
        return;
      }

      // Auth routes (always public)
      if (req.url === '/auth/google') {
        this.handleAuthGoogle(req, res);
        return;
      }
      if (req.url?.startsWith('/auth/google/callback')) {
        this.handleAuthCallback(req, res);
        return;
      }
      if (req.url === '/auth/logout') {
        this.handleAuthLogout(req, res);
        return;
      }

      // Enforce auth for all other routes when enabled
      if (this.authConfig.authEnabled && !this.isAuthenticated(req)) {
        // API requests get 401; browser requests get redirected
        if (req.url?.startsWith('/api/') || req.headers['upgrade'] === 'websocket') {
          res.writeHead(401, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'Unauthorized' }));
        } else {
          res.writeHead(302, { Location: '/auth/google' });
          res.end();
        }
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
      if (req.url === '/api/import/openclaw/preview' && req.method === 'GET') {
        this.handleOpenClawPreview(res);
        return;
      }
      if (req.url === '/api/import/openclaw' && req.method === 'POST') {
        this.handleOpenClawImport(res);
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

    this.wss = new WebSocketServer({
      server: this.httpServer,
      verifyClient: ({ req }: { req: IncomingMessage }) => {
        if (!this.authConfig.authEnabled) return true;
        return this.isAuthenticated(req);
      },
    });
    this.setupWebSocket();
    this.setupEventBroadcast();

    // Wire notification push → broadcast 'notification.new' to all WS clients
    this.fort.notifications.onNotification((n) => {
      this.broadcast({ id: 'notification.new', type: 'notification.new', payload: n });
    });

    // Wire approval.required bus event → broadcast 'approval.new' to all WS clients
    this.fort.bus.subscribe('approval.required', (event) => {
      this.broadcast({ id: event.id, type: 'approval.new', payload: event.payload });
    });
  }

  private isAuthenticated(req: IncomingMessage): boolean {
    const email = getSessionEmail(req.headers.cookie, this.authConfig.sessionSecret);
    if (!email) return false;
    return isEmailAllowed(email, this.authConfig.allowedEmails);
  }

  private handleAuthGoogle(req: IncomingMessage, res: ServerResponse): void {
    if (!this.authConfig.clientId) {
      res.writeHead(500, { 'Content-Type': 'text/plain' });
      res.end('GOOGLE_CLIENT_ID not configured');
      return;
    }
    const state = generateOAuthState();
    const isSecure = !req.headers.host?.startsWith('localhost');
    const stateCookie = buildStateCookieHeader(state, isSecure);
    const url = buildGoogleAuthUrl(this.authConfig.clientId, this.authConfig.callbackUrl, state);
    res.writeHead(302, { Location: url, 'Set-Cookie': stateCookie });
    res.end();
  }

  private handleAuthCallback(req: IncomingMessage, res: ServerResponse): void {
    const urlStr = `http://localhost${req.url}`;
    let code: string | null = null;
    let callbackState: string | null = null;
    try {
      const parsed = new URL(urlStr);
      code = parsed.searchParams.get('code');
      callbackState = parsed.searchParams.get('state');
    } catch {
      res.writeHead(400, { 'Content-Type': 'text/plain' });
      res.end('Bad request');
      return;
    }

    if (!code) {
      res.writeHead(400, { 'Content-Type': 'text/plain' });
      res.end('Missing code parameter');
      return;
    }

    // CSRF: Verify state parameter matches the cookie we set
    const cookieState = getOAuthState(req.headers.cookie);
    if (!callbackState || !cookieState || callbackState !== cookieState) {
      res.writeHead(403, { 'Content-Type': 'text/plain' });
      res.end('Invalid OAuth state — possible CSRF attack. Try logging in again.');
      return;
    }

    exchangeCodeForEmail(
      code,
      this.authConfig.clientId,
      this.authConfig.clientSecret,
      this.authConfig.callbackUrl,
    ).then((email) => {
      if (!isEmailAllowed(email, this.authConfig.allowedEmails)) {
        res.writeHead(403, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'access_denied', email }));
        return;
      }

      const isSecure = !req.headers.host?.startsWith('localhost');
      const sessionCookie = buildSessionCookieHeader(
        email,
        this.authConfig.sessionSecret,
        isSecure,
      );
      const clearStateCookie = buildClearStateCookieHeader();
      res.writeHead(302, {
        Location: '/',
        'Set-Cookie': [sessionCookie, clearStateCookie],
      });
      res.end();
    }).catch((err) => {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: err instanceof Error ? err.message : String(err) }));
    });
  }

  private handleAuthLogout(_req: IncomingMessage, res: ServerResponse): void {
    res.writeHead(302, {
      Location: '/auth/google',
      'Set-Cookie': buildClearCookieHeader(),
    });
    res.end();
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
      const task = (payload?.task ?? payload) as Record<string, unknown>;
      this.broadcast({ id: event.id, type: 'task.status_changed', payload: task });

      // Persist agent response to thread when task completes
      const taskId = task?.id as string | undefined;
      if (taskId && this.taskToThread.has(taskId)) {
        const result = task?.result as string | undefined;
        const agentId = task?.assignedAgent as string | undefined;
        if (result) {
          const threadId = this.taskToThread.get(taskId)!;
          this.taskToThread.delete(taskId);
          try {
            this.fort.threads.addMessage(threadId, {
              role: 'agent',
              content: result,
              agentId: agentId ?? undefined,
            });
          } catch {
            // Non-fatal — thread may have been deleted
          }
        }
      }
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

    // Broadcast tool execution events so the dashboard can show tool calls live
    this.fort.bus.subscribe('tool.executed', (event) => {
      this.broadcast({ id: event.id, type: 'tool.executed', payload: event.payload });
    });

    this.fort.bus.subscribe('tool.denied', (event) => {
      this.broadcast({ id: event.id, type: 'tool.denied', payload: event.payload });
    });

    this.fort.bus.subscribe('tool.error', (event) => {
      this.broadcast({ id: event.id, type: 'tool.error', payload: event.payload });
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

        // Persist user message to thread (skip internal greeting messages)
        let chatThreadId: string | undefined;
        const chatAgentId = chatPayload.agentId ?? this.fort.orchestrator.findDefaultAgentId();
        if (!isGreeting && chatAgentId) {
          try {
            chatThreadId = this.getOrCreateAgentThread(chatAgentId);
            this.fort.threads.addMessage(chatThreadId, {
              role: 'user',
              content: chatPayload.text,
            });
          } catch {
            // Non-fatal — chat still proceeds without persistence
          }
        }

        const task = await this.fort.chat(
          chatText,
          isGreeting ? 'background' : 'user_chat',
          chatPayload.agentId,
          isGreeting ? 'fast' : chatPayload.modelTier,
        );

        // Track task → thread so agent response can be persisted on task.status_changed
        if (!isGreeting && chatThreadId) {
          this.taskToThread.set(task.id, chatThreadId);
        }

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

      case 'tasks.query': {
        const queryPayload = (msg.payload ?? {}) as {
          status?: string | string[];
          assignedAgent?: string;
          since?: string;
          limit?: number;
          offset?: number;
        };
        const tasks = this.fort.taskGraph.queryTasksFromStore({
          status: queryPayload.status as any,
          assignedAgent: queryPayload.assignedAgent,
          since: queryPayload.since ? new Date(queryPayload.since) : undefined,
          limit: queryPayload.limit ?? 50,
          offset: queryPayload.offset ?? 0,
        });
        // Enrich each task with its direct subtasks
        const tasksWithSubtasks = tasks.map((task) => ({
          ...task,
          subtasks: this.fort.taskGraph.getSubtasksFromStore(task.id),
        }));
        return { id: msg.id, type: 'tasks.query.response', payload: tasksWithSubtasks };
      }

      case 'agents':
      case 'agents.list':
        return {
          id: msg.id,
          type: 'agents.response',
          payload: this.buildAgentList(),
        };

      case 'agent.get': {
        const getPayload = (msg.payload ?? {}) as { id: string };
        if (!getPayload.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'agent.get requires id' };
        }
        const record = this.fort.agentStore.get(getPayload.id);
        if (!record) {
          return { id: msg.id, type: 'error', payload: null, error: `Agent not found: ${getPayload.id}` };
        }
        const registryAgent = this.fort.agents.get(getPayload.id);
        return {
          id: msg.id,
          type: 'agent.get.response',
          payload: {
            ...record,
            runtimeStatus: registryAgent?.status ?? 'stopped',
            taskCount: registryAgent?.info.taskCount ?? 0,
            soul: record.soul ?? this.fort.agentFactory.getSoul(getPayload.id) ?? null,
          },
        };
      }

      case 'agent.create': {
        const createPayload = (msg.payload ?? {}) as {
          name: string;
          description?: string;
          emoji?: string;
          soul?: string;
          modelPreference?: string;
          capabilities?: string[];
          eventSubscriptions?: string[];
        };
        if (!createPayload.name?.trim()) {
          return { id: msg.id, type: 'error', payload: null, error: 'agent.create requires name' };
        }
        try {
          const agent = this.fort.agentFactory.create({
            name: createPayload.name.trim(),
            description: createPayload.description,
            capabilities: createPayload.capabilities,
            eventSubscriptions: createPayload.eventSubscriptions,
            emoji: createPayload.emoji,
          });
          // Write soul if provided
          if (createPayload.soul) {
            const { writeFileSync } = await import('node:fs');
            const { join: pathJoin } = await import('node:path');
            const agentDir = this.fort.agentFactory.getAgentDir(agent.identity.id);
            writeFileSync(pathJoin(agentDir, 'SOUL.md'), createPayload.soul, 'utf-8');
            agent.refreshSoul();
          }
          await agent.start();
          // Persist to AgentStore
          this.fort.agentStore.create({
            id: agent.identity.id,
            name: agent.identity.name,
            description: agent.identity.description ?? '',
            capabilities: agent.identity.capabilities ?? [],
            eventSubscriptions: agent.identity.eventSubscriptions ?? [],
            memoryPartition: agent.identity.memoryPartition ?? agent.identity.id,
            emoji: createPayload.emoji,
            soul: createPayload.soul ?? this.fort.agentFactory.getSoul(agent.identity.id) ?? undefined,
            modelPreference: createPayload.modelPreference,
          });
          this.broadcast({
            id: 'agent.created',
            type: 'agents.updated',
            payload: this.buildAgentList(),
          });
          return {
            id: msg.id,
            type: 'agent.create.response',
            payload: { id: agent.identity.id, name: agent.identity.name, emoji: agent.identity.emoji },
          };
        } catch (err) {
          return {
            id: msg.id,
            type: 'error',
            payload: null,
            error: err instanceof Error ? err.message : String(err),
          };
        }
      }

      case 'agent.update': {
        const updatePayload = (msg.payload ?? {}) as {
          id: string;
          name?: string;
          description?: string;
          emoji?: string;
          soul?: string;
          modelPreference?: string;
        };
        if (!updatePayload.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'agent.update requires id' };
        }
        try {
          const record = this.fort.agentStore.update(updatePayload.id, {
            name: updatePayload.name,
            description: updatePayload.description,
            emoji: updatePayload.emoji,
            soul: updatePayload.soul,
            modelPreference: updatePayload.modelPreference,
          });
          this.broadcast({
            id: 'agent.updated',
            type: 'agents.updated',
            payload: this.buildAgentList(),
          });
          return { id: msg.id, type: 'agent.update.response', payload: record };
        } catch (err) {
          return {
            id: msg.id,
            type: 'error',
            payload: null,
            error: err instanceof Error ? err.message : String(err),
          };
        }
      }

      case 'agent.start': {
        const startPayload = (msg.payload ?? {}) as { id: string };
        if (!startPayload.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'agent.start requires id' };
        }
        try {
          let agent = this.fort.agents.get(startPayload.id);
          if (!agent) {
            // Agent not in registry — try to revive
            agent = this.fort.agentFactory.revive(startPayload.id);
          }
          if (agent.status !== 'running') {
            await agent.start();
          }
          this.broadcast({
            id: 'agent.started.broadcast',
            type: 'agents.updated',
            payload: this.buildAgentList(),
          });
          return { id: msg.id, type: 'agent.start.response', payload: { id: startPayload.id, status: 'running' } };
        } catch (err) {
          return {
            id: msg.id,
            type: 'error',
            payload: null,
            error: err instanceof Error ? err.message : String(err),
          };
        }
      }

      case 'agent.stop': {
        const stopPayload = (msg.payload ?? {}) as { id: string };
        if (!stopPayload.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'agent.stop requires id' };
        }
        try {
          const agent = this.fort.agents.get(stopPayload.id);
          if (!agent) {
            return { id: msg.id, type: 'error', payload: null, error: `Agent not in registry: ${stopPayload.id}` };
          }
          if (agent.status === 'running') {
            await agent.stop();
          }
          this.broadcast({
            id: 'agent.stopped.broadcast',
            type: 'agents.updated',
            payload: this.buildAgentList(),
          });
          return { id: msg.id, type: 'agent.stop.response', payload: { id: stopPayload.id, status: 'stopped' } };
        } catch (err) {
          return {
            id: msg.id,
            type: 'error',
            payload: null,
            error: err instanceof Error ? err.message : String(err),
          };
        }
      }

      case 'agent.delete': {
        const deletePayload = (msg.payload ?? {}) as { id: string };
        if (!deletePayload.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'agent.delete requires id' };
        }
        try {
          // Soft-delete: retire in YAML + mark deleted in store
          this.fort.agentFactory.retire(deletePayload.id, 'Deleted via UI');
          this.fort.agentStore.softDelete(deletePayload.id);
          this.broadcast({
            id: 'agent.deleted.broadcast',
            type: 'agents.updated',
            payload: this.buildAgentList(),
          });
          return { id: msg.id, type: 'agent.delete.response', payload: { id: deletePayload.id } };
        } catch (err) {
          return {
            id: msg.id,
            type: 'error',
            payload: null,
            error: err instanceof Error ? err.message : String(err),
          };
        }
      }

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

      case 'threads.list':
        const listPayload = (msg.payload ?? {}) as { agentId?: string };
        return {
          id: msg.id,
          type: 'threads.list.response',
          payload: {
            threads: this.fort.threads.listThreads(
              listPayload.agentId ? { agentId: listPayload.agentId } : undefined,
            ),
          },
        };

      case 'thread.history':
        const histPayload = (msg.payload ?? {}) as { agentId?: string; threadId?: string; limit?: number };
        if (histPayload.threadId) {
          const histMsgs = this.fort.threads.getMessages(
            histPayload.threadId,
            histPayload.limit ? { limit: histPayload.limit } : undefined,
          );
          return {
            id: msg.id,
            type: 'thread.history.response',
            payload: { threadId: histPayload.threadId, messages: histMsgs },
          };
        }
        if (histPayload.agentId) {
          const agentThreads = this.fort.threads.listThreads({ agentId: histPayload.agentId, status: 'active' });
          if (agentThreads.length > 0) {
            const tid = agentThreads[0].id;
            const histMsgs = this.fort.threads.getMessages(
              tid,
              histPayload.limit ? { limit: histPayload.limit } : { limit: 200 },
            );
            return {
              id: msg.id,
              type: 'thread.history.response',
              payload: { threadId: tid, agentId: histPayload.agentId, messages: histMsgs },
            };
          }
          return {
            id: msg.id,
            type: 'thread.history.response',
            payload: { threadId: null, agentId: histPayload.agentId, messages: [] },
          };
        }
        return { id: msg.id, type: 'error', payload: null, error: 'thread.history requires agentId or threadId' };

      case 'thread.create':
        const createPayload = msg.payload as { name: string; agentId?: string; description?: string };
        if (!createPayload?.name) {
          return { id: msg.id, type: 'error', payload: null, error: 'thread.create requires name' };
        }
        const newThread = this.fort.threads.createThread({
          name: createPayload.name,
          assignedAgent: createPayload.agentId,
          description: createPayload.description,
        });
        return { id: msg.id, type: 'thread.create.response', payload: { thread: newThread } };

      case 'notifications.list': {
        const nlPayload = (msg.payload ?? {}) as { unreadOnly?: boolean; limit?: number };
        return {
          id: msg.id,
          type: 'notifications.list.response',
          payload: this.fort.notifications.notificationStore.list({
            unreadOnly: nlPayload.unreadOnly,
            limit: nlPayload.limit ?? 50,
          }),
        };
      }

      case 'notifications.unread_count':
        return {
          id: msg.id,
          type: 'notifications.unread_count.response',
          payload: this.fort.notifications.notificationStore.getUnreadCount(),
        };

      case 'notification.mark_read': {
        const mrPayload = (msg.payload ?? {}) as { id: string };
        if (!mrPayload.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'notification.mark_read requires id' };
        }
        this.fort.notifications.notificationStore.markRead(mrPayload.id);
        return { id: msg.id, type: 'notification.mark_read.response', payload: { id: mrPayload.id } };
      }

      case 'notifications.mark_all_read':
        this.fort.notifications.notificationStore.markAllRead();
        return { id: msg.id, type: 'notifications.mark_all_read.response', payload: null };

      case 'doctor':
        const results = await this.fort.runDoctor();
        return { id: msg.id, type: 'doctor.response', payload: results };

      // ─── Scheduler (SPEC-008) ──────────────────────────────────────────

      case 'schedules.list':
        return {
          id: msg.id,
          type: 'schedules.list.response',
          payload: this.fort.scheduler.listSchedules(),
        };

      case 'schedule.create': {
        const createSched = (msg.payload ?? {}) as {
          name?: string;
          description?: string;
          agentId?: string;
          scheduleType?: 'cron' | 'interval';
          scheduleValue?: string;
          taskTitle?: string;
          taskDescription?: string;
        };
        if (!createSched.name?.trim()) {
          return { id: msg.id, type: 'error', payload: null, error: 'schedule.create requires name' };
        }
        if (!createSched.agentId) {
          return { id: msg.id, type: 'error', payload: null, error: 'schedule.create requires agentId' };
        }
        if (!createSched.scheduleType) {
          return { id: msg.id, type: 'error', payload: null, error: 'schedule.create requires scheduleType' };
        }
        if (!createSched.scheduleValue) {
          return { id: msg.id, type: 'error', payload: null, error: 'schedule.create requires scheduleValue' };
        }
        if (!createSched.taskTitle?.trim()) {
          return { id: msg.id, type: 'error', payload: null, error: 'schedule.create requires taskTitle' };
        }
        try {
          const schedule = this.fort.scheduler.createSchedule({
            name: createSched.name.trim(),
            description: createSched.description,
            agentId: createSched.agentId,
            scheduleType: createSched.scheduleType,
            scheduleValue: createSched.scheduleValue,
            taskTitle: createSched.taskTitle.trim(),
            taskDescription: createSched.taskDescription,
          });
          return { id: msg.id, type: 'schedule.create.response', payload: schedule };
        } catch (err) {
          return { id: msg.id, type: 'error', payload: null, error: err instanceof Error ? err.message : String(err) };
        }
      }

      case 'schedule.update': {
        const updateSched = (msg.payload ?? {}) as {
          id?: string;
          name?: string;
          description?: string;
          agentId?: string;
          scheduleType?: 'cron' | 'interval';
          scheduleValue?: string;
          taskTitle?: string;
          taskDescription?: string;
        };
        if (!updateSched.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'schedule.update requires id' };
        }
        try {
          const updated = this.fort.scheduler.updateSchedule(updateSched.id, {
            name: updateSched.name,
            description: updateSched.description,
            agentId: updateSched.agentId,
            scheduleType: updateSched.scheduleType,
            scheduleValue: updateSched.scheduleValue,
            taskTitle: updateSched.taskTitle,
            taskDescription: updateSched.taskDescription,
          });
          return { id: msg.id, type: 'schedule.update.response', payload: updated };
        } catch (err) {
          return { id: msg.id, type: 'error', payload: null, error: err instanceof Error ? err.message : String(err) };
        }
      }

      case 'schedule.delete': {
        const deleteSched = (msg.payload ?? {}) as { id?: string };
        if (!deleteSched.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'schedule.delete requires id' };
        }
        try {
          this.fort.scheduler.deleteSchedule(deleteSched.id);
          return { id: msg.id, type: 'schedule.delete.response', payload: { id: deleteSched.id } };
        } catch (err) {
          return { id: msg.id, type: 'error', payload: null, error: err instanceof Error ? err.message : String(err) };
        }
      }

      case 'schedule.pause': {
        const pauseSched = (msg.payload ?? {}) as { id?: string };
        if (!pauseSched.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'schedule.pause requires id' };
        }
        try {
          this.fort.scheduler.pauseSchedule(pauseSched.id);
          return { id: msg.id, type: 'schedule.pause.response', payload: { id: pauseSched.id, enabled: false } };
        } catch (err) {
          return { id: msg.id, type: 'error', payload: null, error: err instanceof Error ? err.message : String(err) };
        }
      }

      case 'schedule.resume': {
        const resumeSched = (msg.payload ?? {}) as { id?: string };
        if (!resumeSched.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'schedule.resume requires id' };
        }
        try {
          this.fort.scheduler.resumeSchedule(resumeSched.id);
          return { id: msg.id, type: 'schedule.resume.response', payload: { id: resumeSched.id, enabled: true } };
        } catch (err) {
          return { id: msg.id, type: 'error', payload: null, error: err instanceof Error ? err.message : String(err) };
        }
      }

      case 'schedule.run_now': {
        const runNowSched = (msg.payload ?? {}) as { id?: string };
        if (!runNowSched.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'schedule.run_now requires id' };
        }
        try {
          this.fort.scheduler.runNow(runNowSched.id);
          return { id: msg.id, type: 'schedule.run_now.response', payload: { id: runNowSched.id } };
        } catch (err) {
          return { id: msg.id, type: 'error', payload: null, error: err instanceof Error ? err.message : String(err) };
        }
      }

// ─── Agent Memory (SPEC-012) ──────────────────────────────────────

      case 'agent.memories': {
        const memPayload = (msg.payload ?? {}) as { agentId?: string; category?: string };
        if (!memPayload.agentId) {
          return { id: msg.id, type: 'error', payload: null, error: 'agent.memories requires agentId' };
        }
        const memories = this.fort.agentMemory.list(memPayload.agentId, {
          category: memPayload.category as any,
        });
        return { id: msg.id, type: 'agent.memories.response', payload: memories };
      }

      case 'agent.memory.delete': {
        const delMemPayload = (msg.payload ?? {}) as { id?: string };
        if (!delMemPayload.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'agent.memory.delete requires id' };
        }
        this.fort.agentMemory.delete(delMemPayload.id);
        return { id: msg.id, type: 'agent.memory.delete.response', payload: { id: delMemPayload.id } };
      }

      case 'agent.memory.clear': {
        const clearMemPayload = (msg.payload ?? {}) as { agentId?: string };
        if (!clearMemPayload.agentId) {
          return { id: msg.id, type: 'error', payload: null, error: 'agent.memory.clear requires agentId' };
        }
        this.fort.agentMemory.clear(clearMemPayload.agentId);
        return { id: msg.id, type: 'agent.memory.clear.response', payload: { agentId: clearMemPayload.agentId } };
      }

      // ─── Approvals (SPEC-011) ─────────────────────────────────────────

      case 'approvals.list': {
        const pending = this.fort.approvalStore.getPending();
        return { id: msg.id, type: 'approvals.list.response', payload: { pending } };
      }

      case 'approvals.for_task': {
        const forTaskPayload = (msg.payload ?? {}) as { taskId?: string };
        if (!forTaskPayload.taskId) {
          return { id: msg.id, type: 'error', payload: null, error: 'approvals.for_task requires taskId' };
        }
        const approvals = this.fort.approvalStore.getForTask(forTaskPayload.taskId);
        return { id: msg.id, type: 'approvals.for_task.response', payload: { approvals } };
      }

      case 'approval.respond': {
        const respondPayload = (msg.payload ?? {}) as {
          id?: string;
          approved?: boolean;
          rejectionReason?: string;
        };
        if (!respondPayload.id) {
          return { id: msg.id, type: 'error', payload: null, error: 'approval.respond requires id' };
        }
        if (typeof respondPayload.approved !== 'boolean') {
          return { id: msg.id, type: 'error', payload: null, error: 'approval.respond requires approved (boolean)' };
        }
        try {
          const updated = this.fort.approvalStore.resolve(
            respondPayload.id,
            respondPayload.approved,
            respondPayload.rejectionReason,
          );
          this.fort.toolExecutor.resolveApproval(
            respondPayload.id,
            respondPayload.approved,
            respondPayload.rejectionReason,
          );
          return { id: msg.id, type: 'approval.respond.response', payload: updated };
        } catch (err) {
          return {
            id: msg.id,
            type: 'error',
            payload: null,
            error: err instanceof Error ? err.message : String(err),
          };
        }
      }

      // ─── Diagnostics (SPEC-013) ─────────────────────────────────────────

      case 'diagnostics.health': {
        const results = await this.fort.doctor.runAll();
        const { FortDoctor } = await import('../diagnostics/index.js');
        const summary = FortDoctor.summarize(results);
        const checks: Record<string, unknown> = {};
        for (const r of results) {
          const detail: Record<string, unknown> = { status: r.status };
          detail['message'] = r.checks.map((c) => c.message).join('; ');
          const allDetails: Record<string, unknown> = {};
          for (const c of r.checks) {
            if (c.details) { Object.assign(allDetails, c.details); }
          }
          if (Object.keys(allDetails).length > 0) { detail['details'] = allDetails; }
          checks[r.module] = detail;
        }
        return {
          id: msg.id,
          type: 'diagnostics.health.response',
          payload: {
            status: summary.overall,
            timestamp: new Date().toISOString(),
            uptime: Math.round(process.uptime()),
            checks,
          },
        };
      }

      case 'diagnostics.errors': {
        const errLimit = ((msg.payload ?? {}) as { limit?: number }).limit ?? 50;
        return {
          id: msg.id,
          type: 'diagnostics.errors.response',
          payload: this.fort.errorLog.getRecent(errLimit),
        };
      }

      case 'diagnostics.metrics': {
        const { heapUsed } = process.memoryUsage();
        const heapMb = Math.round(heapUsed / 1024 / 1024 * 10) / 10;

        // Task counts: today's completed + failed + total active
        const today = new Date();
        today.setHours(0, 0, 0, 0);
        const todayTasks = this.fort.taskGraph.queryTasksFromStore({
          since: today,
          limit: 1000,
        });
        const activeTasks = this.fort.taskGraph.getActiveTasks().length;

        // DB size
        let dbSizeMb = 0;
        try {
          const { statSync } = await import('node:fs');
          const stat = statSync((this.fort as any).taskDbPath as string);
          dbSizeMb = Math.round(stat.size / 1024 / 1024 * 100) / 100;
        } catch { /* non-fatal */ }

        return {
          id: msg.id,
          type: 'diagnostics.metrics.response',
          payload: {
            uptime: Math.round(process.uptime()),
            heapMb,
            dbSizeMb,
            tasksToday: todayTasks.length,
            activeTasks,
            errorsToday: this.fort.errorLog.getRecent(1000).filter(
              (e) => e.createdAt >= today
            ).length,
          },
        };
      }

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

  private handleHealthCheck(res: ServerResponse): void {
    const startedAt = (this.fort as any)._startedAt as number | undefined;
    const uptime = startedAt ? Math.round((Date.now() - startedAt) / 1000) : process.uptime();

    this.fort.doctor.runAll().then((results) => {
      const summary = (this.fort.doctor.constructor as typeof import('../diagnostics/index.js').FortDoctor).summarize(results);

      // Build per-check map keyed by module name
      const checks: Record<string, unknown> = {};
      for (const r of results) {
        const detail: Record<string, unknown> = { status: r.status };
        if (r.checks.length > 0) {
          detail['message'] = r.checks.map((c) => c.message).join('; ');
          const allDetails: Record<string, unknown> = {};
          for (const c of r.checks) {
            if (c.details) Object.assign(allDetails, c.details);
          }
          if (Object.keys(allDetails).length > 0) detail['details'] = allDetails;
        }
        checks[r.module] = detail;
      }

      const body = JSON.stringify({
        status: summary.overall,
        timestamp: new Date().toISOString(),
        uptime,
        checks,
      });

      const statusCode = summary.overall === 'unhealthy' ? 503 : 200;
      res.writeHead(statusCode, { 'Content-Type': 'application/json' });
      res.end(body);
    }).catch((err) => {
      res.writeHead(503, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ status: 'unhealthy', error: String(err) }));
    });
  }

  private broadcast(msg: WSResponse): void {
    const data = JSON.stringify(msg);
    for (const client of this.clients) {
      if (client.readyState === WebSocket.OPEN) {
        client.send(data);
      }
    }
  }

  private handleOpenClawPreview(res: ServerResponse): void {
    import('../import/openclaw.js').then(({ scanOpenClaw }) => {
      const preview = scanOpenClaw(this.fort);
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(preview));
    }).catch((err) => {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: String(err) }));
    });
  }

  private handleOpenClawImport(res: ServerResponse): void {
    import('../import/openclaw.js').then(async ({ importOpenClaw }) => {
      const result = await importOpenClaw(this.fort);
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(result));
    }).catch((err) => {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: String(err) }));
    });
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

  /**
   * Find the active thread for an agent, or create one if none exists.
   */
  private getOrCreateAgentThread(agentId: string): string {
    const existing = this.fort.threads.listThreads({ agentId, status: 'active' });
    if (existing.length > 0) return existing[0].id;

    const thread = this.fort.threads.createThread({
      name: 'Chat',
      assignedAgent: agentId,
    });
    return thread.id;
  }

  /**
   * Build the canonical agent list by combining AgentStore records with runtime
   * status from AgentRegistry and soul content from AgentFactory.
   */
  private buildAgentList(): unknown[] {
    // Start from the live registry — these are agents currently running/stopped
    const registryInfos = this.fort.agents.listInfo();
    const result: unknown[] = registryInfos.map((info) => ({
      ...info,
      soul: this.fort.agentStore.get(info.config.id)?.soul
        ?? this.fort.agentFactory.getSoul(info.config.id)
        ?? undefined,
      emoji: this.getAgentEmoji(info.config.id),
    }));
    return result;
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
