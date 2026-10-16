import { PassThrough, Writable } from 'node:stream';
import { CodexConnection } from '/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016/successor/runner/codex-rpc.ts';
import { CodexChat } from '/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016/successor/runner/codex-chat.ts';
const output = new PassThrough();
const abort = new AbortController();
const methods = [];
let sentAfterAbort = false;
let connection;
const emit = value => output.write(JSON.stringify(value)+'\n');
const input = new Writable({write(chunk, _encoding, done) {
 const f = JSON.parse(chunk.toString()); methods.push(f.method);
 const reply = value => emit({id:f.id,result:value});
 if(f.method==='initialize') reply({userAgent:'synthetic'});
 if(f.method==='account/read') reply({account:{type:'chatgpt'},requiresOpenaiAuth:true});
 if(f.method==='model/list') reply({data:[{id:'selected'}],nextCursor:null});
 if(f.method==='thread/start') { reply({thread:{id:'thread',ephemeral:true},model:'selected',modelProvider:'openai',cwd:'/synthetic/empty',approvalPolicy:'untrusted',sandbox:{type:'readOnly',networkAccess:false},instructionSources:[]}); abort.abort(); }
 if(f.method==='turn/start') { sentAfterAbort = abort.signal.aborted; queueMicrotask(()=>connection.close()); }
 done();
}});
connection = new CodexConnection({input,output,close(){output.destroy();input.destroy();}});
const runtime = new CodexChat({connection,workspace:'/synthetic/empty',runtimeVersion:'0.157.1'});
const result = await runtime.execute({runId:'run',requestId:'request',conversationId:'home',text:'sample',history:[],configuration:{agentId:'agent',agentName:'Agent',bindingRevisionId:'binding',selectedModel:'selected',instructions:'Reply.',skills:[]}},abort.signal);
console.log(JSON.stringify({methods,sentAfterAbort,result}));
connection.close();
