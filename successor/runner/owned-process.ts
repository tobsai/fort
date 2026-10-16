import { spawn } from 'node:child_process';
import { isAbsolute } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { CodexConnection } from './codex-rpc.ts';

export type OwnedProcessOptions = {
  executable: string; args: string[]; cwd: string; env: Record<string,string>;
  deadlineMs: number; terminationGraceMs?: number; killGraceMs?: number;
};
export function launchOwnedProcess(options: OwnedProcessOptions) {
  if (!isAbsolute(options.executable) || !isAbsolute(options.cwd) || !Number.isInteger(options.deadlineMs) || options.deadlineMs < 1 || options.deadlineMs > 300_000) throw new Error('Use explicit process paths and a bounded deadline');
  const terminationGraceMs = options.terminationGraceMs ?? 2_000, killGraceMs = options.killGraceMs ?? 2_000;
  if (![terminationGraceMs,killGraceMs].every(value => Number.isInteger(value) && value > 0 && value <= 2_000)) throw new Error('Invalid cleanup deadline');
  const child = spawn(options.executable,options.args,{ cwd:options.cwd,env:options.env,stdio:['pipe','pipe','pipe'],detached:true });
  let exit: { code:number|null; signal:NodeJS.Signals|null } | undefined;
  let cleanupConfirmed = false, stopping: Promise<void> | undefined, stderrBytes = 0;
  const groupPresent = () => {
    if (!child.pid) return false;
    try { process.kill(-child.pid,0); return true; }
    catch (error) { if (error instanceof Error && 'code' in error && error.code === 'ESRCH') return false; throw new Error('Owned process group status unavailable'); }
  };
  const signal = (value: NodeJS.Signals) => {
    if (!child.pid) return;
    try { process.kill(-child.pid,value); }
    catch (error) { if (!(error instanceof Error && 'code' in error && error.code === 'ESRCH')) throw new Error('Owned process signal failed'); }
  };
  const waitForExit = async (milliseconds: number) => {
    const deadline = performance.now()+milliseconds;
    do { if (exit !== undefined && !groupPresent()) return true; await delay(Math.min(20,Math.max(1,deadline-performance.now()))); } while (performance.now()<deadline);
    return exit !== undefined && !groupPresent();
  };
  const stop = (): Promise<void> => {
    if (stopping) return stopping;
    clearTimeout(deadline);
    stopping = (async () => {
      child.stdin.destroy(); child.stdout.destroy(); child.stderr.destroy();
      signal('SIGTERM');
      if (!await waitForExit(terminationGraceMs)) { signal('SIGKILL'); if (!await waitForExit(killGraceMs)) throw new Error('Owned process exit was not confirmed; restart is blocked'); }
      cleanupConfirmed = true;
    })();
    return stopping;
  };
  const connection = new CodexConnection({ input:child.stdin,output:child.stdout,close:stop });
  const deadline = setTimeout(() => connection.close(),options.deadlineMs);
  child.on('close',(code,receivedSignal) => { exit = { code,signal:receivedSignal }; connection.close(); });
  child.on('error',() => connection.close());
  child.stderr.on('data',(bytes:Buffer) => { stderrBytes+=bytes.byteLength; if (stderrBytes>1_000_000) connection.close(); });
  child.stderr.on('error',() => connection.close());
  return { connection, dispose:() => connection.dispose(), get cleanupConfirmed() { return cleanupConfirmed; }, get exit() { return exit; } };
}
