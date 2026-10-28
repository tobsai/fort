#!/bin/bash
# Checks a Mac before it runs Fort turns for the cloud hub (Spec 063, revision 2026-10-28).
# Read-only: it changes nothing, needs no sudo, and never reads a token or a credential.
#
#   successor/setup/mac-runner-check.sh --role primary    # the Mac mini
#   successor/setup/mac-runner-check.sh --role overflow   # the laptop
#
# Exit status: 0 when nothing must be fixed, 1 when something must, 2 for a usage error.
# Written for the bash 3.2 that ships with macOS.
set -u

role=primary
hub=https://fort.mtree.io
while [ $# -gt 0 ]; do
  case "$1" in
    --role) role="${2:-}"; shift 2 ;;
    --hub) hub="${2:-}"; shift 2 ;;
    -h|--help) sed -n '2,10p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done
case "$role" in primary|overflow) ;; *) echo "Use --role primary or --role overflow" >&2; exit 2 ;; esac
case "$hub" in https://*) ;; *) echo "Use the hub's https address" >&2; exit 2 ;; esac

failures=0
ok()   { printf '  ok    %s\n' "$1"; }
warn() { printf '  note  %s\n' "$1"; }
fix()  { printf '  FIX   %s\n' "$1"; failures=$((failures + 1)); }

# The value of one pmset setting in the AC power profile, or empty when pmset doesn't list it.
ac_setting() {
  pmset -g custom 2>/dev/null | awk -v key="$1" '
    /^AC Power:/ { ac = 1; next } /^[A-Za-z].*Power:/ { ac = 0 }
    ac && $1 == key { print $2; exit }'
}

echo "Fort runner check ($role) for $(scutil --get ComputerName 2>/dev/null || hostname)"

echo "macOS"
version=$(sw_vers -productVersion 2>/dev/null || true)
if [ -z "$version" ]; then fix "This isn't macOS, or sw_vers is missing."; else ok "macOS $version"; fi

echo "Disk encryption"
if fdesetup status 2>/dev/null | grep -q 'FileVault is On'; then
  ok "FileVault is on."
  if [ "$role" = primary ]; then
    warn "With FileVault on, a Mac that restarts after a power cut waits at the unlock screen. Fort can't run until someone logs in, so runs queue and the hub alerts after 15 minutes."
  fi
elif [ "$role" = primary ]; then
  fix "FileVault is off. Turn it on in System Settings, Privacy & Security, FileVault (decided Oct 28)."
else
  warn "FileVault is off on this laptop."
fi

if [ "$role" = primary ]; then
  echo "Power (on AC)"
  sleep_after=$(ac_setting sleep)
  if [ "$sleep_after" = 0 ]; then ok "The Mac never sleeps on power."; else fix "The Mac sleeps after ${sleep_after:-an unknown number of} minutes. Run mac-runner-power.sh --apply."; fi
  if [ "$(ac_setting autorestart)" = 1 ]; then ok "It starts up again after a power failure."; else fix "It won't start up after a power failure. Run mac-runner-power.sh --apply."; fi
  if [ "$(ac_setting womp)" = 1 ]; then ok "Wake for network access is on."; else warn "Wake for network access is off."; fi
  if pmset -g ps 2>/dev/null | grep -qi 'ups'; then ok "A UPS is connected."; else warn "No UPS shows up over USB, so the Mac can't shut down cleanly when the battery backup runs low."; fi
fi

echo "Fort"
app=""
for candidate in /Applications/Fort.app "$HOME/Applications/Fort.app"; do [ -d "$candidate" ] && app="$candidate" && break; done
if [ -n "$app" ]; then ok "Fort is installed at $app."; else fix "Fort isn't in Applications. Install the Mac build first."; fi
if [ -n "$app" ] && [ ! -d "$app/Contents/Helpers/FortHost.app" ]; then fix "This Fort build has no host helper, so it can't run turns."; fi
if pgrep -f 'FortHost.app/Contents/MacOS' >/dev/null 2>&1; then ok "The Fort host is running."; else fix "The Fort host isn't running. Open Fort."; fi

echo "Hub"
if reply=$(curl -fsS --max-time 10 "$hub/auth/session" 2>/dev/null) && printf '%s' "$reply" | grep -q '"google"'; then
  ok "$hub answers."
else
  fix "$hub didn't answer. Check the network, or that the hub is up."
fi

echo "Still to do by hand (this script can't check them):"
echo "  - Sign in to the same ChatGPT and Claude accounts as the other Mac."
echo "  - Enroll: in the web app, Setup, Macs, Add a Mac; paste the invitation into Fort, Devices, Connect to cloud hub."
if [ "$role" = primary ]; then
  echo "  - Let Fort's host start at login so it survives a restart (a startup item, so it needs your OK)."
fi

if [ "$failures" -gt 0 ]; then echo "$failures thing(s) to fix."; exit 1; fi
echo "Nothing to fix."
