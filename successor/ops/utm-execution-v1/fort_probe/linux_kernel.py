"""Linux syscall bindings. Not constructed or invoked by fixture tests."""
import errno
import hashlib
import json
import os
from pathlib import Path
import re
import socket
import stat
import time
from .bounded import Commands, require_live
from .guest_programs import PROVIDER, PUBLIC
from .linux import parse_stat
from .evidence import RUNTIME_CGROUP, TOOL_CGROUP, exact
from .serial import _pairs, _constant, _float
from .ipc import _connect

SESSION='/run/fort-probe/session'
HEARTBEAT='/run/fort-runtime-probe/heartbeat.json'
REPORT='/run/fort-runtime-probe/report/ready.json'
SOCKET='/run/fort-probe/inspection-canary.sock'
UNITS=('fort-runtime.service','fort-tool.service')
PROPERTIES=('Id','ActiveState','SubState','MainPID','InvocationID','ControlGroup','NoNewPrivileges','PrivateNetwork',
            'PrivateIPC','PrivateDevices','PrivateTmp','ProtectSystem','ProtectHome','ProtectControlGroups',
            'CapabilityBoundingSet','AmbientCapabilities','RestrictAddressFamilies','KillMode','Restart',
            'DynamicUser','User','WatchdogUSec','RuntimeMaxUSec','TimeoutStopUSec','TasksMax','MemoryMax','CPUQuotaPerSecUSec')


def bounded_read(path, cap=65536):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    with os.fdopen(fd,'rb') as stream:
        data=stream.read(cap+1)
        if len(data)>cap: raise ValueError('file bound exceeded')
        return data


def json_read(path,cap=4096):
    return json.loads(bounded_read(path,cap),object_pairs_hook=_pairs,parse_constant=_constant,parse_float=_float)


