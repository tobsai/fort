"""Record exact approved preflight, then create only absent private download roots."""
import datetime
import hashlib
import json
import os
import pathlib
import re
import shutil
import stat
import subprocess

HERE = pathlib.Path(__file__).resolve().parent
WORKTREE = HERE.parents[2]
PACKET = WORKTREE / 'successor/ops/isolation-probe-v1'
APP = pathlib.Path('/Users/tobiasgunn/Library/Application Support/FortIsolation')
STATE = pathlib.Path('/Users/tobiasgunn/Library/FortIsolation')
COMMIT = 'e770db9d924e1281b95695b49e809c818f87a4af'
SEAL = '31e22d8e99e3141e9083f7a2246c15469d32362279cf537c56aa35911936e406'


def write_new(name, value):
    with (HERE / name).open('x') as output:
        json.dump(value, output, indent=2)
        output.write('\n')


def command(args, allowed=(0,)):
    result = subprocess.run(args, capture_output=True, text=True, timeout=15,
                            stdin=subprocess.DEVNULL, close_fds=True)
    if result.returncode not in allowed:
        raise RuntimeError((args, result.returncode, result.stderr))
    return {'command': args, 'exit_code': result.returncode,
            'stdout': result.stdout, 'stderr': result.stderr}


def check_absent(path):
    for ancestor in reversed(path.parents):
        info = ancestor.lstat()
        if not stat.S_ISDIR(info.st_mode):
            raise RuntimeError('Non-directory or symlink ancestor: ' + str(ancestor))
    if os.path.lexists(path):
        raise RuntimeError('Target already exists: ' + str(path))


def main():
    os.umask(0o077)
    assert subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=WORKTREE, text=True).strip() == COMMIT
    seal_bytes = (PACKET / 'seal.json').read_bytes()
    assert hashlib.sha256(seal_bytes).hexdigest() == SEAL
    seal = json.loads(seal_bytes)
    for name, digest in seal['files'].items():
        content = (PACKET / name).read_bytes()
        assert hashlib.sha256(content).hexdigest() == digest, name
        committed = subprocess.check_output(['git', 'show', COMMIT + ':successor/ops/isolation-probe-v1/' + name], cwd=WORKTREE)
        assert content == committed, name
    for path in (APP, STATE):
        check_absent(path)
    disk = shutil.disk_usage(APP.parent)
    # Preserve a conservative 32 GiB disk and 8 GiB reclaimable-memory reserve.
    assert disk.free >= (24 + 32) * 1024**3
    resources = command(['/usr/sbin/sysctl', '-n', 'hw.memsize', 'hw.logicalcpu', 'kern.hv_support'])
    ram, cpus, virtualization = map(int, resources['stdout'].split())
    assert ram >= 16 * 1024**3 and cpus >= 2 and virtualization == 1
    memory = command(['/usr/bin/vm_stat'])
    page_size = int(re.search(r'page size of (\d+) bytes', memory['stdout'])[1])
    pages = {name: int(re.search(r'Pages ' + name + r':\s+(\d+)', memory['stdout'])[1])
             for name in ('free', 'inactive', 'speculative')}
    available_estimate = sum(pages.values()) * page_size
    assert available_estimate >= (4 + 8) * 1024**3
    listener = command(['/usr/sbin/lsof', '-nP', '-iTCP:22058', '-sTCP:LISTEN'], (0, 1))
    assert listener['exit_code'] == 1 and not listener['stdout'] and not listener['stderr']
    node = command(['/opt/homebrew/opt/node@26/bin/node', '--version'])
    assert node['stdout'].strip() == 'v26.5.0'
    ca = pathlib.Path('/etc/ssl/cert.pem').lstat()
    assert stat.S_ISREG(ca.st_mode) and ca.st_uid == 0 and not ca.st_mode & 0o022
    now = datetime.datetime.now(datetime.timezone.utc).isoformat()
    write_new('preflight.json', {
        'time_utc': now, 'commit': COMMIT, 'seal_sha256': SEAL,
        'sealed_files_verified': len(seal['files']), 'sealed_files_match_commit': True,
        'roots_absent_and_ancestors_non_symlink': [str(APP), str(STATE)],
        'disk': {'total_bytes': disk.total, 'used_bytes': disk.used, 'free_bytes': disk.free,
                 'experiment_budget_bytes': 24 * 1024**3, 'preserved_reserve_bytes': 32 * 1024**3},
        'resource_query': resources, 'memory_query': memory,
        'reclaimable_memory_estimate_bytes': available_estimate,
        'memory_estimate_note': 'Free + inactive + speculative is an estimate, not a reservation or quota.',
        'port_22058_listener_query': listener, 'node': node,
        'system_ca': {'path': '/etc/ssl/cert.pem', 'regular_root_owned_not_group_or_world_writable': True},
        'resource_read_authority': 'Scoped escalation under preserved original approval and accepted recovery',
    })
    created = []
    for path, role in ((APP, 'private task root'), (APP / 'downloads', 'pinned public downloads')):
        os.mkdir(path, 0o700)
        info = path.lstat()
        created.append({'path': str(path), 'role': role, 'uid': info.st_uid,
                        'device': info.st_dev, 'inode': info.st_ino,
                        'mode': oct(stat.S_IMODE(info.st_mode)), 'type': 'directory',
                        'created_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat()})
        (HERE / 'created-paths.json').write_text(json.dumps(created, indent=2) + '\n')
    print(json.dumps({'preflight': 'passed', 'sealed_files': len(seal['files']),
                      'created_directories': len(created), 'free_disk_bytes': disk.free,
                      'reclaimable_memory_estimate_bytes': available_estimate}))


if __name__ == '__main__':
    main()
