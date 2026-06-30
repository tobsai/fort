#!/usr/bin/env bash
# recon-hermes.sh — SAFE probes for the `hermes` (Hermes Agent) CLI.
# Throwaway recon for AO-002 / runtime-recon.md. Runs ONLY --help/--version.
# No real task fires: the headless invocation is documented but commented out.
set -euo pipefail

CLI="${HERMES_BIN:-/Users/tobiasgunn/.local/bin/hermes}"
PROBE="timeout 15"

echo "===================================================================="
echo " recon: hermes  ($CLI)"
echo "===================================================================="

echo "--- version ---"
$PROBE "$CLI" --version 2>&1 | head -6 || true

echo "--- top-level help (head) ---"
$PROBE "$CLI" --help 2>&1 | head -60 || true

echo "--- acp subcommand help (head) ---"
$PROBE "$CLI" acp --help 2>&1 | head -30 || true

# --------------------------------------------------------------------
# DOCUMENTED HEADLESS INVOCATION (do NOT uncomment in recon — burns tokens)
# --------------------------------------------------------------------
# One-shot, non-interactive. `-z/--oneshot` prints ONLY the final response text
# (no banner/spinner/session line); approvals auto-bypassed. Add --accept-hooks
# for fully unattended runs that can't prompt for shell-hook approval:
#
#   hermes --oneshot "<task prompt>" \
#     --model anthropic/claude-sonnet-4.6 \
#     --accept-hooks \
#     --yolo
#
# Resume by id/title:    hermes --oneshot "<follow-up>" --resume <session-id>
# Continue most recent:  hermes --oneshot "<follow-up>" --continue
# Editor/streaming mode:  hermes acp        (JSON-RPC ACP server over stdio)
# --------------------------------------------------------------------

echo "--- done (no task executed) ---"
