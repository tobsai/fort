"""Static checks only. Passing does not authorize or prove provisioning."""
import ast
import base64
import configparser
import hashlib
import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parent
APP = '/Users/tobiasgunn/Library/Application Support/FortIsolation'
STATE = '/Users/tobiasgunn/Library/FortIsolation/state'
LIMA = 'https://github.com/lima-vm/lima/releases/download/v2.2.1/'
UBUNTU = 'https://cloud-images.ubuntu.com/noble/20260926/'
URLS = {
    LIMA + 'lima-2.2.1-Darwin-arm64.tar.gz', LIMA + 'SHA256SUMS', LIMA + 'SHA256SUMS.asc',
    'https://github.com/AkihiroSuda.gpg', UBUNTU + 'noble-server-cloudimg-arm64.img',
    UBUNTU + 'SHA256SUMS', UBUNTU + 'SHA256SUMS.gpg', UBUNTU + 'noble-server-cloudimg-arm64.manifest',
    'https://keyserver.ubuntu.com/pks/lookup?op=get&search=0xD2EB44626FDDC30B513D5BB71A5D6C4C7DB87C81',
    'https://registry.npmjs.org/openpgp/6.3.2', 'https://registry.npmjs.org/-/npm/v1/keys',
    'https://registry.npmjs.org/openpgp/-/openpgp-6.3.2.tgz',
}
VERIFIER_INTEGRITY = 'sha512-wcZTzHz41LV8Y48zH/JlD1JT8YdNmpWOuMAjQ/podvvCwsjBEU37K3znJh4UQcj3hksE5z9TzvhbUbwzsGvlLQ=='


def policy_errors(config, scope):
    errors = []
    expected = {'vmType': 'vz', 'os': 'Linux', 'arch': 'aarch64', 'cpus': 2, 'memory': '4GiB',
                'disk': '20GiB', 'plain': True, 'mounts': [], 'additionalDisks': [], 'networks': [],
                'upgradePackages': False, 'propagateProxyEnv': False, 'env': {}, 'param': {},
                'hostResolver': {'enabled': False}, 'dns': ['127.0.0.1'], 'copyToHost': [],
                'containerd': {'system': False, 'user': False}}
    for key, value in expected.items():
        if config.get(key) != value:
            errors.append('Unexpected setting: ' + key)
    if 'base' in config:
        errors.append('No inherited template allowed')
    if config.get('ssh') != {'localPort': 22058, 'loadDotSSHPubKeys': False, 'forwardAgent': False,
                            'forwardX11': False, 'forwardX11Trusted': False, 'overVsock': False}:
        errors.append('SSH scope changed')
    if config.get('user') != {'name': 'fort-admin', 'comment': 'Fort isolated synthetic probe administrator',
                             'uid': 1000, 'home': '/home/fort-admin', 'shell': '/bin/bash', 'passwordlessSudo': True}:
        errors.append('Guest identity/admin scope changed')
    if config.get('images') != [{'location': APP + '/downloads/noble-server-cloudimg-arm64.img',
                                'arch': 'aarch64', 'digest': 'sha256:1d6bffe64b848468ac97f821d369a4846d983de1800ccf6b5ec8853e85cefc55'}]:
        errors.append('Image must be the exact verified local payload')
    provisions = config.get('provision', [])
    if not provisions or provisions[0] != {'mode': 'dependency', 'skipDefaultDependencyResolution': True,
                                          'script': '#!/bin/sh\nexit 0\n'}:
        errors.append('Default dependency resolution not disabled')
    if any(item.get('mode') not in ('boot', 'data', 'system', 'dependency') or 'file' in item for item in provisions):
        errors.append('Only sealed inline provisioning accepted')
    if scope.get('execution_authorized') is not False or scope.get('status') != 'blocked-for-provisioning':
        errors.append('This static packet grants no execution authority')
    if scope.get('lima_home') != STATE or scope.get('instance') != 'fort-isolation-probe':
        errors.append('State identity changed')
    if len((STATE + '/fort-isolation-probe/ssh.sock.1234567890123456').encode()) >= 104:
        errors.append('macOS Unix socket limit exceeded')
    downloads = scope.get('downloads', [])
    if len(downloads) != len(URLS) or {item['url'] for item in downloads} != URLS:
        errors.append('Download allowlist changed')
    if any(not (len(item.get('sha256', '')) == 64 or len(item.get('sha512', '')) == 128) for item in downloads):
        errors.append('Every payload must have an exact digest')
    verifier = scope.get('private_verifier', {})
    expected_verifier = {'package': 'openpgp@6.3.2', 'runtime': '/opt/homebrew/opt/node@26/bin/node',
        'runtime_version': 'v26.5.0', 'installation': APP + '/tools/openpgp/6.3.2',
        'entrypoint': 'package/dist/node/openpgp.mjs', 'extracted_max_bytes': 25165824,
        'runtime_dependencies': [], 'install_hooks': False, 'network_during_verification': False,
        'auxiliary_margin_bytes': 301989888, 'package_downloaded_or_executed': False}
    if any(verifier.get(key) != value for key, value in expected_verifier.items()):
        errors.append('Private verifier scope changed')
    archive = next((item for item in downloads if item.get('name') == 'openpgp-6.3.2.tgz'), {})
    if archive.get('integrity') != VERIFIER_INTEGRITY or archive.get('sha512') != base64.b64decode(VERIFIER_INTEGRITY[7:]).hex() or archive.get('max_bytes') != 8388608:
        errors.append('Verifier archive pin/bound changed')
    if scope.get('guest_package_installs') != [] or scope.get('host_package_installs') != []:
        errors.append('No package-manager dependency expansion allowed')
    if scope.get('resources') != {'cpus': 2, 'ram_bytes': 4294967296, 'disk_bytes': 21474836480,
                                  'auxiliary_budget_bytes': 4294967296, 'aggregate_budget_bytes': 25769803776}:
        errors.append('Resource envelope changed')
    if scope.get('download_diagnostics') != {
            'directory': APP + '/evidence/download-diagnostics', 'max_receipts': 13,
            'receipt_max_bytes': 8192, 'public_key_body_max_bytes': 65536,
            'public_key_body_names': ['lima.pub', 'ubuntu.pub'], 'aggregate_max_bytes': 262144,
            'inside_existing_evidence_budget_bytes': 134217728,
            'execute_response_bodies': False, 'automatic_retry': False}:
        errors.append('Download diagnostics scope changed')
    return errors


