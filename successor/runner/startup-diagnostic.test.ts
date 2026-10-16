import {test} from 'node:test';
import assert from 'node:assert/strict';
import {StartupCheckError,redactedStartupFailure,type StartupDiagnostic} from './startup-diagnostic.ts';

test('untrusted errors cannot turn diagnostic metadata into credential output',()=>{
  const fallback={stage:'session',check:'entry'} as const;
  const malformed=new StartupCheckError({stage:'protocol',check:'initialize'});
  Object.defineProperty(malformed,'diagnostic',{value:{stage:'protocol',check:'synthetic-secret'}});
  for(const error of [new Error('synthetic-secret'),{diagnostic:{stage:'protocol',check:'initialize'},token:'synthetic-secret'},malformed]) {
    assert.deepEqual(redactedStartupFailure(error,fallback),fallback);
  }
  const extra=new StartupCheckError({stage:'protocol',check:'initialize'});
  Object.defineProperty(extra,'diagnostic',{value:{stage:'protocol',check:'initialize',token:'synthetic-secret'} as StartupDiagnostic});
  assert.deepEqual(redactedStartupFailure(extra,fallback),{stage:'protocol',check:'initialize'});
});
