"""Remove only this failed run's recorded seven public files and two new directories."""
import datetime
import hashlib
import json
import os
import pathlib
import shutil
import stat
import subprocess

HERE = pathlib.Path(__file__).resolve().parent
PACKET = HERE.parents[2] / 'successor/ops/isolation-probe-v1'
APP = pathlib.Path('/Users/tobiasgunn/Library/Application Support/FortIsolation')
STATE_PARENT = pathlib.Path('/Users/tobiasgunn/Library/FortIsolation')


def timestamp():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def main():
    os.umask(0o077)
    scope = json.loads((PACKET / 'scope.json').read_text())
    manifest_bytes = (HERE / 'created-paths.json').read_bytes()
    manifest = json.loads(manifest_bytes)
    by_path = {entry['path']: entry for entry in manifest}
    expected_files = {entry['name']: entry for entry in scope['downloads'][:7]}
    expected_paths = {str(APP), str(APP / 'downloads')} | {str(APP / 'downloads' / name) for name in expected_files}
    assert set(by_path) == expected_paths and len(manifest) == 9
    assert json.loads((HERE / 'download-result.json').read_text())['exit_code'] == 1
    assert json.loads((HERE / 'blocker.json').read_text())['failed_input'] == scope['downloads'][7]['name'] == 'ubuntu.pub'
    assert not os.path.lexists(STATE_PARENT)
    listener = subprocess.run(['/usr/sbin/lsof', '-nP', '-iTCP:22058', '-sTCP:LISTEN'],
                              stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=15)
    assert listener.returncode == 1 and not listener.stdout and not listener.stderr
    for ancestor in reversed(APP.parents):
        assert stat.S_ISDIR(ancestor.lstat().st_mode), str(ancestor)

    def identity(info, path):
        expected = by_path[str(path)]
        assert (info.st_uid, info.st_dev, info.st_ino) == (expected['uid'], expected['device'], expected['inode']), str(path)
        assert info.st_uid == os.getuid()
        assert stat.S_IMODE(info.st_mode) == int(expected['mode'], 8)
        if expected['type'] == 'directory':
            assert stat.S_ISDIR(info.st_mode)
        else:
            assert stat.S_ISREG(info.st_mode) and info.st_nlink == 1 and info.st_size == expected['bytes']

    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
    parent_fd = os.open(APP.parent, flags)
    root_fd = os.open(APP.name, flags, dir_fd=parent_fd)
    downloads_fd = os.open('downloads', flags, dir_fd=root_fd)
    try:
        identity(os.fstat(root_fd), APP)
        identity(os.fstat(downloads_fd), APP / 'downloads')
        assert set(os.listdir(root_fd)) == {'downloads'}
        assert set(os.listdir(downloads_fd)) == set(expected_files)
        for name, pin in expected_files.items():
            path = APP / 'downloads' / name
            fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=downloads_fd)
            with os.fdopen(fd, 'rb') as source:
                identity(os.fstat(source.fileno()), path)
                digest = hashlib.sha256()
                while True:
                    chunk = source.read(1024 * 1024)
                    if not chunk:
                        break
                    digest.update(chunk)
            assert digest.hexdigest() == pin['sha256'] == by_path[str(path)]['sha256']
        evidence_files = [path for path in HERE.iterdir() if path.is_file()]
        assert sum(path.stat().st_size for path in evidence_files) < 128 * 1024**2
        before = {'time_utc': timestamp(), 'created_manifest_sha256': hashlib.sha256(manifest_bytes).hexdigest(),
                  'verified_regular_files': len(expected_files), 'verified_directories': 2,
                  'unexpected_children': [], 'state_parent_absent': True,
                  'vm_never_created': True, 'process_termination': 'not applicable; no manager or VM was executed',
                  'port_22058': {'exit_code': listener.returncode, 'stdout': listener.stdout, 'stderr': listener.stderr},
                  'retained_evidence_hashes': {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in evidence_files}}
        with (HERE / 'cleanup-before.json').open('x') as output:
            json.dump(before, output, indent=2)
            output.write('\n')
        removed = []
        report = {'started_at_utc': timestamp(), 'removed': removed, 'completed': False}
        report_path = HERE / 'cleanup-report.json'
        with report_path.open('x') as output:
            json.dump(report, output, indent=2)
            output.write('\n')
        for name in sorted(expected_files):
            path = APP / 'downloads' / name
            identity(os.stat(name, dir_fd=downloads_fd, follow_symlinks=False), path)
            os.unlink(name, dir_fd=downloads_fd)
            removed.append({'path': str(path), 'action': 'unlink', 'time_utc': timestamp()})
            report_path.write_text(json.dumps(report, indent=2) + '\n')
        assert not os.listdir(downloads_fd)
        identity(os.stat('downloads', dir_fd=root_fd, follow_symlinks=False), APP / 'downloads')
        os.rmdir('downloads', dir_fd=root_fd)
        removed.append({'path': str(APP / 'downloads'), 'action': 'rmdir', 'time_utc': timestamp()})
        report_path.write_text(json.dumps(report, indent=2) + '\n')
        assert not os.listdir(root_fd)
        identity(os.stat(APP.name, dir_fd=parent_fd, follow_symlinks=False), APP)
        os.rmdir(APP.name, dir_fd=parent_fd)
        removed.append({'path': str(APP), 'action': 'rmdir', 'time_utc': timestamp()})
        report.update({'completed': True, 'ended_at_utc': timestamp(), 'remaining_owned_paths': [],
                       'free_disk_bytes_after': shutil.disk_usage(APP.parent).free,
                       'secure_erasure_claimed': False})
        report_path.write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps({'removed_files': 7, 'removed_directories': 2, 'completed': True}))
    finally:
        os.close(downloads_fd)
        os.close(root_fd)
        os.close(parent_fd)


if __name__ == '__main__':
    main()
