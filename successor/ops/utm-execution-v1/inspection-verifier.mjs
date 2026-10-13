// Disabled R7 verifier preparation. Review and exact owner authority precede activation.
import {createHash} from 'node:crypto';
import {openSync,closeSync,fstatSync,lstatSync,readSync,readFileSync,readdirSync,constants} from 'node:fs';
import {resolve,join} from 'node:path';
import {pathToFileURL,fileURLToPath} from 'node:url';
import {verifyRegistryMetadata,verifySignedChecksums} from './verify-inputs.mjs';
export const INSPECTION_ENABLED = false;
const pinsPath=new URL('../utm-synthetic-packet-v1/inputs.json',import.meta.url);

export function hashRegular(path,algorithm,cap,expectedSize=null,capture=false){
  const fd=openSync(path,constants.O_RDONLY|constants.O_NOFOLLOW|constants.O_NONBLOCK);
  try{
    const before=fstatSync(fd,{bigint:true});
    if(!before.isFile()||before.nlink!==1n||before.size>BigInt(cap)||(expectedSize!==null&&before.size!==BigInt(expectedSize)))throw Error('regular input type/size/link mismatch');
    const digest=createHash(algorithm),chunk=Buffer.alloc(65536),parts=[];let size=0,count;
    while((count=readSync(fd,chunk,0,chunk.length,null))!==0){size+=count;if(size>cap)throw Error('file grew beyond cap');digest.update(chunk.subarray(0,count));if(capture)parts.push(Buffer.from(chunk.subarray(0,count)));}
    const same=other=>['dev','ino','size','mtimeNs','ctimeNs'].every(k=>before[k]===other[k]);
    if(!same(fstatSync(fd,{bigint:true}))||!same(lstatSync(path,{bigint:true})))throw Error('input changed during hashing');
    return {bytes:size,[algorithm]:digest.digest('hex'),...(capture?{data:Buffer.concat(parts)}:{})};
  }finally{closeSync(fd);}
}
function small(path,cap=65536){return hashRegular(path,'sha256',cap,null,true).data;}
export function selectedPackage(root,selection){
  if(selection.entry!=='package/dist/node/openpgp.mjs'||!selection.manifest||typeof selection.manifest!=='object')throw Error('selected verifier entry missing');
  let total=0;const names=Object.keys(selection.manifest);
  if(names.length>256||!names.includes('package/package.json')||!names.includes('package/LICENSE')||!names.includes(selection.entry))throw Error('verifier member closure incomplete');
  const observed=[];let directories=0;
  const walk=(directory,prefix='')=>{
    const info=lstatSync(directory);if(!info.isDirectory()||info.isSymbolicLink()||++directories>256)throw Error('verifier directory closure differs');
    for(const entry of readdirSync(directory,{withFileTypes:true})){
      const name=prefix+entry.name;if(entry.isDirectory())walk(join(directory,entry.name),name+'/');
      else if(entry.isFile())observed.push(name);else throw Error('verifier links/special objects forbidden');
      if(observed.length>256)throw Error('verifier file count');
    }
  };
  walk(root);if(observed.sort().join('\n')!==[...names].sort().join('\n'))throw Error('verifier member closure changed');
  for(const name of names){
    if(!name.startsWith('package/')||name.split('/').some(p=>!p||p==='.'||p==='..')||name.includes('\\')||name.includes('\0'))throw Error('verifier path escape');
    const facts=selection.manifest[name];total+=facts.bytes;
    if(!Number.isSafeInteger(facts.bytes)||facts.bytes<0||total>25165824||!/^([a-f0-9]{64})$/.test(facts.sha256))throw Error('verifier member bounds');
    if(hashRegular(join(root,name),'sha256',25165824,facts.bytes).sha256!==facts.sha256)throw Error('selected module/member digest differs');
  }
  const info=JSON.parse(small(join(root,'package/package.json')));
  if(info.name!=='openpgp'||info.version!=='6.3.2'||info.exports?.['.']?.import!=='./dist/node/openpgp.mjs'||Object.keys(info.dependencies??{}).length||Object.keys(info.optionalDependencies??{}).length)throw Error('extracted package identity differs');
  return pathToFileURL(join(root,selection.entry));
}
export async function negativeControls(openpgp,input){
  const flip=bytes=>{const copy=Buffer.from(bytes);if(!copy.length)throw Error('empty negative-control input');copy[0]^=1;return copy;};
  const controls=[{sums:flip(input.sums)},{signature:flip(input.signature)},
    {publicKey:flip(Buffer.from(input.publicKey)).toString('utf8')},{fingerprint:(input.fingerprint[0]==='0'?'1':'0')+input.fingerprint.slice(1)}];
  for(const change of controls){let rejected=false;try{await verifySignedChecksums(openpgp,{...input,...change});}catch{rejected=true;}if(!rejected)throw Error('publisher negative control unexpectedly accepted');}
  return {rejected:4};
}
async function main(){
  if(!INSPECTION_ENABLED){process.stderr.write('Input inspection disabled.\n');process.exitCode=78;return;}
  const [operation,root,selectionSHA]=process.argv.slice(2);
  if(!['registry','publisher'].includes(operation)||!root||resolve(root)!==root)throw Error('fixed inspection operation and absolute root required');
  const pins=JSON.parse(readFileSync(pinsPath));
  const entries=pins.downloads.filter(e=>e.name!=='UTM.dmg');if(entries.length!==8)throw Error('exact eight inputs required');
  const dir=join(root,'downloads');
  if(!JSON.parse(small(join(dir,'complete.json'),8192)).inputs||lstatSync(dir).isSymbolicLink())throw Error('acquisition incomplete');
  try{lstatSync(join(dir,'failure.json'));throw Error('acquisition failure veto');}catch(error){if(error.code!=='ENOENT')throw error;}
  const identities={};
  for(const e of entries){const algorithm=e.sha512?'sha512':'sha256';const facts=hashRegular(join(dir,e.name),algorithm,e.max_bytes);if(facts[algorithm]!==e[algorithm])throw Error('pinned input changed');identities[e.name]=facts;}
  const metadata=JSON.parse(small(join(dir,'openpgp-6.3.2-metadata.json')));
  const registry=verifyRegistryMetadata(metadata,JSON.parse(small(join(dir,'npm-public-signing-keys.json'))));
  let details={registry_signatures:registry.registrySignaturesVerified,archive_sha512:identities['openpgp-6.3.2.tgz'].sha512};
  if(operation==='publisher'){
    const selectionBytes=small(join(root,'verifier-selection.json'),65536);
    if(!/^[a-f0-9]{64}$/.test(selectionSHA??'')||createHash('sha256').update(selectionBytes).digest('hex')!==selectionSHA)throw Error('selected module manifest changed after inspection');
    const selection=JSON.parse(selectionBytes);
    if(selection.archive_sha512!==details.archive_sha512)throw Error('selected module archive provenance differs');
    const moduleURL=selectedPackage(join(root,'verifier'),selection);
    // No package manager, lifecycle hook, dependency install or auto-import precedes this line.
    const openpgp=await import(moduleURL.href);
    const archive=entries.find(e=>e.name==='noble-server-cloudimg-arm64.tar.gz');
    const input={publicKey:small(join(dir,'ubuntu.pub')).toString('utf8'),signature:small(join(dir,'ubuntu.sig')),
      sums:small(join(dir,'ubuntu.sums')),fingerprint:pins.ubuntu_publisher_primary_fingerprint,
      filename:archive.name,pinnedSHA256:archive.sha256,actualSHA256:identities[archive.name].sha256};
    details={publisher:await verifySignedChecksums(openpgp,input),negative_controls:await negativeControls(openpgp,input),
      module_sha256:selection.manifest[selection.entry].sha256};
  }
  process.stdout.write(JSON.stringify({operation,status:'complete',details})+'\n');
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url)){
  main().catch(()=>{process.stderr.write('Offline inspection failed; no acceptance.\n');process.exitCode=1;});
}
