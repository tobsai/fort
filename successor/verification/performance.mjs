import { spawn, spawnSync } from 'node:child_process';
import { mkdirSync, copyFileSync, readFileSync, writeFileSync, realpathSync, symlinkSync, createReadStream } from 'node:fs';
import { stat } from 'node:fs/promises';
import { join, dirname, isAbsolute } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';
import { createHash } from 'node:crypto';
import { cpus, totalmem, platform, release } from 'node:os';
import { runLoad } from './performance-client.mjs';

const root = fileURLToPath(new URL('../', import.meta.url));
const { values } = parseArgs({ options: { mode: { type: 'string', default: 'core' }, destination: { type: 'string' } }, strict: true });
if (!['smoke', 'core'].includes(values.mode) || !values.destination || !isAbsolute(values.destination)) throw new Error('Choose smoke/core and a new absolute destination');
if (process.version !== 'v26.5.0') throw new Error('Use Node 26.5.0');
if (values.mode === 'core' && process.platform !== 'darwin') throw new Error('The adopted core baseline requires the recorded Mac measurement host');
mkdirSync(values.destination, { mode: 0o700 });
const destination = realpathSync(values.destination), mode = values.mode, samples = mode === 'core' ? 5 : 1;
const run = (executable, args, cwd = root) => {
  const started = performance.now(), result = spawnSync(executable, args, { cwd, encoding: 'utf8', timeout: 60_000, maxBuffer: 2_000_000 });
  if (result.error || result.status !== 0) throw new Error('Measurement command failed: ' + executable + '\n' + result.stderr);
  return { ms: performance.now() - started, stdout: result.stdout };
};
const stats = samplesMs => {
  const sorted = [...samplesMs].sort((a, b) => a - b);
  return { samplesMs, p50Ms: sorted[Math.ceil(sorted.length * 0.5) - 1], p95Ms: sorted[Math.ceil(sorted.length * 0.95) - 1] };
};
const progress = phase => process.stderr.write(JSON.stringify({ phase }) + '\n');
const clean = [];
let artifact;
for (let index = 0; index < samples; index++) {
  progress('core-build-' + (index + 1)); artifact = join(destination, 'core-' + index);
  clean.push(run(process.execPath, [join(root, 'packaging/core-artifact.ts'), '--destination', artifact]).ms);
}
const manifest = JSON.parse(readFileSync(join(artifact, 'manifest.json'), 'utf8'));
const source = join(destination, 'incremental-source'); mkdirSync(source);
for (const file of manifest.files.filter(file => file.path.endsWith('.js'))) {
  const relative = file.path.slice(0, -3) + '.ts', target = join(source, relative);
  mkdirSync(dirname(target), { recursive: true }); copyFileSync(join(root, relative), target);
}
for (const name of ['package.json', 'tsconfig.json', 'tsconfig.core.json']) copyFileSync(join(root, name), join(source, name));
symlinkSync(join(root, 'node_modules'), join(source, 'node_modules'), 'dir');
const compileArgs = [join(root, 'node_modules/typescript/bin/tsc'), '-p', join(source, 'tsconfig.core.json'), '--incremental', '--tsBuildInfoFile', join(destination, 'incremental.tsbuildinfo'), '--outDir', join(destination, 'incremental-output')];
run(process.execPath, compileArgs);
const editable = join(source, 'service/openai.ts'), original = readFileSync(editable, 'utf8'), marker = 'No runtime is connected.';
if (original.split(marker).length !== 2) throw new Error('The incremental edit marker changed; review the workload');
const incremental = [];
for (let index = 0; index < samples; index++) {
  writeFileSync(editable, original.replace(marker, marker + ' '.repeat(index + 1)));
  incremental.push(run(process.execPath, compileArgs).ms);
}
const worker = join(destination, 'worker.mjs'); copyFileSync(join(root, 'verification/performance-worker.mjs'), worker);
const metrics = mode === 'core' ? join(destination, 'process-metrics') : undefined;
if (metrics) run('/usr/bin/xcrun', ['clang', '-Wall', '-Wextra', '-Werror', join(root, 'verification/process-metrics.c'), '-o', metrics]);
let activeChild;
const terminate = () => { if (activeChild?.pid) { try { process.kill(-activeChild.pid, 'SIGTERM'); } catch {} } };
process.once('SIGTERM', terminate); process.once('SIGINT', terminate);
async function fingerprint(directory) {
  const files = [];
  for (const name of ['openai-candidate-b.sqlite', 'openai-candidate-b.sqlite-wal']) {
    const path = join(directory, name);
    try {
      const info = await stat(path), hash = createHash('sha256');
      for await (const bytes of createReadStream(path)) hash.update(bytes);
      files.push({ name, bytes: info.size, sha256: hash.digest('hex') });
    } catch (error) { if (error.code !== 'ENOENT') throw error; }
  }
  return files;
}
async function measureWorker(action, data) {
  mkdirSync(data, { recursive: true, mode: 0o700 });
  const started = performance.now();
  const child = spawn(process.execPath, ['--jitless', '--no-experimental-strip-types', '--permission', '--allow-net', '--allow-fs-read=' + destination,
    '--allow-fs-write=' + data, worker, artifact, data, action, mode], { cwd: destination, env: { PATH: '/usr/bin:/bin', LANG: 'C', TZ: 'UTC' }, stdio: ['pipe', 'pipe', 'pipe'], detached: true });
  activeChild = child;
  return await new Promise((resolve, reject) => {
    let pending = '', stderr = '', report, loadResult, dispatched, coldMs, before, after, filesBefore, filesAfter, timeout;
    const owner = new AbortController();
    const stop = error => { clearTimeout(timeout); owner.abort(error); dispatched?.reject(error); try { if (child.pid) process.kill(-child.pid, 'SIGKILL'); } catch {} reject(error); };
    timeout = setTimeout(() => stop(new Error('Measurement worker timed out')), 300_000);
    child.stderr.on('data', bytes => { stderr += bytes.toString(); if (stderr.length > 1_000_000) stop(new Error('Worker diagnostic limit exceeded')); });
    child.stdout.on('data', bytes => {
      pending += bytes.toString(); if (pending.length > 1_000_000) { stop(new Error('Worker output limit exceeded')); return; }
      let newline;
      while ((newline = pending.indexOf('\n')) >= 0) {
        const line = pending.slice(0, newline); pending = pending.slice(newline + 1);
        try {
          const message = JSON.parse(line);
          if (message.kind === 'coldReady') {
            coldMs = performance.now() - started;
            if (action === 'load') runLoad({ ...message.client, mode, signal: owner.signal,
              dispatch: () => new Promise((resolve, reject) => { dispatched = { resolve, reject }; child.stdin.write('dispatch\n'); }),
              baseline: () => fingerprint(data), onProgress: turns => process.stderr.write(JSON.stringify({ phase: 'load', turns }) + '\n')
            }).then(result => { loadResult = result; child.stdin.write('finishLoad\n'); }).catch(stop);
          }
          else if (message.kind === 'progress') process.stderr.write(JSON.stringify(message) + '\n');
          else if (message.kind === 'dispatched') {
            if (!dispatched) throw new Error('Unrequested dispatch evidence');
            dispatched.resolve(); dispatched = undefined;
          } else if (message.kind === 'idleReady') {
            fingerprint(data).then(files => {
              filesBefore = files;
              before = metrics ? JSON.parse(run(metrics, [String(child.pid)]).stdout) : null;
              progress('idle-observation'); child.stdin.write('startIdle\n');
            }).catch(stop);
          } else if (message.kind === 'idleDone') {
            after = metrics ? JSON.parse(run(metrics, [String(child.pid)]).stdout) : null;
            fingerprint(data).then(files => { filesAfter = files; child.stdin.write('finishIdle\n'); }).catch(stop);
          } else if (message.kind === 'report') report = message.data;
          else throw new Error('Unknown measurement event');
        } catch (error) { stop(error); return; }
      }
    });
    child.stdin.on('error', stop);
    child.once('error', stop);
    child.once('close', code => {
      clearTimeout(timeout); if (activeChild === child) activeChild = undefined;
      if (code !== 0 || !report || coldMs === undefined || (action === 'load' && !loadResult)) stop(new Error('Measurement worker failed: ' + stderr));
      else resolve({ ...loadResult, ...report, coldMs, osBefore: before, osAfter: after,
        ...(loadResult ? { workload: { ...loadResult.workload, baselineLedgerBytes: loadResult.baselineFiles.reduce((sum, file) => sum + file.bytes, 0), finalLedgerBytes: filesAfter.reduce((sum, file) => sum + file.bytes, 0) },
          idle: { ...report.idle, filesBefore, filesAfter, ledgerChanged: JSON.stringify(filesBefore) !== JSON.stringify(filesAfter) } } : {}) });
    });
  });
}
const emptyStarts = [];
for (let index = 0; index < samples; index++) emptyStarts.push(await measureWorker('startup', join(destination, 'empty-' + index)));
progress('load-and-request-measurement');
const loadedData = join(destination, 'loaded'), load = await measureWorker('load', loadedData);
const loadedStarts = [];
for (let index = 0; index < samples; index++) loadedStarts.push(await measureWorker('startup', loadedData));
const startStats = rows => ({ cold: stats(rows.map(row => row.coldMs)), warm: stats(rows.map(row => row.warmMs)), coldApplicationOnly: stats(rows.map(row => row.applicationMs)) });
const idle = { ...load.idle, osBefore: load.osBefore, osAfter: load.osAfter,
  interruptWakeupsPerSecond: load.osBefore && load.osAfter ? (load.osAfter.interruptWakeups - load.osBefore.interruptWakeups) / (load.idle.elapsedMs / 1000) : null,
  packageIdleWakeupsPerSecond: load.osBefore && load.osAfter ? (load.osAfter.packageIdleWakeups - load.osBefore.packageIdleWakeups) / (load.idle.elapsedMs / 1000) : null };
