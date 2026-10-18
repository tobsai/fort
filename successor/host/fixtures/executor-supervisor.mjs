import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { createApplication } from '../../service/openai.ts';
import { ExecutorProcess } from '../../runner/executor-process.ts';

const [directory, guardian, scenario] = process.argv.slice(2);
if (!directory || !guardian || !scenario) throw new Error('Select exact synthetic inputs');
const worker = fileURLToPath(new URL('../../runner/fixtures/executor-worker.mjs', import.meta.url));
const lifecycle = { kind: 'macParentExit', guardian };
const runtime = new ExecutorProcess({ executable: process.execPath,
  args: scenario === 'parent-death' ? [worker, scenario] : ['--permission', '--allow-fs-read=' + worker, worker, scenario],
  cwd: directory, env: {}, lifecycle });
lifecycle.guardian = directory + '/unselected-missing-guardian';
const service = createApplication({ dataDirectory: directory, attemptTimeoutMs: scenario === 'terminal-then-hang' ? 200 : 30_000,
  provider: { provider: 'parent-exit-fixture', name: 'Parent exit fixture', runtimeVersion: '1', hostId: 'synthetic', session: 'stateless', capabilities: ['plainText', 'cancel'] }, runtime });
const lifetime = setTimeout(() => { void service.close(); }, 20_000);
try {
  await service.start();
  const enrollment = await fetch(service.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...service.issueEnrollment(), deviceName: 'Synthetic executor owner' }) });
  if (enrollment.status !== 201) throw new Error('Synthetic enrollment failed');
  const device = await enrollment.json();
  const api = async (path, body) => {
    const response = await fetch(service.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + device.deviceToken, 'X-Fort-Installation-Id': service.installation.installationId }, body: body === undefined ? undefined : JSON.stringify(body) });
    if (!response.ok) throw new Error('Synthetic request failed');
    return response.json();
  };
  const agent = await api('/v1/agents', { name: 'Synthetic Home', model: 'exact-fixture-model', instructions: 'Echo.' });
  const path = '/v1/conversations/' + agent.homeConversationId;
  const accepted = await api(path + '/messages', { requestId: 'parent-exit', text: 'Exact synthetic reply', recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: service.installation.recoveryEpoch, skills: [] });
  const dispatching = service.dispatchNext();
  if (scenario === 'await-cancel') await api('/v1/runs/' + accepted.receipt.runId + '/cancel', { recoveryEpoch: service.installation.recoveryEpoch });
  await dispatching;
  const snapshot = await api(path + '/snapshot');
  writeFileSync('owner-done.json', JSON.stringify({ state: snapshot.runs[0].state, messages: snapshot.messages.map(message => message.text), exit: runtime.lastExit }), { mode: 0o600 });
} finally { clearTimeout(lifetime); await service.close(); }
