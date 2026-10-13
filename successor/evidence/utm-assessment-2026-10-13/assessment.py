"""One approved UTM artifact assessment. Never execute downloaded code."""
import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import ssl
import stat
import subprocess
import sys

HERE = Path(__file__).resolve().parent
WORKTREE = HERE.parents[2]
PROPOSAL = WORKTREE / 'successor/ops/utm-isolation-proposal-v1'
ROOT = Path('/Users/tobiasgunn/Library/Application Support/FortUTMIsolation')
DMG = ROOT / 'downloads/UTM.dmg'
MOUNT = ROOT / 'mount/UTM-4.7.5'
APP = ROOT / 'tools/UTM-4.7.5/UTM.app'
COMMIT = 'ec6dd956bd6ebf45ec685331625ccc429e59b7f6'
SEAL = '99decb7c28b2c2daba3972709642a12ed4d3cce084a40bc3c3dd3e0008899159'
PIN = 'a8435c93cfb5f8bbfeea4b134cfad1ac66b67632b75e438c63b1a8ae043bef0e'
SIZE = 250021057
GIB = 1024**3
os.umask(0o077)


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def write(name, value):
    data = (json.dumps(value, indent=2) + '\n').encode()
    assert len(data) <= 16 * 1024**2, name
    with (HERE / name).open('xb') as output:
        output.write(data)
        output.flush()
        os.fsync(output.fileno())


def digest(path):
    hasher = hashlib.sha256()
    with path.open('rb') as source:
        for chunk in iter(lambda: source.read(1024**2), b''):
            hasher.update(chunk)
    return hasher.hexdigest()


def record(path, hash_content=True):
    info = path.lstat()
    kind = ('directory' if stat.S_ISDIR(info.st_mode) else 'regular' if stat.S_ISREG(info.st_mode)
            else 'symlink' if stat.S_ISLNK(info.st_mode) else 'unsupported')
    assert kind != 'unsupported', str(path)
    value = {'path': str(path), 'type': kind, 'uid': info.st_uid, 'device': info.st_dev,
             'inode': info.st_ino, 'mode': oct(stat.S_IMODE(info.st_mode)),
             'bytes': info.st_size, 'allocated_bytes': info.st_blocks * 512,
             'nlink': info.st_nlink}
    if kind == 'regular' and hash_content:
        value['sha256'] = digest(path)
    if kind == 'symlink':
        value['link_target'] = os.readlink(path)
    return value


def inventory(root, hash_content=True):
    entries = [record(root, hash_content)]
    for parent, directories, files in os.walk(root, followlinks=False):
        for name in sorted(directories + files):
            entries.append(record(Path(parent) / name, hash_content))
    assert len(entries) <= 15000
    return entries


def check_ancestors(path):
    for ancestor in reversed(path.parents):
        assert stat.S_ISDIR(ancestor.lstat().st_mode), 'Symlink/non-directory ancestor: ' + str(ancestor)


def absent(path):
    return not os.path.lexists(path)


def candidate_paths():
    return [Path('/Applications/UTM.app'), Path('/Users/tobiasgunn/Applications/UTM.app'),
            Path('/Users/tobiasgunn/Library/Containers/com.utmapp.UTM'), ROOT]


def run(name, args, timeout=60):
    """Bounded output, no stdin, recorded before an exit status can stop the stage."""
    import selectors
    import time
    start = now()
    process = subprocess.Popen(args, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, close_fds=True,
                               env={'PATH': '/usr/bin:/bin:/usr/sbin:/sbin', 'LANG': 'C', 'LC_ALL': 'C'})
    selector = selectors.DefaultSelector()
    streams = {'stdout': bytearray(), 'stderr': bytearray()}
    for pipe, label in ((process.stdout, 'stdout'), (process.stderr, 'stderr')):
        selector.register(pipe, selectors.EVENT_READ, label)
    end = time.monotonic() + timeout
    failure = None
    try:
        while selector.get_map():
            if time.monotonic() > end:
                failure = 'timeout'
                break
            for key, unused in selector.select(0.1):
                data = os.read(key.fileobj.fileno(), 65536)
                if not data:
                    selector.unregister(key.fileobj)
                    continue
                streams[key.data].extend(data)
                if sum(map(len, streams.values())) > 2 * 1024**2:
                    failure = 'output-cap'
                    break
            if failure:
                break
        if failure:
            process.kill()
        code = process.wait(timeout=5)
    finally:
        selector.close()
        process.stdout.close()
        process.stderr.close()
    result = {'command': args, 'started_at_utc': start, 'ended_at_utc': now(),
              'exit_code': code, 'failure': failure,
              **{key: bytes(value).decode('utf-8', 'replace') for key, value in streams.items()}}
    write(name, result)
    print(json.dumps({'receipt': name, 'exit_code': code, 'failure': failure}), flush=True)
    return result


def require(result):
    assert result['exit_code'] == 0 and result['failure'] is None, result
    return result


