#!/bin/sh
set -eu
export PATH=/usr/sbin:/usr/bin:/sbin:/bin LC_ALL=C
test "$(id -u)" = 0
test "$(uname -m)" = aarch64
test "$(systemctl --version | head -1)" = 'systemd 255 (255.4-1ubuntu8.17)'
test -f /sys/fs/cgroup/cgroup.controllers
test "$(id -u fort-admin)" = 1000
nft list table inet fort_probe >/dev/null
for command in python3 nft systemctl systemd-analyze useradd groupadd sshd sudo; do
  command -v "$command" >/dev/null
done
test ! -e /var/lib/fort-probe/installed
test ! -e /var/lib/fort-probe/consumed.json
test -z "$(getent passwd fort-runtime || true)"
test -z "$(getent passwd 2001 || true)"
test -z "$(getent group 2001 || true)"
groupadd --gid 2001 fort-runtime
useradd --uid 2001 --gid 2001 --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin fort-runtime
install -d -m 0755 -o root -g root /var/lib/fort-probe /run/fort-probe
install -d -m 0700 -o fort-runtime -g fort-runtime /var/lib/fort-probe/provider
printf 'synthetic-provider-canary-only\n' >/var/lib/fort-probe/provider/synthetic-canary
chown fort-runtime:fort-runtime /var/lib/fort-probe/provider/synthetic-canary
chmod 0600 /var/lib/fort-probe/provider/synthetic-canary
for unit in apt-daily.timer apt-daily-upgrade.timer apt-daily.service apt-daily-upgrade.service unattended-upgrades.service snapd.service snapd.socket snapd.seeded.service systemd-timesyncd.service; do
  systemctl mask --now "$unit"
done
sshd -t
systemctl reload ssh.service
systemd-analyze verify /etc/systemd/system/fort-runtime.service /etc/systemd/system/fort-tool.service /etc/systemd/system/fort-attempt.slice
systemctl daemon-reload
# No enable/start of any Fort unit. The exact control grant is the only start path.
touch /var/lib/fort-probe/installed
printf 'FORT_PROBE_HOST_ED25519 '
cat /etc/ssh/ssh_host_ed25519_key.pub
printf 'FORT_PROBE_HOST_ED25519 ' >/dev/console
cat /etc/ssh/ssh_host_ed25519_key.pub >/dev/console
