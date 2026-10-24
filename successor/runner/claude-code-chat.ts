import { spawn } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { dirname, isAbsolute, normalize } from 'node:path';
import type { Readable, Writable } from 'node:stream';
import type { AccountIdentity, ModelCatalog, ModelOption, RuntimeDescriptor, RuntimeReadiness } from '../contracts/v1.ts';
import type { ExecutionDispatch } from '../contracts/executor.ts';
import { executionDigest } from '../contracts/executor.ts';
import { parseAccountIdentity, sameAccount } from '../contracts/account.ts';
import type { ChatInput, ChatResult, ChatRuntime } from './chat-runtime.ts';

// Claude subscription usage through the official, unmodified Claude Code CLI (Spec 062).
// Fort never reads, copies or forwards the CLI's credential and never passes an API key.
export function claudeCodeRuntime(runtimeVersion: string): RuntimeDescriptor {
  return { provider: 'claude-code', name: 'Claude', runtimeVersion, hostId: 'local-claude-code', session: 'stateless', capabilities: ['plainText', 'modelCatalog', 'cancel', 'skills', 'memory'] };
}

export type ClaudeCodeLaunch = { executable: string; args: string[]; cwd: string; env: Record<string, string> };
export type ClaudeCodeProcess = { stdin: Writable; stdout: Readable; kill(): void };
type LaunchOptions = { executable: string; workspace: string; home: string; runtimeDirectory: string; model?: string };
type ChatOptions = Omit<LaunchOptions, 'model'> & {
  runtimeVersion: string;
  launch?: (plan: ClaudeCodeLaunch) => ClaudeCodeProcess;
  // Replaces `claude auth status --json` in tests.
  authStatus?: (signal: AbortSignal) => Promise<unknown>;
};

const MAX_TEXT = 65_536, MAX_FRAME = 262_144, MAX_WIRE = 1_000_000, CONTROL_MS = 10_000, INTERRUPT_MS = 2_000;
const SIGN_IN = 'Run `claude auth login` in Terminal to sign in with your Claude subscription.';
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const MODEL_ID = /^[A-Za-z0-9_.:\[\]-]{1,128}$/;
const exactPath = (path: string) => isAbsolute(path) && path !== '/' && normalize(path) === path && path.isWellFormed() && !path.includes('\0');
const record = (value: unknown): Record<string, unknown> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Unexpected Claude Code output');
  return value as Record<string, unknown>;
};
const empty = (value: unknown) => Array.isArray(value) && value.length === 0;

// Every switch the CLI offers for ambient behaviour, set explicitly. `--bare` is never used:
// it ignores the subscription sign-in and bills an API key instead.
export function claudeCodeLaunch(options: LaunchOptions): ClaudeCodeLaunch {
  const { executable, workspace, home, runtimeDirectory, model } = options;
  if (![executable, workspace, home, runtimeDirectory].every(exactPath) || (model !== undefined && !MODEL_ID.test(model))) throw new Error('Use exact absolute paths and an exact model ID');
  // Claude Code puts its local messaging socket under XDG_RUNTIME_DIR only when the socket path fits in
  // 103 bytes; otherwise it falls back to a shared /tmp directory. Keep the private directory short enough.
  if (Buffer.byteLength(runtimeDirectory) > 72) throw new Error('Use a short private runtime directory');
  const args = ['-p', '--input-format', 'stream-json', '--output-format', 'stream-json', '--verbose', '--include-partial-messages',
    '--tools', '', '--strict-mcp-config', '--setting-sources', '', '--disable-slash-commands', '--no-session-persistence', '--permission-mode', 'default',
    ...(model ? ['--model', model] : [])];
  // Built from nothing, so no ANTHROPIC_*, OAuth token or cloud-provider variable can leak in.
  const env: Record<string, string> = { HOME: home, PATH: dirname(executable) + ':/usr/bin:/bin', LANG: 'en_US.UTF-8', TMPDIR: runtimeDirectory, XDG_RUNTIME_DIR: runtimeDirectory,
    CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: '1', DISABLE_TELEMETRY: '1', DISABLE_ERROR_REPORTING: '1', DISABLE_AUTOUPDATER: '1',
    CLAUDE_CODE_DISABLE_AUTO_MEMORY: '1', CLAUDE_CODE_DISABLE_CLAUDE_MDS: '1', CLAUDE_CODE_DISABLE_BUNDLED_SKILLS: '1', CLAUDE_CODE_DISABLE_BACKGROUND_TASKS: '1',
    CLAUDE_CODE_DISABLE_MODEL_ACCESS_FALLBACK: '1', CLAUDE_CODE_DISABLE_REFUSAL_FALLBACK: '1', CLAUDE_CODE_DISABLE_FAST_MODE: '1', CLAUDE_CODE_DISABLE_EXPLORE_PLAN_AGENTS: '1' };
  if (process.env.USER && /^[A-Za-z0-9_.-]{1,64}$/.test(process.env.USER)) env.USER = process.env.USER;
  return { executable, args, cwd: workspace, env };
}

