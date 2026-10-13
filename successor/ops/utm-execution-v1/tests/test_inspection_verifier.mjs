import assert from 'node:assert/strict';
import {mkdtempSync,writeFileSync,mkdirSync,rmSync,symlinkSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {selectedPackage,negativeControls,INSPECTION_ENABLED} from '../inspection-verifier.mjs';
let passed=0;
assert.equal(INSPECTION_ENABLED,true);passed++;
const root=mkdtempSync(join(tmpdir(),'fort-inspection-fixture-'));
try{
  mkdirSync(join(root,'package/dist/node'),{recursive:true});
  const files={'package/package.json':Buffer.from(JSON.stringify({name:'openpgp',version:'6.3.2',exports:{'.':{import:'./dist/node/openpgp.mjs'}}})),
    'package/dist/node/openpgp.mjs':Buffer.from('export const fixture=true;'),'package/LICENSE':Buffer.from('Synthetic fixture')};
  const selection={entry:'package/dist/node/openpgp.mjs',manifest:{}};
  for(const [name,data] of Object.entries(files)){writeFileSync(join(root,name),data);selection.manifest[name]={bytes:data.length,sha256:createHash('sha256').update(data).digest('hex')};}
  assert.equal(selectedPackage(root,selection).protocol,'file:');passed++;
  assert.throws(()=>selectedPackage(root,{...selection,entry:'../outside'}));passed++;
  writeFileSync(join(root,selection.entry),'changed');assert.throws(()=>selectedPackage(root,selection));passed++;
  rmSync(join(root,selection.entry));symlinkSync(join(root,'package/LICENSE'),join(root,selection.entry));assert.throws(()=>selectedPackage(root,selection));passed++;
}finally{rmSync(root,{recursive:true});}
const fingerprint='D2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81';const digest='a'.repeat(64);
const input={publicKey:'synthetic-public',signature:Buffer.from('synthetic-signature'),sums:Buffer.from(digest+'  image\n'),
  fingerprint,filename:'image',pinnedSHA256:digest,actualSHA256:digest};
const boundary={async readKeys({armoredKeys}){if(armoredKeys!==input.publicKey)throw Error('fixture key changed');return [{isPrivate:()=>false,getFingerprint:()=>fingerprint,async verifyPrimaryKey(){},async getSigningKey(){}}];},
  async readSignature({binarySignature}){if(!Buffer.from(binarySignature).equals(input.signature))throw Error('fixture signature changed');return {};},
  async createMessage({binary}){if(!Buffer.from(binary).equals(input.sums))throw Error('fixture message changed');return {};},
  async verify(){return {signatures:[{verified:Promise.resolve(true),keyID:'fixture'}]};}};
assert.deepEqual(await negativeControls(boundary,input),{rejected:4});passed++;
console.log(`${passed} inspection-verifier fixtures passed; no downloaded module or real publisher cryptography executed.`);
