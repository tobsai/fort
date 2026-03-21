/**
 * Plugin System — Extensible Fort capabilities
 *
 * Manages plugin lifecycle: discovery, security scanning, loading,
 * enabling/disabling, and shutdown. All plugins are sandboxed
 * through a restricted PluginContext.
 */

import { readFileSync, readdirSync, existsSync, statSync } from 'node:fs';
import { join, resolve } from 'node:path';
import type { ModuleBus } from '../module-bus/index.js';
import type { PermissionManager } from '../permissions/index.js';
import type {
  PluginManifest,
  PluginSecurityReport,
  SecurityCheck,
  LoadedPlugin,
  FortPlugin,
  PluginContext,
  DiagnosticResult,
} from '../types.js';

// ─── Dangerous Patterns ─────────────────────────────────────────────

interface DangerousPattern {
  pattern: RegExp;
  name: string;
  severity: 'warning' | 'critical';
  message: string;
  /** Permission type that would allow this pattern */
  requiredPermission?: string;
}

const DANGEROUS_PATTERNS: DangerousPattern[] = [
  {
    pattern: /\beval\s*\(/,
    name: 'eval-usage',
    severity: 'critical',
    message: 'Uses eval() which can execute arbitrary code',
  },
  {
    pattern: /new\s+Function\s*\(/,
    name: 'function-constructor',
    severity: 'critical',
    message: 'Uses new Function() which can execute arbitrary code',
  },
  {
    pattern: /child_process/,
    name: 'child-process',
    severity: 'critical',
    message: 'References child_process module',
    requiredPermission: 'command',
  },
  {
    pattern: /\bexecSync\s*\(/,
    name: 'exec-sync',
    severity: 'critical',
    message: 'Uses execSync() for command execution',
    requiredPermission: 'command',
  },
  {
    pattern: /\bspawn\s*\(/,
    name: 'spawn-usage',
    severity: 'critical',
    message: 'Uses spawn() for process creation',
    requiredPermission: 'command',
  },
  {
    pattern: /fs\.rmSync\s*\(/,
    name: 'fs-rm-sync',
    severity: 'critical',
    message: 'Uses fs.rmSync() for destructive filesystem operations',
  },
  {
    pattern: /fs\.unlinkSync\s*\(/,
    name: 'fs-unlink-sync',
    severity: 'critical',
    message: 'Uses fs.unlinkSync() for file deletion',
  },
  {
    pattern: /fs\.rmdirSync\s*\(/,
    name: 'fs-rmdir-sync',
    severity: 'critical',
    message: 'Uses fs.rmdirSync() for directory deletion',
  },
  {
    pattern: /process\.env/,
    name: 'process-env',
    severity: 'warning',
    message: 'Accesses process.env environment variables',
  },
  {
    pattern: /require\s*\(\s*['"]https?['"]\s*\)/,
    name: 'network-require',
    severity: 'warning',
    message: 'Requires http/https module',
    requiredPermission: 'network',
  },
  {
    pattern: /\bfetch\s*\(/,
    name: 'fetch-usage',
    severity: 'warning',
    message: 'Uses fetch() for network requests',
    requiredPermission: 'network',
  },
  {
    pattern: /__dirname/,
    name: 'dirname-access',
    severity: 'warning',
    message: 'Accesses __dirname which may indicate path escape',
  },
  {
    pattern: /__filename/,
    name: 'filename-access',
    severity: 'warning',
    message: 'Accesses __filename which may indicate path escape',
  },
  {
    pattern: /process\.cwd\s*\(\s*\)/,
    name: 'process-cwd',
    severity: 'warning',
    message: 'Accesses process.cwd() which may indicate path escape',
  },
];

// ─── PluginManager ──────────────────────────────────────────────────

export class PluginManager {
  private plugins: Map<string, LoadedPlugin> = new Map();
  private instances: Map<string, FortPlugin> = new Map();
  private pluginsDir: string;
  private bus: ModuleBus;
  private permissions: PermissionManager;

  constructor(pluginsDir: string, bus: ModuleBus, permissions: PermissionManager) {
    this.pluginsDir = pluginsDir;
    this.bus = bus;
    this.permissions = permissions;
  }

  /**
   * Read and parse a plugin's manifest.json
   */
  scanPlugin(pluginPath: string): PluginManifest {
    const manifestPath = join(pluginPath, 'manifest.json');
    if (!existsSync(manifestPath)) {
      throw new Error(`No manifest.json found at ${manifestPath}`);
    }

    const raw = readFileSync(manifestPath, 'utf-8');
    let manifest: PluginManifest;
    try {
      manifest = JSON.parse(raw);
    } catch {
      throw new Error(`Invalid JSON in manifest.json at ${manifestPath}`);
    }

    // Validate required fields
    if (!manifest.name || typeof manifest.name !== 'string') {
      throw new Error('Manifest missing required field: name');
    }
    if (!manifest.version || typeof manifest.version !== 'string') {
      throw new Error('Manifest missing required field: version');
    }
    if (!manifest.description || typeof manifest.description !== 'string') {
      throw new Error('Manifest missing required field: description');
    }
    if (!manifest.entryPoint || typeof manifest.entryPoint !== 'string') {
      throw new Error('Manifest missing required field: entryPoint');
    }

    // Defaults
    manifest.capabilities = manifest.capabilities ?? [];
    manifest.subscriptions = manifest.subscriptions ?? [];
    manifest.emissions = manifest.emissions ?? [];
    manifest.permissions = manifest.permissions ?? [];

    return manifest;
  }

  /**
   * Analyze plugin source code for security issues
   */
  securityScan(manifest: PluginManifest, pluginPath: string): PluginSecurityReport {
    const checks: SecurityCheck[] = [];
    const entryPath = join(pluginPath, manifest.entryPoint);

    // Check entry point exists
    if (!existsSync(entryPath)) {
      checks.push({
        name: 'entry-point-exists',
        passed: false,
        severity: 'critical',
        message: `Entry point not found: ${manifest.entryPoint}`,
      });
      return {
        pluginName: manifest.name,
        timestamp: new Date().toISOString(),
        passed: false,
        checks,
      };
    }

    checks.push({
      name: 'entry-point-exists',
      passed: true,
      severity: 'info',
      message: `Entry point found: ${manifest.entryPoint}`,
    });

    // Gather all source files to scan
    const sourceFiles = this.collectSourceFiles(pluginPath);
    const allSource = sourceFiles
      .map((f) => {
        try {
          return readFileSync(f, 'utf-8');
        } catch {
          return '';
        }
      })
      .join('\n');

    // Declared permission types for checking
    const declaredPermTypes = new Set(manifest.permissions.map((p) => p.type));

    // Check each dangerous pattern
    for (const dp of DANGEROUS_PATTERNS) {
      const found = dp.pattern.test(allSource);
      if (found) {
        // If there's a required permission and it's declared, downgrade to info
        if (dp.requiredPermission && declaredPermTypes.has(dp.requiredPermission as any)) {
          checks.push({
            name: dp.name,
            passed: true,
            severity: 'info',
            message: `${dp.message} (declared in permissions)`,
          });
        } else {
          checks.push({
            name: dp.name,
            passed: false,
            severity: dp.severity,
            message: dp.message,
          });
        }
      } else {
        checks.push({
          name: dp.name,
          passed: true,
          severity: 'info',
          message: `No ${dp.name} detected`,
        });
      }
    }

    // Check for network access without declaration
    const hasNetworkPerm = declaredPermTypes.has('network');
    const usesNetwork = /require\s*\(\s*['"]https?['"]\s*\)/.test(allSource) || /\bfetch\s*\(/.test(allSource);
    if (usesNetwork && !hasNetworkPerm) {
      checks.push({
        name: 'undeclared-network',
        passed: false,
        severity: 'critical',
        message: 'Network access detected but not declared in permissions',
      });
    }

    // Check for filesystem access outside declared scope
    const fsPaths = manifest.permissions
      .filter((p) => p.type === 'filesystem')
      .map((p) => p.scope);
    if (fsPaths.length === 0 && /require\s*\(\s*['"]fs['"]\s*\)/.test(allSource)) {
      checks.push({
        name: 'undeclared-filesystem',
        passed: false,
        severity: 'warning',
        message: 'Filesystem access detected but no filesystem permissions declared',
      });
    }

    // Validate manifest permissions are reasonable
    const excessivePerms = manifest.permissions.filter(
      (p) => p.scope === '*' || p.scope === '/**'
    );
    if (excessivePerms.length > 0) {
      checks.push({
        name: 'excessive-permissions',
        passed: false,
        severity: 'warning',
        message: `Plugin requests wildcard permissions: ${excessivePerms.map((p) => `${p.type}:${p.scope}`).join(', ')}`,
      });
    }

    const hasCriticalFailure = checks.some((c) => !c.passed && c.severity === 'critical');
    const passed = !hasCriticalFailure;

    return {
      pluginName: manifest.name,
      timestamp: new Date().toISOString(),
      passed,
      checks,
    };
  }

  /**
   * Load a plugin: scan manifest, run security scan, instantiate if passed
   */
  async loadPlugin(pluginPath: string): Promise<LoadedPlugin> {
    const absPath = resolve(pluginPath);
    const manifest = this.scanPlugin(absPath);

    // Check if already loaded
    if (this.plugins.has(manifest.name)) {
      throw new Error(`Plugin '${manifest.name}' is already loaded`);
    }

    // Run security scan
    const securityReport = this.securityScan(manifest, absPath);

    if (!securityReport.passed) {
      const loaded: LoadedPlugin = {
        manifest,
        status: 'blocked',
        loadedAt: new Date().toISOString(),
        securityReport,
        error: 'Security scan failed',
      };
      this.plugins.set(manifest.name, loaded);
      return loaded;
    }

    // Create plugin context
    const context = this.createPluginContext(manifest);

    try {
      // Dynamic import of the plugin entry point
      const entryPath = join(absPath, manifest.entryPoint);
      const pluginModule = await import(entryPath);
      const pluginFactory = pluginModule.default ?? pluginModule.create ?? pluginModule;

      let instance: FortPlugin;
      if (typeof pluginFactory === 'function') {
        instance = pluginFactory();
      } else if (typeof pluginFactory === 'object' && pluginFactory.initialize) {
        instance = pluginFactory;
      } else {
        throw new Error('Plugin must export a factory function or an object with initialize()');
      }

      await instance.initialize(context);
      this.instances.set(manifest.name, instance);

      const loaded: LoadedPlugin = {
        manifest,
        status: 'active',
        loadedAt: new Date().toISOString(),
        securityReport,
      };
      this.plugins.set(manifest.name, loaded);

      await this.bus.publish('plugin.loaded', 'plugin-manager', {
        name: manifest.name,
        version: manifest.version,
      });

      return loaded;
    } catch (err) {
      const loaded: LoadedPlugin = {
        manifest,
        status: 'error',
        loadedAt: new Date().toISOString(),
        securityReport,
        error: err instanceof Error ? err.message : String(err),
      };
      this.plugins.set(manifest.name, loaded);
      return loaded;
    }
  }

  /**
   * Shutdown and remove a loaded plugin
   */
  async unloadPlugin(name: string): Promise<boolean> {
    const loaded = this.plugins.get(name);
    if (!loaded) return false;

    const instance = this.instances.get(name);
    if (instance) {
      try {
        await instance.shutdown();
      } catch {
        // Best-effort shutdown
      }
      this.instances.delete(name);
    }

    this.plugins.delete(name);

    await this.bus.publish('plugin.unloaded', 'plugin-manager', { name });
    return true;
  }

  /**
   * Enable a disabled plugin
   */
  enablePlugin(name: string): boolean {
    const loaded = this.plugins.get(name);
    if (!loaded) return false;
    if (loaded.status === 'blocked') return false;

    loaded.status = 'active';
    this.plugins.set(name, loaded);
    return true;
  }

  /**
   * Disable an active plugin
   */
  disablePlugin(name: string): boolean {
    const loaded = this.plugins.get(name);
    if (!loaded) return false;

    loaded.status = 'disabled';
    this.plugins.set(name, loaded);
    return true;
  }

  /**
   * Get a loaded plugin by name
   */
  getPlugin(name: string): LoadedPlugin | undefined {
    return this.plugins.get(name);
  }

  /**
   * List all loaded plugins
   */
  listPlugins(): LoadedPlugin[] {
    return Array.from(this.plugins.values());
  }

  /**
   * Scan the plugins directory for available plugins
   */
  discoverPlugins(): PluginManifest[] {
    if (!existsSync(this.pluginsDir)) {
      return [];
    }

    const manifests: PluginManifest[] = [];
    const entries = readdirSync(this.pluginsDir);

    for (const entry of entries) {
      const pluginPath = join(this.pluginsDir, entry);
      try {
        const stat = statSync(pluginPath);
        if (stat.isDirectory()) {
          const manifest = this.scanPlugin(pluginPath);
          manifests.push(manifest);
        }
      } catch {
        // Skip invalid plugin directories
      }
    }

    return manifests;
  }

  /**
   * Shutdown all loaded plugins
   */
  async shutdownAll(): Promise<void> {
    for (const [name, instance] of this.instances) {
      try {
        await instance.shutdown();
      } catch {
        // Best-effort
      }
    }
    this.instances.clear();
    this.plugins.clear();
  }

  /**
   * Diagnostic report for the plugin system
   */
  diagnose(): DiagnosticResult {
    const allPlugins = this.listPlugins();
    const active = allPlugins.filter((p) => p.status === 'active');
    const errored = allPlugins.filter((p) => p.status === 'error');
    const blocked = allPlugins.filter((p) => p.status === 'blocked');

    const checks = [
      {
        name: 'Plugin count',
        passed: true,
        message: `${allPlugins.length} plugins loaded (${active.length} active)`,
      },
    ];

    if (errored.length > 0) {
      checks.push({
        name: 'Plugin errors',
        passed: false,
        message: `${errored.length} plugins in error state: ${errored.map((p) => p.manifest.name).join(', ')}`,
      });
    }

    if (blocked.length > 0) {
      checks.push({
        name: 'Blocked plugins',
        passed: false,
        message: `${blocked.length} plugins blocked by security scan: ${blocked.map((p) => p.manifest.name).join(', ')}`,
      });
    }

    // Check plugins directory exists
    const dirExists = existsSync(this.pluginsDir);
    checks.push({
      name: 'Plugins directory',
      passed: dirExists,
      message: dirExists
        ? `Plugins directory exists: ${this.pluginsDir}`
        : `Plugins directory not found: ${this.pluginsDir}`,
    });

    let status: 'healthy' | 'degraded' | 'unhealthy' = 'healthy';
    if (errored.length > 0 || blocked.length > 0) status = 'degraded';
    if (!dirExists) status = 'degraded';

    return {
      module: 'plugins',
      status,
      checks,
    };
  }

  // ─── Private Helpers ────────────────────────────────────────────────

  private createPluginContext(manifest: PluginManifest): PluginContext {
    return {
      bus: this.bus,
      memory: {
        createNode: () => { throw new Error('Memory access not yet implemented for plugins'); },
        search: () => { throw new Error('Memory access not yet implemented for plugins'); },
      },
      tools: {
        search: () => [],
        register: () => { throw new Error('Tool registration not yet implemented for plugins'); },
      },
      log: (message: string) => {
        this.bus.publish('plugin.log', manifest.name, { message });
      },
    };
  }

  private collectSourceFiles(dir: string): string[] {
    const files: string[] = [];
    if (!existsSync(dir)) return files;

    const entries = readdirSync(dir);
    for (const entry of entries) {
      if (entry === 'node_modules' || entry.startsWith('.')) continue;
      const fullPath = join(dir, entry);
      try {
        const stat = statSync(fullPath);
        if (stat.isDirectory()) {
          files.push(...this.collectSourceFiles(fullPath));
        } else if (/\.(ts|js|mjs|cjs)$/.test(entry)) {
          files.push(fullPath);
        }
      } catch {
        // Skip inaccessible files
      }
    }
    return files;
  }
}
