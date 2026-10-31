// Signs an assembled Fort.app (packaging/mac-application.ts) for the Mac App Store, checks every signature
// against packaging/mac-signing-plan.ts, and wraps it in the installer package App Store Connect takes.
//
//   node packaging/mac-sign.ts --app <assembled>/Fort.app --mode distribution \
//     --app-identity "Apple Distribution" --installer-identity "3rd Party Mac Developer Installer" \
//     --client-profile Fort.provisionprofile --helper-profile FortHost.provisionprofile \
//     --package Fort.pkg --report signing-report.json
//
// --mode adhoc signs with "-" and no profiles, for CI: it proves the layout, identifiers and entitlements
// but the result can't be uploaded or run as the owner's host. Uses identities already in the keychain;
// it never reads, exports or stores a key or certificate.
import { execFileSync, spawnSync } from 'node:child_process';
import { copyFileSync, existsSync, lstatSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { isAbsolute, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';
import { type Entitlements, type HostConfiguration, type SigningMode, type SigningStep, HELPER, PROFILE, entitlementsPlist, helperExceptions, hostConfiguration, profileProblems, signatureProblems, signingPlan } from './mac-signing-plan.ts';

const run = (tool: string, args: string[]) => execFileSync(tool, args, { encoding: 'utf8', timeout: 300_000, stdio: 'pipe' });
const exact = (path: string | undefined, name: string) => {
  if (typeof path !== 'string' || !isAbsolute(path) || normalize(path) !== path || path === '/') throw new Error(`Give --${name} as an exact absolute path`);
  return path;
};
const json = (file: string, key?: string): unknown => JSON.parse(run('/usr/bin/plutil', key ? ['-extract', key, 'json', '-o', '-', file] : ['-convert', 'json', '-o', '-', file]));

function decodedProfile(path: string, scratch: string): Record<string, unknown> {
  const plist = join(scratch, 'profile-' + Math.random().toString(16).slice(2) + '.plist');
  run('/usr/bin/security', ['cms', '-D', '-i', path, '-o', plist]);
  // Profiles hold <date> and <data> values, which plutil can't write as JSON, so read the fields we check.
  const optional = (key: string, type: 'json' | 'raw') => {
    const result = spawnSync('/usr/bin/plutil', ['-extract', key, type, '-o', '-', plist], { encoding: 'utf8' });
    return result.status === 0 ? (type === 'json' ? JSON.parse(result.stdout) : result.stdout.trim()) : undefined;
  };
  const profile: Record<string, unknown> = { TeamIdentifier: optional('TeamIdentifier', 'json'), Platform: optional('Platform', 'json'), Entitlements: optional('Entitlements', 'json'), ExpirationDate: optional('ExpirationDate', 'raw') };
  if (optional('ProvisionedDevices', 'json') !== undefined) profile.ProvisionedDevices = true;
  if (optional('ProvisionsAllDevices', 'raw') === 'true') profile.ProvisionsAllDevices = true;
  return profile;
}

function signed(path: string, scratch: string) {
  const details = spawnSync('/usr/bin/codesign', ['-d', '--verbose=2', path], { encoding: 'utf8' });
  if (details.status !== 0) throw new Error(`Not signed: ${path}\n${details.stderr}`);
  const field = (name: string) => new RegExp(`^${name}=(.+)$`, 'm').exec(details.stderr)?.[1] ?? null;
  const file = join(scratch, 'signed-' + Math.random().toString(16).slice(2) + '.plist');
  run('/usr/bin/codesign', ['-d', '--entitlements', file, '--xml', path]);
  const entitlements = existsSync(file) && lstatSync(file).size > 0 ? json(file) as Record<string, unknown> : {};
  const team = field('TeamIdentifier');
  return { identifier: field('Identifier'), team: team === 'not set' ? null : team, entitlements };
}

function signStep(app: string, step: SigningStep, identity: string, scratch: string) {
  const target = step.path ? join(app, step.path) : app;
  const args = ['--force', '--sign', identity, '--timestamp=none', '--options', 'runtime'];
  if (step.identifier && step.kind === 'runtime') args.push('--identifier', step.identifier);
  if (step.entitlements) {
    const file = join(scratch, (step.identifier ?? 'library') + '.entitlements');
    writeFileSync(file, entitlementsPlist(step.entitlements));
    run('/usr/bin/plutil', ['-lint', file]);
    args.push('--entitlements', file);
  }
  run('/usr/bin/codesign', [...args, target]);
}

export function signApplication(options: { app: string; mode: SigningMode; appIdentity: string; installerIdentity?: string; clientProfile?: string; helperProfile?: string; helperEntitlements?: string; packagePath: string; reportPath: string }) {
  if (process.platform !== 'darwin') throw new Error('Signing runs on macOS');
  const { app, mode } = options;
  const helper = join(app, HELPER);
  const configBytes = readFileSync(join(app, 'Contents/Resources/fort-host.json'));
  if (!configBytes.equals(readFileSync(join(helper, 'Contents/Resources/fort-host.json')))) throw new Error('Client and helper host configurations differ');
  const config: HostConfiguration = hostConfiguration(JSON.parse(configBytes.toString('utf8')));
  for (const name of readdirSync(app, { recursive: true, encoding: 'utf8' })) if (name.endsWith('/_CodeSignature') || name === 'Contents/_CodeSignature') throw new Error('Sign a freshly assembled Fort.app: found ' + name);
  const exceptions: Entitlements = options.helperEntitlements ? helperExceptions(json(options.helperEntitlements)) : {};
  const frameworks = join(helper, 'Contents/Frameworks');
  const libraries = existsSync(frameworks) ? readdirSync(frameworks) : [];
  const plan = signingPlan(config, libraries, mode, exceptions);
  const scratch = mkdtempSync(join(tmpdir(), 'fort-mac-sign-'));
  try {
    if (mode === 'distribution') {
      const profiles = { client: options.clientProfile, helper: options.helperProfile };
      const problems = (['client', 'helper'] as const).flatMap(kind => {
        const path = profiles[kind];
        if (!path) return [`missing --${kind}-profile`];
        return profileProblems(decodedProfile(path, scratch), kind === 'client' ? config.clientIdentifier : config.serviceIdentifier, config, new Date());
      });
      if (problems.length) throw new Error('Provisioning profiles do not fit:\n' + problems.join('\n'));
      copyFileSync(profiles.client!, join(app, PROFILE));
      copyFileSync(profiles.helper!, join(helper, PROFILE));
    }
    const identity = mode === 'adhoc' ? '-' : options.appIdentity;
    for (const step of plan) signStep(app, step, identity, scratch);

    run('/usr/bin/codesign', ['--verify', '--strict', '--deep', '--verbose=2', app]);
    const problems = plan.flatMap(step => signatureProblems(step, signed(step.path ? join(app, step.path) : app, scratch), config, mode));
    for (const step of plan) if (step.profile && !existsSync(join(app, step.path, PROFILE))) problems.push(`${step.path || 'Fort.app'}: no embedded provisioning profile`);
    if (problems.length) throw new Error('Signed bundle does not match the plan:\n' + problems.join('\n'));

    if (existsSync(options.packagePath)) throw new Error('Package already exists: ' + options.packagePath);
    const build = ['--component', app, '/Applications'];
    if (mode === 'distribution') {
      if (!options.installerIdentity) throw new Error('Give --installer-identity for a distribution package');
      build.push('--sign', options.installerIdentity);
    }
    run('/usr/bin/productbuild', [...build, options.packagePath]);
    if (mode === 'distribution') run('/usr/sbin/pkgutil', ['--check-signature', options.packagePath]);

    const report = { format: 1, mode, configuration: config, uploadable: mode === 'distribution', package: options.packagePath,
      signed: plan.map(step => ({ kind: step.kind, path: step.path || '.', identifier: step.identifier, entitlements: Object.keys(step.entitlements ?? {}).sort(), profile: step.profile })) };
    writeFileSync(options.reportPath, JSON.stringify(report, null, 2) + '\n', { flag: 'wx' });
    return report;
  } finally { rmSync(scratch, { recursive: true, force: true }); }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    const { values } = parseArgs({ strict: true, options: Object.fromEntries(['app', 'mode', 'app-identity', 'installer-identity', 'client-profile', 'helper-profile', 'helper-entitlements', 'package', 'report'].map(name => [name, { type: 'string' as const }])) });
    const mode = values.mode;
    if (mode !== 'distribution' && mode !== 'adhoc') throw new Error('Choose --mode distribution or --mode adhoc');
    if (mode === 'distribution' && !values['app-identity']) throw new Error('Give --app-identity for distribution signing');
    const report = signApplication({ app: exact(values.app, 'app'), mode, appIdentity: values['app-identity'] ?? '-', installerIdentity: values['installer-identity'],
      clientProfile: values['client-profile'] && exact(values['client-profile'], 'client-profile'), helperProfile: values['helper-profile'] && exact(values['helper-profile'], 'helper-profile'),
      helperEntitlements: values['helper-entitlements'] && exact(values['helper-entitlements'], 'helper-entitlements'),
      packagePath: exact(values.package, 'package'), reportPath: exact(values.report, 'report') });
    process.stdout.write(JSON.stringify({ mode: report.mode, package: report.package, signed: report.signed.length, uploadable: report.uploadable }) + '\n');
  } catch (error) { process.stderr.write((error instanceof Error ? error.message : 'Signing failed') + '\n'); process.exitCode = 1; }
}
