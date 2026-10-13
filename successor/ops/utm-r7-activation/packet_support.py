"""Read-only packet identity and bounded owned smoke evidence; no acquisition entrypoint."""
import ast
import hashlib
import importlib
import json
import os
from pathlib import Path
import re
import stat
import sys
from types import SimpleNamespace

HERE = Path(__file__).resolve().parent
SOURCE = HERE.parent / 'utm-execution-v1'
ENV = {'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C', 'TZ': 'UTC'}
LAUNCHER_KEYS = frozenset(('CPATH', 'LIBRARY_PATH', 'MANPATH', 'SDKROOT', '__CF_USER_TEXT_ENCODING'))


def inspect_environment(environment, runtime):
    extras = set(environment) - set(ENV)
    if any(environment.get(k) != v for k, v in ENV.items()) or extras - (LAUNCHER_KEYS if runtime == 'python' else set()):
        raise ValueError('unexpected runtime environment; values omitted')
    return dict(ENV), sorted(extras)


def digest(data): return hashlib.sha256(data).hexdigest()


def checked_file(path, expected):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or path.is_symlink(): raise ValueError('sealed regular file required')
    data = path.read_bytes()
    if len(data) != expected['bytes'] or digest(data) != expected['sha256']: raise ValueError('sealed file differs: ' + str(path))
    return data


def check_packet(expected, state='disabled'):
    if state not in ('disabled', 'activated') or not re.fullmatch('[a-f0-9]{64}', expected or ''):
        raise ValueError('exact packet digest and fixed state required')
    seal_bytes = (HERE / 'seal.json').read_bytes()
    if digest(seal_bytes) != expected: raise ValueError('packet seal differs')
    seal = json.loads(seal_bytes)
    actual = set()
    for path in HERE.rglob('*'):
        if path.is_symlink(): raise ValueError('packet symlink')
        if path.is_dir() or path == HERE / 'seal.json': continue
        if not path.is_file(): raise ValueError('packet object type')
        actual.add(path.relative_to(HERE).as_posix())
    if actual != {e['path'] for e in seal['files']}: raise ValueError('packet file set differs')
    for entry in seal['files']: checked_file(HERE / entry['path'], entry)
    manifest = json.loads((HERE / 'packet.json').read_bytes())
    if str(HERE) != manifest['packet_directory'] or str(SOURCE) != manifest['source_directory']:
        raise ValueError('fixed packet location differs')
    source_seal_bytes = (HERE / (state + '-source-seal.json')).read_bytes()
    if (SOURCE / 'seal.json').read_bytes() != source_seal_bytes: raise ValueError('source activation state differs')
    source_seal = json.loads(source_seal_bytes)
    source_actual = set()
    for path in SOURCE.rglob('*'):
        if path.is_symlink(): raise ValueError('source symlink')
        if path.is_dir() or path == SOURCE / 'seal.json': continue
        if not path.is_file(): raise ValueError('source object type')
        source_actual.add(path.relative_to(SOURCE).as_posix())
    if source_actual != {e['path'] for e in source_seal['files']}: raise ValueError('source file set differs')
    for entry in source_seal['files'] + source_seal['references']: checked_file(SOURCE / entry['path'], entry)
    if digest(source_seal_bytes) != manifest['source_seals'][state]: raise ValueError('source seal declaration differs')
    guard = ast.parse((SOURCE / 'fort_probe/inspection_guard.py').read_bytes())
    flags = [n.value.value for n in guard.body if isinstance(n, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'INSPECTION_ENABLED' for t in n.targets) and isinstance(n.value, ast.Constant)]
    if flags != [state == 'activated']: raise ValueError('Python inspection gate differs')
    node = (SOURCE / 'inspection-verifier.mjs').read_text()
    if node.count('export const INSPECTION_ENABLED = ' + ('true' if state == 'activated' else 'false') + ';') != 1:
        raise ValueError('Node inspection gate differs')
    guest = ast.parse((SOURCE / 'fort_probe/bounded.py').read_bytes())
    flags = [n.value.value for n in guest.body if isinstance(n, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'LIVE_EXECUTION_ENABLED' for t in n.targets) and isinstance(n.value, ast.Constant)]
    if flags != [False]: raise ValueError('guest execution must remain disabled')
    return SimpleNamespace(packet_sha=expected, active_sha=manifest['source_seals']['activated'],
                           manifest=manifest, cases=manifest['smoke']['cases'], identities=manifest['runtime_identities'])


