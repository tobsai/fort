#!/usr/bin/env bash
# recon-claude.sh — SAFE probes for the `claude` (Claude Code) CLI.
# Throwaway recon for AO-002 / runtime-recon.md. Runs ONLY --help/--version.
# No real task fires: the headless invocation is documented but commented out.
set -euo pipefail

CLI="${CLAUDE_BIN:-/Users/tobiasgunn/.local/bin/claude}"
PROBE="timeout 15"

echo "===================================================================="
echo " recon: claude  ($CLI)"
echo "===================================================================="

echo "--- version ---"
$PROBE "$CLI" --version 2>&1 | head -5 || true

echo "--- top-level help (head) ---"
$PROBE "$CLI" --help 2>&1 | head -60 || true

# --------------------------------------------------------------------
# DOCUMENTED HEADLESS INVOCATION (do NOT uncomment in recon — burns tokens)
# --------------------------------------------------------------------
# One-shot, non-interactive, JSON-stream output, no TUI, no session file:
#
#   claude -p "<task prompt>" \
#     --output-format stream-json \
#     --include-partial-messages \
#     --model sonnet \
#     --permission-mode bypassPermissions \
#     --no-session-persistence
#
# Plain text (simplest): claude -p "<task prompt>"
# Single JSON result:    claude -p "<task prompt>" --output-format json
# Resume a session:      claude -p "<task prompt>" --resume <session-uuid>
# Pin a session id:      claude -p "<task>" --session-id <uuid>
# Human-in-the-loop in:  --input-format stream-json  (feed JSON user msgs on stdin)
# --------------------------------------------------------------------

echo "--- done (no task executed) ---"
