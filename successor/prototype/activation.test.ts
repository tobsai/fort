import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import vm from 'node:vm';

test('approved activation permits only the manual packet entry without starting it',()=>{
  const url=new URL('./activation.ts',import.meta.url);
  let source=stripTypeScriptTypes(readFileSync(url,'utf8'));
  source=source.split(/\nif\s*\(import\.meta\.main\)/)[0]!;
  source=source.replace(/^import .*?;\n/gm,'').replace('export function requireCandidateBActivation','function requireCandidateBActivation').replaceAll('import.meta.url',JSON.stringify(url.href));
  const fakeProcess={argv:[process.execPath,fileURLToPath(new URL('./manual.ts',url))]};
  const context=vm.createContext({process:fakeProcess,resolve,fileURLToPath,URL});
  const check:()=>void=vm.runInContext(source+'\nrequireCandidateBActivation;',context);
  assert.doesNotThrow(check);
  for(const entry of [undefined,fileURLToPath(url),'/tmp/manual.ts',fileURLToPath(new URL('./owned-codex.ts',url))]){
    fakeProcess.argv=entry?[process.execPath,entry]:[process.execPath];
    assert.throws(check,/approved only through the pinned manual packet/);
  }
});
