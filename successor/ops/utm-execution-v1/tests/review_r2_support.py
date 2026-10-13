"""Frozen R2 boundary fixture: deliberately lacks runtime C05 evidence."""
from copy import deepcopy
import errno
from support import fixture_process
from test_guest_programs import ToolIO
from fort_probe.guest_programs import tool_probe

class LinuxFixture:
    """Only raw OS boundary values are doubled; LinuxGuest policy executes unchanged."""
    def __init__(self):
        self.active=False; self.tool=False; self.commands=[]; self.files={}; self.remain=False
    def boot(self): return '12345678-1234-1234-1234-123456789abc'
    def units(self):
        return {role:{'active':self.active if role=='runtime' else self.tool,'invocation':('1' if role=='runtime' else '2')*32,
                'pid':40 if role=='runtime' else 41, 'cgroup':fixture_process(role)['cgroup']}
                for role in ('runtime','tool')}
    def run(self, action):
        self.commands.append(action)
        if action=='start-runtime': self.active=True
        if action=='start-tool': self.tool=True
        if action=='stop': self.active=self.tool=False
    def pids(self): return [40,41,42] if self.tool or self.remain else [40] if self.active else []
    def write(self, name, value): self.files[name]=deepcopy(value)
    def root_controls(self): return dict.fromkeys(('provider','identity_store','sibling','seed','serial','ipc_listener'),True)
    def topology(self): return {'nics':[],'shares':[]}
    def process(self,pid): return fixture_process({40:'runtime',41:'tool',42:'descendant'}[pid])
    def runtime_report(self): return {'provider_literal_read':True,'guest_boot_id':self.boot(),'invocation_id':'1'*32}
    def tool_report(self,unit):
        result=tool_probe(ToolIO()); result['opens'][-1]['errno']=errno.EROFS; result['descendant']={'pid':42,'start_ticks':100}; return result
    def ipc_start(self): return {'device':1,'inode':2,'uid':0,'connected':True}
    def ipc_finish(self): return self.ipc_start()
    def detached(self,pid,tool): return True
