import { request, Agent } from 'node:http';

export async function runLoad({ baseURL, installation, enrollment, dispatch, mode, baseline, onProgress, signal }) {
  const agent = new Agent({ keepAlive: true, maxSockets: 1 });
  try {
    let token;
    const api = (path, body, expected = 200) => new Promise((resolve, reject) => {
      signal.throwIfAborted();
      const encoded = body === undefined ? undefined : JSON.stringify(body);
      const call = request(baseURL + path, { agent, signal, timeout: 10_000, method: body === undefined ? 'GET' : 'POST', headers: {
        'Content-Type': 'application/json', ...(encoded ? { 'Content-Length': Buffer.byteLength(encoded) } : {}),
        ...(token ? { Authorization: 'Bearer ' + token, 'X-Fort-Installation-Id': installation.installationId } : {})
      } }, response => {
        const parts = [];
        response.on('data', bytes => parts.push(bytes));
        response.once('end', () => {
          if (response.statusCode !== expected) { reject(new Error('Unexpected benchmark HTTP status ' + response.statusCode)); return; }
          try { resolve(JSON.parse(Buffer.concat(parts).toString('utf8'))); } catch (error) { reject(error); }
        });
        response.once('error', reject);
      });
      call.once('timeout', () => call.destroy(new Error('Measurement HTTP timeout')));
      call.once('error', reject); call.end(encoded);
    });
    token = (await api('/v1/devices/enroll', { ...enrollment, deviceName: 'Performance fixture' }, 201)).deviceToken;
    const homes = [], homeCount = mode === 'core' ? 100 : 3, turnsPerHome = mode === 'core' ? 50 : 5, requestCount = mode === 'core' ? 100 : 3;
    for (let index = 0; index < homeCount; index++) homes.push(await api('/v1/agents', { name: 'Synthetic Home ' + index, model: 'synthetic-fixed', instructions: 'Produce the fixed synthetic reply.' }, 201));
    const submit = (home, requestId) => api('/v1/conversations/' + home.homeConversationId + '/messages', {
      requestId, text: 'u'.repeat(1024), recipientAgentId: home.agentId, bindingRevisionId: home.binding.bindingRevisionId,
      recoveryEpoch: installation.recoveryEpoch, skills: []
    }, 201);
    let turns = 0;
    const loadStart = performance.now();
    for (const [homeIndex, home] of homes.entries()) for (let turn = 0; turn < turnsPerHome; turn++) {
      await submit(home, `load-${homeIndex}-${turn}`); await dispatch();
      if (++turns % 500 === 0) onProgress(turns);
    }
    const loadMs = performance.now() - loadStart;
    const baselineFiles = await baseline();
    const acceptanceMs = [], executionMs = [], pageMs = [];
    for (let index = 0; index < requestCount; index++) {
      const home = homes[index % homes.length], start = performance.now();
      await submit(home, 'measured-' + index); acceptanceMs.push(performance.now() - start);
      const execute = performance.now(); await dispatch(); executionMs.push(performance.now() - execute);
    }
    for (let index = 0; index < requestCount; index++) {
      const start = performance.now();
      const page = await api('/v1/conversations/' + homes[index % homes.length].homeConversationId + '/snapshot');
      if (page.messages.length > 100 || (mode === 'core' && page.messages.length !== 100)) throw new Error('Unexpected history page size');
      pageMs.push(performance.now() - start);
    }
    const home = homes[0];
    const routine = await api('/v1/routines', { definition: { schemaVersion: 1, name: 'Future idle fixture', agentId: home.agentId,
      conversationId: home.homeConversationId, bindingRevisionId: home.binding.bindingRevisionId, instructions: 'Synthetic future work only.', inputs: [], skills: [],
      steps: [{ stepId: 'task', name: 'Task', type: 'task', instructions: 'Produce a synthetic reply.', inputNames: [], outputStepIds: [] }] } }, 201);
    const tomorrow = new Date(Date.now() + 86_400_000).toISOString().slice(0, 10);
    const schedule = await api('/v1/schedules', { definition: { schemaVersion: 1, routineId: routine.routineId, routineRevisionId: routine.revisionId,
      timezone: 'UTC', time: '12:00', kind: 'once', date: tomorrow, inputs: {}, missed: 'skip' } }, 201);
    const preview = await api('/v1/schedules/' + schedule.scheduleId + '/preview');
    await api('/v1/schedules/' + schedule.scheduleId + '/state', { recoveryEpoch: installation.recoveryEpoch, revisionId: schedule.revisionId, previewDigest: preview.digest, enable: true });
    return { clientPid: process.pid, baselineFiles, acceptanceMs, executionMs, pageMs, workload: { homes: homeCount, turnsPerHome, messageBytes: 1024, baselineMessages: turns * 2, finalMessages: (turns + requestCount) * 2, loadMs } };
  } finally { agent.destroy(); }
}
