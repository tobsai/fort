import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,rmSync,realpathSync} from 'node:fs';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {execFileSync,spawn} from 'node:child_process';
import {randomUUID} from 'node:crypto';
import {once} from 'node:events';

test('the actual macOS bridge detects and gracefully terminates an owned inert app',async t=>{
  const root=realpathSync(mkdtempSync('/tmp/fort-native-stop-canary-'));t.after(()=>rmSync(root,{recursive:true,force:true}));
  const app=join(root,'FortShutdownCanary.app'),executable=join(app,'Contents/MacOS/FortShutdownCanary'),bundle='io.fort.tests.shutdown.'+randomUUID().toLowerCase();
  mkdirSync(join(app,'Contents/MacOS'),{recursive:true});
  writeFileSync(join(app,'Contents/Info.plist'),`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>${bundle}</string><key>CFBundleExecutable</key><string>FortShutdownCanary</string><key>CFBundlePackageType</key><string>APPL</string><key>LSUIElement</key><true/></dict></plist>`);
  execFileSync('/usr/bin/xcrun',['swiftc','-module-cache-path',join(root,'module-cache'),'-framework','AppKit',fileURLToPath(new URL('./fixtures/native-shutdown-canary.swift',import.meta.url)),'-o',executable],{timeout:60_000,stdio:'pipe'});
  const child=spawn(executable,[],{stdio:['ignore','pipe','pipe']}),exited=once(child,'exit');
  t.after(async()=>{if(child.exitCode===null&&child.signalCode===null)child.kill('SIGTERM');await exited;});
  let output='';await new Promise<void>((resolve,reject)=>{const timer=setTimeout(()=>reject(new Error('Inert app did not become ready')),5_000);child.stdout.on('data',bytes=>{output+=bytes.toString();if(output.includes('READY')){clearTimeout(timer);resolve();}});child.once('exit',()=>{clearTimeout(timer);reject(new Error('Inert app exited before readiness'));});});
  const control=(action:'status'|'stop',expectedApp=app,expectedExecutable=executable)=>JSON.parse(execFileSync('/usr/bin/osascript',['-l','JavaScript',fileURLToPath(new URL('./mvp-app.jxa',import.meta.url)),action,bundle,expectedApp,expectedExecutable],{encoding:'utf8',timeout:1_500,stdio:['ignore','pipe','pipe']}));
  const status=control('status');assert.equal(status.length,1);assert.equal(status[0].pid,child.pid);assert.equal(realpathSync(status[0].bundlePath),app);assert.equal(realpathSync(status[0].executablePath),executable);
  assert.throws(()=>control('stop',app+'-different'),/different app/);assert.equal(child.exitCode,null);
  assert.throws(()=>control('stop',app,executable+'-different'),/different app/);assert.equal(child.exitCode,null);
  assert.equal(control('status')[0].pid,child.pid);
  control('stop');
  let timer:ReturnType<typeof setTimeout>|undefined;
  try{
    const stopped=await Promise.race([exited.then(()=>true),new Promise<boolean>(resolve=>{timer=setTimeout(()=>resolve(false),3_000);})]);
    assert.equal(stopped,true,'The actual bridge must stop the real canary process before its safety timer');
    assert.deepEqual(await exited,[0,null]);assert.deepEqual(control('status'),[]);assert.deepEqual(control('stop'),[]);
  }finally{clearTimeout(timer);}
});
