import { createInterface } from 'node:readline';
import { spawn } from 'node:child_process';

const scenario = process.argv[2] ?? 'normal';
const watchdog = setTimeout(() => process.exit(2), 20_000);
watchdog.unref();
let descendant;
if (scenario === 'ignore-term' || scenario === 'parent-death') {
  process.on('SIGTERM', () => {});
  setInterval(() => {}, 1000);
  descendant = spawn(process.execPath, ['-e', 'process.on("SIGTERM",()=>{});setTimeout(()=>process.exit(2),20000);process.stdout.write("ready\\n");'], { env: {}, stdio: ['ignore', 'pipe', 'ignore'] });
  await new Promise((resolve, reject) => {
    descendant.once('error', reject);
    descendant.stdout.once('data', resolve);
    descendant.once('exit', () => reject(new Error('Synthetic descendant exited before readiness')));
  });
}
const lines = createInterface({ input: process.stdin });
for await (const line of lines) {
  const request = JSON.parse(line);
  if (request.method !== 'ping' || typeof request.id !== 'string') process.exit(2);
  process.stdout.write(JSON.stringify({ id: request.id, result: { pid: process.pid, guardian: process.ppid, descendant: descendant?.pid,
    environment: Object.keys(process.env), echo: request.params } }) + '\n');
}
