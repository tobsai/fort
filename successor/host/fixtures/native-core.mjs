import { writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
const [directory, corePath, channelPath] = process.argv.slice(2);
if (!directory || !corePath || !channelPath) throw new Error('Select exact synthetic inputs');
const { createApplication } = await import(pathToFileURL(corePath).href);
const { servePrivateChannel } = await import(pathToFileURL(channelPath).href);
let executions = 0;
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
  writeFileSync('setup.json', JSON.stringify({ installation: service.installation, enrollment: service.issueEnrollment() }), { mode: 0o600 });
  await servePrivateChannel({ endpoint: service.endpoint, input: process.stdin, output: process.stdout });
} catch { process.exitCode = 1; }
finally { clearTimeout(lifetime); await service.close(); }
if (process.exitCode === 1) process.exit(1);
