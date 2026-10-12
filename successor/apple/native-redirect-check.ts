import { createServer } from 'node:http';
import { once } from 'node:events';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

const executable = process.argv[2];
if (!executable) throw new Error('Pass the built FortNativeCheck executable');
let forwarded = 0;
const installation = {contractVersion:'1.0',installationId:'synthetic-installation',recoveryEpoch:'epoch',mode:'fixture',serviceName:'test'};
const sink = createServer(async (req,res) => {
  for await (const _chunk of req) { /* Drain synthetic body without logging it. */ }
  forwarded += 1;
  res.writeHead(200,{'Content-Type':'application/json'});
  res.end(JSON.stringify(req.url?.includes('enroll') ? {installation,deviceId:'fake',deviceToken:'synthetic',expiresAt:'2030-01-01T00:00:00Z'} : installation));
});
sink.listen(0,'127.0.0.1'); await once(sink,'listening');
const sinkAddress = sink.address();
if (!sinkAddress || typeof sinkAddress === 'string') throw new Error('Expected ephemeral loopback port');
let status = 307;
const initial = createServer((req,res) => { res.writeHead(status,{Location:`http://127.1:${sinkAddress.port}${req.url}`}); res.end(); });
initial.listen(0,'127.0.0.1'); await once(initial,'listening');
const address = initial.address();
if (!address || typeof address === 'string') throw new Error('Expected ephemeral loopback port');
const directory = await mkdtemp(join(tmpdir(),'fort-redirect-check-'));
try {
  const path = join(directory,'config.json');
  await writeFile(path,JSON.stringify({baseURL:`http://127.0.0.1:${address.port}`,installationId:installation.installationId,codeA:'',codeB:'',stateDirectory:pathToFileURL(directory).href}),{mode:0o600});
  for (status of [301,302,303,307,308]) {
    const result = await promisify(execFile)(resolve(executable),['redirect',path],{env:{PATH:'/usr/bin:/bin'},timeout:20_000});
    if (forwarded !== 0) throw new Error('A redirect destination received a native request');
    process.stdout.write(`${status}: ${result.stdout}`);
  }
} finally {
  await Promise.all([new Promise<void>(r=>initial.close(()=>r())),new Promise<void>(r=>sink.close(()=>r()))]);
  await rm(directory,{recursive:true,force:true});
}