def verify_tools(context):
    for pin in context.manifest['tools']:
        path = Path(pin['path'])
        if str(path.resolve()) != pin['resolved_path']: raise ValueError('runtime selection differs')
        data = path.read_bytes()
        if len(data) != pin['bytes'] or digest(data) != pin['sha256']: raise ValueError('trusted tool pin differs')


def source_module(name):
    sys.path.insert(0, str(SOURCE))
    module = importlib.import_module('fort_probe.' + name)
    if Path(module.__file__).resolve() != SOURCE / 'fort_probe' / (name + '.py'):
        raise ValueError('source module location differs')
    return module


def require_active(): source_module('inspection_guard').require_inspection()


class Journal:
    """Fixed separate smoke root; exclusive records and no implicit reuse/deletion."""
    def __init__(self, context, create=False):
        require_active()
        self.context = context
        self.path = Path(context.manifest['smoke']['output_directory'])
        chain_type = source_module('lifecycle').DirectoryChain
        self.chain = chain_type(self.path.parent if create else self.path)
        try:
            if create: self.chain.create(self.path.name)
            info = os.fstat(self.chain.fd)
            if info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o700:
                raise ValueError('owned private smoke root required')
            self.identity = {'device': info.st_dev, 'inode': info.st_ino, 'uid': info.st_uid}
            if create:
                self.record('attempt.json', {'packet_sha': context.packet_sha, 'source_seal': context.active_sha,
                                            'identity': self.identity, 'status': 'consumed'})
            else:
                expected = {'packet_sha': context.packet_sha, 'source_seal': context.active_sha,
                            'identity': self.identity, 'status': 'consumed'}
                if self.read('attempt.json') != expected: raise ValueError('smoke root identity or binding differs')
        except BaseException:
            self.chain.close()
            raise

    def close(self): self.chain.close()

    def names(self):
        self.chain.check()
        return os.listdir(self.chain.fd)

    def record(self, name, value):
        if not re.fullmatch('[a-z0-9-]+\.json', name): raise ValueError('fixed evidence name required')
        data = (json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False) + '\n').encode()
        if len(data) > 16384: raise ValueError('smoke record size')
        total = 0
        for item in self.names():
            info = os.stat(item, dir_fd=self.chain.fd, follow_symlinks=False)
            if (not re.fullmatch(r'[a-z0-9-]+\.json', item) or not stat.S_ISREG(info.st_mode) or
                    info.st_nlink != 1 or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o600 or
                    info.st_size > 16384): raise ValueError('smoke evidence type/ownership/bounds')
            total += info.st_size
        if total + len(data) > 65536: raise ValueError('smoke evidence budget')
        self.chain.write_file(name, data)

    def read(self, name):
        if not re.fullmatch('[a-z0-9-]+\.json', name): raise ValueError('fixed evidence name required')
        self.chain.check()
        fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=self.chain.fd)
        try:
            before = os.fstat(fd)
            if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or before.st_uid != os.getuid() or
                    stat.S_IMODE(before.st_mode) != 0o600 or before.st_size > 16384):
                raise ValueError('bounded owned evidence required')
            data = bytearray()
            while len(data) <= 16384:
                chunk = os.read(fd, 16385 - len(data))
                if not chunk: break
                data.extend(chunk)
            after = os.fstat(fd); named = os.stat(name, dir_fd=self.chain.fd, follow_symlinks=False)
            identity = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_ctime_ns)
            if len(data) != before.st_size or identity(before) != identity(after) or identity(after) != identity(named):
                raise ValueError('smoke evidence changed while reading')
            self.chain.check()
            return json.loads(data)
        finally: os.close(fd)
