/**
 * Shared Fort instance for CLI commands
 */

import { join } from 'node:path';
import { homedir } from 'node:os';
import { Fort } from '@fort-ai/core';

let instance: Fort | null = null;

export function getFortDataDir(): string {
  return join(homedir(), '.fort');
}

export function getFort(): Fort {
  if (!instance) {
    const dataDir = getFortDataDir();
    instance = new Fort({
      dataDir: join(dataDir, 'data'),
      specsDir: join(process.cwd(), 'specs'),
      agentsDir: join(dataDir, 'agents'),
      permissionsPath: join(dataDir, 'permissions.yaml'),
    });
  }
  return instance;
}

export async function withFort<T>(fn: (fort: Fort) => Promise<T>): Promise<T> {
  const fort = getFort();
  try {
    await fort.start();
    return await fn(fort);
  } finally {
    await fort.stop();
  }
}
