// What signing Fort.app for the Mac App Store (and so TestFlight) means, as data. mac-sign.ts carries it
// out on a Mac; the tests check it against the contract the app enforces at runtime (FortKit
// MacHostBundle.swift and MacHostProgram.swift):
//   - Fort.app and Contents/Helpers/FortHost.app each run in their own App Sandbox, share exactly one
//     App Group, never inherit a sandbox, and carry their own embedded provisioning profile.
//   - node, FortProcessGuardian and codex inside the helper are signed as <service>.node, .guardian and
//     .codex with exactly two entitlements: app-sandbox and inherit, so they run inside the helper's sandbox.
// Code is signed inside out: Node's libraries, then the three runtimes, then the helper, then the app.

export type HostConfiguration = { format: '1'; clientIdentifier: string; serviceIdentifier: string; signingTeam: string; appGroup: string };
export type SigningMode = 'distribution' | 'adhoc';
export type Entitlements = Record<string, boolean | string | string[]>;
export type SigningStep = {
  kind: 'library' | 'runtime' | 'helper' | 'client';
  path: string;                 // relative to Fort.app
  identifier: string | null;    // passed to codesign --identifier; null keeps codesign's default (libraries)
  entitlements: Entitlements | null;
  profile: 'client' | 'helper' | null;
};

export const HELPER = 'Contents/Helpers/FortHost.app';
export const PROFILE = 'Contents/embedded.provisionprofile';
export const RUNTIMES = [
  ['node', HELPER + '/Contents/MacOS/node'],
  ['guardian', HELPER + '/Contents/MacOS/FortProcessGuardian'],
  ['codex', HELPER + '/Contents/MacOS/codex'],
] as const;

const SANDBOX = 'com.apple.security.app-sandbox';
const INHERIT = 'com.apple.security.inherit';
const GROUPS = 'com.apple.security.application-groups';
const APPLICATION = 'com.apple.application-identifier';
const TEAM = 'com.apple.developer.team-identifier';
// The only extra entitlement the helper may carry from apple/Host/FortHost.entitlements: Spec 062's
// read-only view of the owner's installed Claude Code.
const HELPER_EXCEPTIONS = new Set(['com.apple.security.temporary-exception.files.home-relative-path.read-only']);

