"""Read-only exact source/dependency closure before an authorized input operation."""
import hashlib
import json
from pathlib import Path
import stat


def verify_source(accepted_sha256):
    from .bounded import LIVE_EXECUTION_ENABLED
    if LIVE_EXECUTION_ENABLED is not False:raise ValueError('input inspection cannot authorize guest execution')
    root=Path(__file__).resolve().parents[1];data=(root/'seal.json').read_bytes()
    if hashlib.sha256(data).hexdigest()!=accepted_sha256:raise ValueError('source seal differs from exact accepted revision')
    seal=json.loads(data);expected={item['path']:item for item in seal['files']}
    actual=set()
    for path in root.rglob('*'):
        info=path.lstat()
        if stat.S_ISLNK(info.st_mode):raise ValueError('source symlink outside closure')
        if path.is_dir():continue
        relative=path.relative_to(root).as_posix()
        if relative=='seal.json':continue
        if not stat.S_ISREG(info.st_mode):raise ValueError('source object type differs')
        actual.add(relative);entry=expected.get(relative);content=path.read_bytes()
        if entry is None or len(content)!=entry['bytes'] or hashlib.sha256(content).hexdigest()!=entry['sha256']:raise ValueError('source content differs')
    if actual!=set(expected):raise ValueError('source closure incomplete')
    for entry in seal['references']:
        content=(root/entry['path']).read_bytes()
        if len(content)!=entry['bytes'] or hashlib.sha256(content).hexdigest()!=entry['sha256']:raise ValueError('referenced input changed')
    return seal['revision_id']
