import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync, existsSync, chmodSync, cpSync, symlinkSync, linkSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const script = fileURLToPath(new URL('./mac-application.ts', import.meta.url));
const codex = process.env.FORT_TEST_CODEX;
const nodeRuntime = process.env.FORT_TEST_NODE_BUNDLE;
const hash = (bytes: Uint8Array) => createHash('sha256').update(bytes).digest('hex');
const configuration = { format:'1', clientIdentifier:'io.fort.synthetic.mac', serviceIdentifier:'io.fort.synthetic.mac.host', signingTeam:'AAAAAAAAAA', appGroup:'group.io.fort.synthetic.mac' };
function fixture(fullNode=false) {
  const root = mkdtempSync('/tmp/fort059-assembly-');
  const put = (name:string, data:string|Buffer, mode=0o600) => {
    const path=join(root,name); mkdirSync(join(path,'..'),{recursive:true,mode:0o700}); writeFileSync(path,data,{mode}); return path;
  };
  for (const [kind,identifier,executable] of [['client',configuration.clientIdentifier,'FortMac'],['helper',configuration.serviceIdentifier,'FortHost']]) {
    put(`${kind}/Contents/Info.plist`,`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>${identifier}</string><key>CFBundleExecutable</key><string>${executable}</string></dict></plist>`);
    put(`${kind}/Contents/MacOS/${executable}`,'inert synthetic code; never execute',0o700);
  }
  const hostFiles=['main.mjs','core/LICENSE','executor/LICENSE','context/LICENSE','node-openssl.cnf','executor/node-openssl.cnf'];
  const files=hostFiles.map(path=>{const bytes=Buffer.from(path.endsWith('.cnf')?'nodejs_conf = fort_node\n[fort_node]\n':'synthetic '+path);put('host/'+path,bytes);return {path,bytes:bytes.length,sha256:hash(bytes)};});
  put('host/manifest.json',JSON.stringify({format:1,artifact:'fort-host-resources',files}));
  const node=Buffer.from('synthetic Node; never execute'),library=Buffer.from('synthetic library'),notice=Buffer.from('synthetic attribution');
  put('node/bin/node',node,0o700);put('node/lib/libnode.dylib',library,0o700);put('node/licenses/synthetic/LICENSE',notice);
  put('node/manifest.json',JSON.stringify({schemaVersion:1,images:[{file:'bin/node',bundledSHA256:hash(node)},{file:'lib/libnode.dylib',bundledSHA256:hash(library)}],notices:[{file:'licenses/synthetic/LICENSE',sha256:hash(notice)}]}));
  if(fullNode) { assert.ok(nodeRuntime); rmSync(join(root,'node'),{recursive:true});cpSync(nodeRuntime,join(root,'node'),{recursive:true}); }
  put('guardian','synthetic guardian; never execute',0o700);
  put('configuration.json',JSON.stringify(configuration));
  const destination=join(root,'output');
  const args=['--client',join(root,'client'),'--helper',join(root,'helper'),'--host-resources',join(root,'host'),'--node-runtime',join(root,'node'),'--codex',codex ?? join(root,'absent-codex'),'--guardian',join(root,'guardian'),'--configuration',join(root,'configuration.json'),'--destination',destination];
  return {root,put,destination,args,run:(entry=script)=>spawnSync(process.execPath,[entry,...args],{env:{},encoding:'utf8',timeout:30_000})};
}

test('Mac assembly produces only an inert review bundle and exact local host layout', {skip:process.platform!=='darwin'||!codex||!nodeRuntime?'requires macOS and explicit reviewed Codex source path':false}, t=>{
  const f=fixture(true);t.after(()=>rmSync(f.root,{recursive:true,force:true}));
  const result=f.run();assert.equal(result.status,0,result.stderr);
  const app=join(f.destination,'Fort.app'),helper=join(app,'Contents/Helpers/FortHost.app');
  assert.deepEqual(JSON.parse(readFileSync(join(app,'Contents/Resources/fort-host.json'),'utf8')),configuration);
  assert.equal(readFileSync(join(app,'Contents/Resources/fort-host.json'),'utf8'),readFileSync(join(helper,'Contents/Resources/fort-host.json'),'utf8'));
  const plist=spawnSync('/usr/bin/plutil',['-convert','json','-o','-',join(app,`Contents/Library/LaunchAgents/${configuration.serviceIdentifier}.plist`)],{env:{},encoding:'utf8'});
  assert.equal(plist.status,0,plist.stderr);
  assert.deepEqual(JSON.parse(plist.stdout),{Label:configuration.serviceIdentifier,BundleProgram:'Contents/Helpers/FortHost.app/Contents/MacOS/FortHost',MachServices:{[configuration.appGroup+'.fort.host']:true}});
  const sourceManifest=JSON.parse(readFileSync(join(f.root,'node/manifest.json'),'utf8'));
  for(const notice of sourceManifest.notices) assert.equal(hash(readFileSync(join(helper,'Contents/Resources/NodeLicenses',notice.file.slice('licenses/'.length)))),notice.sha256);
  assert.ok(existsSync(join(helper,'Contents/Resources/host/main.mjs')));
  for (const name of ['THIRD-PARTY-NOTICES.zip','THIRD-PARTY-SOURCES.zip','THIRD-PARTY.json','SOURCE-AVAILABILITY.txt']) {
    const original=readFileSync(fileURLToPath(new URL('./vendor/codex-0.157.1/'+name,import.meta.url)));
    assert.deepEqual(readFileSync(join(helper,'Contents/Resources/CodexLicenses',name)),original);
  }
  assert.equal(existsSync(join(app,'Contents/embedded.provisionprofile')),false);
  const report=JSON.parse(readFileSync(join(f.destination,'package-report.json'),'utf8'));
  assert.equal(report.status,'review-only');assert.equal(report.signed,false);assert.equal(report.started,false);
  for (const entry of report.files) assert.equal(hash(readFileSync(join(app,entry.path))),entry.sha256);
  assert.ok(report.remaining.includes('exact signing and provisioning'));
  assert.equal(readFileSync(join(f.root,'client/Contents/MacOS/FortMac'),'utf8'),'inert synthetic code; never execute');
});

