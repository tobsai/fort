"""Fixed worker jobs; the supported entry refuses all effects while inspection is disabled."""
import hashlib
import json
import os
from pathlib import Path
import stat
import sys
from .acquisition import Acquisition,HTTPS,LocalStore,fixed_plan
from .inspection_guard import require_inspection
from .inspection_archive import verifier_package,ubuntu_inventory
from .archive import inspect_archive


def regular(path,cap):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW);info=os.fstat(fd)
    if not stat.S_ISREG(info.st_mode) or info.st_nlink!=1 or info.st_size>cap:
        os.close(fd);raise ValueError('input identity/type/size differs')
    return os.fdopen(fd,'rb'),info


def unchanged(path,stream,before):
    identity=lambda value:(value.st_dev,value.st_ino,value.st_size,value.st_mtime_ns,value.st_ctime_ns)
    if identity(os.fstat(stream.fileno()))!=identity(before) or identity(os.lstat(path))!=identity(before):
        raise ValueError('input changed during inspection')


def write_new(path,data,cap):
    if len(data)>cap:raise ValueError('inspection output artifact cap')
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    with os.fdopen(fd,'wb') as stream:stream.write(data);stream.flush();os.fsync(stream.fileno())
    directory=os.open(path.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:os.fsync(directory)
    finally:os.close(directory)


def install_verifier(root,result):
    target=root/'verifier';os.mkdir(target,0o700)
    for name,data in result['files'].items():
        path=target/name
        # Paths and every member's type were checked before any output is created.
        parent=target
        for part in Path(name).parts[:-1]:
            parent=parent/part
            try:os.mkdir(parent,0o700)
            except FileExistsError:
                if not stat.S_ISDIR(os.lstat(parent).st_mode):raise ValueError('verifier directory changed type')
        write_new(path,data,24*1024**2)
    selected={key:result[key] for key in ('entry','archive_sha512','manifest')}
    data=(json.dumps(selected,sort_keys=True,separators=(',',':'))+'\n').encode()
    write_new(root/'verifier-selection.json',data,65536)
    return {'selection_sha256':hashlib.sha256(data).hexdigest(),'entry':result['entry'],'archive_sha512':result['archive_sha512']}


def run_job(operation,root,names=None):
    entries,policy=fixed_plan();pins={e['name']:e for e in entries};directory=root/'downloads'
    if operation=='acquire':
        progress=lambda name:print(json.dumps({'progress_input':name}),flush=True)
        result=Acquisition(entries,policy,HTTPS(),LocalStore(directory),progress=progress).run()
        return {'completed':result['completed']}
    complete=json.loads((directory/'complete.json').read_bytes())
    if (complete!={'status':'complete','inputs':[e['name'] for e in entries]} or (directory/'failure.json').exists()):
        raise ValueError('failed or incomplete acquisition veto')
    name='openpgp-6.3.2.tgz' if operation=='verifier_archive' else 'noble-server-cloudimg-arm64.tar.gz'
    pin=pins[name];path=directory/name
    stream,before=regular(path,pin['max_bytes'])
    with stream:
        if operation=='verifier_archive':
            result=verifier_package(stream,pin['sha512']);unchanged(path,stream,before)
            return install_verifier(root,result)
        if operation=='ubuntu_inventory':result=ubuntu_inventory(stream,pin['sha256'])
        elif operation=='ubuntu_selected':result=inspect_archive(stream,pin['sha256'],names)
        else:raise ValueError('fixed Python worker operation required')
        unchanged(path,stream,before)
        return result


def main():
    require_inspection()
    if len(sys.argv)!=4:raise ValueError('fixed worker arguments required')
    operation,root,names=sys.argv[1:];root=Path(root)
    info=os.lstat(root)
    if not root.is_absolute() or not stat.S_ISDIR(info.st_mode) or info.st_uid!=os.getuid() or stat.S_IMODE(info.st_mode)!=0o700:
        raise ValueError('owned private input root required')
    result=run_job(operation,root,json.loads(names))
    data=json.dumps({'operation':operation,'status':'complete','details':result},sort_keys=True,separators=(',',':'))+'\n'
    if len(data.encode())>65536:raise ValueError('worker result cap')
    sys.stdout.write(data)
