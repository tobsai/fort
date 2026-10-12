import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { verifyRegistryMetadata, verifySignedChecksums } from './verify-inputs.mjs';

const metadataRoot = new URL('../../evidence/provisioning-corrections-2026-10-12/', import.meta.url);
const metadata = JSON.parse(readFileSync(new URL('openpgp-6.3.2-metadata.json', metadataRoot)));
const registryKeys = JSON.parse(readFileSync(new URL('npm-public-signing-keys.json', metadataRoot)));
const fingerprint = 'C020EA876CE4E06C7AB95AEF49524C6F9F638F1A';
const hash = 'a'.repeat(64);
const original = Buffer.from(`${hash}  artifact.img\n`);
const signatureBytes = Buffer.from('mock detached signature, not cryptographic evidence');
const inputs = { publicKey: 'mock public key', sums: original, signature: signatureBytes,
  fingerprint, filename: 'artifact.img', pinnedSHA256: hash, actualSHA256: hash };

// A contract double exercises wrapper control flow only. No OpenPGP library is loaded.
function backend(overrides = {}) {
  const key = {
    isPrivate: () => false,
    getFingerprint: () => fingerprint.toLowerCase(),
    verifyPrimaryKey: async () => {},
    getSigningKey: async () => ({}),
    ...overrides.key,
  };
  return {
    readKeys: async () => [key],
    readSignature: async value => value.binarySignature ?? Buffer.from(value.armoredSignature),
    createMessage: async value => value.binary,
    verify: async options => {
      assert.equal(options.expectSigned, true);
      assert.equal(options.format, 'binary');
      assert.equal(options.config, undefined, 'Do not weaken library policy');
      const valid = Buffer.from(options.message).equals(original) && Buffer.from(options.signature).equals(signatureBytes);
      return { signatures: [{ keyID: 'mock signing key', verified: valid ? Promise.resolve(true) : Promise.reject(new Error('bad signature')) }] };
    },
    ...overrides.api,
  };
}

test('real public npm metadata signatures verify with existing Node crypto', () => {
  const result = verifyRegistryMetadata(metadata, registryKeys);
  assert.equal(result.registrySignaturesVerified, 2);
  assert.equal(result.tarballVerified, false);
});

test('changed registry integrity is rejected', () => {
  const changed = structuredClone(metadata);
  changed.dist.integrity += 'changed';
  assert.throws(() => verifyRegistryMetadata(changed, registryKeys));
});

test('changed registry signature is rejected', () => {
  const changed = structuredClone(metadata);
  changed.dist.signatures[0].sig = Buffer.from('tampered').toString('base64');
  assert.throws(() => verifyRegistryMetadata(changed, registryKeys));
});

test('wrong registry key is rejected', () => {
  assert.throws(() => verifyRegistryMetadata(metadata, { keys: [] }));
});

test('wrapper awaits detached verification and accepts the exact signed entry', async () => {
  const result = await verifySignedChecksums(backend(), inputs);
  assert.equal(result.filename, 'artifact.img');
  assert.equal(result.sha256, hash);
});

test('changed checksum bytes fail detached verification', async () => {
  await assert.rejects(verifySignedChecksums(backend(), { ...inputs, sums: Buffer.concat([original, Buffer.from('x')]) }), /signature/);
});

test('changed signature bytes fail detached verification', async () => {
  await assert.rejects(verifySignedChecksums(backend(), { ...inputs, signature: Buffer.from('changed') }), /signature/);
});

test('unrelated primary key is rejected before signature verification', async () => {
  await assert.rejects(verifySignedChecksums(backend({ key: { getFingerprint: () => 'b'.repeat(40) } }), inputs), /fingerprint/);
});

test('expired or revoked primary key is rejected', async () => {
  for (const reason of ['expired', 'revoked']) {
    await assert.rejects(verifySignedChecksums(backend({ key: { verifyPrimaryKey: async () => { throw new Error(reason); } } }), inputs), new RegExp(reason));
  }
});

test('invalid current signing subkey is rejected', async () => {
  await assert.rejects(verifySignedChecksums(backend({ key: { getSigningKey: async () => { throw new Error('expired subkey'); } } }), inputs), /expired subkey/);
});

test('absent or unresolved signatures are rejected', async () => {
  await assert.rejects(verifySignedChecksums(backend({ api: { verify: async () => ({ signatures: [] }) } }), inputs), /signature/);
  await assert.rejects(verifySignedChecksums(backend({ api: { verify: async () => ({ signatures: [{ verified: Promise.resolve(false) }] }) } }), inputs), /signature/);
});

test('one valid signature cannot hide another rejected signature', async () => {
  const api = { verify: async () => ({ signatures: [
    { keyID: 'valid mock', verified: Promise.resolve(true) },
    { keyID: 'invalid mock', verified: Promise.reject(new Error('unresolved signature')) },
  ] }) };
  await assert.rejects(verifySignedChecksums(backend({ api }), inputs), /unresolved signature/);
});

test('wrong file, wrong payload hash and duplicate signed entries are rejected', async () => {
  await assert.rejects(verifySignedChecksums(backend(), { ...inputs, filename: 'other.img' }), /entry/);
  await assert.rejects(verifySignedChecksums(backend(), { ...inputs, actualSHA256: 'b'.repeat(64) }), /digest/);
  const duplicated = Buffer.concat([original, original]);
  const api = { verify: async () => ({ signatures: [{ keyID: 'mock', verified: Promise.resolve(true) }] }) };
  await assert.rejects(verifySignedChecksums(backend({ api }), { ...inputs, sums: duplicated }), /entry/);
});
