import { Command } from 'commander';
import { existsSync, rmSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { bold, dim, green, yellow, red, cyan } from '../utils/format.js';

export function createResetCommand(): Command {
  const cmd = new Command('reset')
    .description('Reset Fort data (agents, memory, tasks, etc.)');

  cmd
    .command('all')
    .description('Remove all Fort data and start fresh')
    .option('--yes', 'Skip confirmation')
    .action(async (opts) => {
      const fortDir = join(homedir(), '.fort');

      if (!existsSync(fortDir)) {
        console.log(dim('\n  Nothing to reset — .fort directory does not exist.\n'));
        return;
      }

      if (!opts.yes) {
        console.log(bold('\n  This will permanently delete:\n'));
        showContents(fortDir);
        console.log(`\n  ${yellow('This cannot be undone.')}`);
        console.log(`  Run with ${cyan('--yes')} to confirm.\n`);
        return;
      }

      // Stop services first
      const { execSync } = await import('node:child_process');
      for (const port of [4001, 4077]) {
        try {
          const pids = execSync(`lsof -ti :${port} 2>/dev/null`, { encoding: 'utf-8' }).trim();
          for (const pid of pids.split('\n').filter(Boolean)) {
            try { process.kill(parseInt(pid, 10), 'SIGTERM'); } catch {}
          }
        } catch {}
      }

      rmSync(fortDir, { recursive: true, force: true });
      console.log(`\n  ${green('✓')} All Fort data removed.`);
      console.log(`  Run ${cyan('fort init')} to start fresh.\n`);
    });

  cmd
    .command('setup')
    .description('Reset agents and data but keep your API key — re-run the setup wizard')
    .option('--yes', 'Skip confirmation')
    .action(async (opts) => {
      const fortDir = join(homedir(), '.fort');
      if (!existsSync(fortDir)) {
        console.log(dim('\n  Nothing to reset — .fort directory does not exist.\n'));
        return;
      }

      // These are kept
      const keepFiles = ['.env'];

      // These are wiped
      const wipeTargets = ['data', 'agents'];

      if (!opts.yes) {
        console.log(bold('\n  Reset Setup (keep API key)\n'));
        console.log('  Will delete:');
        for (const target of wipeTargets) {
          const p = join(fortDir, target);
          if (existsSync(p)) {
            console.log(`    ${red('✗')} ${target}/ ${dim(`(${countRecursive(p)} files)`)}`);
          }
        }
        console.log('\n  Will keep:');
        for (const keep of keepFiles) {
          const p = join(fortDir, keep);
          if (existsSync(p)) {
            console.log(`    ${green('✓')} ${keep}`);
          }
        }
        console.log(`\n  After reset, run ${cyan('fort init')} to re-run the wizard.`);
        console.log(`  ${yellow('Run with')} ${cyan('--yes')} ${yellow('to confirm.')}\n`);
        return;
      }

      // Stop services first
      const { execSync } = await import('node:child_process');
      for (const port of [4001, 4077]) {
        try {
          const pids = execSync(`lsof -ti :${port} 2>/dev/null`, { encoding: 'utf-8' }).trim();
          for (const pid of pids.split('\n').filter(Boolean)) {
            try { process.kill(parseInt(pid, 10), 'SIGTERM'); } catch {}
          }
        } catch {}
      }

      for (const target of wipeTargets) {
        const p = join(fortDir, target);
        if (existsSync(p)) rmSync(p, { recursive: true, force: true });
      }

      console.log(`\n  ${green('✓')} Setup reset. API key preserved at ${cyan('~/.fort/.env')}`);
      console.log(`  Run ${cyan('fort init')} to start the setup wizard again.\n`);
    });

  cmd
    .command('agents')
    .description('Remove all agents and their SOUL.md files')
    .option('--yes', 'Skip confirmation')
    .action(async (opts) => {
      const agentsDir = join(homedir(), '.fort', 'agents');
      resetDir(agentsDir, 'agents', opts.yes);
    });

  cmd
    .command('memory')
    .description('Clear the memory graph')
    .option('--yes', 'Skip confirmation')
    .action(async (opts) => {
      const dbPath = join(homedir(), '.fort', 'data', 'memory.db');
      resetFile(dbPath, 'memory database', opts.yes);
    });

  cmd
    .command('tasks')
    .description('Clear all tasks and threads')
    .option('--yes', 'Skip confirmation')
    .action(async (opts) => {
      const dataDir = join(homedir(), '.fort', 'data');
      let removed = 0;
      for (const db of ['threads.db', 'threads.db-wal', 'threads.db-shm']) {
        const p = join(dataDir, db);
        if (existsSync(p)) {
          if (!opts.yes) {
            console.log(`\n  Would delete: ${dim(p)}`);
            continue;
          }
          rmSync(p, { force: true });
          removed++;
        }
      }
      if (!opts.yes && removed === 0) {
        console.log(dim('\n  Nothing to reset.\n'));
        return;
      }
      if (!opts.yes) {
        console.log(`\n  ${yellow('Run with')} ${cyan('--yes')} ${yellow('to confirm.')}\n`);
        return;
      }
      console.log(`\n  ${green('✓')} Tasks and threads cleared.\n`);
    });

  cmd
    .command('tokens')
    .description('Clear token usage history')
    .option('--yes', 'Skip confirmation')
    .action(async (opts) => {
      const dbPath = join(homedir(), '.fort', 'data', 'tokens.db');
      resetFile(dbPath, 'token database', opts.yes);
    });

  // Show what exists without --yes
  cmd
    .action(() => {
      const fortDir = join(homedir(), '.fort');
      if (!existsSync(fortDir)) {
        console.log(dim('\n  Nothing to reset — .fort directory does not exist.\n'));
        return;
      }

      console.log(bold('\n  Fort Reset\n'));
      console.log('  Choose what to reset:\n');
      console.log(`    ${cyan('fort reset setup')}     ${dim('— Wipe agents + data, keep API key, re-run wizard')}`);
      console.log(`    ${cyan('fort reset all')}       ${dim('— Remove everything including API key')}`);
      console.log(`    ${cyan('fort reset agents')}    ${dim('— Remove all agents and SOUL.md files')}`);
      console.log(`    ${cyan('fort reset memory')}    ${dim('— Clear the memory graph')}`);
      console.log(`    ${cyan('fort reset tasks')}     ${dim('— Clear tasks and threads')}`);
      console.log(`    ${cyan('fort reset tokens')}    ${dim('— Clear token usage history')}`);
      console.log();
      console.log(dim('  All commands require --yes to actually delete.\n'));
    });

  return cmd;
}

function showContents(dir: string, indent = '    '): void {
  if (!existsSync(dir)) return;
  const entries = readdirSync(dir);
  for (const entry of entries) {
    const full = join(dir, entry);
    const isDir = statSync(full).isDirectory();
    if (isDir) {
      const count = countRecursive(full);
      console.log(`${indent}${entry}/ ${dim(`(${count} files)`)}`);
    } else {
      const size = statSync(full).size;
      console.log(`${indent}${entry} ${dim(`(${formatSize(size)})`)}`);
    }
  }
}

function countRecursive(dir: string): number {
  let count = 0;
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      count += countRecursive(full);
    } else {
      count++;
    }
  }
  return count;
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
}

