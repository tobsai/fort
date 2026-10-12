// Test-only outer-host crash target. No service enrollment or provider access.
import { startSyntheticProcess } from './synthetic-process.ts';
import type { SyntheticDispatch } from '../contracts/runner.ts';

process.once('message',(dispatch:SyntheticDispatch)=>{
  const run=startSyntheticProcess(dispatch,{directory:process.argv[2],scenario:'blocked-input',deliver:async event=>({attemptId:event.attemptId,sequence:event.sequence,cursor:event.sequence,duplicate:false})});
  process.send?.({kind:'ownedProcess',pid:run.pid});
  setTimeout(()=>process.send?.({kind:'writeWindow'}),150);
  void run.result.then(()=>process.exit(0));
});
