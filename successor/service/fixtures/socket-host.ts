import { createApplication } from '../openai.ts';

const [directory, name] = process.argv.slice(2);
if (!directory || !name) throw new Error('Provide synthetic directory and socket name');
const service = createApplication({ dataDirectory: directory, autoRun: true,
  provider: { provider: 'socket-fixture', name: 'Socket fixture', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
  runtime: { async execute(input) { return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} }
});
const stop = async () => { await service.close(); if (process.connected) process.disconnect?.(); };
process.once('disconnect', () => { void stop(); });
process.on('message', message => { if (message === 'close') void stop(); });
const lifetime = setTimeout(() => { void stop(); }, 15_000); lifetime.unref();
try {
  const listener = { kind: 'privateSocket' as const, name };
  const starting = service.start({ listener });
  listener.name = 'changed.sock';
  await starting;
  let tcpURLAvailable = false;
  try { tcpURLAvailable = Boolean(service.baseURL); } catch {}
  process.send?.({ kind: 'ready', endpoint: service.endpoint, tcpURLAvailable, installation: service.installation, enrollment: service.issueEnrollment() });
} catch (error) {
  process.send?.({ kind: 'failed', message: error instanceof Error ? error.message : 'Startup failed' });
  process.exitCode = 1;
  await stop();
}