// Accepts only a Claude subscription sign-in. Console, API-key and cloud-provider sign-ins are refused, not converted.
export function parseClaudeAuthStatus(value: unknown): AccountIdentity | null {
  const row = record(value);
  if (row.loggedIn === false && row.authMethod === 'none' && row.apiKeySource === undefined) return null;
  if (row.loggedIn !== true || row.authMethod !== 'claude.ai' || row.apiProvider !== 'firstParty' || row.apiKeySource !== undefined
    || typeof row.orgId !== 'string' || !UUID.test(row.orgId) || typeof row.email !== 'string' || !/^[^\s@]+@[^\s@]+$/.test(row.email)) throw new Error('Unsupported Claude sign-in');
  return parseAccountIdentity({ accountId: row.orgId, userLabel: row.email });
}

function catalog(value: unknown): ModelOption[] {
  if (!Array.isArray(value) || value.length > 64) throw new Error('Invalid model catalog');
  const models = new Map<string, ModelOption>();
  for (const entry of value) {
    const row = record(entry);
    if (typeof row.resolvedModel !== 'string' || !MODEL_ID.test(row.resolvedModel) || typeof row.value !== 'string') throw new Error('Invalid model catalog');
    if (row.value === 'default' || models.has(row.resolvedModel)) continue;
    const name = typeof row.displayName === 'string' && row.displayName.length > 0 && row.displayName.length <= 128 ? row.displayName : row.resolvedModel;
    models.set(row.resolvedModel, { id: row.resolvedModel, name, availability: 'available' });
  }
  return [...models.values()];
}

// The initialize response must show this exact runtime, a subscription token, no API key and nothing ambient enabled.
// Returns the names of failed checks only, never their values, so a report can say what differed.
export function initializeProblems(value: unknown, runtimeVersion: string, account: AccountIdentity): string[] {
  let row: Record<string, unknown>, signedIn: Record<string, unknown>;
  try { row = record(value); signedIn = record(row.account); } catch { return ['shape']; }
  const checks: [string, boolean][] = [
    ['claude_code_version', row.claude_code_version === runtimeVersion], // Signed in with a claude.ai subscription, 2.1.299 sends no tokenSource; other sources name themselves.
    ['account.tokenSource', signedIn.tokenSource === undefined || signedIn.tokenSource === 'claude.ai'],
    ['account.subscriptionType', typeof signedIn.subscriptionType === 'string' && signedIn.subscriptionType.length > 0],
    ['account.apiProvider', signedIn.apiProvider === 'firstParty'], ['account.apiKeySource', signedIn.apiKeySource === undefined],
    ['account.email', signedIn.email === account.userLabel], ['commands', empty(row.commands)], ['current_permission_mode', row.current_permission_mode === 'default'],
    ['analytics_disabled', row.analytics_disabled === true], ['remote_control_auto_enable', row.remote_control_auto_enable === false],
    ['remote_control_auto_connect_default', row.remote_control_auto_connect_default === false], ['remote_control_auto_on_by_default', row.remote_control_auto_on_by_default === false],
  ];
  const problems = checks.filter(([, ok]) => !ok).map(([name]) => name);
  try { catalog(row.models); } catch { problems.push('models'); }
  return problems;
}
function checkInitialize(value: unknown, runtimeVersion: string, account: AccountIdentity): ModelOption[] {
  if (initializeProblems(value, runtimeVersion, account).length) throw new Error('Claude Code controls were not confirmed');
  return catalog(record(value).models);
}

