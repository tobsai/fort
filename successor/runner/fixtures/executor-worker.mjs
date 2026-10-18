import { createHash } from 'node:crypto';
import { createInterface } from 'node:readline';
import { setTimeout as delay } from 'node:timers/promises';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { createConnection } from 'node:net';

const digest = value => createHash('sha256').update(JSON.stringify(value)).digest('hex');
const scenario = process.argv[2];
async function hang() {
  process.on('SIGTERM', () => {});
  process.stdin.pause();
  setInterval(() => {}, 1_000);
  await new Promise(() => {});
}
if (scenario === 'unread-input') await hang();
const lines = createInterface({ input: process.stdin });
let active;
for await (const line of lines) {
  const command = JSON.parse(line);
  if (command.kind === 'cancel' && active) {
    if (command.dispatchDigest !== active.dispatchDigest || command.attemptId !== active.attemptId || command.generation !== active.generation) process.exit(4);
    process.stdout.write(JSON.stringify({ ...active, result: { kind: 'canceled', message: 'The exact fixture attempt was canceled.' } }) + '\n', () => process.exit(0));
    break;
  }
  if (command.kind !== 'dispatch') process.exit(2);
  const { dispatch, input } = command;
  const { dispatchDigest, ...body } = dispatch;
  if (digest(body) !== dispatchDigest || digest(input) !== dispatch.inputDigest || digest(input.configuration) !== dispatch.configurationDigest || digest(input.history) !== dispatch.historyDigest || digest(dispatch.policy) !== dispatch.policyDigest || dispatch.policy.kind !== 'plainText' || dispatch.policy.grants.length || dispatch.attempt.attemptId !== input.attempt.attemptId || dispatch.deadline <= Date.now()) process.exit(3);
  if (scenario === 'permission-controls') {
    if (!readFileSync(new URL(import.meta.url)).length || Object.keys(process.env).some(key => key !== '__CF_USER_TEXT_ENCODING')) process.exit(5);
    for (const probe of [
      () => readFileSync(process.cwd() + '/openai-candidate-b.sqlite'),
      () => spawnSync(process.execPath, ['--version'])
    ]) {
      let denied = false;
      try { probe(); } catch (error) { denied = error.code === 'ERR_ACCESS_DENIED'; }
      if (!denied) process.exit(6);
    }
    const networkDenied = await new Promise(resolve => {
      const socket = createConnection({ host: '127.0.0.1', port: 9 });
      socket.once('error', error => resolve(error.code === 'ERR_ACCESS_DENIED'));
      socket.once('connect', () => { socket.destroy(); resolve(false); });
    });
    if (!networkDenied) process.exit(7);
  }
  const frame = { protocolVersion: 1, kind: 'terminal', sequence: 1, dispatchDigest, attemptId: dispatch.attempt.attemptId, generation: dispatch.attempt.generation, result: { kind: 'completed', text: input.text, effectiveModel: null } };
  if (scenario === 'await-cancel') { active = frame; continue; }
  if (scenario === 'stale-generation') frame.generation++;
  if (scenario === 'foreign-attempt') frame.attemptId = 'foreign-attempt';
  if (scenario === 'foreign-dispatch') frame.dispatchDigest = '0'.repeat(64);
  if (scenario === 'oversized-reply') frame.result.text = 'x'.repeat(65_537);
  if (scenario === 'no-terminal') process.exit(0);
  if (scenario === 'late-terminal') await delay(Math.max(1, dispatch.deadline - Date.now() + 50));
  if (scenario === 'hang') await hang();
  if (scenario === 'terminal-then-hang') { process.stdout.write(JSON.stringify(frame) + '\n'); await hang(); }
  if (scenario === 'invalid-utf8') { process.stdout.write(Buffer.from([255, 10]), () => process.exit(0)); break; }
  if (scenario === 'stdout-flood' || scenario === 'stderr-flood') {
    (scenario === 'stdout-flood' ? process.stdout : process.stderr).write('x'.repeat(262_145), () => process.exit(0)); break;
  }
  let output = JSON.stringify(frame) + '\n';
  if (scenario === 'duplicate') output += output;
  if (scenario === 'conflicting-terminals') output += JSON.stringify({ ...frame, result: { kind: 'canceled', message: 'Contradictory fixture result.' } }) + '\n';
  process.stdout.write(output, () => process.exit(scenario === 'nonzero-exit' ? 1 : 0));
  break;
}
