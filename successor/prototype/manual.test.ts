import {test} from 'node:test';
import assert from 'node:assert/strict';
import {runManualPacket} from './manual.ts';

test('manual entry refuses before pin reads, native inspection, enrollment or process launch',async()=>{
  await assert.rejects(()=>runManualPacket(),/not approved for execution/);
});
