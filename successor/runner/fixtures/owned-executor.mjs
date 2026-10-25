import { serveExecutorWorker } from '../executor-worker.ts';
import { portraitPng } from './portrait-png.ts';

const scenario = process.argv[2];
try {
  await serveExecutorWorker({ input: process.stdin, output: process.stdout,
    binding: { provider: scenario === 'models' ? 'catalog-fixture' : 'worker-fixture', runtimeVersion: 'fixture-1', hostId: 'fixture-host' },
    createRuntime: () => ({
      async execute(input) { return { kind: 'completed', text: input.text, effectiveModel: null }; },
      async dispose() { if (scenario === 'cleanup-fail') throw new Error('Synthetic cleanup failure'); },
    }),
    createDiscovery: () => ({
      async execute() { throw new Error('Discovery must not execute'); },
      async models() { return { provider: 'catalog-fixture', state: 'ready', checkedAt: '2026-10-18T00:00:00.000Z',
        models: [{ id: 'exact-text', name: 'Text model', availability: 'available' },
          { id: 'exact-audio', name: 'Audio model', availability: 'unavailable', detail: 'Text input unavailable.' }] }; },
      async dispose() {},
    }),
    createPortrait: () => ({
      async execute() { throw new Error('Portraits must not execute chat'); },
      async portrait(request) {
        if (scenario === 'portrait-limit') return { kind: 'failed', reason: 'usageLimit', message: 'Plan limit reached.' };
        const png = portraitPng(1024, true);
        return request.subject.name === 'Pascal' ? { kind: 'drawn', png, width: 1024, height: 1024 } : { kind: 'failed', reason: 'refused', message: 'Wrong subject.' };
      },
      async dispose() {},
    }),
  });
} catch { process.exitCode = 1; }
