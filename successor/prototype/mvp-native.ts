import {existsSync,lstatSync,readdirSync,readFileSync,writeFileSync,renameSync,realpathSync} from 'node:fs';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {userInfo} from 'node:os';
import {fileURLToPath} from 'node:url';
import {createNativeSession} from './native-session.ts';

export type MvpApplication={pid:number;bundleIdentifier:string;bundlePath:string;executablePath:string};
type AppIdentity={bundleIdentifier:string;appPath:string;executable:string};
type NativeEnvironment={homeDirectory?:string;readPlist?:(path:string,key:string)=>string;control?:(action:'status'|'stop',identity:AppIdentity)=>MvpApplication[]};
export function createMvpNative(appPath:string,environment:NativeEnvironment={}){
  const readPlist=environment.readPlist??((path,key)=>execFileSync('/usr/bin/plutil',['-extract',key,'raw',path],{encoding:'utf8',timeout:1_000,stdio:['ignore','pipe','pipe']}).trim());
  const canonicalApp=realpathSync(appPath),plist=join(canonicalApp,'Contents/Info.plist');
  const bundleIdentifier=readPlist(plist,'CFBundleIdentifier'),executableName=readPlist(plist,'CFBundleExecutable');
  if(bundleIdentifier!=='io.fort.candidateb.mac'||executableName!=='FortMac')throw new Error('Unexpected native app identity');
  const container=join(environment.homeDirectory??userInfo().homedir,'Library/Containers',bundleIdentifier);
  if(readPlist(join(container,'.com.apple.containermanagerd.metadata.plist'),'MCMMetadataIdentifier')!==bundleIdentifier)throw new Error('Native container identity is unconfirmed');
  const directory=join(container,'Data/Library/Application Support/FortCandidateB');
  const identity={bundleIdentifier,appPath:canonicalApp,executable:realpathSync(join(canonicalApp,'Contents/MacOS',executableName))};
  const control=environment.control??((action,expected)=>JSON.parse(execFileSync('/usr/bin/osascript',['-l','JavaScript',fileURLToPath(new URL('./mvp-app.jxa',import.meta.url)),action,expected.bundleIdentifier,expected.appPath,expected.executable],{encoding:'utf8',timeout:1_500,stdio:['ignore','pipe','pipe']})) as MvpApplication[]);
  const appExited=()=>{
    const apps=control('status',identity);
    if(!Array.isArray(apps)||apps.some(app=>!Number.isSafeInteger(app.pid)||app.pid<=0||app.bundleIdentifier!==identity.bundleIdentifier||realpathSync(app.bundlePath)!==identity.appPath||realpathSync(app.executablePath)!==identity.executable))throw new Error('A different app has the native bundle identity');
    return apps.length===0;
  };
  return {...identity,directory,assertExited(){if(!appExited())throw new Error('The exact native app is still running');},
    session:createNativeSession({directory,appExecutable:identity.executable,appExited,stopApp:async()=>{control('stop',identity);}})};
}
export function archiveMvpNativeCache(root:string,directory:string,assertAppExited:()=>void):void {
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
