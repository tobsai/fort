"""Render JSON (a YAML subset) from reviewed local fixture files only."""
import json
import pathlib

ROOT = pathlib.Path(__file__).resolve().parent
APP = '/Users/tobiasgunn/Library/Application Support/FortIsolation'
IMAGE = 'noble-server-cloudimg-arm64.img'
IMAGE_SHA = '1d6bffe64b848468ac97f821d369a4846d983de1800ccf6b5ec8853e85cefc55'


def configuration():
    rules = (ROOT / 'guest/egress.nft').read_text()
    boot = '''#!/bin/sh
set -eu
export PATH=/usr/sbin:/usr/bin:/sbin:/bin LC_ALL=C
fail() { systemctl --no-block poweroff; exit 1; }
command -v nft >/dev/null || fail
# Replace our table in one nft transaction; leave other tables untouched.
{ if nft list table inet fort_probe >/dev/null 2>&1; then
    printf 'delete table inet fort_probe\\n'
  fi
  cat <<'FORT_NFT'
''' + rules + '''FORT_NFT
} | nft -f - || fail
'''
    provisions = [
        {'mode': 'dependency', 'skipDefaultDependencyResolution': True, 'script': '#!/bin/sh\nexit 0\n'},
        {'mode': 'boot', 'script': boot},
    ]
    destinations = {
        'controller.py': '/opt/fort-probe/controller.py',
        'runtime.py': '/opt/fort-probe/runtime.py',
        'tool.py': '/opt/fort-probe/tool.py',
        'egress.nft': '/opt/fort-probe/egress.nft',
        'sshd.conf': '/etc/ssh/sshd_config.d/00-fort-probe.conf',
        'fort-runtime.service': '/etc/systemd/system/fort-runtime.service',
        'fort-tool.service': '/etc/systemd/system/fort-tool.service',
        'fort-attempt.slice': '/etc/systemd/system/fort-attempt.slice',
    }
    for source, target in destinations.items():
        provisions.append({'mode': 'data', 'path': target, 'content': (ROOT / 'guest' / source).read_text(),
                           'owner': 'root:root', 'permissions': '0644', 'overwrite': False})
    provisions.append({'mode': 'system', 'script': (ROOT / 'guest/install.sh').read_text()})
    return {
        'minimumLimaVersion': '2.2.1', 'vmType': 'vz', 'os': 'Linux', 'arch': 'aarch64',
        'cpus': 2, 'memory': '4GiB', 'disk': '20GiB', 'plain': True,
        'images': [{'location': APP + '/downloads/' + IMAGE, 'arch': 'aarch64', 'digest': 'sha256:' + IMAGE_SHA}],
        'mounts': [], 'additionalDisks': [], 'networks': [],
        'ssh': {'localPort': 22058, 'loadDotSSHPubKeys': False, 'forwardAgent': False,
                'forwardX11': False, 'forwardX11Trusted': False, 'overVsock': False},
        'user': {'name': 'fort-admin', 'comment': 'Fort isolated synthetic probe administrator', 'uid': 1000,
                 'home': '/home/fort-admin', 'shell': '/bin/bash', 'passwordlessSudo': True},
        'containerd': {'system': False, 'user': False},
        'vmOpts': {'vz': {'diskImageFormat': 'raw', 'rosetta': {'enabled': False, 'binfmt': False}}},
        'video': {'display': 'none'}, 'audio': {'device': 'none'},
        'upgradePackages': False, 'propagateProxyEnv': False,
        'hostResolver': {'enabled': False}, 'dns': ['127.0.0.1'], 'timezone': 'Etc/UTC',
        'portForwards': [{'guestIP': '0.0.0.0', 'guestPortRange': [1, 65535], 'proto': 'any', 'ignore': True}],
        'copyToHost': [], 'env': {}, 'param': {},
        'caCerts': {'removeDefaults': False, 'files': [], 'certs': []},
        'provision': provisions,
    }


if __name__ == '__main__':
    print(json.dumps(configuration(), indent=2))