const identifier = (value: unknown) => typeof value === 'string' && /^[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+$/.test(value) && value.length <= 127;

export function hostConfiguration(value: unknown): HostConfiguration {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Host configuration must be an object');
  const config = value as Record<string, unknown>;
  if (Object.keys(config).sort().join(',') !== 'appGroup,clientIdentifier,format,serviceIdentifier,signingTeam' || config.format !== '1'
    || !identifier(config.clientIdentifier) || !identifier(config.serviceIdentifier) || !identifier(config.appGroup)
    || config.clientIdentifier === config.serviceIdentifier || typeof config.signingTeam !== 'string' || !/^[A-Z0-9]{10}$/.test(config.signingTeam)
    || typeof config.appGroup !== 'string' || !config.appGroup.startsWith('group.') || (config.appGroup + '.fort.host').length >= 128) throw new Error('Invalid host configuration');
  return config as HostConfiguration;
}

export function helperExceptions(value: unknown): Entitlements {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Helper entitlements must be a dictionary');
  const result: Entitlements = {};
  for (const [key, entry] of Object.entries(value)) {
    if (!HELPER_EXCEPTIONS.has(key)) throw new Error('Unreviewed helper entitlement: ' + key);
    if (!Array.isArray(entry) || !entry.length || entry.some(path => typeof path !== 'string' || !path.startsWith('/') || path.includes('..'))) throw new Error('Invalid helper exception paths');
    result[key] = [...entry];
  }
  return result;
}

const identity = (config: HostConfiguration, bundle: string, mode: SigningMode): Entitlements =>
  mode === 'distribution' ? { [APPLICATION]: config.signingTeam + '.' + bundle, [TEAM]: config.signingTeam } : {};

export function runtimeEntitlements(): Entitlements { return { [SANDBOX]: true, [INHERIT]: true }; }

export function helperEntitlements(config: HostConfiguration, mode: SigningMode, exceptions: Entitlements = {}): Entitlements {
  return { [SANDBOX]: true, [GROUPS]: [config.appGroup], 'com.apple.security.network.client': true, 'com.apple.security.network.server': true,
    ...helperExceptions(exceptions), ...identity(config, config.serviceIdentifier, mode) };
}

export function clientEntitlements(config: HostConfiguration, mode: SigningMode): Entitlements {
  return { [SANDBOX]: true, [GROUPS]: [config.appGroup], 'com.apple.security.network.client': true, ...identity(config, config.clientIdentifier, mode) };
}

export function signingPlan(config: HostConfiguration, libraries: string[], mode: SigningMode, exceptions: Entitlements = {}): SigningStep[] {
  const profile = mode === 'distribution';
  return [
    ...[...libraries].sort().map(name => {
      if (!/^[A-Za-z0-9._+-]+\.dylib$/.test(name)) throw new Error('Unexpected library name: ' + name);
      return { kind: 'library' as const, path: HELPER + '/Contents/Frameworks/' + name, identifier: null, entitlements: null, profile: null };
    }),
    ...RUNTIMES.map(([suffix, path]) => ({ kind: 'runtime' as const, path, identifier: config.serviceIdentifier + '.' + suffix, entitlements: runtimeEntitlements(), profile: null })),
    { kind: 'helper', path: HELPER, identifier: config.serviceIdentifier, entitlements: helperEntitlements(config, mode, exceptions), profile: profile ? 'helper' : null },
    { kind: 'client', path: '', identifier: config.clientIdentifier, entitlements: clientEntitlements(config, mode), profile: profile ? 'client' : null },
  ];
}

const escape = (value: string) => value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
export function entitlementsPlist(entitlements: Entitlements): string {
  const values = Object.keys(entitlements).sort().map(key => {
    const value = entitlements[key];
    const body = typeof value === 'boolean' ? (value ? '<true/>' : '<false/>')
      : typeof value === 'string' ? `<string>${escape(value)}</string>`
      : `<array>${value.map(item => `<string>${escape(item)}</string>`).join('')}</array>`;
    return `\t<key>${escape(key)}</key>\n\t${body}`;
  });
  return `<?xml version="1.0" encoding="UTF-8"?>\n<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n<plist version="1.0">\n<dict>\n${values.join('\n')}\n</dict>\n</plist>\n`;
}

// Compares what codesign reports for one signed item with the plan. Returns problems, empty when it matches.
export function signatureProblems(step: SigningStep, signed: { identifier: string | null; team: string | null; entitlements: Record<string, unknown> }, config: HostConfiguration, mode: SigningMode): string[] {
  const where = step.path || 'Fort.app', problems: string[] = [];
  if (step.identifier !== null && signed.identifier !== step.identifier) problems.push(`${where}: identifier ${signed.identifier} is not ${step.identifier}`);
  if (mode === 'distribution' && signed.team !== config.signingTeam) problems.push(`${where}: team ${signed.team} is not ${config.signingTeam}`);
  const expected = step.entitlements ?? {};
  const actualKeys = Object.keys(signed.entitlements).sort(), expectedKeys = Object.keys(expected).sort();
  if (actualKeys.join(',') !== expectedKeys.join(',')) problems.push(`${where}: entitlements ${actualKeys.join(', ') || 'none'} are not ${expectedKeys.join(', ') || 'none'}`);
  else for (const key of expectedKeys) if (JSON.stringify(signed.entitlements[key]) !== JSON.stringify(expected[key])) problems.push(`${where}: ${key} does not match`);
  return problems;
}

// A Mac App Store distribution profile for one bundle: right team, identifier and App Group, macOS, not
// tied to devices, not expired. Takes the profile already decoded from CMS and converted to JSON.
export function profileProblems(profile: Record<string, unknown>, bundle: string, config: HostConfiguration, now: Date): string[] {
  const problems: string[] = [], entitlements = (profile.Entitlements ?? {}) as Record<string, unknown>;
  const teams = profile.TeamIdentifier, platforms = profile.Platform, groups = entitlements[GROUPS];
  if (!Array.isArray(teams) || !teams.includes(config.signingTeam)) problems.push(`${bundle}: profile is for another team`);
  if (entitlements[APPLICATION] !== config.signingTeam + '.' + bundle) problems.push(`${bundle}: profile is for ${String(entitlements[APPLICATION])}`);
  if (!Array.isArray(groups) || !groups.includes(config.appGroup)) problems.push(`${bundle}: profile lacks App Group ${config.appGroup}`);
  if (!Array.isArray(platforms) || !platforms.includes('OSX')) problems.push(`${bundle}: profile is not a macOS profile`);
  if ('ProvisionedDevices' in profile || profile.ProvisionsAllDevices === true) problems.push(`${bundle}: profile is a development or Developer ID profile, not Mac App Store`);
  const expires = typeof profile.ExpirationDate === 'string' ? Date.parse(profile.ExpirationDate) : NaN;
  if (!(expires > now.getTime())) problems.push(`${bundle}: profile has expired`);
  return problems;
}
