import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, cyan, timeAgo } from '../utils/format.js';

export function createToolsCommand(): Command {
  const cmd = new Command('tools')
    .description('Tool registry — reuse before you build');

  cmd
    .command('list')
    .description('List all registered tools')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const tools = fort.tools.list();

        if (opts.json) {
          console.log(JSON.stringify(tools, null, 2));
          return;
        }

        console.log(bold('\n  Tool Registry\n'));

        if (tools.length === 0) {
          console.log(dim('  No tools registered.\n'));
          return;
        }

        for (const tool of tools) {
          console.log(`  ${cyan(tool.module)} ${bold(tool.name)} ${dim(`v${tool.version}`)}`);
          console.log(`    ${tool.description}`);
          console.log(`    ${dim(`Used ${tool.usageCount} times | Tags: ${tool.tags.join(', ') || 'none'}`)}`);
          console.log();
        }
      });
    });

  cmd
    .command('search <query>')
    .description('Search tools by capability')
    .option('--json', 'Output as JSON')
    .action(async (query, opts) => {
      await withFort(async (fort) => {
        const tools = fort.tools.search(query);

        if (opts.json) {
          console.log(JSON.stringify(tools, null, 2));
          return;
        }

        console.log(bold(`\n  Tool Search: "${query}"\n`));

        if (tools.length === 0) {
          console.log(dim('  No matching tools found. You may need to build one.\n'));
          return;
        }

        for (const tool of tools) {
          console.log(`  ${cyan(tool.module)} ${bold(tool.name)}`);
          console.log(`    ${tool.description}`);
          console.log(`    ${dim(`Capabilities: ${tool.capabilities.join(', ')}`)}`);
          console.log();
        }
      });
    });

  cmd
    .command('inspect <toolId>')
    .description('Show detailed tool information')
    .action(async (toolId) => {
      await withFort(async (fort) => {
        const tool = fort.tools.get(toolId);
        if (!tool) {
          console.error(`  Tool not found: ${toolId}`);
          return;
        }

        console.log(bold(`\n  Tool: ${tool.name}\n`));
        console.log(`  ID:           ${tool.id}`);
        console.log(`  Module:       ${tool.module}`);
        console.log(`  Version:      ${tool.version}`);
        console.log(`  Description:  ${tool.description}`);
        console.log(`  Capabilities: ${tool.capabilities.join(', ')}`);
        console.log(`  Input Types:  ${tool.inputTypes.join(', ')}`);
        console.log(`  Output Types: ${tool.outputTypes.join(', ')}`);
        console.log(`  Tags:         ${tool.tags.join(', ')}`);
        console.log(`  Usage Count:  ${tool.usageCount}`);
        console.log(`  Last Used:    ${tool.lastUsedAt?.toISOString() ?? 'never'}`);
        console.log();
      });
    });

  return cmd;
}
