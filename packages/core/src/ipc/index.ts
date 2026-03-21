/**
 * IPC Server — WebSocket bridge between Fort core and native clients
 *
 * Connects the TypeScript core to the Swift menu bar app and the
 * Tauri dashboard over ws://localhost:4001/shell. Subscribes to
 * ModuleBus events and broadcasts relevant state changes to all
 * connected clients. Handles incoming action requests.
 */

import { WebSocketServer, WebSocket } from 'ws';
import { IncomingMessage } from 'node:http';
import type { Fort } from '../fort.js';
import type { DiagnosticResult, FortEvent } from '../types.js';

const DEFAULT_PORT = 4001;

interface IPCMessage {
  type: string;
  data: unknown;
}

interface IPCAction {
  action: string;
  payload?: Record<string, unknown>;
}

interface ClientInfo {
  id: number;
  connectedAt: Date;
  messagesSent: number;
  messagesReceived: number;
}

export class IPCServer {
  private fort: Fort;
  private port: number;
  private wss: WebSocketServer | null = null;
  private clients: Map<WebSocket, ClientInfo> = new Map();
  private unsubscribers: (() => void)[] = [];
  private startedAt: Date | null = null;
  private clientIdCounter = 0;
  private totalMessagesSent = 0;
  private totalMessagesReceived = 0;

  constructor(fort: Fort, port: number = DEFAULT_PORT) {
    this.fort = fort;
    this.port = port;
  }

