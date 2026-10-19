import { writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import {Server as HTTPSServer} from 'node:https';
const [modulePath, directory, start = 'restore', behavior = 'ordinary', network = 'disabled'] = process.argv.slice(2);
if (!modulePath || !directory || !['restore', 'resume'].includes(start)) throw new Error('Select exact synthetic host inputs');
const { serveOwnedCore } = await import(pathToFileURL(modulePath).href);
if(behavior==='network-slow-open' || behavior==='network-stalled-open') {
  const listen=HTTPSServer.prototype.listen;
  HTTPSServer.prototype.listen=function(options,ready){return listen.call(this,options,()=>{if(behavior==='network-slow-open')setTimeout(ready,100);});};
}
if(behavior==='network-close-failure') {
  const close=HTTPSServer.prototype.close;
  HTTPSServer.prototype.close=function(){return close.call(this);};
}
const abort = new AbortController();
process.on('message', value => { if (value === 'abort') abort.abort(); });
if (behavior === 'aborted') abort.abort();
writeFileSync('core-pid.json', JSON.stringify(process.pid), { mode: 0o600 });
const lifetime = setTimeout(() => abort.abort(), 15_000);
try {
  const options={ dataDirectory: directory, start, input: process.stdin, output: process.stdout, signal: abort.signal, network: network === 'network' ? 'explicit' : undefined,
    provider: { provider: 'owned-core-fixture', name: 'Owned core fixture', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
    runtime: behavior === 'missing-runtime' ? undefined : {
      async execute(input) { process.send?.({ kind: 'executed' }); return { kind: 'completed', text: input.text, effectiveModel: null }; },
      async dispose() { process.send?.({ kind: 'disposed' }); if (behavior === 'cleanup-failure') throw new Error('Synthetic dispose failure'); },
    },
  };
  const serving=serveOwnedCore(options);
  if(behavior==='network-option-mutation')options.network='explicit';
  await serving;
} catch { process.exitCode = 1; }
finally { clearTimeout(lifetime); process.disconnect?.(); }
