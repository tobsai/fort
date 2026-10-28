import { test } from 'node:test';
import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import type { ChildProcess } from 'node:child_process';
import { caffeinate } from './keep-awake.ts';

function fake() {
  const calls: unknown[][] = [], kills: number[] = [];
  const child = Object.assign(new EventEmitter(), { exitCode: null as number | null, signalCode: null, unref() {}, kill() { kills.push(1); child.exitCode = 0; return true; } });
  return { calls, kills, child, run: (...args: unknown[]) => { calls.push(args); return child as unknown as ChildProcess; } };
}

test('caffeinate holds system sleep on AC power, tied to this process, and releases once', () => {
  const f = fake();
  const assertion = caffeinate(f.run);
  assert.deepEqual(f.calls, [['/usr/bin/caffeinate', ['-s', '-w', String(process.pid)], { stdio: 'ignore' }]]);
  assertion.release(); assertion.release();
  assert.equal(f.kills.length, 1);
});

test('a Mac without caffeinate carries on: the spawn error is swallowed', () => {
  const f = fake();
  const assertion = caffeinate(f.run);
  f.child.emit('error', new Error('ENOENT'));
  assertion.release();
});
