import { spawnFixtureProcess } from './process.ts';

export type FixtureDispatch = { protocolVersion: 1; provider: 'fixture'; selectedModel: 'fixture-echo-v1'; attemptId: string; generation: number; dispatchToken: string; text: string };
export type FixtureEvent = { protocolVersion: 1; attemptId: string; generation: number; dispatchToken: string; kind: 'started' | 'completed'; effectiveModel?: string; text?: string };
export async function executeFixture(dispatch: FixtureDispatch, onEvent: (event: FixtureEvent) => void): Promise<void> {
  const child = spawnFixtureProcess(new URL('./fixture-worker.mjs', import.meta.url));
  let buffer = ''; let outputBytes = 0; let started = false; let terminal = false; let protocolError: Error | undefined;
  const ended = new Promise<void>((resolve, reject) => {
    child.once('error', reject);
    child.stdout.setEncoding('utf8');
    child.stdout.on('data', chunk => {
      try {
        outputBytes += Buffer.byteLength(chunk); if (outputBytes > 1_000_000) throw new Error('Fixture output exceeds limit');
        buffer += chunk;
        for (;;) {
          const newline = buffer.indexOf('\n'); if (newline < 0) break;
          const event = JSON.parse(buffer.slice(0, newline)) as FixtureEvent; buffer = buffer.slice(newline + 1);
          if (event.protocolVersion !== 1 || event.attemptId !== dispatch.attemptId || event.generation !== dispatch.generation || event.dispatchToken !== dispatch.dispatchToken || terminal) throw new Error('Uncorrelated fixture event');
          if (event.kind === 'started' && !started) started = true;
          else if (event.kind === 'completed' && started && typeof event.text === 'string' && event.effectiveModel === dispatch.selectedModel) terminal = true;
          else throw new Error('Invalid fixture transition');
          onEvent(event);
        }
      } catch (error) { protocolError = error as Error; child.kill(); }
    });
    // Consume bounded diagnostics without recording runtime input or environment.
    child.stderr.resume();
    child.once('close', code => {
      if (protocolError) reject(protocolError);
      else if (code !== 0 || !terminal || buffer !== '') reject(new Error('Fixture ended without valid terminal evidence'));
      else resolve();
    });
  });
  child.stdin.on('error', () => {});
  child.stdin.end(JSON.stringify(dispatch));
  return ended;
}
