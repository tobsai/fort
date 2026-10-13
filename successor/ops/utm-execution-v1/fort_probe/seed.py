"""In-memory dormant NoCloud preview. No ISO, VM bundle or boot operation exists here."""
import json

DISABLED = "raise SystemExit('UTM probe execution disabled: source review and guest adapter required')\n"


def _unit(role):
    common = '''Restart=no
KillMode=control-group
SendSIGKILL=yes
TimeoutStopSec=2
NoNewPrivileges=yes
PrivateNetwork=yes
PrivateIPC=yes
PrivateDevices=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ProtectControlGroups=yes
CapabilityBoundingSet=
AmbientCapabilities=
RestrictAddressFamilies=AF_UNIX
LimitCORE=0
CPUQuota=25%
MemoryMax=128M
'''
    extra = ('User=fort-runtime\nGroup=fort-runtime\nType=notify\nWatchdogSec=2\nRuntimeMaxSec=40\nTasksMax=8\n'
             if role == 'runtime' else
             'DynamicUser=yes\nRuntimeMaxSec=38\nTasksMax=16\nInaccessiblePaths=-/home -/root -/mnt -/var/lib/cloud -/etc/ssh -/run/fort-probe -/var/lib/fort-probe -/run/dbus -/run/systemd\n')
    dependencies = '' if role == 'runtime' else 'BindsTo=fort-runtime.service\nAfter=fort-runtime.service\n'
    return ('[Unit]\nDescription=Disabled Fort synthetic ' + role + '\n' + dependencies
            + '[Service]\nSlice=fort-attempt.slice\nExecStart=/usr/bin/python3 -I -B /opt/fort-probe/disabled.py\n'
            + common + extra)


def preview():
    files = {'/opt/fort-probe/disabled.py': DISABLED,
             '/etc/systemd/system/fort-runtime.service': _unit('runtime'),
             '/etc/systemd/system/fort-tool.service': _unit('tool'),
             '/etc/systemd/system/fort-attempt.slice': '[Unit]\nDescription=Fort synthetic attempt\n[Slice]\nCPUQuota=100%\nMemoryMax=512M\nTasksMax=32\n',
             '/etc/nftables-fort-probe.conf': 'table inet fort_probe {\n chain input { type filter hook input priority 0; policy drop; iifname "lo" accept; }\n chain output { type filter hook output priority 0; policy drop; oifname "lo" accept; }\n chain forward { type filter hook forward priority 0; policy drop; }\n}\n',
             '/etc/ssh/sshd_config.d/99-fort-disabled.conf': 'PasswordAuthentication no\nPubkeyAuthentication no\nPermitRootLogin no\n'}
    users = [{'name': name, 'uid': uid, 'lock_passwd': True, 'shell': '/usr/sbin/nologin',
              'sudo': False, 'groups': []} for name, uid in (('fort-admin', 1000), ('fort-runtime', 2001))]
    config = {'datasource_list': ['NoCloud'], 'users': users, 'disable_root': True,
              'ssh_pwauth': False, 'ssh_deletekeys': True, 'ssh_genkeytypes': [],
              'growpart': {'mode': 'off'}, 'resize_rootfs': False,
              'package_update': False, 'package_upgrade': False,
              'cloud_init_modules': ['users-groups', 'write-files'],
              'cloud_config_modules': [], 'cloud_final_modules': [],
              'write_files': [{'path': path, 'content': content, 'owner': 'root:root', 'permissions': '0600'}
                              for path, content in sorted(files.items())]}
    encode = lambda value: (json.dumps(value, sort_keys=True, indent=2) + '\n').encode()
    return {'meta-data': encode({'instance-id': 'fort-utm-probe-v1', 'local-hostname': 'fort-utm-probe'}),
            'user-data': b'#cloud-config\n' + encode(config),
            'network-config': encode({'version': 2, 'ethernets': {}})}


def package_bytes():
    raise RuntimeError('execution disabled: actual archive members, raw label, kernel and guest adapter remain unverified')
