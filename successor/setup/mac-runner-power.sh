#!/bin/bash
# Power settings for the Mac mini that runs Fort turns (Spec 063, revision 2026-10-28): never sleep on
# power, start again after a power failure, wake for network access, and shut down cleanly when the
# battery backup runs low.
#
#   successor/setup/mac-runner-power.sh            # shows what it would change (the default)
#   successor/setup/mac-runner-power.sh --apply    # changes it, asking for your password through sudo
#
# Only for the always-on primary. The laptop keeps its own power settings.
# Undo: System Settings, Energy, or `sudo pmset -c restoredefaults`.
set -eu

apply=false
haltlevel=20
while [ $# -gt 0 ]; do
  case "$1" in
    --apply) apply=true; shift ;;
    --halt-at) haltlevel="${2:-}"; shift 2 ;;
    -h|--help) sed -n '2,11p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done
case "$haltlevel" in ''|*[!0-9]*) echo "Use a battery percentage for --halt-at" >&2; exit 2 ;; esac
if [ "$haltlevel" -lt 5 ] || [ "$haltlevel" -gt 80 ]; then echo "Use --halt-at between 5 and 80" >&2; exit 2; fi

# On AC power: never sleep the system or disks, let the display sleep, wake for network access,
# start up after a power failure, and keep network connections alive.
ac=(pmset -c sleep 0 disksleep 0 displaysleep 15 womp 1 autorestart 1 powernap 0 tcpkeepalive 1)
# On UPS power: shut down cleanly at the battery level given.
ups=(pmset -u haltlevel "$haltlevel")

echo "Fort runner power settings for $(scutil --get ComputerName 2>/dev/null || hostname):"
echo "  sudo ${ac[*]}"
echo "  sudo ${ups[*]}"
if [ "$apply" != true ]; then
  echo "Nothing changed. Run again with --apply to make these changes."
  exit 0
fi
sudo "${ac[@]}"
sudo "${ups[@]}"
echo "Done. Current AC settings:"
pmset -g custom | sed -n '/^AC Power:/,/^[A-Za-z].*Power:/p'
