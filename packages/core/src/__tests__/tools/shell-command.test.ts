import { describe, it, expect } from 'vitest';
import { shellCommandTool } from '../../tools/builtins/shell-command.js';
import { ToolExecutor } from '../../tools/executor.js';
import { PermissionManager } from '../../permissions/index.js';
import { ModuleBus } from '../../module-bus/index.js';
import { TokenTracker } from '../../tokens/index.js';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';

describe('shell-command tool', () => {
  it('has correct metadata', () => {
    expect(shellCommandTool.name).toBe('shell-command');
    expect(shellCommandTool.tier).toBe(3);
    expect(shellCommandTool.description).toBeTruthy();
  });

  it('runs a simple command and returns stdout', async () => {
    const result = await shellCommandTool.execute({ command: 'echo hello' });

    expect(result.success).toBe(true);
    expect(result.output).toContain('hello');
    const artifact = (result.artifacts as any[])[0];
    expect(artifact.stdout.trim()).toBe('hello');
    expect(artifact.exitCode).toBe(0);
  });

  it('captures non-zero exit code as failure', async () => {
    const result = await shellCommandTool.execute({ command: 'exit 2', });

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/exit.*2/i);
    const artifact = (result.artifacts as any[])[0];
    expect(artifact.exitCode).toBe(2);
  });

  it('returns error when command is missing', async () => {
    const result = await shellCommandTool.execute({});
    expect(result.success).toBe(false);
    expect(result.error).toMatch(/command/i);
  });

  it('captures stderr in the artifact', async () => {
    const result = await shellCommandTool.execute({ command: 'echo error >&2; exit 1' });

    expect(result.success).toBe(false);
    const artifact = (result.artifacts as any[])[0];
    expect(artifact.stderr).toContain('error');
  });

  it('respects workingDir option', async () => {
    const result = await shellCommandTool.execute({ command: 'pwd', workingDir: '/tmp' });

    expect(result.success).toBe(true);
    expect(result.output).toContain('/tmp');
  });

  it('is blocked by ToolExecutor (Tier 3) without approval', async () => {
    const tmpDir = mkdtempSync(join(tmpdir(), 'fort-shell-exec-'));
    try {
      const permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
      const bus = new ModuleBus();
      const tokens = new TokenTracker(join(tmpDir, 'tokens.db'), bus);
      const executor = new ToolExecutor(permissions, bus, tokens);

      const result = await executor.execute(shellCommandTool, { command: 'echo hello' });

      expect(result.success).toBe(false);
      expect(result.error).toMatch(/tier 3/i);
      expect(result.error).toMatch(/approval/i);

      tokens.close();
    } finally {
      rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('formats output with STDOUT/STDERR labels', async () => {
    const result = await shellCommandTool.execute({ command: 'echo hi' });

    expect(result.success).toBe(true);
    expect(result.output).toContain('STDOUT:');
    expect(result.output).toContain('Exit code: 0');
  });
});