def atomic_json(path,value,gid=None):
    payload=(json.dumps(value,sort_keys=True,separators=(',',':'))+'\n').encode()
    if len(payload)>4096: raise ValueError('guest record bound exceeded')
    temporary=path+'.pending'
    fd=os.open(temporary,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o640 if gid is not None else 0o600)
    with os.fdopen(fd,'wb') as stream:
        if gid is not None: os.fchown(stream.fileno(),0,gid)
        stream.write(payload); stream.flush(); os.fsync(stream.fileno())
    os.replace(temporary,path)
    directory=os.open(str(Path(path).parent),os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try: os.fsync(directory)
    finally: os.close(directory)


def decode_units(raw):
    result={}
    for block in raw.strip().split('\n\n'):
        fields={}
        for line in block.splitlines():
            key,sep,value=line.partition('=')
            if not sep or key in fields: raise ValueError('ambiguous systemd output')
            fields[key]=value
        name=fields.get('Id')
        if name not in UNITS or name in result: raise ValueError('unexpected systemd unit')
        if set(fields)!=set(PROPERTIES): raise ValueError('incomplete systemd properties')
        policy={'NoNewPrivileges':'yes','PrivateNetwork':'yes','PrivateIPC':'yes','PrivateDevices':'yes',
                'PrivateTmp':'yes','ProtectSystem':'strict','ProtectHome':'yes','ProtectControlGroups':'yes',
                'CapabilityBoundingSet':'','AmbientCapabilities':'','RestrictAddressFamilies':'AF_UNIX',
                'KillMode':'control-group','Restart':'no','TimeoutStopUSec':'2s','MemoryMax':'134217728',
                'CPUQuotaPerSecUSec':'250ms'}
        role='runtime' if name==UNITS[0] else 'tool'
        policy.update(RuntimeMaxUSec='40s' if role=='runtime' else '38s',TasksMax='8' if role=='runtime' else '16')
        policy.update({'User':'fort-runtime','WatchdogUSec':'2s'} if role=='runtime' else {'DynamicUser':'yes'})
        if any(fields[k]!=v for k,v in policy.items()): raise ValueError('systemd hardening/resource policy differs')
        if fields['ActiveState'] not in ('active','inactive','failed','activating','deactivating','reloading'):
            raise ValueError('unknown unit activity state')
        result[name]={'active':fields['ActiveState'] not in ('inactive','failed'), 'ready':fields['ActiveState']=='active',
                      'invocation':fields['InvocationID'],'pid':int(fields['MainPID']), 'cgroup':fields['ControlGroup']}
    if set(result)!=set(UNITS): raise ValueError('both exact units required')
    return {'runtime':result[UNITS[0]],'tool':result[UNITS[1]]}


def decode_process(stat_before, status, cgroup, namespaces, descriptors, stat_after):
    before,after=parse_stat(stat_before),parse_stat(stat_after)
    if before!=after: raise ValueError('process changed during observation')
    values={line.split(':',1)[0]:line.split(':',1)[1].strip() for line in status.splitlines() if ':' in line}
    ids=[int(n) for n in values['Uid'].split()]
    if len(ids)!=4 or len(set(ids))!=1 or set(descriptors)!={'0','1','2'}: raise ValueError('unexpected UID/fd set')
    if descriptors['0']!='/dev/null' or any(v!='/dev/null' and not re.fullmatch(r'socket:\[[0-9]+\]',v) for v in descriptors.values()):
        raise ValueError('management or unexpected descriptor inherited')
    groups=cgroup.strip().splitlines()
    if len(groups)!=1 or not groups[0].startswith('0::'): raise ValueError('cgroup v2 identity required')
    for key in ('CapInh','CapPrm','CapEff','CapBnd','CapAmb'):
        if values[key]!='0000000000000000': raise ValueError('capability present')
    return {'pid':before['pid'],'start_ticks':before['start_ticks'],'uid':ids[0], 'cgroup':groups[0][3:],
            'namespaces':namespaces,'capabilities':values['CapEff'],'no_new_privileges':values['NoNewPrivs']=='1',
            'fd_classes':['stdio','journal'] if any(v.startswith('socket:') for v in descriptors.values()) else ['stdio'], 'alive':True}


class LinuxKernel:
    def __init__(self, commands=None):
        require_live()
        if os.geteuid()!=0: raise ValueError('trusted guest root required')
        self.commands=commands if commands is not None else Commands()
        self.listener=None; self.socket_identity=None; self.identity=None; self.channels=None

    def boot(self): return bounded_read('/proc/sys/kernel/random/boot_id',64).decode().strip()
    def wait(self,seconds): time.sleep(seconds)
    def units(self):
        output,_=self.commands.run(['/usr/bin/systemctl','show','--no-pager','--property='+','.join(PROPERTIES),*UNITS])
        return decode_units(output.decode())
    def run(self,action):
        argv={'start-runtime':['start','--no-block',UNITS[0]],'start-tool':['start','--no-block',UNITS[1]],
              'stop':['stop','--no-block','fort-attempt.slice',*UNITS]}[action]
        self.commands.run(['/usr/bin/systemctl',*argv])
    def write(self,name,value):
        if name=='identity':
            self.identity=value
            atomic_json(SESSION+'/identity.json',value)
        elif name=='heartbeat': atomic_json(HEARTBEAT,value,2001)
        else: raise ValueError('fixed record name required')
    def pids(self):
        root=Path('/sys/fs/cgroup/fort.slice/fort-attempt.slice')
        if not root.exists(): return []
        result=set(); count=0
        for base,dirs,files in os.walk(root,followlinks=False):
            count+=1
            if count>64 or any(Path(base,d).is_symlink() for d in dirs): raise ValueError('unexpected cgroup tree')
            for n in bounded_read(str(Path(base)/'cgroup.procs'),4096).split(): result.add(int(n))
        if len(result)>32: raise ValueError('task cap exceeded')
        return sorted(result)
    def process(self,pid):
        if type(pid) is not int or pid<=0: raise ValueError('invalid exact PID')
        root='/proc/'+str(pid)
        before=bounded_read(root+'/stat',4096).decode()
        status=bounded_read(root+'/status',16384).decode()
        cgroup=bounded_read(root+'/cgroup',4096).decode()
        ns={name:int(re.fullmatch(name+r':\[([0-9]+)\]',os.readlink(root+'/ns/'+name)).group(1)) for name in ('net','mnt','ipc')}
        fds={name:os.readlink(root+'/fd/'+name) for name in os.listdir(root+'/fd')}
        return decode_process(before,status,cgroup,ns,fds,bounded_read(root+'/stat',4096).decode())
    def detached(self,pid,tool):
        child=parse_stat(bounded_read('/proc/'+str(pid)+'/stat',4096).decode())
        parent=parse_stat(bounded_read('/proc/'+str(tool)+'/stat',4096).decode())
        return child['parent']==1 and child['session']!=parent['session'] and child['session']>0
    def topology(self):
        nics=sorted(name for name in os.listdir('/sys/class/net') if name!='lo')
        mounts=bounded_read('/proc/self/mountinfo').decode().splitlines()
        shares=[line.split(' - ',1)[1].split()[0] for line in mounts if ' - ' in line and line.split(' - ',1)[1].split()[0] in ('virtiofs','9p','fuse.sshfs','nfs','cifs')]
        return {'nics':nics,'shares':shares}
    def root_controls(self):
        expected={'provider':('/var/lib/fort-probe/provider/synthetic-canary',2001,PROVIDER),
                  'sibling':('/var/lib/fort-probe/sibling/canary',0,PUBLIC)}
        for path,uid,data in expected.values():
            info=os.lstat(path)
            if not stat.S_ISREG(info.st_mode) or info.st_uid!=uid or stat.S_IMODE(info.st_mode)!=0o600 or bounded_read(path,4096)!=data:
                raise ValueError('root target positive control failed')
        identity=json_read(SESSION+'/identity.json')
        if self.identity is not None and not exact(identity,self.identity): raise ValueError('identity file modified')
        info=os.lstat('/var/lib/cloud/seed/nocloud/user-data')
        if not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or stat.S_IMODE(info.st_mode)!=0o600: raise ValueError('seed ownership')
        seed=bounded_read('/var/lib/cloud/seed/nocloud/user-data',8*1024*1024)
        expected_hash=bounded_read('/opt/fort-probe/seed.sha256',65).decode().strip()
        if hashlib.sha256(seed).hexdigest()!=expected_hash: raise ValueError('seed identity failed')
        if self.channels is None: raise ValueError('broker channel mapping missing')
        for channel,path in (('inspection','/dev/hvc0'),('control','/dev/hvc1')):
            info=os.lstat(path)
            if not stat.S_ISCHR(info.st_mode) or info.st_uid!=0 or stat.S_IMODE(info.st_mode)!=0o600: raise ValueError('guest serial identity failed')
            if os.fstat(self.channels[channel]).st_rdev!=info.st_rdev: raise ValueError('broker channel mapping changed')
        self.ipc_start()
        return dict.fromkeys(('provider','identity_store','sibling','seed','serial','ipc_listener'),True)
    def runtime_report(self):
        info=os.lstat(REPORT)
        if info.st_uid!=2001 or stat.S_IMODE(info.st_mode)!=0o600 or not stat.S_ISREG(info.st_mode): raise ValueError('runtime report ownership')
        return json_read(REPORT)
    def tool_report(self,unit):
        if not re.fullmatch('[0-9a-f]{32}',unit['invocation']): raise ValueError('tool invocation missing')
        output,_=self.commands.run(['/usr/bin/journalctl','--no-pager','--output=json','--output-fields=MESSAGE,_PID,_UID,_SYSTEMD_INVOCATION_ID',
                                   '_SYSTEMD_INVOCATION_ID='+unit['invocation']],cap=65536)
        records=[json.loads(line) for line in output.splitlines() if line]
        reports=[r for r in records if r.get('_PID')==str(unit['pid']) and r.get('_SYSTEMD_INVOCATION_ID')==unit['invocation'] and str(r.get('MESSAGE','')).startswith('FORT_TOOL=')]
        if len(reports)!=1: raise ValueError('exactly one bound tool report required')
        actual=self.process(unit['pid'])
        if reports[0].get('_UID')!=str(actual['uid']): raise ValueError('journal UID binding failed')
        self.root_controls()
        return json.loads(reports[0]['MESSAGE'][10:],object_pairs_hook=_pairs,parse_constant=_constant,parse_float=_float)
    def ipc_start(self):
        if self.listener is None:
            self.listener=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM)
            self.listener.bind(SOCKET); os.chmod(SOCKET,0o600); self.listener.listen(4)
            self.socket_identity=os.lstat(SOCKET).st_ino
        result=_connect(SOCKET)
        self.listener.settimeout(.15)
        client,_=self.listener.accept(); client.close()
        return result
    def ipc_finish(self): return self.ipc_start()
    def close(self):
        if self.listener is not None:
            self.listener.close()
            if os.lstat(SOCKET).st_ino==self.socket_identity: os.unlink(SOCKET)