// Runs `claude auth status --json` with the same isolated environment as a session. It exits 1 when signed out.
export function readAuthStatus(options: Omit<LaunchOptions, 'model'>, signal: AbortSignal): Promise<unknown> {
  const plan = claudeCodeLaunch(options);
  return new Promise((resolve, reject) => {
    const child = spawn(plan.executable, ['auth', 'status', '--json'], { cwd: plan.cwd, env: plan.env, stdio: ['ignore', 'pipe', 'ignore'], signal, timeout: CONTROL_MS });
    const chunks: Buffer[] = [];
    let bytes = 0;
    child.stdout.on('data', (chunk: Buffer) => { bytes += chunk.byteLength; if (bytes > 16_384) child.kill(); else chunks.push(chunk); });
    child.on('error', reject);
    child.on('close', code => {
      if (bytes > 16_384 || (code !== 0 && code !== 1)) { reject(new Error('Claude Code account status was not confirmed')); return; }
      try { resolve(JSON.parse(Buffer.concat(chunks).toString('utf8'))); } catch (error) { reject(error); }
    });
  });
}

type Outcome = ChatResult | undefined;

// One stream-json session: Fort's control requests, then turn evidence until a confirmed result.
class Session {
  private readonly process: ClaudeCodeProcess;
  private readonly decoder = new TextDecoder('utf-8', { fatal: true });
  private readonly pending = new Map<string, (response: Record<string, unknown> | undefined) => void>();
  private buffer = '';
  private wire = 0;
  private closed = false;
  onFrame: (frame: Record<string, unknown>) => void = () => { throw new Error('Unexpected Claude Code output'); };
  onClose: () => void = () => {};
  constructor(process: ClaudeCodeProcess) {
    this.process = process;
    process.stdout.on('data', (bytes: Buffer) => this.receive(bytes));
    for (const event of ['end', 'close', 'error']) process.stdout.on(event, () => this.close());
    process.stdin.on('error', () => this.close());
  }
  write(frame: unknown) { if (!this.closed) this.process.stdin.write(JSON.stringify(frame) + '\n'); }
  control(request: Record<string, unknown>, signal: AbortSignal): Promise<Record<string, unknown>> {
    const id = randomUUID();
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => settle(undefined), CONTROL_MS), abort = () => settle(undefined);
      const settle = (response: Record<string, unknown> | undefined) => {
        clearTimeout(timer); signal.removeEventListener('abort', abort); this.pending.delete(id);
        if (response) resolve(response); else reject(new Error('Claude Code control request was not confirmed'));
      };
      if (this.closed || signal.aborted) { settle(undefined); return; }
      signal.addEventListener('abort', abort, { once: true });
      this.pending.set(id, settle);
      this.write({ type: 'control_request', request_id: id, request });
    });
  }
  close() {
    if (this.closed) return;
    this.closed = true;
    for (const settle of [...this.pending.values()]) settle(undefined);
    try { this.process.stdin.end(); } catch {}
    this.process.kill();
    this.onClose();
  }
  private receive(bytes: Uint8Array) {
    if (this.closed) return;
    try {
      this.wire += bytes.byteLength;
      if (this.wire > MAX_WIRE) throw new Error('Output limit');
      this.buffer += this.decoder.decode(bytes, { stream: true });
      for (let end = this.buffer.indexOf('\n'); end >= 0 && !this.closed; end = this.buffer.indexOf('\n')) {
        const line = this.buffer.slice(0, end); this.buffer = this.buffer.slice(end + 1);
        if (Buffer.byteLength(line) > MAX_FRAME) throw new Error('Frame limit');
        if (!line.trim()) continue;
        const frame = record(JSON.parse(line));
        if (frame.type === 'control_response') {
          const response = record(frame.response), settle = typeof response.request_id === 'string' ? this.pending.get(response.request_id) : undefined;
          settle?.(response.subtype === 'success' ? record(response.response ?? {}) : undefined);
        } else this.onFrame(frame);
      }
      if (Buffer.byteLength(this.buffer) > MAX_FRAME) throw new Error('Frame limit');
    } catch { this.close(); }
  }
}

