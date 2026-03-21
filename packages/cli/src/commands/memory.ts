import { Command } from 'commander';
import { withFort } from '../utils/fort-instance.js';
import { bold, dim, cyan, timeAgo } from '../utils/format.js';

export function createMemoryCommand(): Command {
  const cmd = new Command('memory')
    .description('Memory graph operations');

  cmd
    .command('inspect')
    .description('Browse the memory graph')
    .option('--type <type>', 'Filter by node type')
    .option('--limit <n>', 'Limit results', '20')
    .option('--json', 'Output as JSON')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const results = fort.memory.search({
          nodeType: opts.type,
          limit: parseInt(opts.limit),
        });

        if (opts.json) {
          console.log(JSON.stringify(results, null, 2));
          return;
        }

        console.log(bold('\n  Memory Graph\n'));
        const stats = fort.memory.stats();
        console.log(`  ${stats.nodeCount} nodes, ${stats.edgeCount} edges\n`);

        if (results.nodes.length === 0) {
          console.log(dim('  No nodes found.\n'));
          return;
        }

        for (const node of results.nodes) {
          console.log(`  ${cyan(node.type)} ${bold(node.label)}`);
          console.log(`    ${dim(`id: ${node.id.slice(0, 8)} | source: ${node.source || 'unknown'} | ${timeAgo(node.updatedAt)}`)}`);
          if (Object.keys(node.properties).length > 0) {
            console.log(`    ${dim(JSON.stringify(node.properties))}`);
          }
        }
        console.log();
      });
    });

  cmd
    .command('search <query>')
    .description('Search memory nodes')
    .option('--json', 'Output as JSON')
    .action(async (query, opts) => {
      await withFort(async (fort) => {
        const results = fort.memory.search({ text: query });

        if (opts.json) {
          console.log(JSON.stringify(results, null, 2));
          return;
        }

        console.log(bold(`\n  Memory Search: "${query}"\n`));

        if (results.nodes.length === 0) {
          console.log(dim('  No results found.\n'));
          return;
        }

        for (const node of results.nodes) {
          console.log(`  ${cyan(node.type)} ${bold(node.label)}`);
          console.log(`    ${dim(`id: ${node.id.slice(0, 8)} | ${timeAgo(node.updatedAt)}`)}`);
        }
        console.log();
      });
    });

  cmd
    .command('history')
    .description('Show recent memory changes')
    .option('--since <date>', 'Show changes since date')
    .option('--limit <n>', 'Limit results', '20')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const events = fort.bus.getHistory('memory.node_created', parseInt(opts.limit));
        console.log(bold('\n  Recent Memory Changes\n'));

        if (events.length === 0) {
          console.log(dim('  No recent changes.\n'));
          return;
        }

        for (const event of events) {
          const node = (event.payload as any).node;
          console.log(`  ${dim(timeAgo(event.timestamp))} ${cyan('created')} ${node?.label ?? 'unknown'}`);
        }
        console.log();
      });
    });

  cmd
    .command('export')
    .description('Export the full memory graph')
    .option('--format <format>', 'Export format: json', 'json')
    .action(async (opts) => {
      await withFort(async (fort) => {
        const graph = fort.memory.exportGraph();
        console.log(JSON.stringify(graph, null, 2));
      });
    });

  cmd
    .command('stats')
    .description('Show memory statistics')
    .action(async () => {
      await withFort(async (fort) => {
        const stats = fort.memory.stats();
        console.log(bold('\n  Memory Statistics\n'));
        console.log(`  Nodes: ${stats.nodeCount}`);
        console.log(`  Edges: ${stats.edgeCount}`);
        if (Object.keys(stats.nodeTypes).length > 0) {
          console.log(bold('\n  Node Types:'));
          for (const [type, count] of Object.entries(stats.nodeTypes)) {
            console.log(`    ${type}: ${count}`);
          }
        }
        if (Object.keys(stats.edgeTypes).length > 0) {
          console.log(bold('\n  Edge Types:'));
          for (const [type, count] of Object.entries(stats.edgeTypes)) {
            console.log(`    ${type}: ${count}`);
          }
        }
        console.log();
      });
    });

  return cmd;
}