def preflight():
    assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=WORKTREE, text=True).strip() == COMMIT
    assert digest(PROPOSAL / 'seal.json') == SEAL
    seal = json.loads((PROPOSAL / 'seal.json').read_text())
    for name, pin in seal['files'].items():
        path = PROPOSAL / name
        assert digest(path) == pin, name
        assert path.read_bytes() == subprocess.check_output(
            ['git', 'show', COMMIT + ':successor/ops/utm-isolation-proposal-v1/' + name], cwd=WORKTREE)
    check_ancestors(ROOT)
    assert all(absent(path) for path in candidate_paths()), 'Existing/unattributed UTM path'
    disk = shutil.disk_usage(ROOT.parent)
    assert disk.free >= 35 * GIB
    ca = Path('/etc/ssl/cert.pem').lstat()
    assert stat.S_ISREG(ca.st_mode) and ca.st_uid == 0 and not ca.st_mode & 0o022
    version = require(run('host-version.json', ['/usr/bin/sw_vers']))
    machine = require(run('host-architecture.json', ['/usr/bin/uname', '-m']))
    assert machine['stdout'].strip() == 'arm64'
    write('authority-and-preflight.json', {
        'time_utc': now(), 'approved_proposal_commit': COMMIT, 'proposal_seal_sha256': SEAL,
        'sealed_files_verified': len(seal['files']),
        'approval_reference': 'Sentinel_ba1df7211ad48191994b113b470cfe43',
        'approval_time_utc': '2026-10-13T00:10:00Z',
        'reminder_reference': 'Sentinel_8046e6f9b2708191b1af3e4b8c328e10',
        'original_request_reference': 'Sentinel_19acd0b905fc81919a84acf25719506d',
        'scope': 'Only pinned official UTM 4.7.5 DMG download, existing OS signature/notarization assessment, bounded staging and owned cleanup. No downloaded-code launch, VM, accounts, keys, permissions or security bypass.',
        'authority_source': 'Delegated owner approval; identifiers and scope only, no historical message bodies copied.',
        'candidate_paths_absent': [str(path) for path in candidate_paths()],
        'task_root_ancestors_real_directories': True,
        'disk_free_bytes': disk.free, 'task_budget_bytes': 3 * GIB,
        'staged_app_cap_bytes': 2 * GIB, 'required_disk_reserve_bytes': 32 * GIB,
        'system_ca_regular_root_owned_not_group_or_world_writable': True,
        'host_version_receipt': 'host-version.json', 'host_architecture_receipt': 'host-architecture.json'})
    created = []
    for path in [ROOT, ROOT / 'downloads', ROOT / 'mount', MOUNT, ROOT / 'tools', ROOT / 'tools/UTM-4.7.5']:
        os.mkdir(path, 0o700)
        created.append(record(path))
        with (HERE / 'created-directories.jsonl').open('a') as output:
            output.write(json.dumps(created[-1]) + '\n')
            output.flush()
            os.fsync(output.fileno())
    print(json.dumps({'preflight': 'passed', 'created_private_directories': len(created), 'free_disk_bytes': disk.free}))


def download():
    json.loads((HERE / 'authority-and-preflight.json').read_text())
    script = WORKTREE / 'successor/ops/isolation-probe-v1/fetch-public.py'
    assert script.read_bytes() == subprocess.check_output(
        ['git', 'show', COMMIT + ':successor/ops/isolation-probe-v1/fetch-public.py'], cwd=WORKTREE)
    spec = importlib.util.spec_from_file_location('approved_downloader', script)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    entry = json.loads((PROPOSAL / 'scope.json').read_text())['downloads'][0]
    assert entry['name'] == 'UTM.dmg' and entry['sha256'] == PIN and entry['max_bytes'] == 268435456
    assert entry['url'] == 'https://github.com/utmapp/UTM/releases/download/v4.7.5/UTM.dmg'
    for path in (DMG, DMG.with_name('UTM.dmg.partial')):
        assert absent(path)
    write('downloader-provenance.json', {'module': str(script.relative_to(WORKTREE)),
        'sha256': digest(script), 'unchanged_from_commit': COMMIT,
        'called_function': 'download', 'main_not_called': True, 'input_count': 1,
        'allowed_https_hosts': ['github.com', 'release-assets.githubusercontent.com'],
        'proxies_disabled': True, 'cookies_auth_netrc_not_used': True,
        'redirect_query_omitted_from_receipts': True, 'download_timeout_seconds': 600})
    target = module.private_directory(DMG.parent)
    evidence = os.open(HERE, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        import signal
        def timeout(signum, frame):
            raise module.DownloadFailure('total-download-timeout')
        signal.signal(signal.SIGALRM, timeout)
        signal.alarm(600)
        module.download(entry, 1, target, evidence, {'github.com', 'release-assets.githubusercontent.com'},
                        ssl.create_default_context(cafile='/etc/ssl/cert.pem'))
        signal.alarm(0)
    finally:
        os.close(target)
        os.close(evidence)
    assert DMG.lstat().st_size == SIZE and digest(DMG) == PIN
    write('download-manifest.json', inventory(ROOT))
    print(json.dumps({'download': 'accepted', 'bytes': SIZE, 'sha256': PIN}), flush=True)


if __name__ == '__main__':
    assert sys.argv[1:] in (['preflight'], ['download'])
    {'preflight': preflight, 'download': download}[sys.argv[1]]()
