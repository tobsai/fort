"""Mac attribution/absence policy and disabled read-only collector. No UTM control API."""
from copy import deepcopy
import hashlib
import plistlib
import re
import time
from pathlib import Path
from .bounded import Commands,require_live
from .linux_kernel import bounded_read


def parse_processes(raw):
    result=[]
    for line in raw.splitlines():
        parts=line.split(None,8)
        if len(parts)!=9: raise ValueError('incomplete process inventory')
        result.append({'pid':int(parts[0]),'parent':int(parts[1]),'uid':int(parts[2]),
                       'start_time':' '.join(parts[3:8]),'executable':parts[8]})
    return result


class Host:
    def __init__(self,kernel,pin): self.kernel,self.pin=kernel,deepcopy(pin); self.bound=None
    def bind(self,pid,start_time,ptys):
        if self.bound is not None or len(set(ptys))!=2 or any(not re.fullmatch(r'/dev/ttys[0-9]{3,}',p) for p in ptys):
            raise ValueError('one exact two-PTY binding required')
        signature=self.kernel.signature()
        if signature!={'team':self.pin['team'],'executable_sha256':self.pin['executable_sha256'],'strict_valid':True,'gatekeeper_accepted':True}:
            raise ValueError('app signature or executable pin mismatch')
        topology=self.kernel.config()
        if topology!={'uuid':self.pin['vm_uuid'],'nics':[],'shares':[],'serials':['Ptty','Ptty'],'displays':[],
                      'audio':False,'clipboard':False,'rosetta':False}: raise ValueError('unexpected VM device topology')
        observed=self.kernel.collect()
        if observed['complete'] is not True: raise ValueError('host observation incomplete')
        apps=[p for p in observed['processes'] if p['role']=='app' and p['pid']==pid and p['start_time']==start_time
              and p['uid']==501 and p['executable_sha256']==self.pin['executable_sha256']]
        if len(apps)!=1 or len([p for p in observed['processes'] if p['role']=='app'])!=1: raise ValueError('exact app process not proven')
        processes=observed['processes']
        if not processes or any(p['role'] not in ('app','virtualization','helper') for p in processes): raise ValueError('unknown process role')
        if any(p['role']!='app' and p.get('vm_related') is not True for p in processes): raise ValueError('helper not attributed to this VM')
        owners={p['pid'] for p in processes}
        if any(not observed['pty_owners'].get(path) or not set(observed['pty_owners'][path])<=owners for path in ptys):
            raise ValueError('PTY unowned or competing consumer')
        self.bound={'vm_uuid':self.pin['vm_uuid'],'processes':deepcopy(processes),'ptys':list(ptys)}
        return deepcopy(self.bound)
    def quiescence(self,registration_removed,vm_stopped):
        if self.bound is None: raise ValueError('no prior process attribution')
        observed=self.kernel.collect()
        if (observed['complete'] is not True or observed['processes'] or observed['mounts'] or observed['handles']
                or any(observed['pty_owners'].get(path,[None]) for path in self.bound['ptys'])
                or registration_removed is not True or vm_stopped is not True): raise ValueError('host termination/ownership unresolved')
        return {'vm_package_uuid':self.pin['vm_uuid'],
                'expected_processes':[{key:p[key] for key in ('role','pid','start_time','executable_sha256')} for p in self.bound['processes']],
                'observed_processes':[],'open_handles':[],'mounted_devices':[],'registration_removed':True,
                'vm_stopped':True,'observation_complete':True}


