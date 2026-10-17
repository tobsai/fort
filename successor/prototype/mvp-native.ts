import {existsSync,lstatSync,readdirSync,readFileSync,writeFileSync,renameSync} from 'node:fs';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';

export const MVP_NATIVE_DIRECTORY='/Users/tobiasgunn/Library/Application Support/FortCandidateB';
function assertNoMacApp(){
  try{execFileSync('/usr/bin/pgrep',['-x','FortMac'],{stdio:'ignore',timeout:1_000});}
  catch(error){if(error instanceof Error&&'status' in error&&error.status===1)return;throw new Error('FortMac process status is unknown');}
  throw new Error('Quit FortMac before preserving the native cache');
}
export function archiveMvpNativeCache(root:string,directory=MVP_NATIVE_DIRECTORY,assertAppExited=assertNoMacApp):void {
  assertAppExited();
  const archive=join(root,'native-cache-before');if(existsSync(archive))throw new Error('Native cache archive already exists');
  if(!existsSync(directory))return;
  const files:{path:string;bytes:number;sha256:string}[]=[];
  const walk=(path:string,relative:string)=>{
    const stat=lstatSync(path);if(stat.isSymbolicLink())throw new Error('Native cache contains a symlink');
    if(stat.isDirectory()){for(const name of readdirSync(path))walk(join(path,name),relative?join(relative,name):name);}
    else if(stat.isFile()){files.push({path:relative,bytes:stat.size,sha256:createHash('sha256').update(readFileSync(path)).digest('hex')});}
    else throw new Error('Unexpected native cache file type');
  };
  walk(directory,'');writeFileSync(join(root,'native-cache-before.json'),JSON.stringify({originalDirectory:directory,archive,files},null,2)+'\n',{flag:'wx',mode:0o600});
  renameSync(directory,archive);
  for(const file of files)if(createHash('sha256').update(readFileSync(join(archive,file.path))).digest('hex')!==file.sha256)throw new Error('Native cache archive verification failed');
}
