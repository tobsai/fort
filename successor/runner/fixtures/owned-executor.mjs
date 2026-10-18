import { serveExecutorWorker } from '../executor-worker.ts';

const scenario = process.argv[2];
try {
  await serveExecutorWorker({ input: process.stdin, output: process.stdout,
    binding: { provider: 'worker-fixture', runtimeVersion: 'fixture-1', hostId: 'fixture-host' },
    createRuntime: () => ({
      async execute(input) { return { kind: 'completed', text: input.text, effectiveModel: null }; },
      async dispose() { if (scenario === 'cleanup-fail') throw new Error('Synthetic cleanup failure'); },
    }),
  });
} catch { process.exitCode = 1; }
