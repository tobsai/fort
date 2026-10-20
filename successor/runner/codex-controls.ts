import { CodexConnection, object } from './codex-rpc.ts';
import {StartupCheckError,redactedStartupFailure,type StartupDiagnostic} from './startup-diagnostic.ts';
import {CODEX_PERMISSION_PROFILE,verifyPermissionDeclaration,verifyPermissionCatalog} from './codex-permissions.ts';

export const CODEX_SETTINGS = [
  ['analytics.enabled',false],['check_for_update_on_startup',false],['model_provider','openai'],['notify',[]],['web_search','disabled'],['project_doc_max_bytes',0],
  ['skills.include_instructions',false],['skills.bundled.enabled',false],['cloud.skills.enabled',false],['orchestrator.mcp.enabled',false],
  ['default_permissions',CODEX_PERMISSION_PROFILE],
] as const satisfies ReadonlyArray<readonly [string,string|number|boolean|readonly string[]]>;
// Requested CLI opt-outs; pinned-runtime observations are checked separately below.
export const CODEX_DISABLED_FEATURES = ['apps','plugins','hooks','plugin_hooks','remote_control','remote_models','memories','memory_tool','multi_agent','multi_agent_v2','shell_tool','unified_exec','js_repl','skill_search','skill_mcp_dependency_install','skill_env_var_dependency_prompt','image_generation','imagegenext','browser_use','browser_use_external','browser_use_full_cdp_access','computer_use','cli_daybreak','api_key_model_discovery','api_key_cyber_access_programs','ultrafast_mode'] as const;

export function verifyEffectiveConfig(value: unknown): void {
  const config = object(value);
  for (const [path,expected] of CODEX_SETTINGS) {
    try {
      const actual = path.split('.').reduce<unknown>((parent,key) => object(parent)[key],config);
      if (JSON.stringify(actual) !== JSON.stringify(expected)) throw new Error('Control mismatch');
    } catch {throw new StartupCheckError({stage:'config',check:path});}
  }
  const servers = object(config.mcp_servers);
  if (Object.values(servers).some(server => object(server).enabled !== false)) throw new StartupCheckError({stage:'config',check:'ambient-mcp'});
  if (config.model_providers != null && Object.hasOwn(object(config.model_providers),'openai')) throw new StartupCheckError({stage:'config',check:'provider-override'});
  for (const [key,normal] of [['chatgpt_base_url','https://chatgpt.com/backend-api'],['openai_base_url','https://api.openai.com/v1']]) {
    if (config[key]!=null && config[key]!==normal && config[key]!==normal+'/') throw new StartupCheckError({stage:'config',check:'provider-routing'});
  }
  if (config.otel!=null) {
    const telemetry=object(config.otel);
    if (['exporter','trace_exporter','metrics_exporter'].some(key=>telemetry[key]!=null&&telemetry[key]!=='none') || telemetry.log_user_prompt===true) throw new StartupCheckError({stage:'config',check:'telemetry'});
  }
  if (config.forced_login_method != null && config.forced_login_method !== 'chatgpt') throw new StartupCheckError({stage:'config',check:'authentication-class'});
  for (const key of ['instructions','developer_instructions','compact_prompt']) {
    if (config[key] != null && config[key] !== '') throw new StartupCheckError({stage:'config',check:'ambient-instructions'});
  }
  for (const key of ['browser_use','computer_use']) {
    if (config[key] != null && Object.values(object(config[key])).some(value => value != null)) throw new StartupCheckError({stage:'config',check:'computer-capabilities'});
  }
  verifyPermissionDeclaration(config);
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
  let checkpoint:StartupDiagnostic={stage:'protocol',check:'config/read'};
  try {
    const config=object(object(await connection.request('config/read',{ includeLayers:false,cwd:workspace },signal)).config);
    verifyEffectiveConfig(config);
    checkpoint={stage:'protocol',check:'permissionProfile/list'};
    verifyPermissionCatalog(await readRuntimePages(connection,'permissionProfile/list',{cwd:workspace},signal));
    const scope = threadId === undefined ? {} : { threadId };
    checkpoint={stage:'protocol',check:'experimentalFeature/list'};
    const features = await readRuntimePages(connection,'experimentalFeature/list',scope,signal);
    const states = new Map<string,boolean>();
    for (const value of features) {
      const feature=object(value);
      if (typeof feature.name !== 'string' || typeof feature.enabled !== 'boolean' || states.has(feature.name)) throw new StartupCheckError({stage:'features',check:'inventory'});
      states.set(feature.name,feature.enabled);
    }
    const unconfirmed=CODEX_DISABLED_FEATURES.find(name => {
      // 0.163.0 forces this backend selector on unless managed requirements pin it.
      // Shell authority is blocked by explicit shell_tool:false plus empty environments.
      if (name==='unified_exec') return !states.has(name);
      // Codex 0.163.0 lists canonical keys, not these accepted input aliases.
      const canonical=name==='memory_tool'?'memories':name==='imagegenext'?'image_generation':name;
      return states.get(canonical)!==false || (canonical!==name && states.has(name) && states.get(name)!==false);
    });
    if (unconfirmed) throw new StartupCheckError({stage:'features',check:unconfirmed});
    if (states.get('skip_host_skill_discovery')!==true) throw new StartupCheckError({stage:'features',check:'skip_host_skill_discovery'});
    checkpoint={stage:'protocol',check:'mcpServerStatus/list'};
    const servers=await readRuntimePages(connection,'mcpServerStatus/list',{ ...scope,detail:'full' },signal);
    checkpoint={stage:'mcp',check:'inventory'};
    const configured=object(config.mcp_servers), names=new Set<string>();
    for (const value of servers) {
      const server=object(value);
      if (typeof server.name!=='string' || !Object.hasOwn(configured,server.name) || names.has(server.name) ||
          object(configured[server.name]).enabled!==false || server.runtimeStatus!==(threadId===undefined?null:'disabled') ||
          server.authStatus!=='unsupported' || server.pluginId!==null || server.serverInfo!==null ||
          server.serverCapabilities!==null || server.toolsError!==null ||
          (server.httpOrigin!==null && typeof server.httpOrigin!=='string') || Object.keys(object(server.tools)).length!==0 ||
          !Array.isArray(server.resources) || server.resources.length!==0 ||
          !Array.isArray(server.resourceTemplates) || server.resourceTemplates.length!==0) throw new StartupCheckError(checkpoint);
      names.add(server.name);
    }
  } catch(error) {throw new StartupCheckError(redactedStartupFailure(error,checkpoint));}
}
