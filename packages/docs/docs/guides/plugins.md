---
sidebar_position: 12
title: Plugins
description: Plugin system with security scanning and sandboxed execution
---

# Plugins

Fort supports plugins that extend its capabilities. Every plugin goes through a security scan before it can be loaded, and runs within a controlled context that limits what it can access.

## Plugin Manifest

Each plugin is a directory with a `manifest.json`:

```json
{
  "name": "weather-lookup",
  "version": "1.0.0",
  "description": "Look up current weather for a location",
  "author": "you",
  "entry": "index.ts",
  "capabilities": ["network", "tools"],
  "permissions": {
    "network": ["api.openweathermap.org"],
    "tools": ["register"]
  }
}
```

### Manifest Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique plugin identifier |
| `version` | Yes | Semver version string |
| `description` | Yes | What the plugin does |
| `author` | No | Plugin author |
| `entry` | Yes | Entry point file relative to plugin directory |
| `capabilities` | Yes | List of capabilities the plugin needs |
| `permissions` | Yes | Detailed permission declarations per capability |

## Security Scanner

Before a plugin can be loaded, Fort scans its source code for dangerous patterns. The scanner checks for 14 patterns:

1. `eval()` — Dynamic code execution
2. `Function()` — Constructor-based code execution
3. `child_process` — Shell command spawning
4. `execSync` — Synchronous shell execution
5. `execFile` — File execution
6. `spawn` — Process spawning
7. `fs.unlinkSync` — Synchronous file deletion
8. `fs.rmdirSync` — Synchronous directory deletion
9. `fs.rmSync` — Synchronous recursive deletion
10. `process.env` — Environment variable access
11. `require('http')` — Raw HTTP server creation
12. `require('net')` — Raw TCP socket access
13. `require('dgram')` — UDP socket access
14. `global.__proto__` — Prototype pollution

Scan a plugin before loading:

```bash
fort plugins scan ./my-plugin
# Scanning ./my-plugin...
# [WARN] Found child_process import in lib/helper.ts:14
# [INFO] child_process is declared in permissions — downgraded to info
# [PASS] No undeclared dangerous patterns found
```

If a dangerous pattern is found but the plugin has declared it in its `permissions`, the finding is downgraded from a warning to an informational note. Undeclared dangerous patterns cause the scan to fail.

## Loading and Managing Plugins

Load a plugin (runs scan automatically):

```bash
fort plugins load ./my-plugin
# Scanning my-plugin...
# [PASS] Security scan passed
# Loaded: weather-lookup v1.0.0
```

List loaded plugins:

```bash
fort plugins list
# weather-lookup  v1.0.0  enabled   network, tools
# task-exporter   v0.3.1  disabled  file_write
```

Disable a plugin without unloading:

```bash
fort plugins disable weather-lookup
```

Re-enable:

```bash
fort plugins enable weather-lookup
```

Unload completely:

```bash
fort plugins unload weather-lookup
```

## Plugin Context

When a plugin is loaded, it receives a `PluginContext` object that provides controlled access to Fort internals:

```typescript
interface PluginContext {
  bus: ModuleBus;          // Publish and subscribe to events
  tools: ToolRegistry;     // Register and query tools
  memory: MemoryManager;   // Read and write memory graph
  logger: Logger;          // Structured logging
  config: PluginConfig;    // Plugin-specific configuration
}
```

A minimal plugin entry point:

```typescript
import { PluginContext } from '@fort/core';

export function activate(ctx: PluginContext) {
  ctx.tools.register({
    name: 'weather-lookup',
    description: 'Get current weather for a location',
    source: 'plugin:weather-lookup',
    handler: async (args: { location: string }) => {
      const resp = await fetch(
        `https://api.openweathermap.org/data/2.5/weather?q=${args.location}`
      );
      return resp.json();
    },
  });

  ctx.bus.publish('plugin:activated', { name: 'weather-lookup' });
}

export function deactivate(ctx: PluginContext) {
  ctx.tools.unregister('weather-lookup');
}
```

## Plugin Events

The plugin system publishes on the ModuleBus:

- `plugin:loaded` — Plugin passed scan and was loaded.
- `plugin:activated` — Plugin's `activate` function ran.
- `plugin:deactivated` — Plugin's `deactivate` function ran.
- `plugin:unloaded` — Plugin was fully removed.
- `plugin:scan:failed` — Security scan found undeclared dangerous patterns.

## Best Practices

- Declare all capabilities honestly in the manifest. Undeclared patterns will block loading.
- Keep plugins focused. One plugin, one purpose.
- Use the `ToolRegistry` to register new tools rather than reaching into Fort internals directly.
- Subscribe to bus events rather than polling for state changes.
- Implement both `activate` and `deactivate` for clean lifecycle management.