function resetDir(dir: string, label: string, confirmed: boolean): void {
  if (!existsSync(dir)) {
    console.log(dim(`\n  No ${label} to reset.\n`));
    return;
  }
  if (!confirmed) {
    console.log(bold(`\n  Would delete ${label} directory:\n`));
    showContents(dir);
    console.log(`\n  ${yellow('Run with')} ${cyan('--yes')} ${yellow('to confirm.')}\n`);
    return;
  }
  rmSync(dir, { recursive: true, force: true });
  console.log(`\n  ${green('✓')} All ${label} removed.\n`);
}

function resetFile(path: string, label: string, confirmed: boolean): void {
  // Also handle WAL/SHM files
  const files = [path, `${path}-wal`, `${path}-shm`].filter(existsSync);
  if (files.length === 0) {
    console.log(dim(`\n  No ${label} to reset.\n`));
    return;
  }
  if (!confirmed) {
    const size = files.reduce((sum, f) => sum + statSync(f).size, 0);
    console.log(`\n  Would delete: ${dim(path)} ${dim(`(${formatSize(size)})`)}`);
    console.log(`\n  ${yellow('Run with')} ${cyan('--yes')} ${yellow('to confirm.')}\n`);
    return;
  }
  for (const f of files) rmSync(f, { force: true });
  console.log(`\n  ${green('✓')} ${label.charAt(0).toUpperCase() + label.slice(1)} cleared.\n`);
}