for (const scenario of ['existing','identity','group','host-hash','path-escape','unexpected','node-hash','symlink','hardlink','writable','missing-license','missing-executable','wrong-codex','missing-openssl','altered-openssl']) {
  test('Mac assembly refuses '+scenario+' without changing source or existing files',{skip:process.platform!=='darwin'},t=>{
    const f=fixture();t.after(()=>rmSync(f.root,{recursive:true,force:true}));
    const source=join(f.root,'client/Contents/MacOS/FortMac'),original=readFileSync(source);
    switch(scenario) {
      case 'existing':mkdirSync(f.destination);writeFileSync(join(f.destination,'keep'),'preserve');break;
      case 'identity':f.put('configuration.json',JSON.stringify({...configuration,clientIdentifier:'io.fort.other'}));break;
      case 'group':f.put('configuration.json',JSON.stringify({...configuration,appGroup:'not-a-group'}));break;
      case 'host-hash':f.put('host/main.mjs','tampered');break;
      case 'path-escape':f.put('host/manifest.json',JSON.stringify({format:1,artifact:'fort-host-resources',files:[{path:'../escape',sha256:'0'.repeat(64)}]}));break;
      case 'unexpected':f.put('host/extra.mjs','unreviewed');break;
      case 'node-hash':f.put('node/bin/node','tampered',0o700);break;
      case 'symlink':rmSync(source);symlinkSync(join(f.root,'guardian'),source);break;
      case 'hardlink':linkSync(source,join(f.root,'linked'));break;
      case 'writable':chmodSync(source,0o777);break;
      case 'missing-license':rmSync(join(f.root,'host/core/LICENSE'));break;
      case 'missing-executable':chmodSync(source,0o600);break;
      case 'missing-openssl':
      case 'altered-openssl': {
        const path='executor/node-openssl.cnf', manifestPath=join(f.root,'host/manifest.json'), metadata=JSON.parse(readFileSync(manifestPath,'utf8'));
        if(scenario==='missing-openssl') {rmSync(join(f.root,'host',path));metadata.files=metadata.files.filter((entry:{path:string})=>entry.path!==path);}
        else {const data=Buffer.from('nodejs_conf = other\n[other]\n');f.put('host/'+path,data);Object.assign(metadata.files.find((entry:{path:string})=>entry.path===path),{bytes:data.length,sha256:hash(data)});}
        writeFileSync(manifestPath,JSON.stringify(metadata));break;
      }
      case 'wrong-codex':f.args[f.args.indexOf('--codex')+1]=f.put('wrong-codex','inert',0o700);break;
    }
    const result=f.run();assert.notEqual(result.status,0);assert.ok(result.stderr.trim());
    if(scenario==='existing')assert.equal(readFileSync(join(f.destination,'keep'),'utf8'),'preserve');
    else assert.equal(existsSync(f.destination),false);
    if(scenario!=='symlink')assert.deepEqual(readFileSync(source),original);
  });
}

test('Mac assembly removes only its new output after native metadata validation fails',{skip:process.platform!=='darwin'||!codex?'requires explicit reviewed Codex source':false},t=>{
  const f=fixture();t.after(()=>rmSync(f.root,{recursive:true,force:true}));
  const result=f.run();assert.notEqual(result.status,0);
  assert.match(result.stderr,/install_name_tool|Mach-O|otool/);
  assert.equal(existsSync(f.destination),false);
  assert.ok(existsSync(join(f.root,'client/Contents/MacOS/FortMac')));
});

for (const scenario of ['missing-source','changed-notices','unexpected-legal-file']) {
  test('Mac assembly rejects Codex attribution '+scenario+' before creating output', {skip:process.platform!=='darwin'||!codex||!nodeRuntime?'requires explicit reviewed runtime inputs':false},t=>{
    const f=fixture(true);t.after(()=>rmSync(f.root,{recursive:true,force:true}));
    const isolated=join(f.root,'assembler'),entry=join(isolated,'mac-application.ts');
    mkdirSync(isolated,{mode:0o700});writeFileSync(entry,readFileSync(script),{mode:0o600});
    const legal=join(isolated,'vendor/codex-0.157.1');
    cpSync(fileURLToPath(new URL('./vendor/codex-0.157.1',import.meta.url)),legal,{recursive:true});
    if(scenario==='missing-source')rmSync(join(legal,'THIRD-PARTY-SOURCES.zip'));
    else if(scenario==='changed-notices')writeFileSync(join(legal,'THIRD-PARTY-NOTICES.zip'),'changed');
    else writeFileSync(join(legal,'unexpected.txt'),'unreviewed');
    const result=f.run(entry);assert.notEqual(result.status,0);assert.match(result.stderr,/Codex attribution/);
    assert.equal(existsSync(f.destination),false);
  });
}
