import errno
import unittest
from fort_probe.guest_programs import Runtime, tool_probe
from support import Clock


class RuntimeIO:
    def denied_open(self,path,write=False):return errno.ENOENT
    def __init__(self): self.notifications=[]; self.reports=[]; self.lease=None
    def provider(self): return b'synthetic-provider-canary-only\n'
    def report(self, value): self.reports.append(value)
    def notify(self, value): self.notifications.append(value)
    def heartbeat(self): return self.lease


class ToolIO:
    def denied_open(self, path, write=False): return errno.ENOENT
    def management_connect(self): return {'connected': False, 'errno': errno.ENOENT}
    def own_scratch(self): return True
    def own_socketpair(self): return True
    def ip_socket(self, family): return errno.EPERM
    def descendant(self): return {'pid': 43, 'start_ticks': 101}


class ProgramTests(unittest.TestCase):
    def test_runtime_prepares_canary_without_tool_work_then_requires_fresh_bound_lease(self):
        io, clock = RuntimeIO(), Clock()
        runtime = Runtime(io, clock, 'boot', '1' * 32)
        runtime.start()
        self.assertTrue(io.reports[0]['provider_literal_read'])
        self.assertEqual(io.notifications, ['READY=1'])
        self.assertTrue(runtime.step())
        io.lease={'guest_boot_id':'boot','invocation_id':'1'*32,'at':clock(),'granted':True}
        self.assertTrue(runtime.step())
        clock.advance(1.51)
        self.assertFalse(runtime.step())
        self.assertNotIn('WATCHDOG=1', io.notifications[-1:])

    def test_expired_ungranted_runtime_and_replayed_identity_stop(self):
        for fault in ('expired', 'identity'):
            clock, io = Clock(), RuntimeIO()
            runtime=Runtime(io,clock,'boot','1'*32); runtime.start()
            if fault=='expired': clock.advance(4.01)
            else: io.lease={'guest_boot_id':'old','invocation_id':'2'*32,'at':0,'granted':True}
            self.assertFalse(runtime.step())

    def test_tool_only_reports_fixed_denials_and_own_positive_controls(self):
        result=tool_probe(ToolIO())
        self.assertTrue(result['scratch'])
        self.assertTrue(result['socketpair'])
        self.assertEqual(result['ipc'], {'connected':False,'errno':errno.ENOENT})
        self.assertTrue(all(x['errno']==errno.ENOENT for x in result['opens']))
        self.assertEqual(result['ip'], {'inet':errno.EPERM,'inet6':errno.EPERM})
        self.assertEqual(result['descendant']['pid'],43)
