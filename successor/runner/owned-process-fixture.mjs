import { createInterface } from 'node:readline';
import { syntheticConfig,syntheticFeatures } from './codex-preflight-fixture.ts';
process.stdout.on('error',()=>{});
if (process.env.FORT_SYNTHETIC_MODE === 'ignore-term') process.on('SIGTERM',()=>{ process.stdout.write('{"method":"late/synthetic"}\n'); });
const reply=(id,result)=>process.stdout.write(JSON.stringify({ id,result })+'\n');
createInterface({ input:process.stdin }).on('line',line=>{
  const frame=JSON.parse(line);
  if(frame.method==='ping' || frame.method==='initialize')reply(frame.id,{ synthetic:true });
  if(frame.method==='config/read')reply(frame.id,{ config:syntheticConfig() });
  if(frame.method==='experimentalFeature/list')reply(frame.id,{ data:syntheticFeatures(),nextCursor:null });
  if(frame.method==='mcpServerStatus/list')reply(frame.id,{ data:[],nextCursor:null });
  if(frame.method==='account/read')reply(frame.id,{ account:{ type:'chatgpt' },requiresOpenaiAuth:true });
  if(frame.method==='model/list')reply(frame.id,{ data:[{ id:'gpt-6-astra',model:'gpt-6-astra' }],nextCursor:null });
  if(frame.method==='permissionProfile/list')reply(frame.id,{data:[{id:'fort_candidate_b_no_files',description:null,allowed:true}],nextCursor:null});
  if(frame.method==='thread/start')reply(frame.id,{ thread:{ id:'synthetic-thread',ephemeral:true },model:'gpt-6-astra',modelProvider:'openai',cwd:frame.params.cwd,approvalPolicy:'untrusted',approvalsReviewer:'user',activePermissionProfile:{id:'fort_candidate_b_no_files',extends:null},runtimeWorkspaceRoots:[],sandbox:{ type:'readOnly',networkAccess:false },instructionSources:[] });
  if(frame.method==='turn/start'){
    reply(frame.id,{turn:{id:'synthetic-turn',status:'inProgress'}});
    if(process.env.FORT_SYNTHETIC_MODE==='complete'){
      process.stdout.write(JSON.stringify({method:'item/completed',params:{threadId:'synthetic-thread',turnId:'synthetic-turn',item:{id:'reply',type:'agentMessage',phase:'final_answer',text:'Synthetic replacement reply'}}})+'\n');
      process.stdout.write(JSON.stringify({method:'turn/completed',params:{threadId:'synthetic-thread',turn:{id:'synthetic-turn',status:'completed',error:null}}})+'\n');
    }
  }
});
setInterval(()=>{},1000);
