---
sidebar_position: 14
title: fort plugins
---

# fort plugins

Load, unload, and manage plugins that extend Fort's capabilities.

## Usage

```
fort plugins <subcommand> [options]
```

## Subcommands

### list

List all known plugins and their status.

```
fort plugins list [--json]
```

### scan

Scan a directory for plugin manifests without loading them.

```
fort plugins scan <path>
```

Reports discovered plugins, their declared capabilities, and any compatibility issues.

### load

Load a plugin from a directory path. The directory must contain a valid plugin manifest.

```
fort plugins load <path>
```

### unload

Unload a currently loaded plugin by name.

```
fort plugins unload <name>
```

### enable / disable

Toggle a plugin without fully unloading it. Disabled plugins remain registered but do not receive events.

```
fort plugins enable <name>
fort plugins disable <name>
```

## Examples

```bash
# Scan a directory for plugins
fort plugins scan ./my-plugins

# Load and enable a plugin
fort plugins load ./my-plugins/github-integration

# Temporarily disable a plugin
fort plugins disable github-integration
```
