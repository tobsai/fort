import { serveOpenAIPlanExecutor } from '../openai-plan-executor.ts';

// Routes every provider HTTPS request to the test's local fake server; the executor code is unchanged.
const [, , state, origin] = process.argv;
const fake = async (url, init) => { const target = new URL(url); return fetch(origin + '/' + target.host + target.pathname + target.search, init); };
try { await serveOpenAIPlanExecutor({ input: process.stdin, output: process.stdout, hostId: 'plan-fixture-host', stateDirectory: state, fetch: fake }); }
catch { process.exitCode = 1; }
