import { setTimeout as delay } from 'node:timers/promises';
const [directory, coreModule, channelModule, mode, recoveryBehavior] = process.argv.slice(2);
if (!directory || !coreModule || !channelModule) throw new Error('Select exact synthetic inputs');
const { createApplication } = await import(coreModule);
const { servePrivateChannel } = await import(channelModule);
const service = createApplication({ dataDirectory: directory, autoRun: true,
  ...(mode === 'owner' ? { ownerPairingPolicy: { expiresAfterDays: null } } : {}),
  provider: { provider: 'pipe-fixture', name: 'Pipe fixture', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
  runtime: { async execute(input) { process.send?.({ kind: 'executed' }); return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() { if (recoveryBehavior === 'slow') await delay(100); } }
});
const lifetime = setTimeout(() => { process.stdin.destroy(new Error('Synthetic deadline')); }, 15_000);
try {
  await service.start({ listener: { kind: 'privateSocket', name: 'core.sock' } });
  process.send?.({ kind: 'setup', installation: service.installation, enrollment: service.issueEnrollment() });
  await servePrivateChannel({ endpoint: service.endpoint, input: process.stdin, output: process.stdout, ownerRecovery: mode === 'owner' ? async identity => {
    const proof = await service.recoverOwnerPairing(identity);
    return recoveryBehavior === 'wrong-proof' ? { ...proof, deviceId: 'another-device' } : proof;
  } : undefined });
} catch { process.exitCode = 1; }
finally { clearTimeout(lifetime); await service.close(); process.disconnect?.(); }
if (process.exitCode === 1) process.exit(1);
