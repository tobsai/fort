"""Exact owned-tree reconciliation. Caller supplies a fresh trusted OS observation.

No process killing, UTM registration, protected-state access or recursive rm exists here.
"""
from copy import deepcopy
import hashlib
import os
from pathlib import Path
import stat
import re
import uuid
from .evidence import exact


def _identity(info):
    return {'device': info.st_dev, 'inode': info.st_ino, 'uid': info.st_uid,
            'mode': stat.S_IMODE(info.st_mode), 'kind': stat.S_IFMT(info.st_mode)}


def _open_directory(path):
    path = Path(path)
    if not path.is_absolute() or '..' in path.parts:
        raise ValueError('absolute non-traversing directory required')
    fd = os.open('/', os.O_RDONLY | os.O_DIRECTORY)
    try:
        for part in path.parts[1:]:
            next_fd = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
            os.close(fd)
            fd = next_fd
        return fd
    except Exception:
        os.close(fd)
        raise


class OwnedTree:
    def __init__(self, root, vm_uuid, expected_processes):
        if not isinstance(vm_uuid,str) or str(uuid.UUID(vm_uuid))!=vm_uuid:
            raise ValueError('canonical VM UUID required')
        if not isinstance(expected_processes,list): raise ValueError('process list required')
        self.processes = deepcopy(expected_processes)
        if not self.processes or len(self.processes) > 32:
            raise ValueError('bounded attributed process inventory required')
        for process in self.processes:
            if (not isinstance(process,dict) or set(process) != {'role', 'pid', 'start_time', 'executable_sha256'}
                    or type(process['pid']) is not int or process['pid'] <= 0
                    or process['role'] not in ('app','virtualization','helper')
                    or not isinstance(process['start_time'],str) or not process['start_time']
                    or not isinstance(process['executable_sha256'],str) or not re.fullmatch('[0-9a-f]{64}',process['executable_sha256'])):
                raise ValueError('incomplete process identity')
        self.path = Path(root)
        self.parent = _open_directory(self.path.parent)
        self.fd = None
        try:
            os.mkdir(self.path.name, mode=0o700, dir_fd=self.parent)
            self.fd = os.open(self.path.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=self.parent)
            info = os.fstat(self.fd)
            if info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o700:
                raise ValueError('owned private root required')
            self.root_identity = _identity(info)
            self.vm_uuid = vm_uuid
            self.processes = deepcopy(expected_processes)
            self.initial = self.reconciled = None
            self.mutable = set()
        except Exception:
            self.close()
            raise

    def __enter__(self): return self
    def __exit__(self, *args): self.close()

    def close(self):
        if self.fd is not None:
            os.close(self.fd)
            self.fd = None
        if self.parent is not None:
            os.close(self.parent)
            self.parent = None

    def _quiescent(self, proof):
        keys = {'vm_package_uuid', 'expected_processes', 'observed_processes', 'open_handles', 'mounted_devices',
                'registration_removed', 'vm_stopped', 'observation_complete'}
        if (set(proof) != keys or proof['vm_package_uuid'] != self.vm_uuid
                or not exact(proof['expected_processes'], self.processes)
                or proof['observed_processes'] != [] or proof['open_handles'] != [] or proof['mounted_devices'] != []
                or any(proof[k] is not True for k in ('registration_removed', 'vm_stopped', 'observation_complete'))):
            raise ValueError('owned VM/processes/handles/mounts not proven absent')

    def _snapshot(self):
        if self.fd is None or _identity(os.stat(self.path.name, dir_fd=self.parent, follow_symlinks=False)) != self.root_identity:
            raise ValueError('owned root identity changed')
        records, total = {}, [0]
        def walk(directory, prefix):
            for name in sorted(os.listdir(directory)):
                path = prefix + name
                if len(records) >= 4096:
                    raise ValueError('entry bound exceeded')
                info = os.stat(name, dir_fd=directory, follow_symlinks=False)
                if info.st_uid != os.getuid() or info.st_dev != self.root_identity['device']:
                    raise ValueError('unowned or mounted object')
                if not (stat.S_ISREG(info.st_mode) or stat.S_ISDIR(info.st_mode)):
                    raise ValueError('links/devices/sockets cannot enter deletion manifest')
                record = _identity(info)
                records[path] = record
                if stat.S_ISDIR(info.st_mode):
                    child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=directory)
                    try:
                        if _identity(os.fstat(child)) != record: raise ValueError('directory replaced')
                        walk(child, path + '/')
                    finally: os.close(child)
                else:
                    if info.st_nlink != 1: raise ValueError('hard-linked file')
                    total[0] += info.st_size
                    if total[0] > 24 * 1024 ** 3: raise ValueError('logical storage cap exceeded')
                    file_fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=directory)
                    with os.fdopen(file_fd, 'rb') as stream:
                        before = os.fstat(stream.fileno())
                        if _identity(before) != record or before.st_nlink != 1: raise ValueError('file replaced')
                        digest, read = hashlib.sha256(), 0
                        while True:
                            data = stream.read(1024 ** 2)
                            if not data: break
                            read += len(data)
                            if read > info.st_size: raise ValueError('file grew during observation')
                            digest.update(data)
                        after = os.fstat(stream.fileno())
                        if (read != info.st_size or before.st_mtime_ns != after.st_mtime_ns
                                or before.st_ctime_ns != after.st_ctime_ns or after.st_nlink != 1):
                            raise ValueError('file changed during observation')
                    record.update(size=read, sha256=digest.hexdigest())
        walk(self.fd, '')
        return records

    def seal(self, mutable=()):
        if self.initial is not None: raise ValueError('ownership already sealed')
        initial = self._snapshot()
        if len(set(mutable)) != len(mutable) or any(path not in initial or initial[path]['kind'] != stat.S_IFREG for path in mutable):
            raise ValueError('mutable paths must be exact owned regular files')
        self.mutable = set(mutable)
        self.initial = initial
        return deepcopy(initial)

    def reconcile(self, proof):
        self._quiescent(proof)
        if self.initial is None: raise ValueError('no prior ownership manifest')
        current = self._snapshot()
        if set(current) != set(self.initial): raise ValueError('unknown or missing path; reconcile ownership separately')
        for path, original in self.initial.items():
            actual = current[path]
            keys = set(original) - ({'size', 'sha256'} if path in self.mutable else set())
            if any(actual[key] != original[key] for key in keys): raise ValueError('owned identity or immutable content changed')
        self.reconciled = current
        return deepcopy(current)

    def cleanup(self, proof):
        self._quiescent(proof)
        if self.reconciled is None or self._snapshot() != self.reconciled:
            raise ValueError('fresh exact reconciled manifest required')
        removed = []
        for path in sorted(self.reconciled, key=lambda p: (p.count('/'), p), reverse=True):
            components = path.split('/')
            directory = os.dup(self.fd)
            try:
                for component in components[:-1]:
                    next_fd = os.open(component, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=directory)
                    os.close(directory)
                    directory = next_fd
                info = os.stat(components[-1], dir_fd=directory, follow_symlinks=False)
                if any(_identity(info)[k] != self.reconciled[path][k] for k in _identity(info)):
                    raise ValueError('deletion target changed; preserve remaining state')
                if stat.S_ISDIR(info.st_mode): os.rmdir(components[-1], dir_fd=directory)
                else: os.unlink(components[-1], dir_fd=directory)
                removed.append(path)
            finally: os.close(directory)
        if _identity(os.stat(self.path.name, dir_fd=self.parent, follow_symlinks=False)) != self.root_identity:
            raise ValueError('root changed; preserve remaining state')
        os.rmdir(self.path.name, dir_fd=self.parent)
        try:
            os.stat(self.path.name, dir_fd=self.parent, follow_symlinks=False)
        except FileNotFoundError:
            return {'root_absent': True, 'removed': removed}
        raise ValueError('root recreated after cleanup')
