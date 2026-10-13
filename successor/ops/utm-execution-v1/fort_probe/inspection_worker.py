"""Fixed worker jobs; the supported entry refuses all effects while inspection is disabled."""
import hashlib
import json
import os
from pathlib import Path
import stat
import sys
from contextlib import contextmanager
from .acquisition import Acquisition,HTTPS,LocalStore,fixed_plan
from .inspection_guard import require_inspection
from .inspection_archive import verifier_package,ubuntu_inventory
from .archive import inspect_archive
from .lifecycle import DirectoryChain


@contextmanager
def regular(path,cap):
    path=Path(path)
    with DirectoryChain(path.parent) as chain:
        fd=os.open(path.name,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=chain.fd)
        with os.fdopen(fd,'rb') as stream:
            before=os.fstat(stream.fileno())
            if not stat.S_ISREG(before.st_mode) or before.st_nlink!=1 or before.st_size>cap:
                raise ValueError('input identity/type/size differs')
            chain.check()
            yield stream,before
            identity=lambda value:(value.st_dev,value.st_ino,value.st_mode,value.st_nlink,value.st_size,value.st_mtime_ns,value.st_ctime_ns)
            named=os.stat(path.name,dir_fd=chain.fd,follow_symlinks=False)
            if identity(os.fstat(stream.fileno()))!=identity(before) or identity(named)!=identity(before):
                raise ValueError('input changed during inspection')
            chain.check()


def install_verifier(root,result):
    with DirectoryChain(root) as owner,owner.reopen() as target:
        target.create('verifier');directories={}
        for name,data in result['files'].items():
            parts=name.split('/')
            if any(part in ('','.','..') for part in parts) or len(data)>24*1024**2:
                raise ValueError('selected verifier path or file cap differs')
            with target.reopen() as parent:
                prefix=()
                for part in parts[:-1]:
                    prefix+=part,
                    if prefix in directories:parent.descend(part,directories[prefix])
                    else:
                        parent.create(part);directories[prefix]=parent.entries[-1][2]
                parent.write_file(parts[-1],data)
            target.check()
        selected={key:result[key] for key in ('entry','archive_sha512','manifest')}
        data=(json.dumps(selected,sort_keys=True,separators=(',',':'))+'\n').encode()
        if len(data)>65536:raise ValueError('selection manifest cap')
        owner.write_file('verifier-selection.json',data);target.check()
    return {'selection_sha256':hashlib.sha256(data).hexdigest(),'entry':result['entry'],'archive_sha512':result['archive_sha512']}


def run_job(operation,root,names=None):
    entries,policy=fixed_plan();pins={e['name']:e for e in entries};directory=root/'downloads'
    if operation=='acquire':
        progress=lambda name:print(json.dumps({'progress_input':name}),flush=True)
        result=Acquisition(entries,policy,HTTPS(),LocalStore(directory),progress=progress).run()
        return {'completed':result['completed']}
    with regular(directory/'complete.json',8192) as (stream,unused):complete=json.loads(stream.read(8193))
    if (complete!={'status':'complete','inputs':[e['name'] for e in entries]} or os.path.lexists(directory/'failure.json')):
        raise ValueError('failed or incomplete acquisition veto')
    name='openpgp-6.3.2.tgz' if operation=='verifier_archive' else 'noble-server-cloudimg-arm64.tar.gz'
    pin=pins[name];path=directory/name
    with regular(path,pin['max_bytes']) as (stream,unused):
        if operation=='verifier_archive':
            result=verifier_package(stream,pin['sha512'])
        elif operation=='ubuntu_inventory':result=ubuntu_inventory(stream,pin['sha256'])
        elif operation=='ubuntu_selected':result=inspect_archive(stream,pin['sha256'],names)
        else:raise ValueError('fixed Python worker operation required')
    return install_verifier(root,result) if operation=='verifier_archive' else result


def main():
    require_inspection()
    if len(sys.argv)!=4:raise ValueError('fixed worker arguments required')
    operation,root,names=sys.argv[1:];root=Path(root)
    with DirectoryChain(root) as chain:
        info=os.fstat(chain.fd)
        if info.st_uid!=os.getuid() or stat.S_IMODE(info.st_mode)!=0o700:raise ValueError('owned private input root required')
        result=run_job(operation,root,json.loads(names));chain.check()
    data=json.dumps({'operation':operation,'status':'complete','details':result},sort_keys=True,separators=(',',':'))+'\n'
    if len(data.encode())>65536:raise ValueError('worker result cap')
    sys.stdout.write(data)
