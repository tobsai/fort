"""Source-complete fixed guest entry points; live guard must remain false in this revision."""
import errno
import hashlib
import json
import os
from pathlib import Path
import socket
import stat
import sys
import termios
import time
import tty
from .bounded import Commands,require_live
from .guest_programs import Runtime,tool_probe,PROVIDER,PUBLIC
from .linux_kernel import LinuxKernel,bounded_read,json_read,atomic_json,SESSION,HEARTBEAT,REPORT,SOCKET
from .linux import LinuxGuest,parse_stat
from .controller import Controller
from .broker import Broker,serve


def denied_open(path,write=False):
    try:
        fd=os.open(path,(os.O_WRONLY if write else os.O_RDONLY)|os.O_NOFOLLOW|os.O_NONBLOCK)
    except OSError as error: return error.errno
    os.close(fd); return None

class RuntimeIO:
    denied_open=staticmethod(denied_open)
    def provider(self): return bounded_read('/var/lib/fort-probe/provider/synthetic-canary',4096)
    def report(self,value): atomic_json(REPORT,value)
    def heartbeat(self):
        try: return json_read(HEARTBEAT)
        except FileNotFoundError: return None
    def notify(self,value):
        path=os.environ.get('NOTIFY_SOCKET','')
        if not path or not (path.startswith('/') or path.startswith('@')): raise ValueError('systemd notify socket required')
        with socket.socket(socket.AF_UNIX,socket.SOCK_DGRAM) as client:
            client.settimeout(.1); client.connect('\0'+path[1:] if path.startswith('@') else path); client.sendall(value.encode())


class ToolIO:
    denied_open=staticmethod(denied_open)
    def management_connect(self):
        with socket.socket(socket.AF_UNIX,socket.SOCK_STREAM) as client:
            client.settimeout(.15)
            try: client.connect(SOCKET)
            except OSError as error: return {'connected':False,'errno':error.errno}
            return {'connected':True,'errno':None}
    def own_scratch(self):
        path='/tmp/tool-positive-control'
        fd=os.open(path,os.O_CREAT|os.O_EXCL|os.O_RDWR|os.O_NOFOLLOW,0o600)
        try:
            os.write(fd,PUBLIC); os.lseek(fd,0,os.SEEK_SET)
            return os.read(fd,len(PUBLIC)+1)==PUBLIC
        finally: os.close(fd); os.unlink(path)
    def own_socketpair(self):
        left,right=socket.socketpair()
        try:
            left.settimeout(.1); right.settimeout(.1); left.sendall(b'fort'); return right.recv(4)==b'fort'
        finally: left.close(); right.close()
    def ip_socket(self,family):
        try: handle=socket.socket(socket.AF_INET if family=='inet' else socket.AF_INET6,socket.SOCK_STREAM)
        except OSError as error: return error.errno
        handle.close(); return None
    def descendant(self):
        read_fd,write_fd=os.pipe()
        child=os.fork()
        if child==0:
            os.close(read_fd); os.setsid()
            grandchild=os.fork()
            if grandchild!=0: os._exit(0)
            info=parse_stat(bounded_read('/proc/self/stat',4096).decode())
            os.write(write_fd,json.dumps({'pid':info['pid'],'start_ticks':info['start_ticks']}).encode()+b'\n')
            os.close(write_fd)
            null=os.open('/dev/null',os.O_RDWR)
            for fd in (0,1,2): os.dup2(null,fd)
            if null>2: os.close(null)
            for name in os.listdir('/proc/self/fd'):
                if int(name)>2:
                    try: os.close(int(name))
                    except OSError: pass
            time.sleep(30); os._exit(0)
        os.close(write_fd)
        import select
        try:
            if not select.select([read_fd],[],[],.2)[0]: raise TimeoutError('descendant identity deadline')
            data=os.read(read_fd,256)
            os.waitpid(child,0)
            return json.loads(data)
        finally: os.close(read_fd)


def _directory(path,uid=0,gid=0,mode=0o700):
    os.makedirs(path,mode=mode,exist_ok=True)
    info=os.lstat(path)
    if not stat.S_ISDIR(info.st_mode): raise ValueError('unexpected guest bootstrap object')
    os.chown(path,uid,gid); os.chmod(path,mode)


PERSISTENT_STATE='/var/lib/fort-probe/controller'


