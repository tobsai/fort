import assert from 'node:assert/strict';
import {verifySignedChecksums, verifyUbuntuBundle} from '../verify-inputs.mjs';
const fingerprint='D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81';
const digest='a'.repeat(64);
const input={publicKey:'synthetic public key fixture',signature:Buffer.from('synthetic signature'),
  sums:Buffer.from(digest+'  noble-server-cloudimg-arm64.tar.gz\n'),fingerprint,
  filename:'noble-server-cloudimg-arm64.tar.gz',pinnedSHA256:digest,actualSHA256:digest};
const cryptoBoundary={
  async readKeys(){return [{isPrivate:()=>false,getFingerprint:()=>fingerprint,
    async verifyPrimaryKey(){},async getSigningKey(){return {};}}];},
  async readSignature(){return {};}, async createMessage(value){return value;},
  async verify(){return {signatures:[{verified:Promise.resolve(true),keyID:'synthetic'}]};},
};
assert.equal((await verifySignedChecksums(cryptoBoundary,input)).filename,input.filename);
for (const changed of [{fingerprint:'0'+fingerprint.slice(1)},{filename:'wrong.tar.gz'},
  {sums:Buffer.from(digest+'  '+input.filename+'\n'+digest+'  '+input.filename+'\n')},
  {actualSHA256:'b'.repeat(64)}]) {
  await assert.rejects(()=>verifySignedChecksums(cryptoBoundary,{...input,...changed}));
}
await assert.rejects(()=>verifyUbuntuBundle(cryptoBoundary,{}));
console.log('6 verifier contract fixtures passed; OpenPGP/registry cryptography and actual inputs unrun.');