class MacKernel:
    def __init__(self,root,ptys,expected_process_paths,registry_source=None):
        require_live()
        self.registry_source=registry_source
        self.root=Path(root); self.ptys=tuple(ptys); self.paths=dict(expected_process_paths); self.commands=Commands()
        self.app=self.root/'tools/UTM-4.7.5/UTM.app'
    def signature(self):
        self.commands.run(['/usr/bin/codesign','--verify','--deep','--strict',str(self.app)],timeout=5)
        self.commands.run(['/usr/sbin/spctl','--assess','--type','execute',str(self.app)],timeout=5)
        out,err=self.commands.run(['/usr/bin/codesign','-d','--verbose=4',str(self.app)])
        text=(out+err).decode(); match=re.search(r'^TeamIdentifier=(\S+)$',text,re.M)
        return {'team':match.group(1) if match else None,'executable_sha256':hashlib.sha256(bounded_read(str(self.app/'Contents/MacOS/UTM'),18662240)).hexdigest(),
                'strict_valid':True,'gatekeeper_accepted':True}
    def config(self):
        if self.registry_source is None: raise ValueError('pinned UTMRegistryEntry schema/collector still required')
        registry=self.registry_source()
        if set(registry)!={'uuid','shares','complete'} or registry['complete'] is not True: raise ValueError('registry readback incomplete')
        config=plistlib.loads(bounded_read(str(self.root/'vm/fort-utm-isolation-probe.utm/config.plist')))
        if registry['uuid']!=config['Information']['UUID']: raise ValueError('registry VM mismatch')
        if config.get('SharedDirectory',[]) or registry['shares']: raise ValueError('shared directory configured')
        if (config['Backend']!='Apple' or config['System']['Architecture']!='aarch64'
                or config['System']['CPUCount']!=2 or config['System']['MemorySize']!=4096
                or config['System']['Boot']['UEFIBoot'] is not False
                or [(d['ImageName'],d['ReadOnly'],d['Nvme']) for d in config['Drive']]!=[('root.raw',False,False),('seed.iso',True,False)]
                or config['Virtualization']['Keyboard']!='Disabled' or config['Virtualization']['Pointer']!='Disabled'):
            raise ValueError('VM resource/device layout differs')
        return {'uuid':config['Information']['UUID'],'nics':config['Network'],'shares':registry['shares'],
                'serials':[s['Mode'] for s in config['Serial']],'displays':config['Display'],
                'audio':config['Virtualization']['Audio'],'clipboard':config['Virtualization']['ClipboardSharing'],
                'rosetta':config['Virtualization']['Rosetta']}
    def collect(self):
        before=time.monotonic()
        out,_=self.commands.run(['/bin/ps','-axo','pid=,ppid=,uid=,lstart=,comm='],cap=1048576)
        all_processes=parse_processes(out.decode()); processes=[]
        for p in all_processes:
            if p['executable'] in self.paths:
                processes.append(dict(p,role=self.paths[p['executable']],executable_sha256=hashlib.sha256(bounded_read(p['executable'],256*1024*1024)).hexdigest()))
            elif 'UTM.app/' in p['executable'] or 'Virtualization.VirtualMachine' in p['executable']:
                raise ValueError('unattributed UTM/VZ process')
        for process in processes:
            if process['role']=='app': continue
            output,_=self.commands.run(['/usr/sbin/lsof','-n','-P','-Fn','-p',str(process['pid'])],cap=1048576)
            vm_prefix='n'+str(self.root/'vm/fort-utm-isolation-probe.utm')+'/'
            process['vm_related']=any(line.startswith(vm_prefix) for line in output.decode().splitlines())
        owners={}
        for path in self.ptys:
            out,_=self.commands.run(['/usr/sbin/lsof','-n','-P','-Fp','--',path],allow_empty=True)
            owners[path]=[int(line[1:]) for line in out.decode().splitlines() if line.startswith('p')]
        out,_=self.commands.run(['/usr/sbin/lsof','-n','-P','-Fp','+D',str(self.root)],cap=1048576,allow_empty=True)
        handles=out.decode().splitlines()
        out,_=self.commands.run(['/sbin/mount'],cap=1048576)
        mounts=[line for line in out.decode().splitlines() if str(self.root) in line or 'UTM' in line]
        return {'processes':processes,'pty_owners':owners,'handles':handles,'mounts':mounts,
                'complete':time.monotonic()-before<2}
