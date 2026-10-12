"""Extract the inspected, publisher-verified Lima archive without executing it."""
import datetime
import hashlib
import json
import os
import pathlib
import posixpath
import shutil
import stat
import tarfile

HERE = pathlib.Path(__file__).resolve().parent
APP = pathlib.Path('/Users/tobiasgunn/Library/Application Support/FortIsolation')
INSTALL = APP / 'tools/lima/2.2.1'
manifest_path = HERE / 'created-paths.json'
manifest = json.loads(manifest_path.read_text())
known = {entry['path']: entry for entry in manifest}
inspection = json.loads((HERE / 'lima-archive-inspection.json').read_text())
entries = {entry['path']: entry for entry in inspection['entries']}
publisher = json.loads((HERE / 'publisher-stdout.txt').read_text())
assert json.loads((HERE / 'publisher-result.json').read_text())['exit_code'] == 0
assert publisher['publisherSignaturesVerified'] is True
assert all(row['negativeControlsPassed'] == 3 for row in publisher['receipts'])
assert inspection['expanded_bytes'] <= 512 * 1024**2
assert shutil.disk_usage(APP).free > (24 + 32) * 1024**3
os.umask(0o077)


def check_parent(path):
    old = known[str(path)]
    info = path.lstat()
    assert stat.S_ISDIR(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o700
    assert (info.st_uid, info.st_dev, info.st_ino) == (old['uid'], old['device'], old['inode'])


def record(path, role, digest=None, link=None):
    info = path.lstat()
    kind = 'directory' if stat.S_ISDIR(info.st_mode) else 'symlink' if stat.S_ISLNK(info.st_mode) else 'regular'
    entry = {'path': str(path), 'type': kind, 'uid': info.st_uid, 'device': info.st_dev,
             'inode': info.st_ino, 'mode': oct(stat.S_IMODE(info.st_mode)), 'bytes': info.st_size,
             'allocated_bytes': info.st_blocks * 512, 'hard_links': info.st_nlink,
             'role': role, 'recorded_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat()}
    if digest:
        entry['sha256'] = digest
    if link:
        entry['link_target'] = link
    manifest.append(entry)
    known[str(path)] = entry
    with manifest_path.open('w') as output:
        json.dump(manifest, output, indent=2)
        output.write('\n')
        output.flush()
        os.fsync(output.fileno())


check_parent(APP / 'tools')
archive = APP / 'downloads/lima-2.2.1-Darwin-arm64.tar.gz'
assert hashlib.sha256(archive.read_bytes()).hexdigest() == '9e9eacce88f37e185c346bad73aa6136f738d8cdf8c3bb23cd42b071824bc66e'
with tarfile.open(archive, 'r:gz') as source:
    members = source.getmembers()
    assert len(members) == len(entries) == 215
    names = set()
    for member in members:
        name = str(pathlib.PurePosixPath(member.name))
        assert name not in names and not name.startswith('/') and '..' not in pathlib.PurePosixPath(name).parts
        names.add(name)
        assert member.name in entries and not member.mode & 0o6000
        if member.isfile():
            payload = source.extractfile(member).read()
            assert len(payload) == entries[member.name]['bytes']
            assert hashlib.sha256(payload).hexdigest() == entries[member.name]['sha256']
        elif member.issym():
            assert member.name == './share/doc/lima/templates' and member.linkname == '../../lima/templates'
            assert posixpath.normpath(posixpath.join(posixpath.dirname(name), member.linkname)) == 'share/lima/templates'
            assert not any(str(pathlib.PurePosixPath(other.name)).startswith(name + '/') for other in members)
        else:
            assert member.isdir()
    directories = {APP / 'tools/lima', INSTALL}
    directories.update(INSTALL / str(pathlib.PurePosixPath(member.name)) for member in members if member.isdir())
    for directory in sorted(directories, key=lambda value: (len(value.parts), str(value))):
        check_parent(directory.parent)
        directory.mkdir(mode=0o700)
        record(directory, 'private inspected manager directory')
    for member in members:
        if member.isdir():
            continue
        target = INSTALL / str(pathlib.PurePosixPath(member.name))
        check_parent(target.parent)
        if member.issym():
            os.symlink(member.linkname, target)
            record(target, 'inspected internal documentation link', link=member.linkname)
            assert target.resolve() == INSTALL / 'share/lima/templates'
            continue
        payload = source.extractfile(member).read()
        mode = member.mode & 0o700
        descriptor = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, mode)
        with os.fdopen(descriptor, 'wb') as output:
            output.write(payload)
            output.flush()
            os.fsync(output.fileno())
        record(target, 'inspected public manager file', hashlib.sha256(payload).hexdigest())
        assert target.read_bytes() == payload
result = {'time_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
          'members': len(members), 'expanded_bytes': inspection['expanded_bytes'],
          'all_file_hashes_match_inspection': True, 'legal_files_preserved': ['share/doc/lima/LICENSE'],
          'archive_mode_policy': 'Preserve owner permissions and executable bit; strip group/world permissions inside private roots.',
          'internal_documentation_symlink': 'share/doc/lima/templates -> ../../lima/templates',
          'hooks_or_manager_run': False, 'install': str(INSTALL),
          'free_disk_bytes': shutil.disk_usage(APP).free}
with (HERE / 'manager-extraction.json').open('x') as output:
    json.dump(result, output, indent=2)
    output.write('\n')
print(json.dumps(result))
