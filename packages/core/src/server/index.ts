/**
 * Fort WebSocket Server
 *
 * Provides a WebSocket interface for the Swift menu bar app
 * and other clients to communicate with the Fort core.
 */

import { createServer, type Server } from 'node:http';
import { WebSocketServer, WebSocket } from 'ws';
import type { Fort } from '../fort.js';

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
      res.writeHead(404);
      res.end();
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
      this.broadcast({ id: event.id, type: 'task.status_changed', payload: event.payload });
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
        const taskId = await this.fort.chat(
          msg.payload as string,
          'user_chat'
        );
        return { id: msg.id, type: 'chat.response', payload: { taskId } };

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
          payload: this.fort.agents.listInfo(),
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

  async start(): Promise<void> {
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
