import { CodexConnection, object } from './codex-rpc.ts';

export const CODEX_SETTINGS: ReadonlyArray<readonly [string, string | number | boolean | readonly string[]]> = [
  ['analytics.enabled',false],['check_for_update_on_startup',false],['model_provider','openai'],['notify',[]],['web_search','disabled'],['project_doc_max_bytes',0],
  ['skills.include_instructions',false],['skills.bundled.enabled',false],['cloud.skills.enabled',false],['orchestrator.mcp.enabled',false],
  ['mcp_servers.spritepipe.enabled',false],['mcp_servers.node_repl.enabled',false],['mcp_servers.computer-use.enabled',false],
];
export const CODEX_DISABLED_FEATURES = ['apps','plugins','hooks','plugin_hooks','remote_control','remote_models','memories','memory_tool','multi_agent','multi_agent_v2','shell_tool','unified_exec','js_repl','skill_search','skill_mcp_dependency_install','skill_env_var_dependency_prompt','image_generation','imagegenext','browser_use','browser_use_external','browser_use_full_cdp_access','computer_use'] as const;

export function verifyEffectiveConfig(value: unknown): void {
  const config = object(value);
  for (const [path,expected] of CODEX_SETTINGS) {
    const actual = path.split('.').reduce<unknown>((parent,key) => object(parent)[key],config);
    if (JSON.stringify(actual) !== JSON.stringify(expected)) throw new Error('Effective runtime control was not confirmed');
  }
  const servers = object(config.mcp_servers);
  if (Object.values(servers).some(server => object(server).enabled !== false)) throw new Error('Ambient MCP server is enabled or unknown');
  if (config.model_providers != null && Object.hasOwn(object(config.model_providers),'openai')) throw new Error('Custom OpenAI provider configuration is outside the packet');
  for (const [key,normal] of [['chatgpt_base_url','https://chatgpt.com/backend-api'],['openai_base_url','https://api.openai.com/v1']]) {
    if (config[key]!=null && config[key]!==normal && config[key]!==normal+'/') throw new Error('Custom provider routing is outside the packet');
  }
  if (config.otel!=null) {
    const telemetry=object(config.otel);
    if (['exporter','trace_exporter','metrics_exporter'].some(key=>telemetry[key]!=null&&telemetry[key]!=='none') || telemetry.log_user_prompt===true) throw new Error('Ambient telemetry is outside the packet');
  }
  if (config.forced_login_method != null && config.forced_login_method !== 'chatgpt') throw new Error('Wrong authentication class');
  for (const key of ['instructions','developer_instructions','compact_prompt']) {
    if (config[key] != null && config[key] !== '') throw new Error('Ambient instructions are outside the packet');
  }
  for (const key of ['browser_use','computer_use']) {
    if (config[key] != null && Object.values(object(config[key])).some(value => value != null)) throw new Error('Ambient computer capabilities are outside the packet');
  }
}

export async function readRuntimePages(connection: CodexConnection, method: string, params: Record<string,unknown>, signal: AbortSignal): Promise<unknown[]> {
  const values: unknown[] = [], cursors = new Set<string>();
  let cursor: string | null = null;
  for (let page=0;page<8;page++) {
    const response = object(await connection.request(method,{ ...params,cursor,limit:100 },signal));
    if (!Array.isArray(response.data) || response.data.length>100) throw new Error('Invalid runtime inventory');
    values.push(...response.data);
    if (response.nextCursor === null) return values;
    if (typeof response.nextCursor !== 'string' || !response.nextCursor || cursors.has(response.nextCursor)) throw new Error('Incomplete runtime inventory');
    cursor=response.nextCursor; cursors.add(cursor);
  }
  throw new Error('Runtime inventory exceeded its bound');
}

export async function verifyRuntimeControls(connection: CodexConnection, workspace: string, signal: AbortSignal, threadId?: string): Promise<void> {
  verifyEffectiveConfig(object(await connection.request('config/read',{ includeLayers:false,cwd:workspace },signal)).config);
  const scope = threadId === undefined ? {} : { threadId };
  const features = await readRuntimePages(connection,'experimentalFeature/list',scope,signal);
  const states = new Map<string,boolean>();
  for (const value of features) {
    const feature=object(value);
    if (typeof feature.name !== 'string' || typeof feature.enabled !== 'boolean' || states.has(feature.name)) throw new Error('Invalid feature inventory');
    states.set(feature.name,feature.enabled);
  }
  if (CODEX_DISABLED_FEATURES.some(name => states.get(name)!==false) || states.get('skip_host_skill_discovery')!==true) throw new Error('Effective feature controls were not confirmed');
  const servers=await readRuntimePages(connection,'mcpServerStatus/list',{ ...scope,detail:'full' },signal);
  if (servers.length!==0) throw new Error('Empty effective MCP inventory was not confirmed');
}
