import json
import plistlib
import unittest
from fort_probe.config import candidate,iso_command
from support import BINDING

IMAGE={'raw_member':'synthetic/root.raw','kernel_member':'synthetic/Image','initrd_member':'synthetic/initrd',
       'raw_bytes':1024**3,'root_label':'cloudimg-rootfs','kernel_arm64':True,
       'member_sha256':dict.fromkeys(('raw','kernel','initrd'),'b'*64),'archive_sha256':'c'*64}
SCHEMA={'version':4,'information':{'Name':'fort-utm-isolation-probe','UUID':BINDING['vm_package_uuid']}}


class ConfigTests(unittest.TestCase):
    def test_three_file_seed_contains_guarded_guest_source_and_only_root_broker_bootstrap(self):
        result=candidate(BINDING,IMAGE,SCHEMA)
        self.assertFalse(result['execution_enabled'])
        seed=result['seed']; self.assertEqual(set(seed),{'user-data','meta-data','network-config'})
        config=json.loads(seed['user-data'].split(b'\n',1)[1])
        paths={f['path']:f for f in config['write_files']}
        self.assertIn('LIVE_EXECUTION_ENABLED = False',paths['/opt/fort-probe/fort_probe/bounded.py']['content'])
        self.assertEqual(config['runcmd'],[['/usr/bin/python3','-I','-B','/opt/fort-probe/guest_launcher.py','bootstrap']])
        self.assertNotIn('WantedBy=',paths['/etc/systemd/system/fort-runtime.service']['content'])
        self.assertIn('StandardOutput=journal',paths['/etc/systemd/system/fort-tool.service']['content'])
        self.assertIn('2001',json.dumps(config['users']))
        self.assertNotIn('packages',config)
        import hashlib
        self.assertEqual(json.loads(seed['meta-data'])['fort_seed_sha256'],hashlib.sha256(seed['user-data']).hexdigest())
        vm=plistlib.loads(result['config_plist'])
        self.assertEqual(vm['Network'],[]); self.assertEqual(vm['SharedDirectory'],[])
        self.assertEqual(vm['Serial'],[{'Mode':'Ptty'},{'Mode':'Ptty'}])
        self.assertEqual([d['ReadOnly'] for d in vm['Drive']],[False,True])
        self.assertIn('systemd.mask=ssh.service',vm['System']['Boot']['LinuxCommandLine'])

    def test_unknown_archive_layout_or_schema_never_generates_vm_configuration(self):
        for image,schema in ((None,SCHEMA),(IMAGE,None),(dict(IMAGE,kernel_arm64=False),SCHEMA),(dict(IMAGE,raw_bytes=21*1024**3),SCHEMA)):
            with self.assertRaises(ValueError): candidate(BINDING,image,schema)

    def test_iso_command_is_local_cidata_only_and_does_not_run(self):
        command=iso_command('/fixture/seed','/fixture/seed.iso')
        self.assertEqual(command[0],'/usr/bin/hdiutil'); self.assertIn('cidata',command)
