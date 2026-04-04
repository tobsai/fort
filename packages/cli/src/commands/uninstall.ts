import { Command } from 'commander';
import { existsSync, rmSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { execSync } from 'node:child_process';
import { bold, dim, green, yellow, red, cyan } from '../utils/format.js';

export function createUninstallCommand(): Command {
  return new Command('uninstall')
    .description('Fully uninstall Fort from this system')
    .option('--yes', 'Skip confirmation and execute')
    .option('--keep-data', 'Remove CLI but keep ~/.fort/ data')
    .action(async (opts) => {
      const fortDir = join(homedir(), '.fort');
      const dryRun = !opts.yes;

      console.log(bold('\n  Fort Uninstall\n'));

      // ── Gather info ──────────────────────────────────────────
      const runningPids = findRunningServices();
      const fortBin = findFortBinary();
      const dataExists = existsSync(fortDir);
      const removeData = dataExists && !opts.keepData;

      // ── Show plan ────────────────────────────────────────────
      if (runningPids.length > 0) {
        console.log(`  ${yellow('●')} Stop running Fort services ${dim(`(${runningPids.length} process${runningPids.length > 1 ? 'es' : ''})`)}`);
      }

      if (fortBin) {
        console.log(`  ${red('✗')} Remove CLI binary ${dim(fortBin)}`);
      } else {
        console.log(`  ${dim('○')} CLI binary not found in PATH (already removed or not linked)`);
      }

      if (removeData) {
        const fileCount = countRecursive(fortDir);
        const totalSize = dirSize(fortDir);
        console.log(`  ${red('✗')} Remove ${cyan('~/.fort/')} ${dim(`(${fileCount} files, ${formatSize(totalSize)})`)}`);
        showContents(fortDir);
      } else if (opts.keepData && dataExists) {
        console.log(`  ${green('✓')} Keep ${cyan('~/.fort/')} ${dim('(--keep-data)')}`);
      } else if (!dataExists) {
        console.log(`  ${dim('○')} No ~/.fort/ directory found`);
      }

      console.log();

      if (dryRun) {
        console.log(`  ${yellow('This is a dry run.')} Nothing has been removed.`);
        console.log(`  Run ${cyan('fort uninstall --yes')} to execute.`);
        if (dataExists && !opts.keepData) {
          console.log(`  Run ${cyan('fort uninstall --keep-data --yes')} to keep your data.`);
        }
        console.log();
        return;
      }

      // ── Execute ──────────────────────────────────────────────

      // 1. Stop services
      if (runningPids.length > 0) {
        console.log(dim('  Stopping Fort services...'));
        for (const pid of runningPids) {
          try {
            process.kill(pid, 'SIGTERM');
          } catch {}
        }
        // Brief wait for graceful shutdown
        await new Promise((resolve) => setTimeout(resolve, 1000));
        console.log(`  ${green('✓')} Services stopped`);
      }

      // 2. Remove data
      if (removeData) {
        rmSync(fortDir, { recursive: true, force: true });
        console.log(`  ${green('✓')} Removed ~/.fort/`);
      }

      // 3. Unlink CLI
      if (fortBin) {
        try {
          execSync('npm unlink -g @fort/cli 2>/dev/null', { stdio: 'ignore' });
          console.log(`  ${green('✓')} Unlinked fort CLI`);
        } catch {
          // npm unlink may fail if it was linked differently; try removing the symlink directly
          try {
            rmSync(fortBin, { force: true });
            console.log(`  ${green('✓')} Removed fort binary at ${dim(fortBin)}`);
          } catch {
            console.log(`  ${yellow('⚠')} Could not remove ${fortBin} — remove it manually`);
          }
        }
      }

      // ── Done ─────────────────────────────────────────────────
      console.log(bold('\n  Fort has been uninstalled.\n'));
      if (opts.keepData) {
        console.log(`  Your data is still at ${cyan('~/.fort/')}`);
        console.log(`  To reinstall: ${cyan('npm link --workspace=packages/cli')} then ${cyan('fort init')}`);
      } else {
        console.log(`  To reinstall: ${cyan('npm link --workspace=packages/cli')} then ${cyan('fort init')}`);
      }
      console.log();
    });
}

function findRunningServices(): number[] {
  const pids: number[] = [];
  for (const port of [4001, 4077]) {
    try {
      const output = execSync(`lsof -ti :${port} 2>/dev/null`, { encoding: 'utf-8' }).trim();
      for (const line of output.split('\n').filter(Boolean)) {
        const pid = parseInt(line, 10);
        if (!isNaN(pid)) pids.push(pid);
      }
    } catch {}
  }
  return [...new Set(pids)];
}

function findFortBinary(): string | null {
  try {
    const result = execSync('which fort 2>/dev/null', { encoding: 'utf-8' }).trim();
    return result || null;
  } catch {
    return null;
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

function dirSize(dir: string): number {
  let size = 0;
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      size += dirSize(full);
    } else {
      size += statSync(full).size;
    }
  }
  return size;
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
}

function showContents(dir: string): void {
  if (!existsSync(dir)) return;
  const entries = readdirSync(dir);
  for (const entry of entries) {
    const full = join(dir, entry);
    const isDir = statSync(full).isDirectory();
    if (isDir) {
      const count = countRecursive(full);
      console.log(`      ${dim(entry + '/')} ${dim(`(${count} files)`)}`);
    } else {
      const size = statSync(full).size;
      console.log(`      ${dim(entry)} ${dim(`(${formatSize(size)})`)}`);
    }
  }
}
