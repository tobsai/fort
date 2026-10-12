// Trusted echo fixture only. This worker has no provider or tool implementation.
let input = '';
for await (const chunk of process.stdin) {
  input += chunk;
  if (Buffer.byteLength(input) > 1_000_000) throw new Error('Fixture dispatch exceeds limit');
}
const dispatch = JSON.parse(input);
if (dispatch.protocolVersion !== 1 || dispatch.provider !== 'fixture' || dispatch.selectedModel !== 'fixture-echo-v1' || typeof dispatch.text !== 'string') throw new Error('Unsupported fixture dispatch');
const identity = { protocolVersion: 1, attemptId: dispatch.attemptId, generation: dispatch.generation, dispatchToken: dispatch.dispatchToken };
process.stdout.write(JSON.stringify({ ...identity, kind: 'started' }) + '\n');
process.stdout.write(JSON.stringify({ ...identity, kind: 'completed', effectiveModel: 'fixture-echo-v1', text: '[Fixture reply — not a real agent]\n' + dispatch.text }) + '\n');
