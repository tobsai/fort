---
sidebar_position: 1
title: Installation
---

# Installation

## Prerequisites

- **Node.js** 18 or later
- **npm** (ships with Node.js)

## Install Fort

Clone the repository and install dependencies:

```bash
git clone https://github.com/your-org/fort.git
cd fort
npm install
```

Build all packages:

```bash
npm run build
```

Link the CLI globally so `fort` is available in your terminal:

```bash
npm link --workspace=packages/cli
```

Verify the installation:

```bash
fort --version
```

## Optional: Swift Menu Bar App

Requires Xcode and Swift toolchain.

```bash
cd packages/swift-shell
swift build
```

## Optional: Tauri Dashboard

Requires [Tauri prerequisites](https://tauri.app/start/prerequisites/) (Rust toolchain, system dependencies).

```bash
cd packages/dashboard
npm install
npm run tauri dev
```

:::tip
You only need the core + CLI for most workflows. The Swift shell and dashboard are optional UI layers.
:::
