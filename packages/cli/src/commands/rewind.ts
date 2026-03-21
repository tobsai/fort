import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, cyan, yellow, green, red, table, timeAgo } from '../utils/format.js';

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

export function createRewindCommand(): Command {
  const cmd = new Command('rewind')
    .description('Backup & restore Fort state snapshots')
    .option('--to <id>', 'Restore to a specific snapshot')
    .option('--preview <id>', 'Preview what would change if restored')
    .option('--create [label]', 'Manually create a snapshot')
    .option('--config-only', 'Only restore config files')
    .option('--memory-only', 'Only restore memory database')
    .option('--limit <n>', 'Number of snapshots to show', parseInt)
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        // Create a snapshot
        if (opts.create !== undefined) {
          const label = typeof opts.create === 'string' ? opts.create : undefined;
          const snapshot = fort.rewind.createSnapshot(label);

          if (opts.json) {
            console.log(JSON.stringify(snapshot, null, 2));
            return;
          }

          console.log(bold('\n  Snapshot Created\n'));
          console.log(`  ID:     ${cyan(snapshot.id)}`);
          console.log(`  Label:  ${snapshot.label ?? dim('(none)')}`);
          console.log(`  Files:  ${snapshot.fileCount}`);
          console.log(`  Size:   ${formatBytes(snapshot.totalBytes)}`);
          console.log(`  Time:   ${snapshot.createdAt}`);
          console.log();
          return;
        }

        // Preview a restore
        if (opts.preview) {
          const preview = fort.rewind.previewRestore(opts.preview);

          if (opts.json) {
            console.log(JSON.stringify(preview, null, 2));
            return;
          }

          console.log(bold('\n  Restore Preview\n'));
          console.log(`  Snapshot: ${cyan(preview.snapshotId)}`);
          console.log(`  Label:    ${preview.snapshot.label ?? dim('(none)')}`);
          console.log(`  Created:  ${preview.snapshot.createdAt}`);
          console.log();

          if (!preview.hasChanges) {
            console.log(dim('  No changes — current state matches snapshot.\n'));
            return;
          }

          console.log(`  ${preview.changes.length} change(s):\n`);
          for (const change of preview.changes) {
            const icon = change.type === 'added' ? green('+') : change.type === 'removed' ? red('-') : yellow('~');
            console.log(`    ${icon} ${change.file}  ${dim(change.details)}`);
          }
          console.log();
          return;
        }

        // Restore to a snapshot
        if (opts.to) {
          const restoreOpts: { configOnly?: boolean; memoryOnly?: boolean } = {};
          if (opts.configOnly) restoreOpts.configOnly = true;
          if (opts.memoryOnly) restoreOpts.memoryOnly = true;

          const result = fort.rewind.restore(opts.to, restoreOpts);

          if (opts.json) {
            console.log(JSON.stringify(result, null, 2));
            return;
          }

          console.log(bold('\n  State Restored\n'));
          console.log(`  Snapshot: ${cyan(result.snapshotId)}`);
          console.log(`  Label:    ${result.snapshot.label ?? dim('(none)')}`);
          if (opts.configOnly) console.log(`  Mode:     ${yellow('config-only')}`);
          if (opts.memoryOnly) console.log(`  Mode:     ${yellow('memory-only')}`);
          console.log(`  Changes:  ${result.changes.length} file(s) affected`);
          console.log();
          return;
        }

        // Default: list recent snapshots
        const limit = opts.limit ?? 10;
        const snapshots = fort.rewind.listSnapshots({ limit });

        if (opts.json) {
          console.log(JSON.stringify(snapshots, null, 2));
          return;
        }

        console.log(bold('\n  Snapshots\n'));

        if (snapshots.length === 0) {
          console.log(dim('  No snapshots yet. Create one with: fort rewind --create [label]\n'));
          return;
        }

        const totalSize = fort.rewind.getSnapshotSize();
        console.log(dim(`  Total storage: ${formatBytes(totalSize)}\n`));

        const rows = [
          [dim('ID'), dim('Label'), dim('Trigger'), dim('Files'), dim('Size'), dim('Created')],
        ];

        for (const s of snapshots) {
          rows.push([
            cyan(s.id.substring(0, 8)),
            s.label ?? dim('—'),
            s.trigger,
            String(s.fileCount),
            formatBytes(s.totalBytes),
            timeAgo(new Date(s.createdAt)),
          ]);
        }

        console.log('  ' + table(rows).split('\n').join('\n  '));
        console.log();
      });
    });

  return cmd;
}
