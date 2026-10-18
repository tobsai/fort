import { constants, openSync, closeSync, fstatSync, lstatSync, readFileSync, readdirSync, writeFileSync, mkdirSync, rmSync, realpathSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { join, dirname, basename, isAbsolute, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { parseArgs } from 'node:util';

type Entry = { path: string; source: string; dev: number; ino: number; mode: number; bytes: number; sha256: string };
const hash = (bytes: Uint8Array) => createHash('sha256').update(bytes).digest('hex');
const codexHash = '27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d';
const legalHashes = { LICENSE:'d17f227e4df5da1600391338865ce0f3055211760a36688f816941d58232d8dc', NOTICE:'9d71575ecfd9a843fc1677b0efb08053c6ba9fd686a0de1a6f5382fd3c220915' };
const tool = (path:string, args:string[]) => execFileSync(path,args,{env:{},encoding:'utf8',timeout:20_000,stdio:'pipe'});
function exactPath(path:string): string {
  if (!isAbsolute(path) || path === '/' || normalize(path) !== path || !path.isWellFormed() || path.includes('\0')) throw new Error('Use an exact absolute path');
  return path;
}
function relative(path:unknown): string {
  if (typeof path !== 'string' || !path || !path.isWellFormed() || path.includes('\0') || path.includes('\\') || isAbsolute(path) || normalize(path)!==path || path.split('/').some(p=>!p||p==='.'||p==='..')) throw new Error('Unsafe artifact path');
  return path;
}
function regular(source:string): Entry {
  const info=lstatSync(source);
  if (!info.isFile() || info.nlink!==1 || info.mode&0o022 || info.size>512*1024*1024) throw new Error('Unsafe input file');
  const fd=openSync(source,constants.O_RDONLY|constants.O_NOFOLLOW|constants.O_NONBLOCK);
  try {
    const opened=fstatSync(fd);
    if (!opened.isFile() || opened.dev!==info.dev || opened.ino!==info.ino || opened.size!==info.size) throw new Error('Input changed');
    const bytes=readFileSync(fd),after=fstatSync(fd);
    if (bytes.length!==info.size || after.mtimeMs!==opened.mtimeMs || after.ctimeMs!==opened.ctimeMs) throw new Error('Input changed');
    return {path:'',source,dev:info.dev,ino:info.ino,mode:info.mode&0o777,bytes:bytes.length,sha256:hash(bytes)};
  } finally { closeSync(fd); }
}
function inventory(root:string): Entry[] {
  exactPath(root);
  const files:Entry[]=[];
  function visit(directory:string,prefix=''):void {
    const info=lstatSync(directory);
    if (!info.isDirectory() || info.mode&0o022) throw new Error('Unsafe input directory');
    for (const name of readdirSync(directory).sort()) {
      relative(name);
      const path=prefix ? prefix+'/'+name : name, source=join(directory,name),info=lstatSync(source);
      if (info.isDirectory()) visit(source,path);
      else files.push({...regular(source),path});
      if (files.length>4096) throw new Error('Artifact exceeds file limit');
    }
  }
  visit(root);return files;
}
function bytes(entry:Entry):Buffer {
  const current=regular(entry.source);
  if (current.dev!==entry.dev || current.ino!==entry.ino || current.mode!==entry.mode || current.sha256!==entry.sha256) throw new Error('Input changed');
  const fd=openSync(entry.source,constants.O_RDONLY|constants.O_NOFOLLOW|constants.O_NONBLOCK);
  try {
    const info=fstatSync(fd),data=readFileSync(fd);
    if (info.dev!==entry.dev || info.ino!==entry.ino || hash(data)!==entry.sha256) throw new Error('Input changed');
    return data;
  } finally { closeSync(fd); }
}
function object(value:unknown):Record<string,unknown> {
  if (!value || typeof value!=='object' || Array.isArray(value)) throw new Error('Expected metadata object');
  return value as Record<string,unknown>;
}
function recordFile(files:Entry[],name:string):Entry {
  const file=files.find(file=>file.path===name);
  if (!file) throw new Error('Missing required artifact file: '+name);
  return file;
}
function manifest(files:Entry[]):Record<string,unknown> { return object(JSON.parse(bytes(recordFile(files,'manifest.json')).toString('utf8'))); }
function verifiedManifest(files:Entry[],declarations:unknown, field:'path'|'file', digest:'sha256'|'bundledSHA256'):Entry[] {
  if (!Array.isArray(declarations)) throw new Error('Missing manifest entries');
  const seen=new Set<string>();
  return declarations.map(value=>{
    const row=object(value),name=relative(row[field]);
    if (seen.has(name) || name==='manifest.json') throw new Error('Duplicate artifact entry');
    seen.add(name);
    const file=recordFile(files,name);
    if (file.sha256!==row[digest] || (row.bytes!==undefined && row.bytes!==file.bytes)) throw new Error('Artifact hash mismatch');
    return file;
  });
}
function sameFiles(files:Entry[],declared:Entry[]):void {
  const names=new Set(declared.map(f=>f.path));
  if (files.length!==names.size+1 || files.some(file=>file.path!=='manifest.json'&&!names.has(file.path))) throw new Error('Unexpected artifact payload');
}
function executable(files:Entry[],name:string):Entry {
  const file=recordFile(files,name);
  if (!(file.mode&0o111) || !file.bytes) throw new Error('Missing executable permission');
  return file;
}
function plist(file:Entry):Record<string,unknown> {
  bytes(file);
  return object(JSON.parse(tool('/usr/bin/plutil',['-convert','json','-o','-',file.source])));
}
function validateDependencies(directory:string,names:Set<string>,node=false):void {
  for (const name of node ? ['node'] : names) {
    const path=join(directory,name);
    const dependencies=tool('/usr/bin/otool',['-L',path]).split('\n').slice(1).map(line=>line.trim().split(' (compatibility version')[0]).filter(Boolean);
    for (const dependency of dependencies) {
      if (dependency.startsWith('/usr/lib/') || dependency.startsWith('/System/Library/')) continue;
      const prefix=node?'@loader_path/../Frameworks/':'@loader_path/';
      if (!dependency.startsWith(prefix) || !names.has(dependency.slice(prefix.length))) throw new Error('Runtime has an unbundled dependency');
    }
  }
}

try {
  if (process.platform!=='darwin' || process.version!=='v26.5.0') throw new Error('Assembly requires macOS and Node 26.5.0');
  const names=['client','helper','host-resources','node-runtime','codex','guardian','configuration','destination'];
  const {values}=parseArgs({options:Object.fromEntries(names.map(name=>[name,{type:'string' as const}])),strict:true,allowPositionals:false});
  const input=(name:string)=>{const path=values[name];if(typeof path!=='string')throw new Error('Missing --'+name);return exactPath(path);};
  const destination=input('destination');
  if (lstatSync(destination,{throwIfNoEntry:false})) throw new Error('Destination already exists');
  const configFile=regular(input('configuration')),config=object(JSON.parse(bytes(configFile).toString('utf8')));
  const identifier=(value:unknown)=>typeof value==='string' && /^[A-Za-z0-9][A-Za-z0-9.-]{0,126}$/.test(value) && !value.includes('..');
  if (Object.keys(config).sort().join(',')!=='appGroup,clientIdentifier,format,serviceIdentifier,signingTeam' || config.format!=='1' ||
      !identifier(config.clientIdentifier) || !identifier(config.serviceIdentifier) || !identifier(config.appGroup) || config.clientIdentifier===config.serviceIdentifier ||
      typeof config.signingTeam!=='string' || !/^[A-Z0-9]{10}$/.test(config.signingTeam) || typeof config.appGroup!=='string' || !config.appGroup.startsWith('group.') || (config.appGroup+'.fort.host').length>=128) throw new Error('Invalid exact host configuration');
  const client=inventory(input('client')),helper=inventory(input('helper')),host=inventory(input('host-resources')),node=inventory(input('node-runtime'));
  for (const [files,id,exe] of [[client,config.clientIdentifier,'FortMac'],[helper,config.serviceIdentifier,'FortHost']] as const) {
    const info=plist(recordFile(files,'Contents/Info.plist'));
    if (info.CFBundleIdentifier!==id || info.CFBundleExecutable!==exe) throw new Error('Bundle identity does not match configuration');
    executable(files,'Contents/MacOS/'+exe);
    if (files.some(file=>file.path.includes('/_CodeSignature/') || file.path.endsWith('.provisionprofile') || file.path==='Contents/Resources/fort-host.json' || file.path.startsWith('Contents/Helpers/') || file.path.startsWith('Contents/Library/LaunchAgents/'))) throw new Error('Use fresh unsigned, unassembled client/helper builds');
  }
  const hostManifest=manifest(host);
  if (hostManifest.format!==1 || hostManifest.artifact!=='fort-host-resources') throw new Error('Wrong host artifact');
  sameFiles(host,verifiedManifest(host,hostManifest.files,'path','sha256'));
  for(const name of ['main.mjs','core/LICENSE','executor/LICENSE','context/LICENSE']) recordFile(host,name);
  const nodeManifest=manifest(node);
  if (nodeManifest.schemaVersion!==1) throw new Error('Wrong Node artifact');
  const images=verifiedManifest(node,nodeManifest.images,'file','bundledSHA256'),notices=verifiedManifest(node,nodeManifest.notices,'file','sha256');
  sameFiles(node,[...images,...notices]);
  if (!notices.length || notices.some(f=>!f.path.startsWith('licenses/')) || images.some(f=>f.path!=='bin/node'&&!/^lib\/[^/]+\.dylib$/.test(f.path))) throw new Error('Invalid Node payload layout');
  executable(images,'bin/node');
  const codex=regular(input('codex')),guardian=regular(input('guardian'));
  if (codex.sha256!==codexHash || !(codex.mode&0o111) || !(guardian.mode&0o111) || !guardian.bytes || guardian.bytes>8*1024*1024) throw new Error('Invalid exact runtime executable');
  const legal=Object.entries(legalHashes).map(([name,expected])=>{
    const file=regular(fileURLToPath(new URL('./vendor/codex-0.157.1/'+name,import.meta.url)));
    if(file.sha256!==expected)throw new Error('Codex attribution changed');return {...file,path:name};
  });
  const sources=[configFile,...client,...helper,...host,...node,codex,guardian,...legal];
  mkdirSync(destination,{mode:0o700});const created=lstatSync(destination);
  try {
    const app=join(destination,'Fort.app'),nested=join(app,'Contents/Helpers/FortHost.app');
    function put(path:string,data:Uint8Array,mode=0o600):void {mkdirSync(dirname(path),{recursive:true,mode:0o700});writeFileSync(path,data,{flag:'wx',mode});}
    const copy=(file:Entry,target:string)=>put(target,bytes(file),file.mode);
    for (const file of client) copy(file,join(app,file.path));
    for (const file of helper) copy(file,join(nested,file.path));
    for (const file of host) copy(file,join(nested,'Contents/Resources/host',file.path));
    const libraries=new Set(images.filter(f=>f.path.startsWith('lib/')).map(f=>basename(f.path)));
    for (const file of images) copy(file,join(nested,file.path==='bin/node'?'Contents/MacOS/node':'Contents/Frameworks/'+basename(file.path)));
    for (const file of notices) copy(file,join(nested,'Contents/Resources/NodeLicenses',file.path.slice('licenses/'.length)));
    copy(recordFile(node,'manifest.json'),join(nested,'Contents/Resources/node-source-manifest.json'));
    const nodePath=join(nested,'Contents/MacOS/node'),changes=[...libraries].flatMap(name=>['-change','@loader_path/../lib/'+name,'@loader_path/../Frameworks/'+name]);
    if(changes.length)tool('/usr/bin/install_name_tool',[...changes,nodePath]);
    validateDependencies(join(nested,'Contents/MacOS'),libraries,true);validateDependencies(join(nested,'Contents/Frameworks'),libraries);
    copy(codex,join(nested,'Contents/MacOS/codex'));copy(guardian,join(nested,'Contents/MacOS/FortProcessGuardian'));
    for(const file of legal)copy(file,join(nested,'Contents/Resources/CodexLicenses',file.path));
    const configBytes=Buffer.from(JSON.stringify(config,null,2)+'\n');
    put(join(app,'Contents/Resources/fort-host.json'),configBytes);put(join(nested,'Contents/Resources/fort-host.json'),configBytes);
    const definition={Label:config.serviceIdentifier,BundleProgram:'Contents/Helpers/FortHost.app/Contents/MacOS/FortHost',MachServices:{[config.appGroup+'.fort.host']:true}};
    const launch=join(app,'Contents/Library/LaunchAgents/'+config.serviceIdentifier+'.plist');put(launch,Buffer.from(JSON.stringify(definition)));
    tool('/usr/bin/plutil',['-convert','xml1',launch]);
    for(const source of sources)bytes(source);
    const files=inventory(app).map(({path,mode,bytes,sha256})=>({path,mode,bytes,sha256}));
    const report={format:1,status:'review-only',signed:false,started:false,configuration:config,remaining:['exact signing and provisioning','complete Codex third-party notice inventory','authorized installed provider/UI acceptance'],
      codex:{sourceCommit:'36650394c5b38c2990ccf2a3457165ca3e9d9726',sha256:codexHash},
      inputs:sources.map(({source,mode,sha256})=>({source,mode,sha256})),files};
    put(join(destination,'package-report.json'),Buffer.from(JSON.stringify(report,null,2)+'\n'));
    process.stdout.write(JSON.stringify({destination:realpathSync(destination),status:report.status,files:files.length,signed:false,started:false})+'\n');
  } catch(error) {
    const current=lstatSync(destination,{throwIfNoEntry:false});
    if(current?.isDirectory() && current.dev===created.dev && current.ino===created.ino)rmSync(destination,{recursive:true,force:true});
    throw error;
  }
} catch(error) { process.stderr.write((error instanceof Error?error.message:'Assembly failed')+'\n');process.exitCode=1; }
