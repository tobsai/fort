// Synthetic protocol data only; never loads a provider configuration or auth store.
export const syntheticConfig = () => ({
  model_provider:'openai',analytics:{ enabled:false },check_for_update_on_startup:false,notify:[],web_search:'disabled',project_doc_max_bytes:0,
  skills:{ include_instructions:false,bundled:{ enabled:false } },cloud:{ skills:{ enabled:false } },orchestrator:{ mcp:{ enabled:false } },
  mcp_servers:{ spritepipe:{ enabled:false },node_repl:{ enabled:false },'computer-use':{ enabled:false } },
  default_permissions:'fort_candidate_b_no_files',
  permissions:{fort_candidate_b_no_files:{filesystem:{'/**':'deny'},network:{enabled:false}}},
});
export const syntheticFeatures = () => [
  'apps','plugins','hooks','plugin_hooks','remote_control','remote_models','memories','multi_agent','multi_agent_v2','shell_tool','unified_exec','js_repl',
  'skill_search','skill_mcp_dependency_install','skill_env_var_dependency_prompt','image_generation','browser_use','browser_use_external','browser_use_full_cdp_access','computer_use',
].map(name => ({ name,enabled:name==='unified_exec' })).concat([{ name:'skip_host_skill_discovery',enabled:true }]);

export const syntheticAccountIdentity = { accountId: 'workspace-fixture', userLabel: 'owner@fixture.invalid' };
export const syntheticAccount = () => ({ account: { type: 'chatgpt', email: syntheticAccountIdentity.userLabel, planType: 'plus' }, requiresOpenaiAuth: true, workspaceRouting: { chatgptAccountId: syntheticAccountIdentity.accountId, backendOrigin: 'https://chatgpt.com', accountRoutingOverride: null } });
