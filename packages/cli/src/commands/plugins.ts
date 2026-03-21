import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, cyan, green, red, yellow } from '../utils/format.js';

export function createPluginsCommand(): Command {
  const cmd = new Command('plugins')
    .description('Plugin management — extend Fort capabilities');

  cmd
    .command('list')
    .description('List all loaded plugins')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const plugins = fort.plugins.listPlugins();

        if (opts.json) {
          console.log(JSON.stringify(plugins, null, 2));
          return;
        }

        console.log(bold('\n  Loaded Plugins\n'));

        if (plugins.length === 0) {
          console.log(dim('  No plugins loaded.\n'));
          return;
        }

        for (const plugin of plugins) {
          const statusColor =
            plugin.status === 'active' ? green :
            plugin.status === 'disabled' ? yellow :
            red;
          console.log(`  ${cyan(plugin.manifest.name)} ${dim(`v${plugin.manifest.version}`)} ${statusColor(plugin.status)}`);
          console.log(`    ${plugin.manifest.description}`);
          if (plugin.manifest.author) {
            console.log(`    ${dim(`Author: ${plugin.manifest.author}`)}`);
          }
          console.log(`    ${dim(`Capabilities: ${plugin.manifest.capabilities.map((c) => c.name).join(', ') || 'none'}`)}`);
          if (plugin.error) {
            console.log(`    ${red(`Error: ${plugin.error}`)}`);
          }
          console.log();
        }
      });
    });

  cmd
    .command('scan <path>')
    .description('Scan a plugin and show security report')
    .action(async (pluginPath) => {
      await withFort(async (fort) => {
        try {
          const manifest = fort.plugins.scanPlugin(pluginPath);
          const report = fort.plugins.securityScan(manifest, pluginPath);

          console.log(bold(`\n  Security Scan: ${manifest.name}\n`));
          console.log(`  Version:     ${manifest.version}`);
          console.log(`  Description: ${manifest.description}`);
          console.log(`  Entry Point: ${manifest.entryPoint}`);
          console.log(`  Result:      ${report.passed ? green('PASSED') : red('FAILED')}\n`);

          console.log(bold('  Checks:\n'));
          for (const check of report.checks) {
            const icon = check.passed ? green('PASS') : red('FAIL');
            const sev = check.severity === 'critical' ? red(check.severity) :
                        check.severity === 'warning' ? yellow(check.severity) :
                        dim(check.severity);
            console.log(`    ${icon} [${sev}] ${check.name}`);
            console.log(`      ${check.message}`);
          }
          console.log();
        } catch (err) {
          console.error(`  ${red('Error:')} ${err instanceof Error ? err.message : String(err)}\n`);
        }
      });
    });

  cmd
    .command('load <path>')
    .description('Load a plugin (runs security scan first)')
    .action(async (pluginPath) => {
      await withFort(async (fort) => {
        try {
          const loaded = await fort.plugins.loadPlugin(pluginPath);

          if (loaded.status === 'blocked') {
            console.log(red(`\n  Plugin '${loaded.manifest.name}' blocked by security scan.\n`));
            if (loaded.securityReport) {
              const failed = loaded.securityReport.checks.filter((c) => !c.passed);
              for (const check of failed) {
                console.log(`    ${red('FAIL')} [${check.severity}] ${check.name}: ${check.message}`);
              }
            }
          } else if (loaded.status === 'error') {
            console.log(red(`\n  Plugin '${loaded.manifest.name}' failed to load: ${loaded.error}\n`));
          } else {
            console.log(green(`\n  Plugin '${loaded.manifest.name}' loaded successfully.\n`));
          }
        } catch (err) {
          console.error(`  ${red('Error:')} ${err instanceof Error ? err.message : String(err)}\n`);
        }
      });
    });

  cmd
    .command('unload <name>')
    .description('Unload a plugin')
    .action(async (name) => {
      await withFort(async (fort) => {
        const success = await fort.plugins.unloadPlugin(name);
        if (success) {
          console.log(green(`\n  Plugin '${name}' unloaded.\n`));
        } else {
          console.log(red(`\n  Plugin '${name}' not found.\n`));
        }
      });
    });

  cmd
    .command('enable <name>')
    .description('Enable a disabled plugin')
    .action(async (name) => {
      await withFort(async (fort) => {
        const success = fort.plugins.enablePlugin(name);
        if (success) {
          console.log(green(`\n  Plugin '${name}' enabled.\n`));
        } else {
          console.log(red(`\n  Plugin '${name}' not found or cannot be enabled.\n`));
        }
      });
    });

  cmd
    .command('disable <name>')
    .description('Disable a plugin')
    .action(async (name) => {
      await withFort(async (fort) => {
        const success = fort.plugins.disablePlugin(name);
        if (success) {
          console.log(yellow(`\n  Plugin '${name}' disabled.\n`));
        } else {
          console.log(red(`\n  Plugin '${name}' not found.\n`));
        }
      });
    });

  return cmd;
}
