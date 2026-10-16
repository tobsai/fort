import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {PassThrough,Writable} from 'node:stream';
import {CodexConnection,object} from './codex-rpc.ts';
import {CodexChat} from './codex-chat.ts';
import type {ChatInput} from './chat-runtime.ts';
import {redactedStartupFailure} from './startup-diagnostic.ts';

const pinned=JSON.parse(readFileSync(new URL('./fixtures/codex-0.157.1-startup.json',import.meta.url),'utf8'));
type Frame={id?:unknown;method?:string;params:Record<string,unknown>};
type Mutate=(method:string,response:Record<string,unknown>,request:Frame)=>void;
const input:ChatInput={runId:'audit',requestId:'audit',conversationId:'home',text:'Synthetic sample',history:[],configuration:{agentId:'writer',agentName:'Writer',bindingRevisionId:'binding',selectedModel:'gpt-6-astra',instructions:'Use supplied context only.',skills:[]}};

function fixture(mutate:Mutate=()=>{}) {
  const calls:Frame[]=[], output=new PassThrough();
  const emit=(value:unknown)=>output.write(JSON.stringify(value)+'\n');
  const stdin=new Writable({write(chunk,_encoding,done){
    const frame:Frame=JSON.parse(chunk.toString());calls.push(frame);
    queueMicrotask(()=>{
      const page=(data:unknown[])=>{
        const start=Number(frame.params.cursor??0),end=start+Number(frame.params.limit);
        return {data:data.slice(start,end),nextCursor:end<data.length?String(end):null};
      };
      let response:Record<string,unknown>;
      switch(frame.method){
        case 'initialize':response={userAgent:'synthetic'};break;
        case 'config/read':response={config:structuredClone(pinned.config),origins:{},layers:null};break;
        case 'experimentalFeature/list':response=page(structuredClone(pinned.features));break;
        case 'mcpServerStatus/list':response=page(structuredClone(pinned.mcp).map((server:Record<string,unknown>)=>({...server,runtimeStatus:frame.params.threadId?'disabled':null})));break;
        case 'account/read':response=structuredClone(pinned.account);break;
        case 'model/list':response=page(structuredClone(pinned.models));break;
        case 'thread/start':response=structuredClone(pinned.threadStart);break;
        case 'turn/start':response={turn:{id:'synthetic-turn',status:'inProgress'}};break;
        default:return;
      }
      mutate(frame.method,response,frame);
      if(frame.method==='turn/start' && object(object(frame.params.sandboxPolicy).access).type==='restricted'){
        // permissions.rs Deserialize rejects this legacy restriction before executing a turn.
        emit({id:frame.id,error:{code:-32602,message:'readOnly.access is no longer supported; use permissionProfile for restricted reads'}});return;
      }
      emit({id:frame.id,result:response});
      if(frame.method==='turn/start'){
        emit({method:'item/completed',params:{threadId:'synthetic-thread',turnId:'synthetic-turn',item:{id:'reply',type:'agentMessage',phase:'final_answer',text:'Synthetic reply'}}});
        emit({method:'turn/completed',params:{threadId:'synthetic-thread',turn:{id:'synthetic-turn',status:'completed',error:null}}});
      }
    });done();
  }});
  const connection=new CodexConnection({input:stdin,output,close:()=>{stdin.destroy();output.destroy();}});
  return {calls,runtime:new CodexChat({connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1'})};
}

test('pinned inventory retains forced unified_exec with shell_tool explicitly disabled, across both pages and scopes',async()=>{
  const f=fixture((method,response)=>{if(method==='mcpServerStatus/list')response.data=[];});
  try{
    assert.equal(pinned.features.length,150);
    assert.equal((await f.runtime.execute(input,new AbortController().signal)).kind,'outcomeUnknown');
    const pages=f.calls.filter(call=>call.method==='experimentalFeature/list');
    assert.deepEqual(pages.map(call=>call.params.cursor),[null,'100',null,'100']);
    assert.deepEqual(pages.map(call=>call.params.threadId),[undefined,undefined,'synthetic-thread','synthetic-thread']);
  }finally{await f.runtime.dispose();}
});

test('configured disabled MCP rows are inert globally and in thread scope',async()=>{
  const f=fixture();
  try{
    assert.equal((await f.runtime.execute(input,new AbortController().signal)).kind,'outcomeUnknown');
    assert.equal(f.calls.filter(call=>call.method==='mcpServerStatus/list').length,2);
  }finally{await f.runtime.dispose();}
});

test('model availability uses the invocation slug, never a catalog display identifier',async()=>{
  for(const [id,model,expected] of [['display-entry','gpt-6-astra','outcomeUnknown'],['gpt-6-astra','other','notStarted']]){
    const f=fixture((method,response)=>{if(method==='model/list')response.data=[{...pinned.models[0],id,model}];});
    try{assert.equal((await f.runtime.execute(input,new AbortController().signal)).kind,expected);}
    finally{await f.runtime.dispose();}
  }
});

function setPath(record:Record<string,unknown>,path:string,value:unknown){
  const keys=path.split('.'),last=keys.pop()!;
  const parent=keys.reduce((current,key)=>object(current[key]),record);
  if(value===undefined)delete parent[last];else parent[last]=value;
}
async function refused(mutate:Mutate,thread=false){
  const f=fixture(mutate);
  try{
    assert.equal((await f.runtime.execute(input,new AbortController().signal)).kind,'notStarted');
    assert.equal(f.calls.some(call=>call.method==='thread/start'),thread);
    assert.equal(f.calls.some(call=>call.method==='turn/start'),false);
  }finally{await f.runtime.dispose();}
}

const requiredConfig=[
  ['analytics.enabled',true],['check_for_update_on_startup',true],['model_provider','other'],['notify',['synthetic-command']],
  ['web_search','live'],['project_doc_max_bytes',1],['skills.include_instructions',true],['skills.bundled.enabled',true],
  ['cloud.skills.enabled',true],['orchestrator.mcp.enabled',true],['mcp_servers.spritepipe.enabled',true],
  ['mcp_servers.node_repl.enabled',true],['mcp_servers.computer-use.enabled',true],
] as const;
for(const [path,unsafe] of requiredConfig)for(const mode of ['missing','changed'] as const){
  test(`config ${path} ${mode} stops before discovery and dispatch`,async()=>{
    const f=fixture((method,response)=>{if(method==='config/read')setPath(object(response.config),path,mode==='missing'?undefined:unsafe);});
    try{
      await assert.rejects(f.runtime.preflight(new AbortController().signal),error=>{
        assert.deepEqual(redactedStartupFailure(error,{stage:'session',check:'runtime-preflight'}),{stage:'config',check:path});return true;
      });
      assert.equal(f.calls.some(call=>['experimentalFeature/list','mcpServerStatus/list','thread/start','turn/start'].includes(call.method??'')),false);
    }finally{await f.runtime.dispose();}
  });
}
for(const [path,value] of [
  ['mcp_servers',{...pinned.config.mcp_servers,ambient:{}}],['model_providers',{openai:{}}],
  ['chatgpt_base_url','https://unapproved.invalid'],['openai_base_url','https://unapproved.invalid'],
  ['otel',{exporter:{'otlp-http':{endpoint:'https://unapproved.invalid'}}}],['otel',{trace_exporter:'otlp-grpc'}],
  ['otel',{metrics_exporter:'otlp-grpc'}],['otel',{log_user_prompt:true}],['forced_login_method','api'],
  ['instructions','ambient'],['developer_instructions','ambient'],['compact_prompt','ambient'],
  ['browser_use',{cdp_url:'https://unapproved.invalid'}],['computer_use',{display:'ambient'}],
] as const)test(`ambient config ${path} ${JSON.stringify(value)} is refused`,()=>refused((method,response)=>{
  if(method==='config/read')setPath(object(response.config),path,value);
}));

const disabledFeatures=['apps','plugins','hooks','plugin_hooks','remote_control','remote_models','memories','multi_agent','multi_agent_v2','shell_tool','js_repl','skill_search','skill_mcp_dependency_install','skill_env_var_dependency_prompt','image_generation','browser_use','browser_use_external','browser_use_full_cdp_access','computer_use'];
for(const name of [...disabledFeatures,'skip_host_skill_discovery','unified_exec'])for(const mode of ['missing','nonboolean','changed'] as const){
  if(name==='unified_exec'&&mode==='changed')continue;
  for(const thread of [false,true])test(`${thread?'thread':'global'} feature ${name} ${mode} cannot be inferred safe`,()=>refused((method,response,request)=>{
    if(method!=='experimentalFeature/list'||Boolean(request.params.threadId)!==thread)return;
    const rows=response.data as Record<string,unknown>[];
    response.data=mode==='missing'?rows.filter(row=>row.name!==name):rows.map(row=>row.name===name?{...row,enabled:mode==='nonboolean'?'false':name!=='skip_host_skill_discovery'}:row);
  },thread));
}
test('a managed false unified_exec remains compatible with shell_tool false',async()=>{
  const f=fixture((method,response)=>{if(method==='experimentalFeature/list')response.data=(response.data as Record<string,unknown>[]).map(row=>row.name==='unified_exec'?{...row,enabled:false}:row);});
  try{assert.deepEqual(await f.runtime.preflight(new AbortController().signal),{model:'gpt-6-astra',runtimeVersion:'0.157.1'});}
  finally{await f.runtime.dispose();}
});

for(const thread of [false,true])for(const [field,value] of [
  ['name','unknown'],['runtimeStatus','ready'],['runtimeStatus','starting'],['runtimeStatus','notStarted'],['runtimeStatus',undefined],
  ['authStatus','oAuth'],['authStatus',undefined],['pluginId','plugin'],['serverInfo',{}],['serverCapabilities',{}],
  ['toolsError','synthetic-error'],['tools',{dangerous:{}}],['tools',undefined],['resources',[{}]],['resourceTemplates',[{}]],
  ['pluginId',undefined],['httpOrigin',undefined],['serverInfo',undefined],['serverCapabilities',undefined],['toolsError',undefined],
  ['resources',undefined],['resourceTemplates',undefined],
] as const)test(`${thread?'thread':'global'} MCP ${field} ${String(value)} remains refused`,()=>refused((method,response,request)=>{
  if(method==='mcpServerStatus/list'&&Boolean(request.params.threadId)===thread)setPath((response.data as Record<string,unknown>[])[0],field,value);
},thread));
test('MCP duplicate configured names are refused',()=>refused((method,response)=>{
  if(method==='mcpServerStatus/list'){const rows=response.data as unknown[];rows.push(rows[0]);}
}));
test('MCP global null and thread disabled statuses cannot be interchanged',async()=>{
  for(const thread of [false,true])await refused((method,response,request)=>{
    if(method==='mcpServerStatus/list'&&Boolean(request.params.threadId)===thread)(response.data as Record<string,unknown>[])[0].runtimeStatus=thread?null:'disabled';
  },thread);
});

for(const method of ['experimentalFeature/list','mcpServerStatus/list','model/list'])for(const mode of ['missing-cursor','empty-cursor','numeric-cursor','cycle','nine-pages','oversized','nonarray'] as const){
  test(`${method} pagination rejects ${mode}`,()=>refused((current,response,request)=>{
    if(current!==method)return;
    if(mode==='missing-cursor')delete response.nextCursor;
    if(mode==='empty-cursor')response.nextCursor='';
    if(mode==='numeric-cursor')response.nextCursor=100;
    if(mode==='cycle')response.nextCursor='0';
    if(mode==='nine-pages'){response.data=[];response.nextCursor=String(Number(request.params.cursor??0)+1);}
    if(mode==='oversized')response.data=Array(101).fill({});
    if(mode==='nonarray')response.data={};
  }));
}
test('a forbidden feature on the final page is checked',()=>refused((method,response,request)=>{
  if(method==='experimentalFeature/list'&&request.params.cursor==='100')(response.data as unknown[]).push({name:'shell_tool',enabled:true});
}));
test('exact model on a later page is found without selecting the first catalog entry',async()=>{
  const f=fixture((method,response,request)=>{
    if(method==='model/list'&&request.params.cursor===null){response.data=[{...pinned.models[0],model:'other'}];response.nextCursor='1';}
    else if(method==='model/list')response.data=pinned.models;
  });
  try{assert.equal((await f.runtime.preflight(new AbortController().signal)).model,'gpt-6-astra');}
  finally{await f.runtime.dispose();}
});
for(const account of [{account:null,requiresOpenaiAuth:true},{account:{type:'apiKey'},requiresOpenaiAuth:true},{account:{type:'chatgpt'},requiresOpenaiAuth:false},{account:{type:'chatgpt'}},{}]){
  test(`unconfirmed ChatGPT identity ${JSON.stringify(account)} stops dispatch`,()=>refused((method,response)=>{
    if(method==='account/read'){for(const key of Object.keys(response))delete response[key];Object.assign(response,account);}
  }));
}
test('a model id alone cannot confirm invocation availability',()=>refused((method,response)=>{
  if(method==='model/list')response.data=[{id:'gpt-6-astra'}];
}));

for(const [path,unsafe] of [
  ['thread.id',3],['thread.ephemeral',false],['model','other'],['modelProvider','other'],['cwd','/different'],
  ['approvalPolicy','never'],['sandbox.type','dangerFullAccess'],['sandbox.networkAccess',true],['instructionSources',['/ambient/AGENTS.md']],
] as const)for(const mode of ['missing','changed'] as const)test(`thread policy ${path} ${mode} stops before turn dispatch`,()=>refused((method,response)=>{
  if(method==='thread/start')setPath(response,path,mode==='missing'?undefined:unsafe);
},true));

test('the complete request contract preserves controls and exposes the unresolved restricted-read incompatibility',async()=>{
  const f=fixture();
  try{
    const result=await f.runtime.execute(input,new AbortController().signal);
    assert.equal(result.kind,'outcomeUnknown');
    const params=(method:string)=>f.calls.find(call=>call.method===method)!.params;
    assert.deepEqual(params('initialize').capabilities,{experimentalApi:true,requestAttestation:false});
    assert.deepEqual(params('account/read'),{refreshToken:false});
    assert.deepEqual(params('config/read'),{includeLayers:false,cwd:'/synthetic/empty'});
    const start=params('thread/start');
    for(const key of ['environments','dynamicTools','selectedCapabilityRoots'])assert.deepEqual(start[key],[]);
    assert.equal(start.ephemeral,true);assert.equal(start.modelProvider,'openai');assert.equal(start.sandbox,'read-only');
    assert.equal(start.allowProviderModelFallback,undefined);
    const turn=params('turn/start');
    assert.deepEqual(turn.environments,[]);assert.equal(turn.approvalPolicy,'untrusted');assert.equal(turn.model,'gpt-6-astra');
    assert.deepEqual(turn.sandboxPolicy,{type:'readOnly',networkAccess:false,access:{type:'restricted',includePlatformDefaults:false,readableRoots:['/synthetic/empty']}});
    assert.equal(f.calls.some(call=>call.method?.includes('login')||call.method?.includes('config/write')),false);
  }finally{await f.runtime.dispose();}
});