  async start(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.wss = new WebSocketServer({
        port: this.port,
        path: '/shell',
      });

      this.wss.on('error', (err) => {
        reject(err);
      });

      this.wss.on('listening', () => {
        this.startedAt = new Date();
        this.subscribeToBus();
        resolve();
      });

      this.wss.on('connection', (ws: WebSocket, req: IncomingMessage) => {
        this.handleConnection(ws);
      });
    });
  }

  async stop(): Promise<void> {
    // Unsubscribe from bus events
    for (const unsub of this.unsubscribers) {
      unsub();
    }
    this.unsubscribers = [];

    // Close all client connections
    for (const [ws] of this.clients) {
      ws.close(1001, 'Server shutting down');
    }
    this.clients.clear();

    // Close the server
    return new Promise((resolve) => {
      if (this.wss) {
        this.wss.close(() => {
          this.wss = null;
          this.startedAt = null;
          resolve();
        });
      } else {
        resolve();
      }
    });
  }

  diagnose(): DiagnosticResult {
    const uptime = this.startedAt
      ? Date.now() - this.startedAt.getTime()
      : 0;

    const checks = [
      {
        name: 'Server status',
        passed: this.wss !== null,
        message: this.wss
          ? `Listening on port ${this.port}`
          : 'Not running',
      },
      {
        name: 'Connected clients',
        passed: true,
        message: `${this.clients.size} client(s) connected`,
        details: {
          count: this.clients.size,
          clients: Array.from(this.clients.values()).map((c) => ({
            id: c.id,
            connectedAt: c.connectedAt.toISOString(),
            messagesSent: c.messagesSent,
            messagesReceived: c.messagesReceived,
          })),
        },
      },
      {
        name: 'Uptime',
        passed: true,
        message: `${Math.floor(uptime / 1000)}s`,
        details: { uptimeMs: uptime },
      },
      {
        name: 'Message throughput',
        passed: true,
        message: `Sent: ${this.totalMessagesSent}, Received: ${this.totalMessagesReceived}`,
        details: {
          totalSent: this.totalMessagesSent,
          totalReceived: this.totalMessagesReceived,
        },
      },
    ];

    return {
      module: 'ipc',
      status: this.wss ? 'healthy' : 'unhealthy',
      checks,
    };
  }

  // ─── Private ────────────────────────────────────────────────────

  private handleConnection(ws: WebSocket): void {
    const clientInfo: ClientInfo = {
      id: ++this.clientIdCounter,
      connectedAt: new Date(),
      messagesSent: 0,
      messagesReceived: 0,
    };
    this.clients.set(ws, clientInfo);

    ws.on('message', (raw: Buffer | string) => {
      const text = typeof raw === 'string' ? raw : raw.toString('utf-8');
      clientInfo.messagesReceived++;
      this.totalMessagesReceived++;

      try {
        const msg: IPCAction = JSON.parse(text);
        this.handleAction(ws, msg);
      } catch {
        this.sendTo(ws, { type: 'error', data: { message: 'Invalid JSON' } });
      }
    });

    ws.on('close', () => {
      this.clients.delete(ws);
    });

    ws.on('error', () => {
      this.clients.delete(ws);
    });
  }

  private async handleAction(ws: WebSocket, msg: IPCAction): Promise<void> {
    switch (msg.action) {
      case 'get_status':
        this.sendTo(ws, this.buildStatusMessage());
        break;

      case 'get_tasks':
        this.sendTo(ws, this.buildTasksMessage());
        break;

      case 'get_agents':
        this.sendTo(ws, this.buildAgentsMessage());
        break;

      case 'run_doctor': {
        const results = await this.fort.runDoctor();
        this.sendTo(ws, { type: 'doctor', data: { results } });
        break;
      }

      case 'run_routine': {
        const routineId = msg.payload?.routineId as string | undefined;
        if (!routineId) {
          this.sendTo(ws, {
            type: 'error',
            data: { message: 'Missing routineId in payload' },
          });
          break;
        }
        try {
          const execution = await this.fort.routines.executeRoutine(routineId);
          this.sendTo(ws, {
            type: 'routine_result',
            data: { execution },
          });
        } catch (err) {
          this.sendTo(ws, {
            type: 'error',
            data: {
              message: err instanceof Error ? err.message : String(err),
            },
          });
        }
        break;
      }

      case 'spotlight_query': {
        const query = (msg.payload?.query as string) ?? '';
        const results = this.fort.osIntegration.handleSpotlightQuery(query);
        this.sendTo(ws, { type: 'spotlight_results', data: { results } });
        break;
      }

      case 'shortcut_action': {
        const intent = msg.payload?.intent as string;
        const params = (msg.payload?.params as Record<string, unknown>) ?? {};
        if (!intent) {
          this.sendTo(ws, {
            type: 'error',
            data: { message: 'Missing intent in payload' },
          });
          break;
        }
        try {
          const result = await this.fort.osIntegration.handleShortcutAction(intent, params);
          this.sendTo(ws, { type: 'shortcut_result', data: result });
        } catch (err) {
          this.sendTo(ws, {
            type: 'error',
            data: { message: err instanceof Error ? err.message : String(err) },
          });
        }
        break;
      }

      case 'file_action': {
        const filePaths = msg.payload?.filePaths as string[] | undefined;
        if (!filePaths || !Array.isArray(filePaths)) {
          this.sendTo(ws, {
            type: 'error',
            data: { message: 'Missing filePaths in payload' },
          });
          break;
        }
        const fileResult = this.fort.osIntegration.handleFileAction(filePaths);
        this.sendTo(ws, { type: 'file_action_result', data: fileResult });
        break;
      }

      case 'voice_input': {
        const transcript = msg.payload?.transcript as string;
        if (!transcript) {
          this.sendTo(ws, {
            type: 'error',
            data: { message: 'Missing transcript in payload' },
          });
          break;
        }
        try {
          const voiceResult = await this.fort.osIntegration.handleVoiceInput(transcript);
          this.sendTo(ws, { type: 'voice_result', data: voiceResult });
        } catch (err) {
          this.sendTo(ws, {
            type: 'error',
            data: { message: err instanceof Error ? err.message : String(err) },
          });
        }
        break;
      }

      case 'notification_policy': {
        const category = (msg.payload?.category as string) ?? 'general';
        const focusMode = (msg.payload?.focusMode as string | null) ?? null;
        const policy = this.fort.osIntegration.getNotificationPolicy(category, focusMode);
        this.sendTo(ws, { type: 'notification_policy', data: policy });
        break;
      }

      default:
        this.sendTo(ws, {
          type: 'error',
          data: { message: `Unknown action: ${msg.action}` },
        });
    }
  }

  private subscribeToBus(): void {
    // Task events → broadcast task counts
    const taskEvents = [
      'task.created',
      'task.updated',
      'task.completed',
      'task.failed',
    ];
    for (const eventType of taskEvents) {
      this.unsubscribers.push(
        this.fort.bus.subscribe(eventType, () => {
          this.broadcast(this.buildTasksMessage());
        }),
      );
    }

    // Agent events → broadcast status
    const agentEvents = [
      'agent.started',
      'agent.stopped',
      'agent.paused',
      'agent.error',
    ];
    for (const eventType of agentEvents) {
      this.unsubscribers.push(
        this.fort.bus.subscribe(eventType, () => {
          this.broadcast(this.buildStatusMessage());
        }),
      );
    }

    // Notification-worthy events
    this.unsubscribers.push(
      this.fort.bus.subscribe('task.completed', (event: FortEvent) => {
        this.broadcast({
          type: 'notification',
          data: {
            title: 'Task Completed',
            body: String((event.payload as Record<string, unknown>)?.title ?? 'A task has completed'),
            category: 'task',
          },
        });
      }),
    );

    this.unsubscribers.push(
      this.fort.bus.subscribe('task.failed', (event: FortEvent) => {
        this.broadcast({
          type: 'notification',
          data: {
            title: 'Task Failed',
            body: String((event.payload as Record<string, unknown>)?.title ?? 'A task has failed'),
            category: 'task',
          },
        });
      }),
    );

    this.unsubscribers.push(
      this.fort.bus.subscribe('agent.hatched', (event: FortEvent) => {
        this.broadcast({
          type: 'notification',
          data: {
            title: 'Agent Hatched',
            body: String((event.payload as Record<string, unknown>)?.name ?? 'A new agent has been created'),
            category: 'agent',
          },
        });
      }),
    );

    this.unsubscribers.push(
      this.fort.bus.subscribe('flow.completed', (event: FortEvent) => {
        this.broadcast({
          type: 'notification',
          data: {
            title: 'Flow Completed',
            body: String((event.payload as Record<string, unknown>)?.name ?? 'A flow has completed'),
            category: 'flow',
          },
        });
      }),
    );
  }

  private buildStatusMessage(): IPCMessage {
    const agents = this.fort.agents.listInfo();
    const errored = agents.filter((a) => a.status === 'error').length;
    const stopped = agents.filter((a) => a.status === 'stopped').length;

    let health: 'green' | 'yellow' | 'red';
    if (errored > 0) {
      health = 'red';
    } else if (stopped > 0) {
      health = 'yellow';
    } else {
      health = 'green';
    }

    return {
      type: 'status',
      data: {
        agents: agents.map((a) => ({
          id: a.config.id,
          name: a.config.name,
          status: a.status,
          taskCount: a.taskCount,
        })),
        health,
      },
    };
  }

  private buildTasksMessage(): IPCMessage {
    const allTasks = this.fort.taskGraph.getAllTasks();
    const active = allTasks.filter(
      (t) => t.status === 'in_progress' || t.status === 'created',
    ).length;
    const queued = allTasks.filter((t) => t.status === 'blocked').length;
    const completed = allTasks.filter(
      (t) => t.status === 'completed' || t.status === 'failed',
    ).length;

    return {
      type: 'tasks',
      data: {
        active,
        queued,
        history: completed,
      },
    };
  }

  private buildAgentsMessage(): IPCMessage {
    const agents = this.fort.agents.listInfo();
    return {
      type: 'agents',
      data: {
        agents: agents.map((a) => ({
          id: a.config.id,
          name: a.config.name,
          type: a.config.type,
          status: a.status,
          taskCount: a.taskCount,
          errorCount: a.errorCount,
          capabilities: a.config.capabilities,
        })),
      },
    };
  }

  private broadcast(message: IPCMessage): void {
    const json = JSON.stringify(message);
    for (const [ws, info] of this.clients) {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(json);
        info.messagesSent++;
        this.totalMessagesSent++;
      }
    }
  }

  private sendTo(ws: WebSocket, message: IPCMessage): void {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(message));
      const info = this.clients.get(ws);
      if (info) {
        info.messagesSent++;
        this.totalMessagesSent++;
      }
    }
  }
}
