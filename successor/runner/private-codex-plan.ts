import { lstatSync, readdirSync, realpathSync } from 'node:fs';
import { isAbsolute, join, normalize } from 'node:path';
import { CODEX_DISABLED_FEATURES, CODEX_SETTINGS } from './codex-controls.ts';
import { CODEX_PERMISSION_OVERRIDE } from './codex-permissions.ts';

type PrivateCodexPaths = { executable: string; guardian: string; directory: string };

function exactPath(value: string): boolean {
  return typeof value === 'string' && value.isWellFormed() && !value.includes('\0') && Buffer.byteLength(value) <= 4096
    && isAbsolute(value) && value !== '/' && normalize(value) === value;
}

export function privateCodexLaunchPlan({ executable, guardian, directory }: PrivateCodexPaths) {
  if (![executable, guardian, directory].every(exactPath) || !process.getuid) throw new Error('Select exact private runtime paths');
  const uid = process.getuid();
  const home = join(directory, 'home'), state = join(directory, 'state'), workspace = join(directory, 'workspace'), temporary = join(directory, 'tmp');
  const paths = [directory, home, state, workspace, temporary];
  const inspect = (path: string) => {
    const info = lstatSync(path);
    if (!exactPath(path) || !info.isDirectory() || info.uid !== uid || (info.mode & 0o777) !== 0o700 || realpathSync(path) !== path) throw new Error('Runtime directories must be canonical and private');
    return info;
  };
  const identities = paths.map(inspect);
  if (readdirSync(home).length !== 0 || readdirSync(workspace).length !== 0) throw new Error('Runtime home and workspace must be empty');
  for (const name of ['.env', 'config.toml']) {
    if (lstatSync(join(state, name), { throwIfNoEntry: false })) throw new Error('Ambient runtime configuration is not allowed');
  }
  const auth = lstatSync(join(state, 'auth.json'), { throwIfNoEntry: false });
  if (auth && (!auth.isFile() || auth.uid !== uid || auth.nlink !== 1 || (auth.mode & 0o077) !== 0 || auth.size < 1 || auth.size > 128 * 1024)) throw new Error('Runtime credential metadata is not private');
  for (const [index, path] of paths.entries()) {
    const current = inspect(path), prior = identities[index];
    if (current.dev !== prior?.dev || current.ino !== prior.ino) throw new Error('Runtime directory changed during inspection');
  }
  const settings = [...CODEX_SETTINGS, ['forced_login_method', 'chatgpt'], ['cli_auth_credentials_store', 'file'], ['mcp_oauth_credentials_store', 'file'],
    ['otel.exporter', 'none'], ['otel.trace_exporter', 'none'], ['otel.metrics_exporter', 'none'], ['otel.log_user_prompt', false]] as const;
  const args = Object.freeze(['app-server', '--listen', 'stdio://', ...[...settings.map(([name, value]) => `${name}=${JSON.stringify(value)}`), CODEX_PERMISSION_OVERRIDE].flatMap(value => ['-c', value]),
    '--enable', 'skip_host_skill_discovery', ...CODEX_DISABLED_FEATURES.flatMap(value => ['--disable', value])]);
  const env = Object.freeze({ HOME: home, CODEX_HOME: state, TMPDIR: temporary, PATH: '/usr/bin:/bin', LANG: 'en_US.UTF8', TZ: 'UTC' });
  return Object.freeze({ executable, guardian, args, cwd: workspace, env });
}
