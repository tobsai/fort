#!/usr/bin/env bash
# recon-codex.sh — SAFE probes for the `codex` (Codex CLI) CLI.
# Throwaway recon for AO-002 / runtime-recon.md. Runs ONLY --help/--version.
# No real task fires: the headless invocation is documented but commented out.
set -euo pipefail

CLI="${CODEX_BIN:-/opt/homebrew/bin/codex}"
PROBE="timeout 15"

echo "===================================================================="
echo " recon: codex  ($CLI)"
echo "===================================================================="

echo "--- version ---"
$PROBE "$CLI" --version 2>&1 | head -5 || true

echo "--- top-level help (head) ---"
$PROBE "$CLI" --help 2>&1 | head -40 || true

echo "--- exec subcommand help (head) ---"
$PROBE "$CLI" exec --help 2>&1 | head -60 || true

# --------------------------------------------------------------------
# DOCUMENTED HEADLESS INVOCATION (do NOT uncomment in recon — burns tokens)
# --------------------------------------------------------------------
# One-shot, non-interactive, JSONL events, no approvals prompt, no sandbox
# escalation (Fort sandboxes externally):
#
#   codex exec "<task prompt>" \
#     --json \
#     --sandbox workspace-write \
#     --ask-for-approval never \
#     --skip-git-repo-check \
#     --output-last-message /tmp/codex-last.txt
#
# Plain text:            codex exec "<task prompt>"
# Prompt via stdin:      echo "<task>" | codex exec -
# Resume a session:      codex exec resume <session-uuid> "<follow-up>"  (or --last)
# Resume via stdin:      echo "<follow-up>" | codex exec resume <uuid> -
# Structured output:     codex exec "<task>" --output-schema schema.json
# Code review (no TUI):  codex review --uncommitted
# --------------------------------------------------------------------

echo "--- done (no task executed) ---"
