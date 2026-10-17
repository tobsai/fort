import { createHash } from 'node:crypto';
import { readFileSync, readdirSync } from 'node:fs';
import { isAbsolute } from 'node:path';
import { CodexChat } from '../runner/codex-chat.ts';
import { launchOwnedProcess, OWNER_TRIAL_PROCESS_BUDGET, OWNER_DEMO_PROCESS_BUDGET, type OwnedProcessOptions } from '../runner/owned-process.ts';
import { requireCandidateBActivation } from './activation.ts';
import { CODEX_DISABLED_FEATURES,CODEX_SETTINGS } from '../runner/codex-controls.ts';
import {CODEX_PERMISSION_OVERRIDE} from '../runner/codex-permissions.ts';

export const CODEX_EXECUTABLE = '/opt/homebrew/Caskroom/codex/0.157.1/bin/codex';
export const CODEX_SHA256 = '27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d';
export function codexLaunchPlan(paths: { workspace: string; temporaryDirectory: string; remainingWorkMs?:number; packet?:typeof OWNER_TRIAL_PROCESS_BUDGET.packet|typeof OWNER_DEMO_PROCESS_BUDGET.packet }): OwnedProcessOptions {
  if(process.platform!=='darwin')throw new Error('This permission profile is reviewed only for the pinned macOS runtime');
  if (![paths.workspace,paths.temporaryDirectory].every(path => isAbsolute(path) && path !== '/' && !path.includes('\0'))) throw new Error('Use explicit candidate workspace and temporary directory paths');
  const remaining=paths.remainingWorkMs??296_000;
  if(paths.packet!==undefined&&paths.packet!==OWNER_TRIAL_PROCESS_BUDGET.packet&&paths.packet!==OWNER_DEMO_PROCESS_BUDGET.packet)throw new Error('Invalid packet identity');
  const packetBudget=paths.packet===OWNER_TRIAL_PROCESS_BUDGET.packet?OWNER_TRIAL_PROCESS_BUDGET:paths.packet===OWNER_DEMO_PROCESS_BUDGET.packet?OWNER_DEMO_PROCESS_BUDGET:undefined;
  if (!Number.isInteger(remaining) || remaining<1 || remaining>(packetBudget?.workMs??296_000)) throw new Error('Invalid remaining session budget');
  const settings=[...CODEX_SETTINGS.map(([name,value])=>`${name}=${JSON.stringify(value)}`),CODEX_PERMISSION_OVERRIDE];
  return { executable:CODEX_EXECUTABLE,args:['app-server','--listen','stdio://',...settings.flatMap(value => ['-c',value]),'--enable','skip_host_skill_discovery',...CODEX_DISABLED_FEATURES.flatMap(value => ['--disable',value])],cwd:paths.workspace,env:{ HOME:'/Users/tobiasgunn',PATH:'/usr/bin:/bin',LANG:'en_US.UTF8',TZ:'UTC',TMPDIR:paths.temporaryDirectory },deadlineMs:remaining,terminationGraceMs:2_000,killGraceMs:2_000,...(packetBudget?{packetBudget}:{}) };
}
export function launchCandidateCodex(paths: { workspace: string; temporaryDirectory: string; remainingWorkMs?:number; packet?:typeof OWNER_TRIAL_PROCESS_BUDGET.packet }): CodexChat {
  requireCandidateBActivation();
  const plan = codexLaunchPlan(paths);
  if (createHash('sha256').update(readFileSync(CODEX_EXECUTABLE)).digest('hex') !== CODEX_SHA256) throw new Error('Codex executable differs from the reviewed pin');
  if (readdirSync(paths.workspace).length !== 0) throw new Error('Candidate workspace must be empty');
  const owned = launchOwnedProcess(plan);
  return new CodexChat({ connection:owned.connection,workspace:paths.workspace,runtimeVersion:'0.157.1' });
}
