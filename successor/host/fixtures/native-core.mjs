import { readFileSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:net';
import { pathToFileURL } from 'node:url';
const [directory, corePath, channelPath, outside] = process.argv.slice(2);
if (!directory || !corePath || !channelPath) throw new Error('Select exact synthetic inputs');
const { createApplication } = await import(pathToFileURL(corePath).href);
const { servePrivateChannel } = await import(pathToFileURL(channelPath).href);
let executions = 0;
const controls = {};
if (outside) {
  const denied = action => { try { action(); return false; } catch (error) { return ['EPERM', 'EACCES'].includes(error.code); } };
  controls.outsideReadDenied = denied(() => readFileSync(outside));
  controls.outsideWriteDenied = denied(() => writeFileSync(outside, 'unexpected synthetic write'));
  controls.tcpDenied = await new Promise(resolve => {
    const server = createServer();
    server.once('error', error => resolve(['EPERM', 'EACCES'].includes(error.code)));
    server.listen(0, '127.0.0.1', () => server.close(() => resolve(false)));
  });
}
const service = createApplication({ dataDirectory: directory, autoRun: true,
  provider: { provider: 'native-pipe-fixture', name: 'Native pipe fixture', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
  runtime: { async execute(input) {
    writeFileSync('executions.json', JSON.stringify(++executions), { mode: 0o600 });
    return { kind: 'completed', text: input.text, effectiveModel: null };
  }, async dispose() {} }
});
writeFileSync('executions.json', '0', { mode: 0o600 });
const lifetime = setTimeout(() => process.stdin.destroy(new Error('Synthetic deadline')), 15_000);
try {
  await service.start({ listener: { kind: 'privateSocket', name: 'core.sock' } });
  writeFileSync('setup.json', JSON.stringify({ installation: service.installation, enrollment: service.issueEnrollment(), controls }), { mode: 0o600 });
  await servePrivateChannel({ endpoint: service.endpoint, input: process.stdin, output: process.stdout });
} catch { process.exitCode = 1; }
finally { clearTimeout(lifetime); await service.close(); }
if (process.exitCode === 1) process.exit(1);
