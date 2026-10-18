import assert from 'node:assert/strict';
import { pathToFileURL } from 'node:url';
import { join } from 'node:path';
import { createHash } from 'node:crypto';
import { request } from 'node:http';
const [artifact, root] = process.argv.slice(2);
const { createApplication } = await import(pathToFileURL(join(artifact, 'service/openai.js')).href);
const fingerprint = value => createHash('sha256').update(JSON.stringify(value)).digest('hex');
const reply = (history, text) => `${history.filter(message => message.role === 'user').length}|${text}`;
function localHTTP(url, options) {
  return new Promise((resolve, reject) => {
    const outgoing = request(url, { method: options.method, headers: options.headers, timeout: 5000 }, incoming => {
      let body = '';
      incoming.setEncoding('utf8');
      incoming.on('data', part => { body += part; if (body.length > 1_000_000) outgoing.destroy(new Error('Fixture response limit')); });
      incoming.on('error', reject);
      incoming.on('end', () => resolve({ status: incoming.statusCode, ok: incoming.statusCode >= 200 && incoming.statusCode < 300, json: async () => JSON.parse(body) }));
    });
    outgoing.on('error', reject); outgoing.on('timeout', () => outgoing.destroy(new Error('Fixture HTTP timeout')));
    outgoing.end(options.body);
  });
}

function batchRuntime() {
  return {
    async execute(input, signal) {
      if (input.text === 'hold for cancellation') {
        await new Promise(resolve => signal.addEventListener('abort', resolve, { once: true }));
        return { kind: 'canceled', message: 'Fixture batch canceled.' };
      }
      return { kind: 'completed', text: reply(input.history, input.text), effectiveModel: null };
    },
    async models() { return { provider: 'batch-fixture', state: 'ready', checkedAt: '2026-10-18T00:00:00Z', models: [{ id: 'exact-fixture-model', name: 'Fixture model', availability: 'available' }] }; },
    async dispose() {}
  };
}

class SessionEngine {
  sessions = new Map();
  next = 0;
  open(records) { const handle = ++this.next; this.sessions.set(handle, structuredClone(records)); return handle; }
  transcript(handle) { return this.sessions.get(handle); }
  async *append(handle, text, signal) {
    const records = this.sessions.get(handle);
    assert.ok(records);
    if (text === 'hold for cancellation') {
      await new Promise(resolve => signal.addEventListener('abort', resolve, { once: true }));
      yield { canceled: true }; return;
    }
    const result = reply(records, text);
    for (const piece of result) yield { piece };
    records.push({ role: 'user', text }, { role: 'assistant', text: result });
  }
}
function sessionRuntime() {
  const backend = new SessionEngine(), handles = new Map();
  return {
    async execute(input, signal) {
      const key = input.conversationId + ':' + input.configuration.bindingRevisionId;
      const history = input.history.map(({ role, text }) => ({ role, text }));
      let handle = handles.get(key);
      if (handle === undefined) { handle = backend.open(history); handles.set(key, handle); }
      assert.equal(fingerprint(backend.transcript(handle)), fingerprint(history));
      let text = '';
      for await (const event of backend.append(handle, input.text, signal)) {
        if (event.canceled) return { kind: 'canceled', message: 'Fixture session canceled.' };
        text += event.piece;
      }
      return { kind: 'completed', text, effectiveModel: null };
    },
    async dispose() { handles.clear(); backend.sessions.clear(); }
  };
}

const factories = [
  { provider: 'batch-fixture', session: 'stateless', capabilities: ['plainText', 'cancel', 'modelCatalog'], create: batchRuntime },
  { provider: 'session-fixture', session: 'sessionful', capabilities: ['plainText', 'cancel'], create: sessionRuntime }
];
for (const fixture of factories) {
  const provider = { name: fixture.provider, provider: fixture.provider, session: fixture.session, runtimeVersion: 'fixture-1', hostId: 'synthetic', capabilities: fixture.capabilities };
  const dataDirectory = join(root, fixture.provider);
  let executions = 0;
  const runtime = () => {
    const adapter = fixture.create();
    return { ...adapter, async execute(input, signal) {
      assert.equal(input.attempt.runId, input.runId);
      assert.equal(input.attempt.bindingRevisionId, input.configuration.bindingRevisionId);
      assert.equal(input.attempt.selectedModel, 'exact-fixture-model');
      executions++; return adapter.execute(input, signal);
    } };
  };
  let app = createApplication({ dataDirectory, provider, runtime: runtime() });
  try {
    await app.start();
    const response = await localHTTP(app.baseURL + '/v1/devices/enroll', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...app.issueEnrollment(), deviceName: 'Synthetic artifact contract' }) });
    const device = await response.json(); assert.equal(response.status, 201);
    const headers = { 'Content-Type': 'application/json', Authorization: `Bearer ${device.deviceToken}`, 'X-Fort-Installation-Id': app.installation.installationId };
    async function api(path, body) {
      const response = await localHTTP(app.baseURL + path, { method: body === undefined ? 'GET' : 'POST', headers, body: body === undefined ? undefined : JSON.stringify(body) });
      assert.ok(response.ok, `HTTP ${response.status}`); return response.json();
    }
    const agent = await api('/v1/agents', { name: 'Independent', model: 'exact-fixture-model', instructions: 'Synthetic fixture.' });
    assert.equal(agent.binding.provider, fixture.provider);
    const catalog = await api('/v1/models');
    assert.equal(catalog.provider, fixture.provider);
    assert.equal(catalog.state, fixture.session === 'stateless' ? 'ready' : 'unavailable');
    const path = `/v1/conversations/${agent.homeConversationId}`;
    const command = (requestId, text) => ({ requestId, text, recipientAgentId: agent.agentId, bindingRevisionId: agent.binding.bindingRevisionId, recoveryEpoch: app.installation.recoveryEpoch, skills: [] });
    const inputs = ['  Exact e\u0301 🌲\r\n', 'second turn', 'after restart'];
    let first;
    for (let index = 0; index < inputs.length; index++) {
      if (index === 2) { await app.close(); app = createApplication({ dataDirectory, provider, runtime: runtime() }); await app.start(); }
      const request = command('contract-' + index, inputs[index]);
      const acceptance = await api(path + '/messages', request); first ??= acceptance;
      assert.equal(await app.dispatchNext(), true);
      const repeated = await api(path + '/messages', request);
      assert.deepEqual(repeated.receipt, acceptance.receipt);
      const snapshot = await api(path + '/snapshot');
      assert.equal(snapshot.messages.at(-2).text, inputs[index]);
      assert.equal(snapshot.messages.at(-1).text, `${index}|${inputs[index]}`);
      assert.equal(executions, index + 1);
    }
    assert.deepEqual((await api('/v1/requests/contract-0')).receipt, first.receipt);
    const accepted = await api(path + '/messages', command('contract-cancel', 'hold for cancellation'));
    const dispatch = app.dispatchNext();
    await api(`/v1/runs/${accepted.receipt.runId}/cancel`, { recoveryEpoch: app.installation.recoveryEpoch });
    await dispatch;
    const stopped = await api('/v1/requests/contract-cancel');
    assert.equal(stopped.run.state, 'canceled');
    assert.equal(stopped.snapshot.messages.filter(message => message.requestId === 'contract-cancel').length, 1);
  } finally { await app.close(); }
}
process.stdout.write(JSON.stringify({ providers: factories.map(fixture => fixture.provider), contractsPassed: 2, providerImplementationsPresent: false }) + '\n');
