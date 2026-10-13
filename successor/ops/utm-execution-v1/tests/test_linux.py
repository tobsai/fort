from copy import deepcopy
import errno
import unittest
from fort_probe.linux import LinuxGuest, parse_stat, canary_results
from fort_probe.guest_programs import tool_probe
from test_guest_programs import ToolIO
from support import BINDING, Clock, fixture_process


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
    def runtime_report(self): return {'provider_literal_read':True,'guest_boot_id':self.boot(),'invocation_id':'1'*32,'serial_denials':[{'path':path,'write':False,'errno':errno.ENOENT} for path in ('/dev/hvc0','/dev/hvc1')]}
    def tool_report(self,unit):
        result=tool_probe(ToolIO()); result['opens'][-1]['errno']=errno.EROFS; result['descendant']={'pid':42,'start_ticks':100}; return result
    def ipc_start(self): return {'device':1,'inode':2,'uid':0,'connected':True}
    def ipc_finish(self): return self.ipc_start()
    def detached(self,pid,tool): return True


class LinuxTests(unittest.TestCase):
    def test_adapter_starts_only_runtime_then_grants_tool_and_collects_bound_facts(self):
        kernel,clock=LinuxFixture(),Clock(); guest=LinuxGuest(kernel,clock)
        self.assertTrue(guest.ready()); self.assertFalse(guest.preflight()['runtime_active'])
        identity=guest.prepare(BINDING)
        self.assertEqual(kernel.commands,['start-runtime'])
        self.assertFalse(guest.prepared_controls(identity)['tool_active'])
        guest.grant(identity)
        live=guest.live_snapshot(identity,1,clock())
        self.assertEqual(set(live['processes']),{'runtime','tool','descendant'})
        self.assertTrue(all(live['controls'].values()))
        guest.begin_stop(identity); self.assertTrue(guest.stop_observation(identity)['empty'])
        self.assertEqual(kernel.commands,['start-runtime','start-tool','stop'])

    def test_remaining_descendant_and_wrong_invocation_never_prove_stop(self):
        kernel,clock=LinuxFixture(),Clock(); guest=LinuxGuest(kernel,clock)
        identity=guest.prepare(BINDING); guest.grant(identity)
        kernel.remain=True; guest.begin_stop(identity)
        self.assertFalse(guest.stop_observation(identity)['empty'])
        with self.assertRaises(ValueError): guest.grant(dict(identity,invocation_id='f'*32))

    def test_canary_interpreter_rejects_refused_ipc_successful_open_and_missing_result(self):
        for fault in ('ipc','open','missing'):
            result=tool_probe(ToolIO()); result['opens'][-1]['errno']=errno.EROFS
            if fault=='ipc': result['ipc']['errno']=errno.ECONNREFUSED
            if fault=='open': result['opens'][0]['errno']=None
            if fault=='missing': result['opens'].pop()
            with self.assertRaises(ValueError): canary_results(result,{'device':1,'inode':2,'uid':0,'connected':True},{'device':1,'inode':2,'uid':0,'connected':True})

    def test_proc_stat_parses_parentheses_and_start_ticks_without_pid_reuse(self):
        raw='42 (a weird ) name) S 1 42 41 ' + ' '.join(['0']*15+['12345']+['0']*5)
        self.assertEqual(parse_stat(raw)['start_ticks'],12345)
        self.assertEqual(parse_stat(raw)['session'],41)
