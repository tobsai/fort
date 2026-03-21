#!/usr/bin/env node

/**
 * Fort CLI — Command-line interface for Fort
 *
 * fort doctor    — Run health checks
 * fort status    — Show system status
 * fort tasks     — Show tasks
 * fort memory    — Memory operations
 * fort agents    — Agent management
 * fort tools     — Tool registry
 * fort schedule  — Scheduler management
 * fort specs     — Spec management
 * fort tokens    — Token usage & budget
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

const program = new Command();

program
  .name('fort')
  .description('Fort — A self-improving personal AI agent platform')
  .version('0.1.0');

program.addCommand(createDoctorCommand());
program.addCommand(createStatusCommand());
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

program.parse();
