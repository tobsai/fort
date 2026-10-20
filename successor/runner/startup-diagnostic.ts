const checks = {
  session:['entry','runtime-launch','runtime-preflight','preflight-identity','listener-start','enrollment','cancelled-or-expired'],
  protocol:['initialize','config/read','experimentalFeature/list','mcpServerStatus/list','account/read','model/list','permissionProfile/list'],
  config:['analytics.enabled','check_for_update_on_startup','model_provider','default_permissions','notify','web_search','project_doc_max_bytes','skills.include_instructions','skills.bundled.enabled','cloud.skills.enabled','orchestrator.mcp.enabled','mcp_servers.spritepipe.enabled','mcp_servers.node_repl.enabled','mcp_servers.computer-use.enabled','ambient-mcp','provider-override','provider-routing','telemetry','authentication-class','ambient-instructions','computer-capabilities'],
  permissions:['declaration','catalog','selection'],
  features:['inventory','apps','plugins','hooks','plugin_hooks','remote_control','remote_models','memories','memory_tool','multi_agent','multi_agent_v2','shell_tool','unified_exec','js_repl','skill_search','skill_mcp_dependency_install','skill_env_var_dependency_prompt','image_generation','imagegenext','browser_use','browser_use_external','browser_use_full_cdp_access','computer_use','cli_daybreak','api_key_model_discovery','api_key_cyber_access_programs','ultrafast_mode','skip_host_skill_discovery'],
  mcp:['inventory'],
  account:['chatgpt'],
  model:['availability'],
} as const;
export type StartupDiagnostic = { [Stage in keyof typeof checks]: {stage:Stage;check:(typeof checks)[Stage][number]} }[keyof typeof checks];

export class StartupCheckError extends Error {
  readonly diagnostic:StartupDiagnostic;
  constructor(diagnostic:StartupDiagnostic) {
    super('Session preflight failed; no retry or extra start is authorized.');
    this.diagnostic=Object.freeze({...diagnostic});
  }
}

export function redactedStartupFailure(error:unknown,fallback:StartupDiagnostic):StartupDiagnostic {
  if(error instanceof StartupCheckError && error.diagnostic && typeof error.diagnostic==='object'){
    const {stage,check}=error.diagnostic;
    if(Object.hasOwn(checks,stage) && (checks[stage] as readonly string[]).includes(check)) return {stage,check} as StartupDiagnostic;
  }
  return {...fallback};
}
