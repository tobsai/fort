import { v4 as uuid } from 'uuid';
import type { ModuleBus } from '../module-bus/index.js';
import type { AgentFactory } from '../agents/hatchery.js';

export type ChoiceOption =
  | { action: 'switch_provider'; providerId: string; label: string }
  | { action: 'lighter_model'; tier: 'fast' | 'standard'; label: string }
  | { action: 'use_api_key'; providerId: string; label: string };

export interface ChoiceRequest {
  taskId: string;
  agentId: string;
  gatedModel: string;
  options: ChoiceOption[];
}

export interface ResolvedChoice {
  action: 'switch_provider' | 'lighter_model' | 'use_api_key' | 'fallback';
  providerId?: string;
  tier?: 'fast' | 'standard';
  apiKey?: string;
  remember: boolean;
}

const TIMEOUT_MS = 600_000; // 10 minutes — matches tool approval

/**
 * Blocks an interactive task while the user picks how to handle a gated model.
 * Same shape as ToolExecutor.awaitApproval: a resolver Map + bus event + timeout.
 */
export class ModelChoiceService {
  private pending = new Map<string, { resolve: (c: ResolvedChoice) => void }>();
  private factory: AgentFactory | null = null;

  constructor(private bus: ModuleBus) {}

  /** Injected in Fort so remembered choices can be persisted to identity.yaml. */
  setAgentFactory(factory: AgentFactory): void { this.factory = factory; }

  requestChoice(req: ChoiceRequest): Promise<ResolvedChoice> {
    const id = uuid();
    void this.bus.publish('model-choice.required', 'model-choice', { id, ...req });
    return new Promise<ResolvedChoice>((resolve) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        resolve({ action: 'fallback', remember: false });
      }, TIMEOUT_MS);
      this.pending.set(id, {
        resolve: (c) => { clearTimeout(timer); this.pending.delete(id); resolve(c); },
      });
    });
  }

  resolveChoice(id: string, choice: ResolvedChoice): boolean {
    const p = this.pending.get(id);
    if (!p) return false;
    p.resolve(choice);
    return true;
  }

  /** Persist a remembered choice to the agent's identity. No-op without a factory. */
  persist(agentId: string, patch: { provider?: string; defaultModelTier?: 'fast' | 'standard' | 'powerful' }): void {
    this.factory?.updateIdentity(agentId, patch as any);
  }
}
