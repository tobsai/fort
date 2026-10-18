import { serveExecutorWorker } from '../executor-worker.ts';

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
  });
} catch { process.exitCode = 1; }
