import { writeFileSync } from 'node:fs';
import { serveExecutorWorker } from '../executor-worker.ts';
import { createInterface } from 'node:readline';

const scenario = process.argv[2];
writeFileSync('identity.json', JSON.stringify({ pid: process.pid }), { mode: 0o600 });
try {
  if (scenario.startsWith('wire-')) {
    for await (const line of createInterface({ input: process.stdin })) {
      const request = JSON.parse(line);
      const identity = { protocolVersion: 1, requestId: request.requestId, requestDigest: request.requestDigest };
      const challenge = { ...identity, kind: 'accountChallenge', sequence: 0,
        challenge: { kind: 'deviceCode', verificationURL: 'https://device.fixture.invalid/verify', userCode: 'FORT-TEST' } };
      const terminal = { ...identity, kind: 'accountResult', sequence: 1, result: { kind: 'signedIn' } };
      let frames = [challenge, terminal];
      switch (scenario) {
        case 'wire-foreign': challenge.requestId = 'foreign'; break;
        case 'wire-digest': challenge.requestDigest = '0'.repeat(64); break;
        case 'wire-sequence': challenge.sequence = 1; break;
        case 'wire-duplicate': frames = [challenge, challenge, terminal]; break;
        case 'wire-changed': frames = [challenge, { ...challenge, challenge: { ...challenge.challenge, userCode: 'OTHER-CODE' } }, terminal]; break;
        case 'wire-reordered': frames = [terminal, challenge]; break;
        case 'wire-oversized': challenge.challenge.userCode = 'A'.repeat(20_000); break;
        case 'wire-insecure': challenge.challenge.verificationURL = 'http://device.fixture.invalid/verify'; break;
        case 'wire-credential-url': challenge.challenge.verificationURL = 'https://secret@device.fixture.invalid/verify'; break;
        case 'wire-secret-field': terminal.result.token = 'synthetic-private'; break;
        case 'wire-conflicting-result': frames = [terminal, { ...terminal, result: { kind: 'canceled' } }]; break;
        case 'wire-flood': frames = Array(9).fill(challenge); break;
        case 'wire-no-terminal': frames = [challenge]; break;
        case 'wire-nonzero': process.exitCode = 4; break;
        case 'wire-hang': frames = [challenge]; process.on('SIGTERM', () => {}); break;
      }
      process.stdout.write(frames.map(frame => JSON.stringify(frame)).join('\n') + '\n');
      if (scenario === 'wire-hang') { setTimeout(() => process.exit(2), 20_000); await new Promise(() => {}); }
      process.stdin.destroy(); break;
    }
  } else {
  await serveExecutorWorker({ input: process.stdin, output: process.stdout,
    binding: { provider: 'independent-device-fixture', runtimeVersion: 'fixture-7', hostId: 'account-fixture-host' },
    createRuntime: () => { throw new Error('No chat runtime in this fixture'); },
    createSignIn: () => ({
      execute: async () => { throw new Error('No chat operation'); },
      async signIn(options, signal) {
        if (scenario === 'existing') return { kind: 'signedIn' };
        options.onChallenge({ kind: 'deviceCode', verificationURL: 'https://device.fixture.invalid/verify', userCode: 'FORT-TEST' });
        if (scenario === 'pending') {
          if (!signal.aborted) await new Promise(resolve => signal.addEventListener('abort', resolve, { once: true }));
          return { kind: 'canceled' };
        }
        return { kind: 'signedIn' };
      },
      async dispose() {
        if (scenario === 'cleanup-fails') throw new Error('Synthetic cleanup failed');
        writeFileSync('disposed', 'synthetic cleanup confirmed', { mode: 0o600 });
      },
    }),
  });
  }
} catch { process.exitCode = 1; }
