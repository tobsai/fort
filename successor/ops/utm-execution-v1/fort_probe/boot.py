"""Complete fixed boot policy and pre-start regular-file identity checks."""
from copy import deepcopy
import hashlib
import os
from pathlib import Path
import stat
import time
from .utm_schema import pinned_schema
from .evidence import exact
from .lifecycle import DirectoryChain

MASKS=('ssh.service','ssh.socket','sshd.service','serial-getty@hvc0.service','serial-getty@hvc1.service',
       'getty@tty1.service','apt-daily.service','apt-daily.timer','apt-daily-upgrade.service',
       'apt-daily-upgrade.timer','unattended-upgrades.service','systemd-growfs-root.service')
def vm_configuration(binding):
    schema=pinned_schema(binding)
    return {'Backend':'Apple','ConfigurationVersion':schema['version'],'Information':schema['information'],
            'System':{'Architecture':'aarch64','CPUCount':2,'MemorySize':4096,'GenericPlatform':{},
                      'Boot':{'OperatingSystem':'Linux','UEFIBoot':False,'LinuxKernelPath':'kernel.Image','LinuxInitialRamdiskPath':'initrd',
                              'LinuxCommandLine':'console=hvc0 root=LABEL=cloudimg-rootfs rw '+ ' '.join('systemd.mask='+unit for unit in MASKS)}},
            'Virtualization':{'Audio':False,'Balloon':False,'Entropy':True,'Keyboard':'Disabled','Pointer':'Disabled','Rosetta':False,'ClipboardSharing':False},
            'Network':[],'SharedDirectory':[],'Display':[],'Serial':[{'Mode':'Ptty'},{'Mode':'Ptty'}],
            'Drive':[{'ImageName':'root.raw','ReadOnly':False,'Nvme':False,'Identifier':'fort-root'},
                     {'ImageName':'seed.iso','ReadOnly':True,'Nvme':False,'Identifier':'fort-seed'}]}


def validate_config(config,expected_uuid=None):
    try:
        schema=pinned_schema({'vm_package_uuid':config['Information']['UUID']})
        canonical=schema['information']['UUID']
        normalized=deepcopy(config);normalized['Information']['UUID']=canonical
        if not exact(normalized,vm_configuration({'vm_package_uuid':canonical})):
            raise ValueError('complete accepted VM configuration differs')
        if expected_uuid is not None and canonical!=pinned_schema({'vm_package_uuid':expected_uuid})['information']['UUID']:
            raise ValueError('VM UUID differs from accepted pin')
    except (KeyError,TypeError,AttributeError) as error:raise ValueError('incomplete VM configuration') from error
    return {'uuid':canonical,'nics':[],'shares':[],'serials':['Ptty','Ptty'],'displays':[],
            'audio':False,'clipboard':False,'rosetta':False}


def verify_boot_files(data_directory,manifest,clock=time.monotonic):
    names={'Data/root.raw':20*1024**3,'Data/kernel.Image':192*1024**2,
           'Data/initrd':192*1024**2,'Data/seed.iso':8*1024**2}
    if not isinstance(manifest,dict) or set(manifest)!=set(names):raise ValueError('four accepted Data file identities required')
    for name,cap in names.items():
        facts=manifest[name]
        if (set(facts)!={'bytes','sha256'} or type(facts['bytes']) is not int or not 0<facts['bytes']<=cap
                or not isinstance(facts['sha256'],str) or len(facts['sha256'])!=64
                or any(c not in '0123456789abcdef' for c in facts['sha256'])):raise ValueError('invalid accepted boot file identity')
    if manifest['Data/kernel.Image']['bytes']+manifest['Data/initrd']['bytes']>192*1024**2:
        raise ValueError('combined boot auxiliary budget')
    path=Path(data_directory)
    if not path.is_absolute() or '..' in path.parts:raise ValueError('absolute owned Data directory required')
    deadline=clock()+300
    with DirectoryChain(path) as chain:
        directory=chain.fd
        if set(os.listdir(directory))!={name[5:] for name in names}:raise ValueError('unexpected or missing boot Data file')
        observed={};identities={}
        for name in names:
            if clock()>=deadline:raise TimeoutError('boot identity deadline')
            fd=os.open(name[5:],os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=directory)
            try:
                before=os.fstat(fd)
                if not stat.S_ISREG(before.st_mode) or before.st_nlink!=1 or before.st_size!=manifest[name]['bytes']:
                    raise ValueError('boot file type, links or size differs')
                digest=hashlib.sha256();size=0
                while True:
                    if clock()>=deadline:raise TimeoutError('boot identity deadline')
                    chunk=os.read(fd,1024**2)
                    if not chunk:break
                    size+=len(chunk)
                    if size>manifest[name]['bytes']:raise ValueError('boot file grew')
                    digest.update(chunk)
                after=os.fstat(fd)
                identity=lambda value:(value.st_dev,value.st_ino,value.st_size,value.st_mtime_ns,value.st_ctime_ns)
                current=os.stat(name[5:],dir_fd=directory,follow_symlinks=False)
                if identity(before)!=identity(after) or identity(after)!=identity(current):raise ValueError('boot file changed while hashing')
                observed[name]={'bytes':size,'sha256':digest.hexdigest()};identities[name]=identity(after)
            finally:os.close(fd)
        for name,accepted in identities.items():
            current=os.stat(name[5:],dir_fd=directory,follow_symlinks=False)
            if identity(current)!=accepted:raise ValueError('boot file changed before final readback')
        if set(os.listdir(directory))!={name[5:] for name in names}:raise ValueError('boot Data file set changed')
        if not exact(observed,manifest):raise ValueError('accepted boot file digest differs')
        chain.check()
        if clock()>=deadline:raise TimeoutError('boot identity deadline')
        return observed
