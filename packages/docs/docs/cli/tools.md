---
sidebar_position: 8
title: fort tools
---

# fort tools

Browse and search the tool registry. The tool registry enforces the reuse-before-build principle -- always check here before creating new capabilities.

## Usage

```
fort tools <subcommand> [options]
```

## Subcommands

### list

List all registered tools.

```
fort tools list [--json]
```

Displays each tool's ID, name, description, and providing module.

### search

Search for tools by keyword. Supports multi-term queries for broad matching.

```
fort tools search <query> [--json]
```

The search uses SQLite full-text search across tool names, descriptions, and capability tags.

### inspect

Show full details for a specific tool, including its parameters, return type, and usage history.

```
fort tools inspect <toolId>
```

## Examples

```bash
# Check if a git tool already exists before building one
fort tools search "git commit"

# List all tools in JSON for scripting
fort tools list --json

# View details for a specific tool
fort tools inspect tool-file-reader
```
