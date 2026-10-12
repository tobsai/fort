import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { realpathSync } from 'node:fs';

export const FIXTURE_NODE = '/opt/homebrew/opt/node@26/bin/node';
// Node permissions constrain trusted fixtures. This is not an OS provider sandbox.
export function spawnFixtureProcess(entrypoint: URL) {
  if (process.version !== 'v26.5.0') throw new Error('Fixture requires pinned Node v26.5.0.');
  const worker = realpathSync(fileURLToPath(entrypoint));
  return spawn(FIXTURE_NODE, ['--permission', `--allow-fs-read=${worker}`, worker], {
    env: { LANG: 'en_US.UTF-8', TZ: 'UTC' }, cwd: '/', stdio: ['pipe', 'pipe', 'pipe'], timeout: 10_000,
  });
}
