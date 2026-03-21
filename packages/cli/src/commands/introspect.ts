import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, cyan, green, yellow, statusIcon, statusColor } from '../utils/format.js';

export function createIntrospectCommand(): Command {
  const cmd = new Command('introspect')
    .description('Machine-readable documentation — Fort introspects itself');

  // Default: show system profile summary
  cmd
    .action(async () => {
      await withFort(async (fort) => {
        const profile = await fort.introspect.generateSystemProfile();

        console.log(bold('\n  Fort System Profile\n'));
        console.log(`  Version:        ${cyan(profile.version)}`);
        console.log(`  Uptime:         ${Math.floor(profile.uptime / 1000)}s`);
        console.log(`  Modules:        ${profile.moduleCount}`);
        console.log(`  Agents:         ${profile.agentCount}`);
        console.log(`  Tools:          ${profile.toolCount}`);
        console.log(`  Tasks:          ${profile.taskCount}`);

        if (profile.diagnosticSummary) {
          const ds = profile.diagnosticSummary;
          console.log(`  Health:         ${statusColor(ds.overall)} (${ds.passed}/${ds.totalChecks} checks passed)`);
        }
        console.log();
      });
    });

  // Subcommand: modules
  cmd
    .command('modules')
    .description('List all modules with capabilities')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const manifest = fort.introspect.generateModuleManifest();

        if (opts.json) {
          console.log(JSON.stringify(manifest, null, 2));
          return;
        }

        console.log(bold('\n  Module Manifest\n'));

        for (const mod of manifest.modules) {
          console.log(`  ${statusIcon(mod.healthStatus)} ${bold(mod.name)} ${dim(`[${mod.healthStatus}]`)}`);
          if (mod.capabilities.length > 0) {
            console.log(`    ${dim('Capabilities:')} ${mod.capabilities.join(', ')}`);
          }
          console.log();
        }
      });
    });

  // Subcommand: capabilities
  cmd
    .command('capabilities')
    .description('Flat capability list')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const capabilities = fort.introspect.generateCapabilityMap();

        if (opts.json) {
          console.log(JSON.stringify(capabilities, null, 2));
          return;
        }

        console.log(bold('\n  Capability Map\n'));

        if (capabilities.length === 0) {
          console.log(dim('  No capabilities registered.\n'));
          return;
        }

        // Group by provider type
        const byType: Record<string, typeof capabilities> = {};
        for (const cap of capabilities) {
          if (!byType[cap.providerType]) byType[cap.providerType] = [];
          byType[cap.providerType].push(cap);
        }

        for (const [type, caps] of Object.entries(byType)) {
          console.log(`  ${bold(type.toUpperCase())}`);
          for (const cap of caps) {
            console.log(`    ${cyan(cap.capability)} ${dim('via')} ${cap.provider}`);
          }
          console.log();
        }
      });
    });

  // Subcommand: events
  cmd
    .command('events')
    .description('Event catalog')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const catalog = fort.introspect.generateEventCatalog();

        if (opts.json) {
          console.log(JSON.stringify(catalog, null, 2));
          return;
        }

        console.log(bold('\n  Event Catalog\n'));

        if (catalog.length === 0) {
          console.log(dim('  No events recorded.\n'));
          return;
        }

        for (const entry of catalog) {
          const subs = entry.subscriberCount > 0
            ? green(`${entry.subscriberCount} subscribers`)
            : dim('no subscribers');
          console.log(`  ${cyan(entry.eventType)} ${dim('|')} ${subs}`);
          if (entry.publishers.length > 0) {
            console.log(`    ${dim('Publishers:')} ${entry.publishers.join(', ')}`);
          }
        }
        console.log();
      });
    });

  // Subcommand: search
  cmd
    .command('search <query>')
    .description('Search capabilities')
    .option('--json', 'Output as JSON')
    .action(async (query, opts) => {
      await withFort(async (fort) => {
        const results = fort.introspect.searchCapabilities(query);

        if (opts.json) {
          console.log(JSON.stringify(results, null, 2));
          return;
        }

        console.log(bold(`\n  Capability Search: "${query}"\n`));

        if (results.length === 0) {
          console.log(dim('  No matching capabilities found.\n'));
          return;
        }

        for (const cap of results) {
          console.log(`  ${cyan(cap.capability)} ${dim('via')} ${bold(cap.provider)} ${dim(`[${cap.providerType}]`)}`);
          console.log(`    ${cap.description}`);
          console.log();
        }
      });
    });

  // Subcommand: export
  cmd
    .command('export')
    .description('Export full system documentation')
    .option('--format <format>', 'Output format: json or markdown', 'json')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const format = opts.format === 'markdown' ? 'markdown' : 'json';
        const output = await fort.introspect.exportDocumentation(format);
        console.log(output);
      });
    });

  return cmd;
}
