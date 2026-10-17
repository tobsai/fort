import {createHash} from 'node:crypto';
import {readFileSync,lstatSync,existsSync,readdirSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import {join,normalize,isAbsolute} from 'node:path';

export const MVP_ROOT='/Users/tobiasgunn/dev/fort/.local/mvp-native-tests-20261017';
export const MVP_OPERATOR='01a1451b-81d7-76cf-8e5c-a85d7164201c';
export const MVP_SCOPE='mvp-native-tests-20261017';
export const REPOSITORY=fileURLToPath(new URL('../..',import.meta.url));
export const PLAN_DIRECTORY=join(REPOSITORY,'successor/ops/mvp-native-acceptance-20261017');
export const APPROVAL_FILE=join(PLAN_DIRECTORY,'owner-approval.json');
const object=(value:unknown):Record<string,unknown>=>{if(!value||typeof value!=='object'||Array.isArray(value))throw new Error('Invalid preparation record');return value as Record<string,unknown>;};
const hash=(bytes:Uint8Array)=>createHash('sha256').update(bytes).digest('hex');
export function validateApproval(value:unknown,scopeSHA256:string,now=Date.now()):void {
  const v=object(value);
  if(v.schemaVersion!==1||v.decision!=='approved'||v.actor!=='owner'||v.scopeId!==MVP_SCOPE||v.scopeSHA256!==scopeSHA256||v.operatorThreadId!==MVP_OPERATOR||v.sourceThreadId!=='01a0f307-3940-7333-96b0-a3b0b854e0e8'||typeof v.approvedAtUTC!=='string'||!Number.isFinite(Date.parse(v.approvedAtUTC))||Date.parse(v.approvedAtUTC)>now)throw new Error('Explicit owner approval for this unchanged scope is required');
}
export function verifyMvpPreparation(){
  if(process.version!=='v26.5.0')throw new Error('Use the pinned Node 26.5.0 runtime');
  const pins=object(JSON.parse(readFileSync(join(PLAN_DIRECTORY,'pins.json'),'utf8')));
  const scopeBytes=readFileSync(join(REPOSITORY,'successor/ops/mvp-native-test-scope-20261017.json')),scopeSHA256=hash(scopeBytes);
  if(pins.scopeSHA256!==scopeSHA256||typeof pins.sourceCommit!=='string'||!/^[a-f0-9]{40}$/.test(pins.sourceCommit)||typeof pins.appPath!=='string'||!isAbsolute(pins.appPath))throw new Error('Candidate scope or source pin changed');
  for(const group of [{base:REPOSITORY,files:pins.sourceFiles},{base:pins.appPath,files:pins.appFiles}]){
    if(!Array.isArray(group.files)||group.files.length===0)throw new Error('Candidate file pins are missing');
    for(const item of group.files){const file=object(item);if(typeof file.path!=='string'||isAbsolute(file.path)||normalize(file.path)!==file.path||file.path.startsWith('..')||typeof file.sha256!=='string')throw new Error('Invalid pinned path');const path=join(group.base,file.path);if(!lstatSync(path).isFile()||hash(readFileSync(path))!==file.sha256)throw new Error('Candidate bytes differ from the prepared pins');}
  }
  const appFiles:string[]=[];
  const visit=(relative:string)=>{for(const name of readdirSync(join(pins.appPath as string,relative))){const path=join(relative,name),stat=lstatSync(join(pins.appPath as string,path));if(stat.isSymbolicLink())throw new Error('Unexpected app symlink');if(stat.isDirectory())visit(path);else if(stat.isFile())appFiles.push(path);else throw new Error('Unexpected app file type');}};visit('');
  if(JSON.stringify(appFiles.sort())!==JSON.stringify((pins.appFiles as {path:string}[]).map(file=>file.path).sort()))throw new Error('App file set changed');
  if(!Array.isArray(pins.preservedFiles)||pins.preservedFiles.length===0)throw new Error('Historical preservation pins are missing');
  for(const item of pins.preservedFiles){const file=object(item);if(typeof file.path!=='string'||!isAbsolute(file.path)||!lstatSync(file.path).isFile()||hash(readFileSync(file.path))!==file.sha256)throw new Error('Historical packet bytes changed');}
  const provider=object(object(JSON.parse(scopeBytes.toString('utf8'))).provider);
  if(typeof provider.runtimePath!=='string'||hash(readFileSync(provider.runtimePath))!==provider.runtimeSHA256)throw new Error('Official runtime pin changed');
  return {sourceCommit:pins.sourceCommit,appPath:pins.appPath,scopeSHA256,approvalPresent:existsSync(APPROVAL_FILE)};
}
export function requireMvpApproval(prepared:ReturnType<typeof verifyMvpPreparation>,approvalFile=APPROVAL_FILE):void {
  if(!existsSync(approvalFile)||!lstatSync(approvalFile).isFile())throw new Error('Owner approval is pending. No session may start.');
  validateApproval(JSON.parse(readFileSync(approvalFile,'utf8')),prepared.scopeSHA256);
}
