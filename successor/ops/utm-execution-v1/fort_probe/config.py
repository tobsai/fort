"""In-memory candidate generation. Archive/schema facts are explicit required inputs.

This function is exercised with synthetic facts only. It never writes a VM package.
"""
import hashlib
import json
from pathlib import Path,PurePosixPath
import plistlib
import re
from .seed import preview
from .utm_schema import pinned_schema
from .evidence import exact

from .boot import MASKS,vm_configuration

SOURCES=('__init__','bounded','descriptors','guest_entry','guest_programs','linux_kernel','linux','broker','controller','evidence','serial','ipc')


def validate_image(image):
    if image is None:raise ValueError('actual archive facts required')
    expected={'raw_member','kernel_member','initrd_member','raw_bytes','root_label','kernel_arm64','member_sha256','archive_sha256',
              'member_bytes','kernel_compressed','auxiliary_bytes','auxiliary_peak_bytes','decoded_kernel_bytes','decoded_kernel_sha256'}
    if (set(image)!=expected or type(image['raw_bytes']) is not int or not 0<image['raw_bytes']<=20*1024**3
            or image['root_label']!='cloudimg-rootfs' or image['kernel_arm64'] is not True
            or set(image['member_sha256'])!={'raw','kernel','initrd'}
            or any(not re.fullmatch('[0-9a-f]{64}',v) for v in [image['archive_sha256'],image['decoded_kernel_sha256'],*image['member_sha256'].values()])):
        raise ValueError('archive facts incomplete or unsupported')
    for key in ('raw_member','kernel_member','initrd_member'):
        path=PurePosixPath(image[key])
        if path.is_absolute() or '..' in path.parts or str(path)!=image[key]: raise ValueError('archive member path unsafe')
    if (set(image['member_bytes'])!={'raw','kernel','initrd'}
            or any(type(v) is not int or v<=0 for v in image['member_bytes'].values())
            or image['member_bytes']['raw']!=image['raw_bytes'] or type(image['kernel_compressed']) is not bool
            or any(type(image[k]) is not int or not 0<image[k]<=192*1024**2
                   for k in ('auxiliary_bytes','auxiliary_peak_bytes','decoded_kernel_bytes'))
            or image['auxiliary_bytes']<image['member_bytes']['kernel']+image['member_bytes']['initrd']
            or image['auxiliary_peak_bytes']!=image['auxiliary_bytes']+(image['decoded_kernel_bytes'] if image['kernel_compressed'] else 0)):
        raise ValueError('archive auxiliary or decoded kernel facts incomplete')
    if not image['kernel_compressed'] and (image['decoded_kernel_bytes']!=image['member_bytes']['kernel']
            or image['decoded_kernel_sha256']!=image['member_sha256']['kernel']):
        raise ValueError('uncompressed kernel identity differs')


def candidate(binding,image,schema):
    if schema is None:raise ValueError('actual UTM schema facts required')
    validate_image(image)
    if not exact(schema,pinned_schema(binding)):raise ValueError('pinned UTM schema identity differs')
    seed=preview(); cloud=json.loads(seed['user-data'].split(b'\n',1)[1]); files=cloud['write_files']
    by_path={f['path']:f for f in files}
    def add(path,content,mode='0644'):
        item={'path':path,'content':content,'owner':'root:root','permissions':mode}
        if path in by_path: files.remove(by_path[path])
        by_path[path]=item; files.append(item)
    for role in ('runtime','tool'):
        path='/etc/systemd/system/fort-'+role+'.service'
        unit=by_path[path]['content'].replace('/opt/fort-probe/disabled.py','/opt/fort-probe/guest_launcher.py '+role)
        unit+='StandardInput=null\nStandardOutput=journal\nStandardError=journal\n'
        if role=='runtime': unit+='ReadWritePaths=/run/fort-runtime-probe/report\n'
        else: unit+='InaccessiblePaths=-/run/fort-runtime-probe\n'
        add(path,unit)
    add('/etc/systemd/system/fort-broker.service','[Unit]\nDescription=Fort synthetic root serial broker\nAfter=cloud-final.service\n[Service]\nType=simple\nUser=root\nExecStart=/usr/bin/python3 -I -B /opt/fort-probe/guest_launcher.py broker\nRestart=no\nRuntimeMaxSec=120\nTimeoutStopSec=2\nKillMode=control-group\nLimitCORE=0\nStandardInput=null\nStandardOutput=journal\nStandardError=journal\n')
    for name in SOURCES:
        add('/opt/fort-probe/fort_probe/'+name+'.py',Path(__file__).with_name(name+'.py').read_text())
    add('/opt/fort-probe/guest_launcher.py',"import sys\nsys.path.insert(0, '/opt/fort-probe')\nfrom fort_probe.guest_entry import main\nmain()\n")
    add('/opt/fort-probe/binding.json',json.dumps(binding,sort_keys=True)+'\n','0600')
    add('/etc/udev/rules.d/99-fort-serial.rules','KERNEL=="hvc[01]", OWNER="root", GROUP="root", MODE="0600"\n')
    cloud['cloud_init_modules']=['bootcmd','users-groups','write-files']
    cloud['cloud_config_modules']=['runcmd']; cloud['cloud_final_modules']=['scripts-user']
    cloud['bootcmd']=[['/usr/bin/systemctl','mask',*MASKS]]
    cloud['runcmd']=[['/usr/bin/python3','-I','-B','/opt/fort-probe/guest_launcher.py','bootstrap']]
    for user in cloud['users']:
        user['primary_group']=user['name']
    cloud['bootcmd'] += [['/usr/sbin/groupadd','--gid','1000','fort-admin'],['/usr/sbin/groupadd','--gid','2001','fort-runtime']]
    seed['user-data']=('#cloud-config\n'+json.dumps(cloud,sort_keys=True,indent=2)+'\n').encode()
    seed['meta-data']=(json.dumps({'instance-id':'fort-utm-probe-v1','local-hostname':'fort-utm-probe',
                                'fort_seed_sha256':hashlib.sha256(seed['user-data']).hexdigest()},sort_keys=True)+'\n').encode()
    if sum(map(len,seed.values()))>8*1024**2: raise ValueError('seed content cap exceeded')
    config=vm_configuration(binding)
    data_files={'Data/root.raw':{'bytes':image['raw_bytes'],'sha256':image['member_sha256']['raw']},
                'Data/kernel.Image':{'bytes':image['decoded_kernel_bytes'],'sha256':image['decoded_kernel_sha256']},
                'Data/initrd':{'bytes':image['member_bytes']['initrd'],'sha256':image['member_sha256']['initrd']}}
    return {'execution_enabled':False,'seed':seed,'config_plist':plistlib.dumps(config,sort_keys=True),'data_files':data_files}


def iso_command(seed_directory,output):
    for path in (seed_directory,output):
        if not Path(path).is_absolute() or '..' in Path(path).parts: raise ValueError('owned absolute paths required')
    return ['/usr/bin/hdiutil','makehybrid','-iso','-joliet','-default-volume-name','cidata','-o',output,seed_directory]
