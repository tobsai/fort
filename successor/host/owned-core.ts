import { lstatSync, realpathSync } from 'node:fs';
import { isAbsolute } from 'node:path';
import type { Readable, Writable } from 'node:stream';
import { createApplication } from '../service/openai.ts';
import type { ChatRuntime } from '../runner/chat-runtime.ts';
import { servePrivateChannel } from './private-channel.ts';
import type {SecureNetworkControl} from '../contracts/secure-network.ts';

type CoreOptions = Parameters<typeof createApplication>[0];
export type OwnedCoreOptions = Pick<CoreOptions, 'dataDirectory' | 'provider' | 'context' | 'attemptTimeoutMs' | 'ownerPairingPolicy'> & {
  runtime: ChatRuntime;
  input: Readable;
  output: Writable;
  signal?: AbortSignal;
  start?: 'restore' | 'resume';
  network?: 'explicit';
};

export async function serveOwnedCore(options: OwnedCoreOptions): Promise<void> {
  const { runtime, dataDirectory, provider, context, attemptTimeoutMs, ownerPairingPolicy, start = 'restore', input, output, signal, network:networkMode } = options;
  if (!runtime || typeof runtime.execute !== 'function' || typeof runtime.dispose !== 'function') throw new Error('Select an explicit owned runtime');
  let service: ReturnType<typeof createApplication> | undefined;
  let network:SecureNetworkControl|undefined;
  try {
    signal?.throwIfAborted();
    if(networkMode!==undefined && networkMode!=='explicit')throw new Error('Select explicit network control or leave it disabled');
    if (start !== 'restore' && start !== 'resume') throw new Error('Select restore or explicit resume');
    if (!isAbsolute(dataDirectory)) throw new Error('Select an absolute private owner directory');
    const directory = lstatSync(dataDirectory);
    if (!directory.isDirectory() || !process.getuid || directory.uid !== process.getuid() || (directory.mode & 0o077) !== 0
      || realpathSync(dataDirectory) !== realpathSync(process.cwd())) throw new Error('The owned core requires its private ledger working directory');
    service = createApplication({ dataDirectory, provider, runtime, context, attemptTimeoutMs, autoRun: true,
      ...(networkMode==='explicit'?{ownerAccessChanged:()=>{void network?.reconcileOwner().catch(()=>{});}}:{}),
      ownerPairingPolicy: ownerPairingPolicy ?? { expiresAfterDays: null } });
    await service.start({ listener: { kind: 'privateSocket', name: 'core.sock' }, signal, resumeOwnerHost: start === 'resume' });
    const endpoint = service.endpoint;
    if (endpoint?.kind !== 'privateSocket') throw new Error('The owned core requires private IPC');
    const ownedService=service;
    if(networkMode==='explicit') {
      const {createSecureNetworkControl}=await import('./network-control.ts');
      network=createSecureNetworkControl({endpoint,authorize:(owner,operation)=>{
        const access=ownedService.localOwnerAccess(owner);return operation==='start'?access==='active':access!=='foreign';
      }});
    }
    await servePrivateChannel({ endpoint, input, output, signal, networkControl:network,
      ownerRecovery: service.recoverOwnerPairing,
      ownerSetup: { begin: service.beginOwnerSetup, complete: service.completeOwnerSetup, cancel: service.cancelOwnerSetup } });
  } finally {
    try {await network?.close();}
    finally {if (service) await service.close();else await runtime.dispose();}
  }
}
