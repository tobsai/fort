import test from 'node:test';
import assert from 'node:assert/strict';
import { foundationRouteAllowed, foundationLaunch } from './foundation-policy.mjs';
test('worker escalation and vendor integrations remain unavailable', () => {
  for (const path of ['/api/bots/b1/messages/guarded', '/api/bots/b1/tasks', '/api/threads/t1/respond', '/api/cloud/home', '/api/composio/connect', '/api/boat/start', '/api/voice/start']) {
    assert.equal(foundationRouteAllowed(path), false);
  }
});
test('foundation cannot start an unreviewed runtime', () => {
 assert.throws(() => foundationLaunch(), /source-only/);
});

// Exercise the imported public authorization seam, not a mirrored allowlist.
const { serviceAllowed } = await import('./server/request-auth.ts');
test('service-loopback auth cannot reach Slack worker mutation or read routes', () => {
 for (const path of ['/api/bots/b1/messages/guarded', '/api/bots/b1/tasks', '/api/bots/b1/requests/r1', '/api/bots/b1/requests/r1/interrupt', '/api/bots/b1/queue/q1', '/api/threads/t1/respond', '/api/internal/action']) {
  for (const method of ['GET','POST','DELETE']) assert.equal(serviceAllowed(method, path), false);
 }
 assert.equal(serviceAllowed('GET', '/api/health'), false);
 assert.equal(serviceAllowed('POST', '/api/health'), false);
});
test('server entry gate executes the source-only refusal', async () => {
 await assert.rejects(import('./server/fort-foundation-gate.ts'), /source-only/);
});
