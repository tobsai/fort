import type { RuntimeDescriptor } from '../contracts/v1.ts';

export const CODEX_RUNTIME: RuntimeDescriptor = {
  provider: 'codex', name: 'Codex', runtimeVersion: '0.163.0', hostId: 'local-codex',
  session: 'stateless', capabilities: ['plainText', 'modelCatalog', 'cancel', 'skills', 'memory']
};
