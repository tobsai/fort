import { request } from 'node:http';
import { existsSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';

export async function probeCore(privateRoot) {
  const { createApplication } = await import('./Core/service/openai.js');
  const dataDirectory = join(privateRoot, 'core');
  mkdirSync(dataDirectory, { mode: 0o700 }); process.chdir(dataDirectory);
  let service, executions = 0;
  const open = async () => {
    service = createApplication({ dataDirectory,
      provider: { provider: 'sandbox-core-fixture', name: 'Sandbox core fixture', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText'] },
      runtime: { async execute(input) { executions++; return { kind: 'completed', text: input.text, effectiveModel: null }; }, async dispose() {} }
    });
    await service.start({ listener: { kind: 'privateSocket', name: 'core.sock' } });
  };
  const api = (path, headers = {}, body) => new Promise((resolve, reject) => {
    const encoded = body === undefined ? undefined : JSON.stringify(body);
    const call = request({ socketPath: 'core.sock', path, agent: false, method: encoded === undefined ? 'GET' : 'POST',
      headers: { 'Content-Type': 'application/json', ...(encoded ? { 'Content-Length': Buffer.byteLength(encoded) } : {}), ...headers } }, response => {
      const parts = []; let bytes = 0;
      response.on('data', part => { bytes += part.length; if (bytes > 65_536) call.destroy(new Error('Synthetic response limit')); else parts.push(part); });
      response.once('end', () => { try { resolve({ status: response.statusCode, body: JSON.parse(Buffer.concat(parts).toString()) }); } catch (error) { reject(error); } });
      response.once('error', reject);
    });
    call.setTimeout(2000, () => call.destroy(new Error('Synthetic request timeout')));
    call.once('error', reject); call.end(encoded);
  });
  try {
    await open();
    const initial = service.installation;
    const denied = await api('/v1/installation');
    const enrolled = await api('/v1/devices/enroll', {}, { ...service.issueEnrollment(), deviceName: 'Synthetic sandbox peer' });
    const headers = { Authorization: 'Bearer ' + enrolled.body.deviceToken, 'X-Fort-Installation-Id': initial.installationId };
    const foreign = await api('/v1/installation', { ...headers, 'X-Fort-Installation-Id': 'foreign' });
    const origin = await api('/v1/installation', { ...headers, Origin: 'https://untrusted.invalid' });
    const authenticated = await api('/v1/installation', headers);
    const agent = (await api('/v1/agents', headers, { name: 'Synthetic sandbox Home', model: 'exact-synthetic', instructions: 'Echo the exact synthetic request.' })).body;
    const path = '/v1/conversations/' + agent.homeConversationId;
    const command = { requestId: 'sandbox-core-turn', text: '  Private cedar 🌲\r\n', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: initial.recoveryEpoch, skills: [] };
    const accepted = await api(path + '/messages', headers, command);
    await service.dispatchNext();
    const snapshot = (await api(path + '/snapshot', headers)).body;
    const events = (await api('/v1/events?after=0&recoveryEpoch=' + initial.recoveryEpoch, headers)).body;
    await service.close(); await open();
    const recovered = await api(path + '/messages', headers, command);
    const result = {
      coreAuthentication: denied.status === 401 && enrolled.status === 201 && foreign.status === 401 && origin.status === 403 && authenticated.status === 200,
      coreExactMessages: accepted.status === 201 && snapshot.runs[0]?.state === 'completed' && JSON.stringify(snapshot.messages.map(message => message.text)) === JSON.stringify([command.text, command.text]),
      coreReplay: events.semantics === 'invalidation' && events.events.some(event => event.kind === 'messageAccepted'),
      coreRestart: service.installation.installationId === initial.installationId && service.installation.recoveryEpoch === initial.recoveryEpoch && recovered.status === 201,
      coreNoDuplicate: executions === 1 && JSON.stringify(recovered.body.receipt) === JSON.stringify(accepted.body.receipt) && recovered.body.snapshot.messages.length === 2
    };
    await service.close();
    return { ...result, coreSocketAbsent: !existsSync('core.sock') };
  } finally { await service?.close(); process.chdir(privateRoot); }
}
