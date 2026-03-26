/**
 * Built-in Tools — Starter Set
 *
 * Exports all 5 built-in FortTools and provides a helper to register
 * them into a ToolRegistry instance.
 */

export { readFileTool } from './read-file.js';
export { writeFileTool } from './write-file.js';
export { listFilesTool } from './list-files.js';
export { webSearchTool } from './web-search.js';
export { shellCommandTool } from './shell-command.js';

import { readFileTool } from './read-file.js';
import { writeFileTool } from './write-file.js';
import { listFilesTool } from './list-files.js';
import { webSearchTool } from './web-search.js';
import { shellCommandTool } from './shell-command.js';
import type { ToolRegistry } from '../index.js';

export const BUILTIN_TOOLS = [
  readFileTool,
  writeFileTool,
  listFilesTool,
  webSearchTool,
  shellCommandTool,
] as const;

/**
 * Register all built-in tools into the given ToolRegistry.
 * Safe to call multiple times — duplicate registrations are handled by the registry.
 */
export function registerBuiltins(registry: ToolRegistry): void {
  for (const tool of BUILTIN_TOOLS) {
    registry.registerTool(tool);
  }
}
