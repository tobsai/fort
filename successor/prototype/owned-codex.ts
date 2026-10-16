import { createHash } from 'node:crypto';
import { readFileSync, readdirSync } from 'node:fs';
import { isAbsolute } from 'node:path';
import { CodexChat } from '../runner/codex-chat.ts';
import { launchOwnedProcess, type OwnedProcessOptions } from '../runner/owned-process.ts';
import { requireCandidateBActivation } from './activation.ts';

export const CODEX_EXECUTABLE = '/opt/homebrew/Caskroom/codex/0.157.1/bin/codex';
export const CODEX_SHA256 = '27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d';
export function codexLaunchPlan(paths: { workspace: string; temporaryDirectory: string }): OwnedProcessOptions {
  if (![paths.workspace,paths.temporaryDirectory].every(path => isAbsolute(path) && path !== '/' && !path.includes('\0'))) throw new Error('Use explicit candidate workspace and temporary directory paths');
  const settings = ['analytics.enabled=false','check_for_update_on_startup=false','model_provider="openai"','notify=[]','web_search="disabled"','project_doc_max_bytes=0','skills.include_instructions=false','skills.bundled.enabled=false','cloud.skills.enabled=false','orchestrator.mcp.enabled=false','mcp_servers.spritepipe.enabled=false','mcp_servers.node_repl.enabled=false','mcp_servers.computer-use.enabled=false'];
  const disabled = ['apps','plugins','hooks','plugin_hooks','remote_control','remote_models','memories','memory_tool','multi_agent','multi_agent_v2','shell_tool','unified_exec','js_repl','skill_search','skill_mcp_dependency_install','skill_env_var_dependency_prompt','image_generation','imagegenext'];
  return { executable:CODEX_EXECUTABLE,args:['app-server','--listen','stdio://',...settings.flatMap(value => ['-c',value]),'--enable','skip_host_skill_discovery',...disabled.flatMap(value => ['--disable',value])],cwd:paths.workspace,env:{ HOME:'/Users/tobiasgunn',PATH:'/usr/bin:/bin',LANG:'en_US.UTF8',TZ:'UTC',TMPDIR:paths.temporaryDirectory },deadlineMs:296_000,terminationGraceMs:2_000,killGraceMs:2_000 };
}
export function launchCandidateCodex(paths: { workspace: string; temporaryDirectory: string }): CodexChat {
  requireCandidateBActivation();
  const plan = codexLaunchPlan(paths);
  if (createHash('sha256').update(readFileSync(CODEX_EXECUTABLE)).digest('hex') !== CODEX_SHA256) throw new Error('Codex executable differs from the reviewed pin');
  if (readdirSync(paths.workspace).length !== 0) throw new Error('Candidate workspace must be empty');
  const owned = launchOwnedProcess(plan);
  return new CodexChat({ connection:owned.connection,workspace:paths.workspace,runtimeVersion:'0.157.1' });
}
