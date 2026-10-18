import { readFileSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:net';
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
process.stdout.write(JSON.stringify({ sqlite, outsideReadDenied, outsideWriteDenied, loopbackListenDenied }) + '\n');
if (sqlite !== 'ready' || !outsideReadDenied || !outsideWriteDenied || !loopbackListenDenied) process.exitCode = 1;
