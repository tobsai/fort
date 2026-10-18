import { readFileSync, writeFileSync } from 'node:fs';
import { createServer, createConnection } from 'node:net';
import { join } from 'node:path';
import { DatabaseSync } from 'node:sqlite';

const [privateRoot, outside, scenario] = process.argv.slice(2);
if (!privateRoot || !outside) throw new Error('Missing synthetic fixture paths');
if (scenario === 'hang') {
  process.stdout.write('synthetic child is holding its output pipe open\n');
  setInterval(() => {}, 1000);
  await new Promise(() => {});
}
const database = new DatabaseSync(join(privateRoot, 'fixture.sqlite'));
database.exec('create table messages(text text not null)');
database.prepare('insert into messages values (?)').run('ready');
const sqlite = database.prepare('select text from messages').get().text;
database.close();
const denied = action => {
  try { action(); return false; }
  catch (error) { return error.code === 'EPERM' || error.code === 'EACCES'; }
};
const outsideReadDenied = denied(() => readFileSync(outside));
const outsideWriteDenied = denied(() => writeFileSync(outside + '.write', 'synthetic-only'));
const loopbackListenDenied = await new Promise((resolve, reject) => {
  const server = createServer();
  server.once('error', error => resolve(error.code === 'EPERM' || error.code === 'EACCES'));
  server.listen(0, '127.0.0.1', () => server.close(() => resolve(false)));
  setTimeout(() => { server.close(); reject(new Error('Network probe timeout')); }, 2_000).unref();
});
process.chdir(privateRoot);
const privateSocket = await new Promise(resolve => {
  let client, settled = false;
  const server = createServer(socket => socket.end('private-ipc-ready'));
  const finish = (ready, failure = null) => {
    if (settled) return;
    settled = true; clearTimeout(timer); client?.destroy();
    server.close(() => resolve({ ready, failure }));
  };
  const timer = setTimeout(() => finish(false, 'timeout'), 2_000);
  server.once('error', error => finish(false, error.code ?? 'listen-failed'));
  server.listen('s', () => {
    let received = '';
    client = createConnection('s');
    client.setEncoding('utf8');
    client.on('data', bytes => { received += bytes; if (received.length > 64) finish(false, 'output-limit'); });
    client.once('error', error => finish(false, error.code ?? 'connect-failed'));
    client.once('end', () => finish(received === 'private-ipc-ready', received === 'private-ipc-ready' ? null : 'wrong-reply'));
  });
});
process.stdout.write(JSON.stringify({ sqlite, outsideReadDenied, outsideWriteDenied, loopbackListenDenied, privateSocketRoundTrip: privateSocket.ready, privateSocketFailure: privateSocket.failure }) + '\n');
if (sqlite !== 'ready' || !outsideReadDenied || !outsideWriteDenied || !loopbackListenDenied) process.exitCode = 1;
