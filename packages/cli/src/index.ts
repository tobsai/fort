#!/usr/bin/env node

/**
 * Fort CLI — Command-line interface for Fort
 */

import { Command } from 'commander';
import { createDoctorCommand } from './commands/doctor.js';
import { createStatusCommand } from './commands/status.js';
import { createTasksCommand } from './commands/tasks.js';
import { createMemoryCommand } from './commands/memory.js';
import { createAgentsCommand } from './commands/agents.js';
import { createToolsCommand } from './commands/tools.js';
import { createScheduleCommand } from './commands/schedule.js';
import { createTokensCommand } from './commands/tokens.js';
import { createBehaviorsCommand } from './commands/behaviors.js';
import { createRoutinesCommand } from './commands/routines.js';
import { createFlagsCommand } from './commands/flags.js';
import { createPluginsCommand } from './commands/plugins.js';
import { createHarnessCommand } from './commands/harness.js';
import { createRewindCommand } from './commands/rewind.js';
import { createThreadsCommand } from './commands/threads.js';
import { createIntrospectCommand } from './commands/introspect.js';
import { createLLMCommand } from './commands/llm.js';
import { createStopCommand } from './commands/stop.js';
import { createInitCommand, isFirstRun } from './commands/init.js';
import { createPsCommand } from './commands/ps.js';
import { createResetCommand } from './commands/reset.js';
import { createPortalCommand } from './commands/portal.js';

// First-run: if user just types `fort` with no args, show init
if (isFirstRun() && process.argv.length === 2) {
  const initCmd = createInitCommand();
  initCmd.parseAsync([]).catch(console.error);
} else {
  const program = new Command();

  program
    .name('fort')
    .description('Fort — A self-improving personal AI agent platform')
    .version('0.1.0');

  program.addCommand(createInitCommand());
  program.addCommand(createDoctorCommand());
  program.addCommand(createStatusCommand());
  program.addCommand(createPsCommand());
  program.addCommand(createTasksCommand());
  program.addCommand(createMemoryCommand());
  program.addCommand(createAgentsCommand());
  program.addCommand(createToolsCommand());
  program.addCommand(createScheduleCommand());
  program.addCommand(createTokensCommand());
  program.addCommand(createBehaviorsCommand());
  program.addCommand(createRoutinesCommand());
  program.addCommand(createFlagsCommand());
  program.addCommand(createPluginsCommand());
  program.addCommand(createHarnessCommand());
  program.addCommand(createRewindCommand());
  program.addCommand(createThreadsCommand());
  program.addCommand(createIntrospectCommand());
  program.addCommand(createLLMCommand());
  program.addCommand(createStopCommand());
  program.addCommand(createResetCommand());
  program.addCommand(createPortalCommand());

  program.parse();
}