// Turn evidence. Only the final `result`, matching assistant text from the selected model, counts as a reply.
class Turn {
  private readonly model: string;
  private readonly workspace: string;
  private readonly runtimeDirectory: string;
  private started = false;
  private interrupted = false;
  private failed = false;
  private texts: string[] = [];
  private bytes = 0;
  outcome: Outcome;
  constructor(model: string, workspace: string, runtimeDirectory: string) { this.model = model; this.workspace = workspace; this.runtimeDirectory = runtimeDirectory; }
  interrupt() { this.interrupted = true; }
  private unknown(reason: string) { this.outcome ??= { kind: 'outcomeUnknown', message: `Claude stopped without a confirmed reply (${reason}).` }; }
  receive(frame: Record<string, unknown>) {
    if (this.outcome) return;
    if (frame.type === 'control_request') { this.unknown('unsupported request'); return; }
    if (frame.type === 'system' && frame.subtype === 'init') {
      const socket = frame.messaging_socket_path;
      if (this.started || frame.model !== this.model || frame.cwd !== this.workspace || frame.permissionMode !== 'default' || frame.apiKeySource !== 'none'
        || !empty(frame.tools) || !empty(frame.mcp_servers) || !empty(frame.slash_commands) || !empty(frame.skills)
        || (socket !== undefined && (typeof socket !== 'string' || !normalize(socket).startsWith(this.runtimeDirectory + '/')))) { this.unknown('session controls changed'); return; }
      this.started = true; return;
    }
    if (!this.started) { this.unknown('protocol'); return; }
    if (frame.type === 'system') { if (/fallback/.test(String(frame.subtype))) this.unknown('model changed'); return; }
    if (frame.type === 'stream_event') return;
    if (frame.type === 'assistant') {
      const message = record(frame.message);
      if (frame.parent_tool_use_id !== null || !Array.isArray(message.content)) { this.unknown('protocol'); return; }
      if (frame.error !== undefined) { this.failed = true; return; }
      if (message.model !== this.model) { this.unknown('model changed'); return; }
      for (const block of message.content) {
        const row = record(block);
        if (row.type === 'thinking' || row.type === 'redacted_thinking') continue;
        if (row.type !== 'text' || typeof row.text !== 'string' || !row.text.isWellFormed()) { this.unknown('unsupported action'); return; }
        this.bytes += Buffer.byteLength(row.text);
        if (this.bytes > MAX_TEXT) { this.unknown('output limit'); return; }
        this.texts.push(row.text);
      }
      return;
    }
    if (frame.type === 'result') {
      const denials = frame.permission_denials;
      if (this.interrupted) { this.outcome = { kind: 'canceled', message: 'Claude confirmed the interruption. Earlier effects, if any, remain.' }; return; }
      if (frame.is_error === true || this.failed) { this.outcome = { kind: 'failed', message: 'Claude did not complete the request. Check your Claude sign-in and plan limits.' }; return; }
      const text = this.texts.at(-1);
      if (frame.subtype !== 'success' || frame.is_error !== false || !empty(denials) || text === undefined || !text.trim() || frame.result !== text) { this.unknown('result not confirmed'); return; }
      this.outcome = { kind: 'completed', text, effectiveModel: null };
      return;
    }
    // User frames would carry tool results; with no tools there must be none.
    this.unknown('unsupported action');
  }
}