def read_units(root):
    result = {}
    for path in sorted((root / 'guest').glob('fort-*.*')):
        if path.suffix not in ('.service', '.slice'):
            continue
        parser = configparser.ConfigParser(interpolation=None, strict=True)
        parser.optionxform = str
        parser.read_string(path.read_text())
        result[path.name] = {section: dict(parser[section]) for section in parser.sections()}
    return result


def unit_errors(units):
    errors = []
    for name in ('fort-runtime.service', 'fort-tool.service'):
        unit = units.get(name, {})
        service = unit.get('Service', {})
        expected = {'KillMode': 'control-group', 'SendSIGKILL': 'yes', 'Restart': 'no', 'ExitType': 'main',
                    'NoNewPrivileges': 'yes', 'PrivateNetwork': 'yes', 'PrivateIPC': 'yes', 'PrivateDevices': 'yes',
                    'ProtectSystem': 'strict', 'ProtectHome': 'yes', 'ProtectControlGroups': 'yes',
                    'RestrictNamespaces': 'yes', 'CapabilityBoundingSet': '', 'AmbientCapabilities': '',
                    'Slice': 'fort-attempt.slice', 'RestrictAddressFamilies': 'AF_UNIX', 'TimeoutStopSec': '2s'}
        for key, value in expected.items():
            if service.get(key) != value:
                errors.append(name + ': unexpected ' + key)
        if 'Install' in unit or 'PrivatePIDs' in service or service.get('Delegate') not in (None, 'no'):
            errors.append(name + ': forbidden boot enablement/delegation/unsupported setting')
    runtime = units['fort-runtime.service']['Service']
    if any(runtime.get(k) != v for k, v in {'Type': 'notify', 'NotifyAccess': 'main', 'WatchdogSec': '2s',
                                          'RuntimeMaxSec': '40s', 'User': 'fort-runtime'}.items()):
        errors.append('Runtime lifecycle contract changed')
    tool = units['fort-tool.service']
    if tool['Unit'].get('BindsTo') != 'fort-runtime.service' or tool['Service'].get('DynamicUser') != 'yes':
        errors.append('Tool lifetime/identity changed')
    for path in ('/run/fort-probe', '/var/lib/fort-probe', '/run/systemd', '/mnt', '/etc/ssh'):
        if path not in tool['Service'].get('InaccessiblePaths', '').split():
            errors.append('Tool management exposure: ' + path)
    return errors


def digest_errors(content, manifest):
    errors = []
    if set(content) != set(manifest):
        errors.append('Sealed file set differs')
    for path, data in content.items():
        if manifest.get(path) != hashlib.sha256(data).hexdigest():
            errors.append('Digest mismatch: ' + path)
    return errors


def main():
    import render_config
    config = json.loads((ROOT / 'lima.yaml').read_text())
    scope = json.loads((ROOT / 'scope.json').read_text())
    errors = policy_errors(config, scope) + unit_errors(read_units(ROOT))
    if config != render_config.configuration():
        errors.append('Embedded provisioning differs from reviewed source')
    paths = list(ROOT.rglob('*'))
    if any(path.is_symlink() for path in paths):
        errors.append('Symlink in packet')
    content = {str(path.relative_to(ROOT)): path.read_bytes() for path in paths
               if path.is_file() and not path.is_symlink() and path != ROOT / 'seal.json'}
    seal = json.loads((ROOT / 'seal.json').read_text())
    errors += digest_errors(content, seal['files'])
    for name, data in content.items():
        if name.endswith('.py'):
            ast.parse(data, filename=name)
        if name.endswith('.json'):
            json.loads(data)
    print(json.dumps({'static_checks': 'failed' if errors else 'passed', 'sealed_files': len(content),
                      'execution_authorized': False, 'errors': errors, 'provisioning_blockers': scope['blockers']}, indent=2))
    return 1 if errors else (2 if '--require-executable' in sys.argv else 0)


if __name__ == '__main__':
    raise SystemExit(main())
