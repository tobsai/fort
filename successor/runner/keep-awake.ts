import { spawn, type ChildProcess } from 'node:child_process';

// Spec 063 revision: while an enrolled Mac serves the hub it holds macOS's "prevent system sleep"
// assertion, so queued runs aren't stuck behind idle sleep. `caffeinate -s` holds only on AC power,
// so a laptop on battery still sleeps; `-w` ends the assertion if this process dies.
type Spawn = (command: string, args: string[], options: { stdio: 'ignore' }) => ChildProcess;

export function caffeinate(run: Spawn = spawn): { release(): void } {
  const child = run('/usr/bin/caffeinate', ['-s', '-w', String(process.pid)], { stdio: 'ignore' });
  child.once('error', () => {});
  child.unref();
  return { release: () => { if (child.exitCode === null && child.signalCode === null) child.kill(); } };
}
