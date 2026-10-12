import { createFixtureService } from './service.ts';
const [dataDirectory, point] = process.argv.slice(2);
const service = createFixtureService({ dataDirectory, checkpoint(actual) { if (actual === point) process.kill(process.pid, 'SIGKILL'); } });
await service.start();
process.send?.({ ready: true });
process.on('message', async command => {
  if (command === 'dispatch') { await service.dispatchNextFixture(); process.send?.({ unexpectedlySurvived: true }); }
});
