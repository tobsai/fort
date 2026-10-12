import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { createFixtureService } from '../service/service.ts';

const executable = process.argv[2];
if (!executable) throw new Error('Pass the built FortNativeCheck executable path');
const controls = process.argv[3] === 'controls';
const directory = await mkdtemp(join(tmpdir(), 'fort-native-http-'));
let service = createFixtureService({ dataDirectory: join(directory, 'ledger'), autoRun: true, fixtureBehavior: controls ? 'approval' : 'echo' });
try {
  await service.start();
  const config = { baseURL: service.baseURL, installationId: service.installation.installationId,
    codeA: service.issueEnrollment().code, codeB: service.issueEnrollment().code,
    stateDirectory: pathToFileURL(join(directory, 'native')).href };
  const configPath = join(directory, 'fixture-config.json');
  await writeFile(configPath, JSON.stringify(config), { mode: 0o600 });
  const run = async (phase: string) => {
    const result = await promisify(execFile)(resolve(executable), [phase, configPath], { env: { PATH: '/usr/bin:/bin' }, timeout: 20_000 });
    process.stdout.write(result.stdout);
  };
  await run(controls ? 'controls-prepare' : 'prepare');
  await service.close();
  service = createFixtureService({ dataDirectory: join(directory, 'ledger'), autoRun: true, fixtureBehavior: controls ? 'approval' : 'echo' });
  await service.start();
  config.baseURL = service.baseURL;
  await writeFile(configPath, JSON.stringify(config), { mode: 0o600 });
  await run(controls ? 'controls-replay' : 'replay');
} finally {
  await service.close();
  await rm(directory, { recursive: true, force: true });
}
