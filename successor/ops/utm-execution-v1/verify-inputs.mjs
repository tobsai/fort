// Fort synthetic-input verification. Adapted from the earlier reviewed Fort verifier.
// No entry point, downloads, package hooks, or automatic library import.
import { createHash, createPublicKey, verify as verifyECDSA } from 'node:crypto';
import { readFileSync } from 'node:fs';
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


export async function verifyUbuntuBundle(openpgp, files, now = new Date()) {
  const pins = JSON.parse(readFileSync(new URL('../utm-synthetic-packet-v1/inputs.json', import.meta.url)));
  const entries = pins.downloads.filter(entry => entry.name !== 'UTM.dmg');
  if (Object.keys(files).sort().join('\n') !== entries.map(entry => entry.name).sort().join('\n')) {
    throw new Error('Exactly the eight pinned Ubuntu/verifier inputs are required');
  }
  for (const entry of entries) {
    const bytes = files[entry.name];
    const algorithm = entry.sha512 ? 'sha512' : 'sha256';
    if (!Buffer.isBuffer(bytes) || bytes.length > entry.max_bytes ||
        createHash(algorithm).update(bytes).digest('hex') !== entry[algorithm]) {
      throw new Error('Input cap or digest mismatch: ' + entry.name);
    }
  }
  verifyRegistryMetadata(JSON.parse(files['openpgp-6.3.2-metadata.json']), JSON.parse(files['npm-public-signing-keys.json']), now);
  const archive = entries.find(entry => entry.name === 'noble-server-cloudimg-arm64.tar.gz');
  return verifySignedChecksums(openpgp, {
    publicKey: files['ubuntu.pub'].toString('utf8'), signature: files['ubuntu.sig'], sums: files['ubuntu.sums'],
    fingerprint: pins.ubuntu_publisher_primary_fingerprint, filename: archive.name,
    pinnedSHA256: archive.sha256, actualSHA256: createHash('sha256').update(files[archive.name]).digest('hex'),
  }, now);
}
