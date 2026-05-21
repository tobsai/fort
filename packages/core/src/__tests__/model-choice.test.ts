import { describe, it, expect, vi } from 'vitest';
import { ModuleBus } from '../module-bus/index.js';
import { ModelChoiceService } from '../services/model-choice.js';

describe('ModelChoiceService', () => {
  it('publishes model-choice.required and resolves when answered', async () => {
    const bus = new ModuleBus();
    const svc = new ModelChoiceService(bus);
    const events: any[] = [];
    bus.subscribe('model-choice.required', (e) => { events.push(e.payload); });

    const p = svc.requestChoice({
      taskId: 't1', agentId: 'fort', gatedModel: 'claude-opus-4-6',
      options: [{ action: 'switch_provider', providerId: 'openai', label: 'Switch to OpenAI' }],
    });
    expect(events).toHaveLength(1);
    const id = events[0].id;
    svc.resolveChoice(id, { action: 'switch_provider', providerId: 'openai', remember: false });
    await expect(p).resolves.toEqual({ action: 'switch_provider', providerId: 'openai', remember: false });
  });

  it('times out to a fallback action', async () => {
    vi.useFakeTimers();
    const bus = new ModuleBus();
    const svc = new ModelChoiceService(bus);
    const p = svc.requestChoice({ taskId: 't2', agentId: 'fort', gatedModel: 'm', options: [] });
    vi.advanceTimersByTime(600_001);
    await expect(p).resolves.toEqual({ action: 'fallback', remember: false });
    vi.useRealTimers();
  });
});
