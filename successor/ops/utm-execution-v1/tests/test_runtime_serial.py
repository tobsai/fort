from copy import deepcopy
import errno
import unittest
from fort_probe.guest_programs import Runtime
from fort_probe.linux import LinuxGuest
from support import Clock,BINDING
from test_guest_programs import RuntimeIO
from test_linux import LinuxFixture

class RuntimeSerialTests(unittest.TestCase):
    def test_runtime_attempts_both_fixed_devices_before_ready(self):
        io=RuntimeIO();calls=[]
        io.denied_open=lambda path,write=False:calls.append((path,write)) or errno.ENOENT
        Runtime(io,Clock(),'boot','1'*32).start()
        self.assertEqual(calls,[('/dev/hvc0',False),('/dev/hvc1',False)])
        self.assertEqual([x['errno'] for x in io.reports[0]['serial_denials']],[errno.ENOENT]*2)
        for value in (None,False,0,errno.EIO,'13'):
            io=RuntimeIO();io.denied_open=lambda *args:value
            with self.assertRaises(ValueError):Runtime(io,Clock(),'boot','1'*32).start()
            self.assertEqual(io.notifications,[])

    def test_root_rejects_absent_successful_untyped_reordered_or_unbound_evidence(self):
        valid={'provider_literal_read':True,'guest_boot_id':LinuxFixture().boot(),'invocation_id':'1'*32,
            'serial_denials':[{'path':path,'write':False,'errno':errno.ENOENT} for path in ('/dev/hvc0','/dev/hvc1')]}
        for fault in ('missing','open','bool','wrong-path','reordered','boot','invocation','root-target'):
            kernel=LinuxFixture();guest=LinuxGuest(kernel,Clock());identity=guest.prepare(BINDING)
            report=deepcopy(valid)
            if fault=='missing':report.pop('serial_denials')
            elif fault=='open':report['serial_denials'][0]['errno']=None
            elif fault=='bool':report['serial_denials'][0]['errno']=True
            elif fault=='wrong-path':report['serial_denials'][0]['path']='/dev/null'
            elif fault=='reordered':report['serial_denials'].reverse()
            elif fault=='boot':report['guest_boot_id']='wrong'
            elif fault=='invocation':report['invocation_id']='2'*32
            elif fault=='root-target':kernel.root_controls=lambda:{'serial':False}
            kernel.runtime_report=lambda:report
            with self.subTest(fault=fault),self.assertRaises(ValueError):guest.prepared_controls(identity)
