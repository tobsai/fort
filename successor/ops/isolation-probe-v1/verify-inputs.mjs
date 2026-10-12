// Public-input verification only. The approved library is imported only by main().
import { createHash, createPublicKey, verify as verifyECDSA } from 'node:crypto';
import { createReadStream, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const APP = '/Users/tobiasgunn/Library/Application Support/FortIsolation';
const INTEGRITY = 'sha512-wcZTzHz41LV8Y48zH/JlD1JT8YdNmpWOuMAjQ/podvvCwsjBEU37K3znJh4UQcj3hksE5z9TzvhbUbwzsGvlLQ==';
const REGISTRY_KEY = 'SHA256:DhQ8wR5APBvFHLF/+Tc+AYvPOdTpcIDqOhxsBHRwC7U';

export function verifyRegistryMetadata(metadata, keyDocument, now = new Date()) {
  if (metadata.name !== 'openpgp' || metadata.version !== '6.3.2' || metadata.dist.integrity !== INTEGRITY ||
      metadata.dist.tarball !== 'https://registry.npmjs.org/openpgp/-/openpgp-6.3.2.tgz' ||
      Object.keys(metadata.dependencies ?? {}).length || Object.keys(metadata.optionalDependencies ?? {}).length) {
    throw new Error('Unexpected verifier package identity/dependencies/integrity');
  }
  const signatures = metadata.dist.signatures;
  if (!Array.isArray(signatures) || signatures.length !== 2) throw new Error('Unexpected registry signatures');
  for (const signature of signatures) {
    const matching = keyDocument.keys.filter(key => key.keyid === REGISTRY_KEY && key.keyid === signature.keyid);
    if (matching.length !== 1) throw new Error('Missing or ambiguous registry key');
    const key = matching[0];
    if (key.expires !== null && !(Date.parse(key.expires) > now.getTime())) throw new Error('Expired registry key');
    const publicKey = createPublicKey({ key: Buffer.from(key.key, 'base64'), format: 'der', type: 'spki' });
    const jwk = publicKey.export({ format: 'jwk' });
    if (jwk.kty !== 'EC' || jwk.crv !== 'P-256') throw new Error('Unexpected registry key type');
    const field = value => {
      const length = Buffer.alloc(4);
      length.writeUInt32BE(value.length);
      return Buffer.concat([length, value]);
    };
    const point = Buffer.concat([Buffer.from([4]), Buffer.from(jwk.x, 'base64url'), Buffer.from(jwk.y, 'base64url')]);
    const sshKey = Buffer.concat([field(Buffer.from('ecdsa-sha2-nistp256')), field(Buffer.from('nistp256')), field(point)]);
    const fingerprint = 'SHA256:' + createHash('sha256').update(sshKey).digest('base64').replace(/=+$/, '');
    const message = `${metadata.name}@${metadata.version}:${metadata.dist.integrity}`;
    if (fingerprint !== REGISTRY_KEY || !verifyECDSA('sha256', Buffer.from(message), publicKey, Buffer.from(signature.sig, 'base64'))) {
      throw new Error('Registry signature/fingerprint mismatch');
    }
  }
  return { registrySignaturesVerified: signatures.length, tarballVerified: false };
}

export async function verifySignedChecksums(openpgp, input, now = new Date()) {
  const keys = await openpgp.readKeys({ armoredKeys: input.publicKey });
  if (keys.length !== 1 || keys[0].isPrivate() || keys[0].getFingerprint().toUpperCase() !== input.fingerprint) {
    throw new Error('Publisher primary fingerprint/private-key mismatch');
  }
  const key = keys[0];
  await key.verifyPrimaryKey(now);
  const signatureText = Buffer.from(input.signature).toString('utf8');
  const signature = await openpgp.readSignature(signatureText.startsWith('-----BEGIN PGP SIGNATURE-----')
    ? { armoredSignature: signatureText } : { binarySignature: new Uint8Array(input.signature) });
  const result = await openpgp.verify({
    message: await openpgp.createMessage({ binary: new Uint8Array(input.sums) }),
    signature, verificationKeys: key, expectSigned: true, format: 'binary', date: now,
  });
  if (!Array.isArray(result.signatures) || result.signatures.length === 0) throw new Error('Missing detached signature');
  await Promise.all(result.signatures.map(async entry => {
    if (await entry.verified !== true) throw new Error('Unresolved detached signature');
    await key.getSigningKey(entry.keyID, now);
  }));
  const entries = Buffer.from(input.sums).toString('utf8').split('\n')
    .map(line => /^([a-f0-9]{64}) [ *](.+)$/.exec(line))
    .filter(match => match && match[2] === input.filename);
  if (entries.length !== 1) throw new Error('Missing or ambiguous exact signed entry');
  if (entries[0][1] !== input.pinnedSHA256 || input.actualSHA256 !== input.pinnedSHA256) {
    throw new Error('Signed entry, packet pin and payload digest disagree');
  }
  return { filename: input.filename, sha256: input.actualSHA256, primaryFingerprint: input.fingerprint,
    signaturesVerified: result.signatures.length };
}

async function hashFile(path, algorithm) {
  const hash = createHash(algorithm);
  for await (const chunk of createReadStream(path)) hash.update(chunk);
  return hash.digest('hex');
}

async function mustReject(operation, label) {
  try { await operation(); } catch { return; }
  throw new Error('Negative verification control unexpectedly accepted: ' + label);
}

async function main() {
  const registryOnly = process.argv.length === 3 && process.argv[2] === '--registry-only';
  if (process.version !== 'v26.5.0' || (process.argv.length !== 2 && !registryOnly)) throw new Error('Exact existing Node and reviewed mode required');
  const scope = JSON.parse(readFileSync(new URL('./scope.json', import.meta.url)));
  const downloads = APP + '/downloads/';
  for (const entry of scope.downloads) {
    const algorithm = entry.sha512 ? 'sha512' : 'sha256';
    if (await hashFile(downloads + entry.name, algorithm) !== entry[algorithm]) throw new Error('Download digest mismatch: ' + entry.name);
  }
  const metadata = JSON.parse(readFileSync(downloads + 'openpgp-6.3.2-metadata.json'));
  const registry = verifyRegistryMetadata(metadata, JSON.parse(readFileSync(downloads + 'npm-public-signing-keys.json')));
  if (registryOnly) {
    console.log(JSON.stringify({ registry, verifierArchiveVerified: true, openpgpLibraryImported: false,
      publisherSignaturesVerified: false }, null, 2));
    return;
  }
  const install = APP + '/tools/openpgp/6.3.2/package/';
  const installed = JSON.parse(readFileSync(install + 'package.json'));
  if (installed.name !== 'openpgp' || installed.version !== '6.3.2' ||
      Object.keys(installed.dependencies ?? {}).length || Object.keys(installed.optionalDependencies ?? {}).length ||
      installed.exports['.'].import !== './dist/node/openpgp.mjs') throw new Error('Unexpected extracted package');
  // Archive integrity and inspected extraction precede this import; no package hooks run.
  const openpgp = await import(pathToFileURL(install + 'dist/node/openpgp.mjs').href);
  const releases = [
    { prefix: 'lima', filename: 'lima-2.2.1-Darwin-arm64.tar.gz', other: 'ubuntu' },
    { prefix: 'ubuntu', filename: 'noble-server-cloudimg-arm64.img', other: 'lima' },
  ];
  const receipts = [];
  for (const release of releases) {
    const pin = scope.downloads.find(entry => entry.name === release.filename);
    const input = { publicKey: readFileSync(downloads + release.prefix + '.pub', 'utf8'),
      sums: readFileSync(downloads + release.prefix + '.sums'), signature: readFileSync(downloads + release.prefix + '.sig'),
      fingerprint: scope.signing_roots[release.prefix + '_primary_fingerprint'], filename: release.filename,
      pinnedSHA256: pin.sha256, actualSHA256: await hashFile(downloads + release.filename, 'sha256') };
    const receipt = await verifySignedChecksums(openpgp, input);
    await mustReject(() => verifySignedChecksums(openpgp, { ...input, sums: Buffer.concat([input.sums, Buffer.from('changed')]) }), 'changed checksum bytes');
    const damaged = Buffer.from(input.signature);
    damaged[Math.floor(damaged.length / 2)] ^= 1;
    await mustReject(() => verifySignedChecksums(openpgp, { ...input, signature: damaged }), 'changed signature bytes');
    await mustReject(() => verifySignedChecksums(openpgp, { ...input, publicKey: readFileSync(downloads + release.other + '.pub', 'utf8') }), 'unrelated public key');
    receipts.push({ ...receipt, negativeControlsPassed: 3 });
  }
  console.log(JSON.stringify({ registry, verifierArchiveVerified: true, publisherSignaturesVerified: true, receipts }, null, 2));
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const timeout = setTimeout(() => { console.error('Verification deadline exceeded'); process.exit(1); }, 90000);
  try { await main(); } finally { clearTimeout(timeout); }
}
