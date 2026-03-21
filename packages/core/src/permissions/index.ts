/**
 * Permission & Safety Model
 *
 * Tiered action model with deterministic controls.
 * Tier 1: Auto, Tier 2: Draft, Tier 3: Approve, Tier 4: Never
 */

import { readFileSync, existsSync, writeFileSync } from 'node:fs';
import { parse as parseYaml, stringify as stringifyYaml } from 'yaml';
import type { PermissionConfig, FolderPermission, PermissionLevel, ActionTier, ActionGate } from '../types.js';

const DEFAULT_CONFIG: PermissionConfig = {
  defaults: {
    commandLine: 'deny',
  },
  folders: [],
  system: {
    commandLine: 'deny',
    exceptions: ['open *', 'pbcopy'],
  },
};

const ACTION_GATES: ActionGate[] = [
  // Tier 1 — Auto
  { tier: 1, action: 'read_files', description: 'Reading files in allowed directories', requiresApproval: false },
  { tier: 1, action: 'web_search', description: 'Searching the web via allowed APIs', requiresApproval: false },
  { tier: 1, action: 'read_email', description: 'Reading email', requiresApproval: false },
  { tier: 1, action: 'query_calendar', description: 'Querying calendar', requiresApproval: false },
  { tier: 1, action: 'update_memory', description: 'Updating own memory/config', requiresApproval: false },

  // Tier 2 — Draft
  { tier: 2, action: 'compose_email', description: 'Composing emails (creates draft)', requiresApproval: false },
  { tier: 2, action: 'compose_imessage', description: 'Composing iMessages to non-allowlisted contacts', requiresApproval: false },
  { tier: 2, action: 'create_calendar_event', description: 'Creating calendar event (draft)', requiresApproval: false },

  // Tier 3 — Approve
  { tier: 3, action: 'send_message', description: 'Sending messages to new recipients', requiresApproval: true },
  { tier: 3, action: 'browser_action', description: 'Browser actions on non-allowlisted sites', requiresApproval: true },
  { tier: 3, action: 'financial', description: 'Financial transactions', requiresApproval: true },
  { tier: 3, action: 'destructive_file', description: 'Destructive file operations', requiresApproval: true },
  { tier: 3, action: 'system_command', description: 'System-level command execution', requiresApproval: true },

  // Tier 4 — Never
  { tier: 4, action: 'access_credentials', description: 'Accessing credentials/secrets directly', requiresApproval: true },
  { tier: 4, action: 'modify_permissions', description: 'Modifying permission config without user', requiresApproval: true },
  { tier: 4, action: 'send_unknown', description: 'Sending to contacts not in any allowlist', requiresApproval: true },
  { tier: 4, action: 'exec_unsigned', description: 'Executing unsigned/unreviewed code in production', requiresApproval: true },
];

export class PermissionManager {
  private config: PermissionConfig;
  private configPath: string;

  constructor(configPath: string) {
    this.configPath = configPath;
    this.config = this.loadConfig();
  }

  private loadConfig(): PermissionConfig {
    if (!existsSync(this.configPath)) {
      this.saveConfig(DEFAULT_CONFIG);
      return { ...DEFAULT_CONFIG };
    }

    try {
      const raw = readFileSync(this.configPath, 'utf-8');
      const parsed = parseYaml(raw) as PermissionConfig;
      return { ...DEFAULT_CONFIG, ...parsed };
    } catch {
      return { ...DEFAULT_CONFIG };
    }
  }

  private saveConfig(config: PermissionConfig): void {
    writeFileSync(this.configPath, stringifyYaml(config), 'utf-8');
  }

  checkCommandPermission(command: string, workingDir: string): {
    allowed: boolean;
    reason: string;
  } {
    // Check folder-specific permissions
    for (const folder of this.config.folders) {
      if (workingDir.startsWith(folder.path)) {
        if (folder.commandLine === 'deny') {
          return { allowed: false, reason: `Command line denied in ${folder.path}` };
        }
        if (folder.commandLine === 'read_only') {
          return { allowed: false, reason: `Only read access in ${folder.path}` };
        }

        // Check denied commands
        if (folder.deniedCommands) {
          for (const pattern of folder.deniedCommands) {
            if (this.matchCommand(command, pattern)) {
              return { allowed: false, reason: `Command matches denied pattern: ${pattern}` };
            }
          }
        }

        // Check allowed commands
        if (folder.allowedCommands) {
          for (const pattern of folder.allowedCommands) {
            if (this.matchCommand(command, pattern)) {
              return { allowed: true, reason: `Allowed by folder rule: ${folder.path}` };
            }
          }
          return { allowed: false, reason: `Command not in allowed list for ${folder.path}` };
        }

        return { allowed: true, reason: `Command line allowed in ${folder.path}` };
      }
    }

    // Check system exceptions
    for (const exception of this.config.system.exceptions) {
      if (this.matchCommand(command, exception)) {
        return { allowed: true, reason: `System exception: ${exception}` };
      }
    }

    // Default
    if (this.config.defaults.commandLine === 'allow') {
      return { allowed: true, reason: 'Default: allow' };
    }
    return { allowed: false, reason: 'Default: deny' };
  }

  checkAction(action: string): {
    tier: ActionTier;
    allowed: boolean;
    requiresApproval: boolean;
    description: string;
  } {
    const gate = ACTION_GATES.find((g) => g.action === action);
    if (!gate) {
      return { tier: 3, allowed: false, requiresApproval: true, description: 'Unknown action' };
    }

    if (gate.tier === 4) {
      return { tier: 4, allowed: false, requiresApproval: true, description: gate.description };
    }

    return {
      tier: gate.tier,
      allowed: gate.tier === 1,
      requiresApproval: gate.requiresApproval,
      description: gate.description,
    };
  }

  getActionGates(): ActionGate[] {
    return [...ACTION_GATES];
  }

  getConfig(): PermissionConfig {
    return { ...this.config };
  }

  addFolderPermission(folder: FolderPermission): void {
    this.config.folders = this.config.folders.filter((f) => f.path !== folder.path);
    this.config.folders.push(folder);
    this.saveConfig(this.config);
  }

  private matchCommand(command: string, pattern: string): boolean {
    const regex = new RegExp(
      '^' + pattern.replace(/\*/g, '.*').replace(/\?/g, '.') + '$'
    );
    return regex.test(command);
  }
}
