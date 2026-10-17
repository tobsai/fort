import {lstatSync,mkdirSync,mkdtempSync,writeFileSync,unlinkSync,rmdirSync} from 'node:fs';
import {isAbsolute,join,normalize,dirname} from 'node:path';
export type OwnerDemoStage='preparation'|'approval'|'native-identity'|'write-preflight'|'native-exit'|'session-start'|'native-open'|'session-command';
function absent(path:string):boolean {
  try{lstatSync(path);return false;}catch(error){if(error instanceof Error&&'code' in error&&error.code==='ENOENT')return true;throw error;}
}
function inspectPath(root:string):void {
  if(!isAbsolute(root)||normalize(root)!==root||root==='/'||root.includes('\0'))throw new Error('Use the exact session root');
  for(let path=root;path!=='/';path=dirname(path))if(!absent(path)&&(!lstatSync(path).isDirectory()||lstatSync(path).isSymbolicLink()))throw new Error('Session path is not a plain directory');
}
export function inspectOwnerDemoAttempt(root:string):'not-created'|'created-or-unconfirmed' {
  try{inspectPath(root);return absent(join(root,'session-1'))&&absent(join(root,'active.lock'))?'not-created':'created-or-unconfirmed';}
  catch{return 'created-or-unconfirmed';}
}
export function describeOwnerDemoFailure(error:unknown,stage:OwnerDemoStage,root:string){
  const raw=error instanceof Error&&'code' in error?error.code:undefined;
  const code=typeof raw==='string'&&['EACCES','EPERM','ENOENT','EEXIST','ENOTDIR','ENOSPC','EROFS'].includes(raw)?raw:'unclassified';
  const attempt=inspectOwnerDemoAttempt(root),notCreated=attempt==='not-created',permissionFailure=code==='EACCES'||code==='EPERM';
  return {phase:'refused',stage,code,attempt,clockStarted:notCreated?false:'unconfirmed',retrySameApprovedAction:notCreated&&permissionFailure,
    nextAction:!notCreated?'Preserve the session directory and lock. Inspect redacted state and cleanup; do not retry or reset.':permissionFailure?'No session was created. Use supported exec_command require_escalated for --check-writes and then the same attended launcher under the existing approval.':'No session was created. Check the named stage and prepared pins before retrying the same approved action. No new credential approval is needed for a pre-start repair.'};
}
export class OwnerDemoStartupFailure extends Error {
  readonly report:ReturnType<typeof describeOwnerDemoFailure>;
  constructor(error:unknown,stage:OwnerDemoStage,root:string){super('Owner demo startup failed');this.report=describeOwnerDemoFailure(error,stage,root);}
}
export function checkOwnerDemoWrites(root:string):void {
  inspectPath(root);
  if(inspectOwnerDemoAttempt(root)!=='not-created')throw new Error('Session start is consumed or unconfirmed');
  const created=absent(root);if(created)mkdirSync(root,{mode:0o700});
  let probe:string|undefined,written=false;
  try{probe=mkdtempSync(join(root,'.write-check-'));writeFileSync(join(probe,'probe'),'Fort filesystem preflight\n',{flag:'wx',mode:0o600});written=true;}
  finally{if(probe){if(written)unlinkSync(join(probe,'probe'));rmdirSync(probe);}if(created)rmdirSync(root);}
}
