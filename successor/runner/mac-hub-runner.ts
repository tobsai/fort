import type { ChatRuntime } from './chat-runtime.ts';
import { deleteHubCredential, enrollWithHub, readHubCredential, runAsRunner } from './hub-credential.ts';
import type { RunnerLane, RunnerStatus } from './hub-runner.ts';

// The Mac side of the cloud hub (Spec 063, step 4). Beside its local core, the helper keeps one runner
// connection to the hub for every runtime it serves, once the owner has enrolled this Mac by pasting
// the hub's one-time "fort-runner:" invitation. Each lane runs under the Mac's runner ID, which is
// the hostId the hub pins those turns to.
export type MacHubStatus =
  | { state: 'notEnrolled' }
  | { state: RunnerStatus['state'] | 'failed'; hubURL: string; runnerId: string; retryMs?: number; detail?: string };
export type MacHubInvitation = { hub: string; invitationId: string; code: string; runnerId: string };
type Options = {
  stateDirectory: string; runnerName: string;
  // Builds this Mac's lanes for the runner ID it enrolled with; the first is the primary.
  lanes(runnerId: string): RunnerLane[];
  fetch?: (url: string, init?: RequestInit) => Promise<Response>;
};

const PREFIX = 'fort-runner:';

export function parseRunnerInvitation(text: string): MacHubInvitation {
  const value = text.trim();
  if (!value.startsWith(PREFIX) || value.length > 4096) throw new Error('Paste the whole fort-runner: invitation from the hub');
  let row: Record<string, unknown>;
  try { row = JSON.parse(Buffer.from(value.slice(PREFIX.length), 'base64url').toString('utf8')) as Record<string, unknown>; } catch { throw new Error('That invitation could not be read'); }
  if (!row || typeof row !== 'object' || row.v !== 1 || Object.keys(row).some(key => !['v', 'hub', 'invitationId', 'code', 'runnerId'].includes(key))
    || typeof row.hub !== 'string' || typeof row.invitationId !== 'string' || typeof row.code !== 'string' || typeof row.runnerId !== 'string') throw new Error('That invitation could not be read');
  return { hub: row.hub, invitationId: row.invitationId, code: row.code, runnerId: row.runnerId };
}

// Wraps a runtime the local core also uses, so a finished lease never disposes it.
export function sharedRuntime(runtime: ChatRuntime): () => ChatRuntime {
  return () => ({
    execute: (input, signal, dispatch) => runtime.execute(input, signal, dispatch),
    ...(runtime.models ? { models: runtime.models.bind(runtime) } : {}),
    ...(runtime.signIn ? { signIn: runtime.signIn.bind(runtime) } : {}),
    dispose: async () => {},
  });
}

export function macHubRunner(options: Options) {
  let status: MacHubStatus = { state: 'notEnrolled' }, controller: AbortController | undefined, running: Promise<unknown> | undefined;
  const fetcher = options.fetch ? { fetch: options.fetch } : {};

  async function start(): Promise<MacHubStatus> {
    if (controller && !controller.signal.aborted) return status;
    const credential = await readHubCredential(options.stateDirectory);
    if (!credential) { status = { state: 'notEnrolled' }; return status; }
    const [first, ...rest] = options.lanes(credential.runnerId);
    if (!first) throw new Error('This Mac has no runtime to offer the hub');
    const identity = { hubURL: credential.hubURL, runnerId: credential.runnerId };
    const ours = controller = new AbortController();
    status = { state: 'connecting', ...identity };
    running = runAsRunner({ stateDirectory: options.stateDirectory, ...first, lanes: rest, signal: ours.signal, ...fetcher,
      onStatus: next => { if (controller === ours) status = { ...identity, state: next.state, ...(next.retryMs === undefined ? {} : { retryMs: next.retryMs }) }; } })
      .catch((error: unknown) => { if (controller === ours) status = { ...identity, state: 'failed', detail: error instanceof Error ? error.message : 'The runner stopped' }; });
    return status;
  }

  async function stop(): Promise<void> {
    controller?.abort(); controller = undefined;
    await running; running = undefined;
  }

  return {
    status: (): MacHubStatus => ({ ...status }),
    start,
    stop,
    async enroll(text: string): Promise<MacHubStatus> {
      const invitation = parseRunnerInvitation(text);
      await enrollWithHub({ stateDirectory: options.stateDirectory, hubURL: invitation.hub, invitationId: invitation.invitationId, code: invitation.code,
        runnerId: invitation.runnerId, runnerName: options.runnerName, ...fetcher });
      return start();
    },
    // Disconnects and forgets this Mac's runner token. The hub keeps its record until the owner removes it there.
    async remove(): Promise<MacHubStatus> {
      await stop();
      await deleteHubCredential(options.stateDirectory);
      status = { state: 'notEnrolled' };
      return status;
    },
  };
}
