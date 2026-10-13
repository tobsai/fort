from copy import deepcopy
import unittest
from fort_probe.host import Host,parse_processes
from support import BINDING

PIN={'vm_uuid':BINDING['vm_package_uuid'],'executable_sha256':'a'*64,'team':'WDNLXAD4W8'}
PROCESS={'role':'app','pid':42,'start_time':'Tue Oct 13 10:00:00 2026','uid':501,'executable':'/fixture/UTM.app/Contents/MacOS/UTM','executable_sha256':'a'*64}


class HostFixture:
    def __init__(self): self.snapshot={'processes':[deepcopy(PROCESS)],'pty_owners':{'/dev/ttys001':[42],'/dev/ttys002':[42]},'mounts':[],'handles':[],'complete':True}
    def signature(self): return {'team':'WDNLXAD4W8','executable_sha256':'a'*64,'strict_valid':True,'gatekeeper_accepted':True}
    def collect(self): return deepcopy(self.snapshot)
    def config(self): return {'uuid':PIN['vm_uuid'],'nics':[],'shares':[],'serials':['Ptty','Ptty'],'displays':[],'audio':False,'clipboard':False,'rosetta':False}


class HostTests(unittest.TestCase):
    def test_exact_process_and_two_pty_owners_bind_then_fresh_absence_allows_cleanup(self):
        kernel=HostFixture();host=Host(kernel,PIN)
        binding=host.bind(42,PROCESS['start_time'],['/dev/ttys001','/dev/ttys002'])
        self.assertEqual(binding['vm_uuid'],PIN['vm_uuid'])
        kernel.snapshot.update(processes=[],pty_owners={'/dev/ttys001':[],'/dev/ttys002':[]},handles=[])
        self.assertTrue(host.quiescence(registration_removed=True,vm_stopped=True)['observation_complete'])

    def test_competing_reader_pid_reuse_network_or_unknown_scan_blocks(self):
        for fault in ('reader','reuse','network','unknown'):
            kernel=HostFixture();host=Host(kernel,PIN)
            if fault=='reader': kernel.snapshot['pty_owners']['/dev/ttys001'].append(99)
            if fault=='reuse': kernel.snapshot['processes'][0]['start_time']='different'
            if fault=='network': kernel.config=lambda: dict(HostFixture().config(),nics=['shared'])
            if fault=='unknown': kernel.snapshot['complete']=False
            with self.assertRaises(ValueError): host.bind(42,PROCESS['start_time'],['/dev/ttys001','/dev/ttys002'])

    def test_process_parser_excludes_arguments_and_preserves_start_identity(self):
        rows=parse_processes('42 1 501 Tue Oct 13 10:00:00 2026 /fixture/UTM.app/Contents/MacOS/UTM\n')
        self.assertEqual(rows[0]['start_time'],PROCESS['start_time'])
        self.assertEqual(rows[0]['executable'],PROCESS['executable'])