export class ClaudeCodeChat implements ChatRuntime {
  private readonly options: ChatOptions;
  private readonly launch: (plan: ClaudeCodeLaunch) => ClaudeCodeProcess;
  private busy = false;
  private disposed = false;
  constructor(options: ChatOptions) {
    claudeCodeLaunch(options);
    if (!/^\d+\.\d+\.\d+$/.test(options.runtimeVersion)) throw new Error('Pin an exact Claude Code version');
    this.options = options;
    this.launch = options.launch ?? (plan => {
      const child = spawn(plan.executable, plan.args, { cwd: plan.cwd, env: plan.env, stdio: ['pipe', 'pipe', 'ignore'] });
      child.on('error', () => child.stdout.destroy());
      return { stdin: child.stdin, stdout: child.stdout, kill: () => { child.kill('SIGTERM'); } };
    });
  }

  readiness(): RuntimeReadiness {
    return { state: this.disposed ? 'stopped' : this.busy ? 'busy' : 'ready', canRestart: false, detail: this.disposed ? 'Runtime stopped.' : 'Ready for the exact selected model.' };
  }
  async dispose(): Promise<void> { this.disposed = true; }

  private async account(signal: AbortSignal): Promise<AccountIdentity | null> {
    return parseClaudeAuthStatus(await (this.options.authStatus ?? (signal => readAuthStatus(this.options, signal)))(signal));
  }

  private open(model?: string): Session {
    const { executable, workspace, home, runtimeDirectory } = this.options;
    return new Session(this.launch(claudeCodeLaunch({ executable, workspace, home, runtimeDirectory, ...(model ? { model } : {}) })));
  }

  // Raw initialize response for the manual check's report. Not used on the request path.
  async probe(signal: AbortSignal): Promise<unknown> {
    const session = this.open();
    try { return await session.control({ subtype: 'initialize' }, signal); } finally { session.close(); }
  }

  async models(signal: AbortSignal, selected?: AccountIdentity): Promise<ModelCatalog> {
    const provider = 'claude-code';
    if (this.disposed || this.busy) return { provider, state: 'unavailable', models: [], detail: 'Model discovery is unavailable while the runtime is stopped or busy.' };
    this.busy = true;
    let session: Session | undefined;
    try {
      const account = await this.account(signal);
      if (!account) return { provider, state: 'unavailable', models: [], detail: SIGN_IN };
      if (selected && !sameAccount(parseAccountIdentity(selected), account)) throw new Error('Selected account unavailable');
      session = this.open();
      const models = checkInitialize(await session.control({ subtype: 'initialize' }, signal), this.options.runtimeVersion, account);
      return { provider, state: 'ready', models, checkedAt: new Date().toISOString() };
    } catch { return { provider, state: 'error', models: [], detail: 'Could not verify models from the signed-in Claude Code runtime. Your selection has not changed.' }; }
    finally { session?.close(); this.busy = false; }
  }