const report = {
  format: 1, mode, budgetEligible: mode === 'core', measuredAt: new Date().toISOString(),
  sourceCommit: run('git', ['rev-parse', 'HEAD']).stdout.trim(),
  artifact: { path: artifact, manifestSha256: createHash('sha256').update(readFileSync(join(artifact, 'manifest.json'))).digest('hex'), files: manifest.files.length, bytes: manifest.files.reduce((sum, file) => sum + file.bytes, 0) },
  runtime: { node: process.version, executable: process.execPath, jit: false, controllerPid: process.pid, platform: platform(), release: release(), cpuModel: cpus()[0]?.model, cpuCount: cpus().length, memoryBytes: totalmem() },
  toolchain: { typescript: run(process.execPath, [join(root, 'node_modules/typescript/bin/tsc'), '--version']).stdout.trim(),
    macOS: process.platform === 'darwin' ? run('/usr/bin/sw_vers', ['-productVersion']).stdout.trim() : null,
    hardwareModel: process.platform === 'darwin' ? run('/usr/sbin/sysctl', ['-n', 'hw.model']).stdout.trim() : null,
    clang: metrics ? run('/usr/bin/xcrun', ['clang', '--version']).stdout.split('\n')[0] : null },
  measurementFiles: ['performance.mjs', 'performance-worker.mjs', 'performance-client.mjs', 'process-metrics.c'].map(path => ({ path, sha256: createHash('sha256').update(readFileSync(join(root, 'verification', path))).digest('hex') })),
  conditions: { cache: 'Fresh build outputs and processes; OS file caches are uncontrolled. Incremental edits use a copied source tree.', topology: 'Compiled core and immediate provider fixture in one owned Node worker. Controller process owns the synthetic HTTP client. Acceptance/page timings include client HTTP round trips; execution includes dispatch IPC.', diagnostics: 'Parent hashes ledger/WAL outside the idle counter interval; hashing does not allocate in the measured worker.', gc: 'No forced garbage collection.', percentile: 'Nearest rank; p95 of five samples is the maximum.', nativeBuilds: 'Not measured by this core command.', bundledHost: 'Not measured; Node is supplied from the selected development executable.' },
  build: { clean: stats(clean), incremental: stats(incremental) }, startup: { empty: startStats(emptyStarts), loaded: startStats(loadedStarts) },
  workload: load.workload, requests: { clientPid: load.clientPid, acceptance: stats(load.acceptanceMs), executionWithFixture: stats(load.executionMs), page: stats(load.pageMs) }, idle,
  provider: { kind: 'synthetic-in-process', realProviderUsed: false, separateProcessLatencyMeasured: false },
  budgets: mode === 'core' ? { cleanBuild: stats(clean).p95Ms <= 10_000, incrementalBuild: stats(incremental).p95Ms <= 3_000,
    coldStartup: startStats(loadedStarts).cold.p95Ms <= 2_000, warmStartup: startStats(loadedStarts).warm.p95Ms <= 500,
    acceptance: stats(load.acceptanceMs).p95Ms <= 25, historyPage: stats(load.pageMs).p95Ms <= 50,
    idleRSS: Math.max(idle.rssBefore, idle.rssAfter) <= 128 * 1024 * 1024, idleCPU: idle.cpuPercentOneCore <= 0.5,
    interruptWakeups: idle.interruptWakeupsPerSecond !== null && idle.interruptWakeupsPerSecond <= 1,
    idleWrites: !idle.ledgerChanged, ledgerBytes: load.workload.baselineLedgerBytes <= 64 * 1024 * 1024 } : null
};
const reportPath = join(destination, 'report.json'); writeFileSync(reportPath, JSON.stringify(report, null, 2) + '\n', { mode: 0o600, flag: 'wx' });
process.stdout.write(JSON.stringify({ reportPath, mode, budgetEligible: report.budgetEligible, budgets: report.budgets }) + '\n');
if (report.budgets && Object.values(report.budgets).some(passed => !passed)) process.exitCode = 2;
