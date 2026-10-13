"""Journal attribution from duplicated kernel socket descriptors; fail closed.

Linux ARM64 collector remains guarded and unrun. No bytes are sent or received.
SO_PEERCRED can name PID 1 for systemd socket activation, or journald itself.
"""
import ctypes
import os
import platform
import re
import socket
import stat
import struct
from .bounded import require_live

JOURNAL='/run/systemd/journal/stdout'
FIELDS={'inode','family','type','peer_path','peer_uid','peer_gid','peer_pid','peer_start_ticks',
        'peer_executable','peer_cgroup','path_uid','path_is_socket'}


def validate_journal(descriptors,proofs):
    expected={fd for fd,target in descriptors.items() if target.startswith('socket:')}
    if not expected:
        if proofs not in (None,{}):raise ValueError('unexpected socket evidence')
        return
    if not isinstance(proofs,dict) or set(proofs)!=expected:raise ValueError('journal endpoint attribution missing')
    for fd in expected:
        p=proofs[fd]
        if not isinstance(p,dict) or set(p)!=FIELDS:raise ValueError('incomplete journal socket identity')
        for name in ('inode','family','type','peer_uid','peer_gid','peer_pid','peer_start_ticks','path_uid'):
            if type(p[name]) is not int:raise ValueError('untyped socket identity')
        manager=(p['peer_pid']==1 and p['peer_executable']=='/usr/lib/systemd/systemd' and p['peer_cgroup']=='/init.scope')
        daemon=(p['peer_pid']>1 and p['peer_executable']=='/usr/lib/systemd/systemd-journald'
                and p['peer_cgroup']=='/system.slice/systemd-journald.service')
        if (descriptors[fd]!='socket:['+str(p['inode'])+']' or p['inode']<=0
                or p['family']!=socket.AF_UNIX or p['type']!=socket.SOCK_STREAM or p['peer_path']!=JOURNAL
                or p['peer_uid']!=0 or p['peer_gid']!=0 or p['path_uid']!=0 or p['path_is_socket'] is not True
                or p['peer_start_ticks']<=0 or not (manager or daemon)):
            raise ValueError('management or unattributed socket')


def observe_journal(pid,descriptors,read,parse_stat):
    require_live()
    if platform.system()!='Linux' or platform.machine()!='aarch64' or not hasattr(os,'pidfd_open'):
        raise RuntimeError('reviewed Linux ARM64 pidfd boundary unavailable')
    pidfd=os.pidfd_open(pid,0)
    try:
        libc=ctypes.CDLL(None,use_errno=True)
        libc.syscall.restype=ctypes.c_long
        result={}
        for fd,target in descriptors.items():
            if not re.fullmatch(r'socket:\[[0-9]+\]',target):continue
            duplicate=libc.syscall(ctypes.c_long(438),ctypes.c_int(pidfd),ctypes.c_int(int(fd)),ctypes.c_uint(0))
            if duplicate<0:raise OSError(ctypes.get_errno(),'pidfd_getfd unavailable; attribution unproven')
            try:handle=socket.socket(fileno=duplicate)
            except BaseException:os.close(duplicate);raise
            with handle:
                info=os.fstat(handle.fileno())
                if not stat.S_ISSOCK(info.st_mode):raise ValueError('duplicated descriptor changed type')
                peer_pid,peer_uid,peer_gid=struct.unpack('3i',handle.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,12))
                if peer_pid<=0:raise ValueError('peer identity unavailable')
                root='/proc/'+str(peer_pid)
                before=parse_stat(read(root+'/stat',4096).decode())
                status=read(root+'/status',16384).decode()
                values={line.split(':',1)[0]:line.split(':',1)[1].strip() for line in status.splitlines() if ':' in line}
                if values.get('Uid','').split()!=['0']*4 or values.get('Gid','').split()!=['0']*4:
                    raise ValueError('peer process privilege identity changed')
                executable=os.readlink(root+'/exe')
                cgroup=read(root+'/cgroup',4096).decode().strip()
                if not cgroup.startswith('0::') or '\n' in cgroup:raise ValueError('peer cgroup incomplete')
                path=os.lstat(JOURNAL)
                result[fd]={'inode':info.st_ino,'family':int(handle.family),
                    'type':handle.getsockopt(socket.SOL_SOCKET,socket.SO_TYPE),'peer_path':handle.getpeername(),
                    'peer_uid':peer_uid,'peer_gid':peer_gid,'peer_pid':peer_pid,'peer_start_ticks':before['start_ticks'],
                    'peer_executable':executable,'peer_cgroup':cgroup[3:],'path_uid':path.st_uid,
                    'path_is_socket':stat.S_ISSOCK(path.st_mode)}
                if before!=parse_stat(read(root+'/stat',4096).decode()):raise ValueError('peer process replaced')
        validate_journal(descriptors,result)
        return result
    finally:os.close(pidfd)
