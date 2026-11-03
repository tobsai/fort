import type { AccountIdentity, AccountSignInOptions, AccountSignInResult, ModelCatalog, RuntimeReadiness } from '../contracts/v1.ts';
import type { ExecutionDispatch } from '../contracts/executor.ts';
import type { ChatInput, ChatResult, ChatRuntime } from '../runner/chat-runtime.ts';
import type { RunPins } from './runner-pool.ts';

// Cloud first (Spec 063, revision 2026-11-03). The hub runs Agents itself; Macs are extensions. One deterministic rule per turn:
//   1. A run that already started somewhere stays there (its pin), so a routine never hops machines mid-way.
//   2. Otherwise the hub takes it, when it serves this lane and is signed in.
//   3. Otherwise it goes to the Macs, exactly as before (the pool, or the one enrolled Mac).
// Lanes the hub can't serve (Claude today) never come here: they keep their Mac route unchanged.
export const CLOUD_RUNNER_ID = 'cloud';

export type CloudLane = {
  // The hub's own executor for this lane.
  runtime: ChatRuntime;
  // Whether the hub is signed in to this provider. Read on every decision; never cached.
  signedIn(): boolean;
};

type Options = {
  cloud: CloudLane;
  // The Mac route for the same lane: routePool or routeByNode. Undefined when no Mac is configured at all.
  macs?: ChatRuntime;
  pins: RunPins;
  // Called after a sign-in on the hub finishes, so queued work can start.
  onSignedIn?: () => void;
};

const offline = (detail: string): RuntimeReadiness => ({ state: 'unavailable', canRestart: false, detail });

export function routeCloudFirst(options: Options): ChatRuntime {
  const { cloud, macs, pins } = options;
  // Rule 2 or 3 for a turn with no pin yet.
  const target = (): 'cloud' | 'macs' | undefined => cloud.signedIn() ? 'cloud' : macs ? 'macs' : undefined;
  return {
    readiness(): RuntimeReadiness {
      if (cloud.signedIn()) {
        const own = cloud.runtime.readiness?.();
        // The hub's executor is busy with one turn at a time; the core waits and tries again, as it does for a Mac.
        return own && own.state !== 'ready' ? own : { state: 'ready', canRestart: false, detail: 'Runs on the hub.' };
      }
      const mac = macs?.readiness?.() ?? (macs ? { state: 'ready', canRestart: false, detail: 'Runs go to your Mac.' } as RuntimeReadiness : undefined);
      if (mac?.state === 'ready') return mac;
      return offline('Sign in to ChatGPT on the hub (Settings, Accounts) so Agents run without a Mac.' + (mac ? ' ' + mac.detail : ''));
    },
    async execute(input: ChatInput, signal: AbortSignal, dispatch: ExecutionDispatch): Promise<ChatResult> {
      const pinned = pins.get(dispatch.attempt.runId);
      if (pinned !== undefined && pinned !== CLOUD_RUNNER_ID) {
        // Rule 1: started on a Mac, finishes on a Mac. The Mac route enforces which one.
        if (!macs) return { kind: 'notStarted', message: 'This run started on a Mac that is no longer set up. Nothing was sent.' };
        return macs.execute(input, signal, dispatch);
      }
      const where = pinned === CLOUD_RUNNER_ID ? 'cloud' : target();
      if (where === 'cloud') {
        if (!cloud.signedIn()) return { kind: 'notStarted', message: 'This run started on the hub, which is no longer signed in to ChatGPT. Nothing was sent.' };
        pins.pin(dispatch.attempt.runId, CLOUD_RUNNER_ID);
        return cloud.runtime.execute(input, signal, dispatch);
      }
      if (where === 'macs') return macs!.execute(input, signal, dispatch);
      return { kind: 'notStarted', message: 'Sign in to ChatGPT on the hub first. Nothing was sent.' };
    },
    // Model lists come from wherever turns would run now.
    models(signal: AbortSignal, account?: AccountIdentity): Promise<ModelCatalog> {
      if (cloud.signedIn() || !macs?.models) return cloud.runtime.models?.(signal, account) ?? Promise.resolve({ provider: 'codex', state: 'unavailable', models: [], detail: 'Model discovery is unavailable.' });
      return macs.models(signal, account);
    },
    // The hub can sign in whenever its own executor is free, signed in or not and whatever the Macs are doing.
    canSignIn: () => (cloud.runtime.readiness?.().state ?? 'ready') === 'ready',
    // Signing in always happens on the hub: that is what lets Fort run with no Mac at all.
    async signIn(request: AccountSignInOptions, signal: AbortSignal): Promise<AccountSignInResult> {
      if (!cloud.runtime.signIn) return { kind: 'unconfirmed' };
      const result = await cloud.runtime.signIn(request, signal);
      if (result.kind === 'signedIn') options.onSignedIn?.();
      return result;
    },
    dispose: async () => { await cloud.runtime.dispose(); },
  };
}
