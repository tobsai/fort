import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { createInterface } from 'node:readline';
import { setTimeout as delay } from 'node:timers/promises';

const [artifact, directory, action, mode] = process.argv.slice(2);
const { createApplication } = await import(pathToFileURL(join(artifact, 'service/openai.js')).href);
const commands = createInterface({ input: process.stdin }), iterator = commands[Symbol.asyncIterator]();
const owner = new AbortController();
process.stdin.once('end', () => owner.abort());
const lifetime = setTimeout(() => process.exit(124), 300_000);
const emit = value => process.stdout.write(JSON.stringify(value) + '\n');
const provider = { provider: 'performance-fixture', name: 'Performance fixture', runtimeVersion: '1', hostId: 'benchmark-host', session: 'stateless', capabilities: ['plainText'] };
const runtime = () => ({ async execute() { return { kind: 'completed', text: 'r'.repeat(1024), effectiveModel: null }; }, async dispose() {} });
let service;
const open = async autoRun => {
  const started = performance.now();
  service = createApplication({ dataDirectory: directory, provider, runtime: runtime(), autoRun });
  await service.start();
  return performance.now() - started;
};
async function command(expected) {
  const next = await iterator.next();
  if (next.done || next.value !== expected) throw new Error('Measurement controller disconnected');
}

try {
  const first = await open(false);
  emit({ kind: 'coldReady', applicationMs: first, ...(action === 'load' ? { client: { baseURL: service.baseURL, installation: service.installation, enrollment: service.issueEnrollment() } } : {}) });
  if (action === 'startup') {
    await service.close();
    const warmMs = await open(false);
    emit({ kind: 'report', data: { warmMs, applicationMs: first } });
  } else if (action === 'load') {
    for (;;) {
      const next = await iterator.next();
      if (next.done) throw new Error('Measurement controller disconnected');
      if (next.value === 'finishLoad') break;
      if (next.value !== 'dispatch') throw new Error('Unknown measurement command');
      await service.dispatchNext();
      emit({ kind: 'dispatched' });
    }
    await service.close(); await open(true);
    const memoryBefore = process.memoryUsage(), rssBefore = memoryBefore.rss;
    emit({ kind: 'idleReady' });
    await command('startIdle');
    const cpu = process.cpuUsage(), idleStart = performance.now();
    await delay(mode === 'core' ? 60_000 : 50, undefined, { signal: owner.signal });
    const elapsedMs = performance.now() - idleStart, used = process.cpuUsage(cpu), memoryAfter = process.memoryUsage(), rssAfter = memoryAfter.rss;
    emit({ kind: 'idleDone' });
    await command('finishIdle');
    emit({ kind: 'report', data: {
      idle: { workerPid: process.pid, elapsedMs, cpuUserMicros: used.user, cpuSystemMicros: used.system, cpuPercentOneCore: (used.user + used.system) / (elapsedMs * 10),
        rssBefore, rssAfter, memoryBefore, memoryAfter }
    } });
  } else throw new Error('Unknown worker action');
} finally {
  clearTimeout(lifetime); await service?.close(); commands.close(); process.stdin.destroy();
}