def bootstrap():
    """New disposable guest only, after the guarded entry and reviewed offline seed."""
    commands=Commands()
    import pwd,grp
    for name,uid in (('fort-admin',1000),('fort-runtime',2001)):
        user=pwd.getpwnam(name)
        if user.pw_uid!=uid or user.pw_gid!=uid or user.pw_shell!='/usr/sbin/nologin' or grp.getgrnam(name).gr_gid!=uid:
            raise ValueError('locked synthetic user/group identity mismatch')
    _directory('/var/lib/fort-probe',0,0,0o711)
    fd=os.open('/var/lib/fort-probe/bootstrap-consumed.json',os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    with os.fdopen(fd,'wb') as stream:
        stream.write(b'{"consumed":true}\n');stream.flush();os.fsync(stream.fileno())
    directory=os.open('/var/lib/fort-probe',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:os.fsync(directory)
    finally:os.close(directory)
    for path,uid,gid,mode in ((PERSISTENT_STATE,0,0,0o700),('/run/fort-probe',0,0,0o700),(SESSION,0,0,0o700),('/var/lib/fort-probe/provider',2001,2001,0o700),
        ('/var/lib/fort-probe/sibling',0,0,0o700),('/run/fort-runtime-probe',0,2001,0o750),
        ('/run/fort-runtime-probe/report',2001,2001,0o700),('/run/fort-seed',0,0,0o700)):
        _directory(path,uid,gid,mode)
    for path,uid,data in (('/var/lib/fort-probe/provider/synthetic-canary',2001,PROVIDER),('/var/lib/fort-probe/sibling/canary',0,PUBLIC)):
        fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
        with os.fdopen(fd,'wb') as stream: os.fchown(stream.fileno(),uid,uid); stream.write(data)
    commands.run(['/usr/bin/mount','-o','ro,nodev,nosuid,noexec','/dev/disk/by-label/cidata','/run/fort-seed'])
    try:
        metadata=json_read('/run/fort-seed/meta-data',4096)
        data=bounded_read('/run/fort-seed/user-data',8*1024*1024)
        if hashlib.sha256(data).hexdigest()!=metadata['fort_seed_sha256']: raise ValueError('NoCloud seed digest mismatch')
        _directory('/var/lib/cloud/seed/nocloud')
        for path,content in (('/var/lib/cloud/seed/nocloud/user-data',data),('/opt/fort-probe/seed.sha256',(metadata['fort_seed_sha256']+'\n').encode())):
            fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
            with os.fdopen(fd,'wb') as stream: stream.write(content)
    finally: commands.run(['/usr/bin/umount','/run/fort-seed'])
    atomic_json(SESSION+'/identity.json',json_read('/opt/fort-probe/binding.json'))
    commands.run(['/usr/sbin/nft','-f','/etc/nftables-fort-probe.conf'])
    commands.run(['/usr/bin/systemctl','daemon-reload'])
    commands.run(['/usr/bin/systemctl','start','--no-block','fort-broker.service'])


def main(argv=None):
    require_live()
    args=sys.argv[1:] if argv is None else argv
    if len(args)!=1 or args[0] not in ('bootstrap','broker','runtime','tool'): raise ValueError('fixed guest role required')
    role=args[0]
    if role=='bootstrap': return bootstrap()
    if role=='runtime':
        runtime=Runtime(RuntimeIO(),time.monotonic,bounded_read('/proc/sys/kernel/random/boot_id',64).decode().strip(),os.environ['INVOCATION_ID'])
        runtime.start()
        while runtime.step(): time.sleep(.1)
        return
    if role=='tool':
        result=tool_probe(ToolIO())
        message='FORT_TOOL='+json.dumps(result,separators=(',',':'))+'\n'
        if len(message)>4096: raise ValueError('tool report bound')
        sys.stdout.write(message); sys.stdout.flush(); time.sleep(30); return
    kernel=LinuxKernel(); descriptors={}; settings={}
    try:
        for channel,path in (('inspection','/dev/hvc0'),('control','/dev/hvc1')):
            info=os.lstat(path)
            if not stat.S_ISCHR(info.st_mode) or info.st_uid!=0 or stat.S_IMODE(info.st_mode)!=0o600: raise ValueError('guest channel identity')
            fd=os.open(path,os.O_RDWR|os.O_NOFOLLOW|os.O_NOCTTY|os.O_NONBLOCK)
            if os.fstat(fd).st_rdev!=info.st_rdev: raise ValueError('guest device replaced')
            descriptors[channel]=fd; settings[channel]=termios.tcgetattr(fd); tty.setraw(fd)
        kernel.channels=descriptors
        with Controller(PERSISTENT_STATE,json_read('/opt/fort-probe/binding.json'),LinuxGuest(kernel,time.monotonic),time.monotonic) as controller:
            serve(Broker(controller,time.monotonic),descriptors)
    finally:
        for channel,fd in descriptors.items():
            try: termios.tcsetattr(fd,termios.TCSANOW,settings[channel])
            finally: os.close(fd)
        kernel.close()
