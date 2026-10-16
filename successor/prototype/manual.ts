import {readFileSync,writeFileSync,mkdirSync,lstatSync,existsSync,unlinkSync,openSync,closeSync} from 'node:fs';
import {join,normalize,isAbsolute} from 'node:path';
import {fileURLToPath} from 'node:url';
import {createHash} from 'node:crypto';
import {createInterface} from 'node:readline';
import {requireCandidateBActivation} from './activation.ts';
import {launchCandidateCodex} from './owned-codex.ts';
import {CandidatePacket} from './session.ts';
import {createNativeSession} from './native-session.ts';
import {redactedStartupFailure} from '../runner/startup-diagnostic.ts';

function verifyPins(root:string,app:string) {
  const pins=JSON.parse(readFileSync(join(root,'successor/evidence/2026-10-16-candidate-b-third/pins.json'),'utf8')) as { sourceFiles:{path:string;sha256:string}[];appFiles:{path:string;sha256:string}[];sourceCommit:string };
  for(const [base,files] of [[root,pins.sourceFiles],[app,pins.appFiles]] as const){
    if(!Array.isArray(files)||files.length===0||files.length>2_000)throw new Error('Invalid candidate pins');
    for(const file of files){
      if(isAbsolute(file.path)||normalize(file.path)!==file.path||file.path.startsWith('..')||file.path.includes('\0'))throw new Error('Invalid candidate pin path');
      const path=join(base,file.path);
      if(!lstatSync(path).isFile()||createHash('sha256').update(readFileSync(path)).digest('hex')!==file.sha256)throw new Error('Candidate bytes differ from the reviewed pins');
    }
  }
  if(!/^[a-f0-9]{40}$/.test(pins.sourceCommit))throw new Error('Invalid source identity');
}

export async function runManualPacket():Promise<void> {
  requireCandidateBActivation();
  await runPreparedPacket();
}
async function runPreparedPacket():Promise<void> {
  const root=fileURLToPath(new URL('../..',import.meta.url));
  const app=join(root,'successor/.build/candidate-b-review-mac/Build/Products/Debug/FortMac.app');
  verifyPins(root,app);
  const sessionRoot='/Users/tobiasgunn/dev/fort/.local/prototype-candidate-b-third-20261016';
  const workspace=join(sessionRoot,'empty'),temporaryDirectory=join(sessionRoot,'tmp');
  for(const path of [sessionRoot,workspace,temporaryDirectory]){mkdirSync(path,{recursive:true,mode:0o700});if(lstatSync(path).isSymbolicLink())throw new Error('Session path cannot be a symlink');}
  const native=createNativeSession({directory:'/Users/tobiasgunn/Library/Application Support/FortCandidateB',appExecutable:join(app,'Contents/MacOS/FortMac')});
  const packet=new CandidatePacket({root:sessionRoot,attempt:'third-20261016',native,launchRuntime:remainingWorkMs=>launchCandidateCodex({workspace,temporaryDirectory,remainingWorkMs})});
  const terminal=createInterface({input:process.stdin,output:process.stdout});
  const cancellation=new AbortController();
  let active:Awaited<ReturnType<CandidatePacket['start']>>|undefined;
  const enrollmentFile=join(sessionRoot,'local-enrollment.json');let wroteEnrollment=false;
  const removeEnrollment=()=>{if(wroteEnrollment&&existsSync(enrollmentFile)){unlinkSync(enrollmentFile);wroteEnrollment=false;}};
  const stop=()=>{cancellation.abort();terminal.close();};
  process.on('SIGINT',stop);process.on('SIGTERM',stop);
  terminal.once('close',stop);
  process.stdout.write('Prepared one-start third attempt. Type start only when the UI operator is ready. No restart is authorized. Commands: start, status, done (after quitting Fort), abort.\n');
  try{
    for await(const line of terminal){
      const command=line.trim();
      if(command==='start'){
        if(active){process.stdout.write('A session is already active.\n');continue;}
        const session=await packet.start(cancellation.signal);active=session;
        if(cancellation.signal.aborted){await session.abort();break;}
        if(session.enrollment){
          const file=openSync(enrollmentFile,'wx',0o600);wroteEnrollment=true;
          try{writeFileSync(file,JSON.stringify({baseURL:session.baseURL,...session.enrollment},null,2)+'\n');}finally{closeSync(file);}
        }
        process.stdout.write(`Preflight passed. Open this exact app: ${app}\n`);
        process.stdout.write(session.enrollment?`Enter local enrollment from ${enrollmentFile}; never paste it into chat.\n`:'Use the existing candidate connection; do not enroll again.\n');
        process.stdout.write(`Work cutoff: ${new Date(session.workDeadline).toISOString()}. Quit Fort fully before typing done.\n`);
        void session.closed.then(state=>{
          removeEnrollment();if(active===session)active=undefined;
          process.stdout.write(JSON.stringify({phase:state.phase,starts:state.starts,receiptCount:state.receipts.length,credentialCleanup:state.credentialCleanup})+'\n');
          if(state.phase!=='between')terminal.close();
        });
      }else if(command==='status'){
        if(active)process.stdout.write(JSON.stringify({runs:active.status().runs.map(run=>({requestId:run.requestId,runId:run.runId,state:run.state,selectedModel:run.selectedModel,effectiveModel:null}))})+'\n');
        else process.stdout.write('No active session.\n');
      }else if(command==='done'){
        if(active)await active.finish();else process.stdout.write('No active session.\n');
      }else if(command==='abort'){
        if(active)await active.abort();terminal.close();break;
      }else process.stdout.write('Use start, status, done, or abort. No prompt text is accepted here.\n');
    }
  }finally{
    process.off('SIGINT',stop);process.off('SIGTERM',stop);
    cancellation.abort();
    await active?.abort();await packet.cancelBetweenSessions();removeEnrollment();terminal.close();
  }
}

if(import.meta.main){try{await runManualPacket();}catch(error){process.stderr.write(JSON.stringify({failure:redactedStartupFailure(error,{stage:'session',check:'entry'})})+'\n');process.stderr.write('Candidate session refused or stopped. Inspect its redacted state; do not retry automatically.\n');process.exitCode=1;}}