  async execute(input: ChatInput, signal: AbortSignal, dispatch: ExecutionDispatch): Promise<ChatResult> {
    const attempt = input.attempt, configuration = input.configuration;
    if (!attempt || attempt.runId !== input.runId || attempt.bindingRevisionId !== configuration.bindingRevisionId || attempt.selectedModel !== configuration.selectedModel
      || attempt.runtimeVersion !== this.options.runtimeVersion || !Number.isSafeInteger(attempt.generation) || attempt.generation < 1 || !MODEL_ID.test(configuration.selectedModel)) return { kind: 'notStarted', message: 'Service attempt identity was not confirmed. Nothing was sent.' };
    if (this.disposed) return { kind: 'notStarted', message: 'Runtime stopped. Nothing was sent.' };
    if (this.busy) return { kind: 'failed', message: 'This runtime already has an active turn.' };
    const prepared = input.context;
    if (!prepared || prepared.kind === 'unavailable') return { kind: 'notStarted', message: prepared?.kind === 'unavailable' ? prepared.message : 'Prepared context is missing. Nothing was sent.' };
    if (prepared.kind === 'empty' && (configuration.skills.length || configuration.memory?.text)) return { kind: 'notStarted', message: 'Required context is missing. Nothing was sent.' };
    if (prepared.kind === 'prepared' && (prepared.configurationDigest !== executionDigest(configuration) || prepared.module.id !== configuration.contextModule?.id || prepared.module.version !== configuration.contextModule?.version)) return { kind: 'notStarted', message: 'Prepared context does not match the pinned configuration. Nothing was sent.' };
    const memory = prepared.kind === 'prepared' ? prepared.content.memory : undefined;
    const skills = prepared.kind === 'prepared' ? prepared.content : { instructions: '', bundles: [] };
    const systemPrompt = configuration.instructions + (skills.instructions ? '\n\n' + skills.instructions : '');
    const text = JSON.stringify({ conversationId: input.conversationId, conversationMemory: memory,
      history: input.history.map(message => ({ role: message.role, text: message.text, agentId: message.agentId, agentName: message.agentName, recipientAgentId: message.recipientAgentId, recipientAgentName: message.recipientAgentName })),
      currentMessage: { recipientAgentId: configuration.agentId, text: input.text }, selectedSkillBundles: skills.bundles });
    if (input.history.length > 256 || input.history.some(message => message.conversationId !== input.conversationId) || !text.isWellFormed() || !systemPrompt.isWellFormed()
      || Buffer.byteLength(text) + Buffer.byteLength(systemPrompt) > MAX_TEXT) return { kind: 'failed', message: 'Conversation context exceeds this runtime’s limit or identity. Nothing was sent.' };
    if (signal.aborted) return { kind: 'notStarted', message: 'Canceled before runtime dispatch.' };
    this.busy = true;
    let session: Session | undefined;
    try {
      try {
        const account = await this.account(signal);
        if (!account) return { kind: 'notStarted', message: 'Claude is signed out. ' + SIGN_IN + ' Nothing was sent.' };
        if (!sameAccount(configuration.account, account)) return { kind: 'notStarted', message: 'The selected Claude account is not the signed-in account. Nothing was sent.' };
        session = this.open(configuration.selectedModel);
        const models = checkInitialize(await session.control({ subtype: 'initialize', systemPrompt }, signal), this.options.runtimeVersion, account);
        if (!models.some(model => model.id === configuration.selectedModel)) throw new Error('Selected model unavailable');
      } catch { return { kind: 'notStarted', message: 'Claude Code preflight was not confirmed. Nothing was sent.' }; }
      if (signal.aborted) return { kind: 'notStarted', message: 'Canceled before runtime dispatch.' };
      return await this.turn(session, configuration.selectedModel, text, signal, dispatch.deadline);
    } finally { session?.close(); this.busy = false; }
  }

  private turn(session: Session, model: string, text: string, signal: AbortSignal, deadline: number): Promise<ChatResult> {
    const turn = new Turn(model, this.options.workspace, this.options.runtimeDirectory);
    return new Promise(resolve => {
      let interruptTimer: ReturnType<typeof setTimeout> | undefined;
      const finish = (outcome: ChatResult) => {
        clearTimeout(deadlineTimer); clearTimeout(interruptTimer); signal.removeEventListener('abort', interrupt);
        session.onFrame = () => {}; session.onClose = () => {}; resolve(outcome);
      };
      const interrupt = () => {
        if (interruptTimer) return;
        turn.interrupt();
        session.write({ type: 'control_request', request_id: randomUUID(), request: { subtype: 'interrupt' } });
        interruptTimer = setTimeout(() => finish({ kind: 'outcomeUnknown', message: 'Claude did not confirm the interruption in time.' }), INTERRUPT_MS);
      };
      const deadlineTimer = setTimeout(interrupt, Math.max(1, deadline - Date.now()));
      session.onFrame = frame => { turn.receive(frame); if (turn.outcome) finish(turn.outcome); };
      session.onClose = () => finish(turn.outcome ?? { kind: 'outcomeUnknown', message: 'Claude Code exited without a confirmed reply.' });
      signal.addEventListener('abort', interrupt, { once: true });
      session.write({ type: 'user', message: { role: 'user', content: [{ type: 'text', text }] }, parent_tool_use_id: null, session_id: '' });
    });
  }
}
