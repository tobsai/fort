from copy import deepcopy
import hashlib
from pathlib import Path
import plistlib
import tempfile
import unittest
from fort_probe.boot import validate_config,verify_boot_files
from fort_probe.config import candidate
from fort_probe.utm_schema import pinned_schema
from test_config import IMAGE
from support import BINDING

class BootTests(unittest.TestCase):
    def config(self):return plistlib.loads(candidate(BINDING,IMAGE,pinned_schema(BINDING))['config_plist'])
    def test_complete_configuration_not_just_topology_is_bound(self):
        original=self.config()
        self.assertEqual(validate_config(original)['uuid'],BINDING['vm_package_uuid'])
        for field in original['System']['Boot']:
            for value in ('missing','unexpected'):
                config=deepcopy(original)
                if value=='missing':config['System']['Boot'].pop(field)
                else:config['System']['Boot'][field]='unexpected'
                with self.subTest(field=field,value=value),self.assertRaises(ValueError):validate_config(config)
        for change in (lambda c:c['System']['Boot'].update(EfiVariableStoragePath='outside'),
                       lambda c:c['System']['Boot'].update(LinuxKernelPath='/outside'),
                       lambda c:c['System']['Boot'].update(LinuxInitialRamdiskPath='sub/initrd'),
                       lambda c:c['System']['Boot'].update(LinuxCommandLine=c['System']['Boot']['LinuxCommandLine']+' init=/bin/sh'),
                       lambda c:c['System']['Boot'].update(LinuxCommandLine=c['System']['Boot']['LinuxCommandLine'].replace('systemd.mask=ssh.service','')),
                       lambda c:c['Drive'][0].update(Bookmark=b'outside'),
                       lambda c:c['Drive'][1].update(ReadOnly=False),
                       lambda c:c['System'].update(MacPlatform={}),
                       lambda c:c['Virtualization'].update(Balloon=True)):
            config=deepcopy(original);change(config)
            with self.assertRaises(ValueError):validate_config(config)

    def test_uuid_case_is_canonical_but_different_identity_does_not_match_pin(self):
        config=self.config();config['Information']['UUID']=BINDING['vm_package_uuid'].upper()
        self.assertEqual(validate_config(config)['uuid'],BINDING['vm_package_uuid'])
        config['Information']['UUID']='248664c8-5334-5c17-8b7a-d60351f3a79f'
        with self.assertRaises(ValueError):validate_config(config,BINDING['vm_package_uuid'])

    def test_prestart_hashes_exact_regular_data_files_and_rejects_mutation_or_alias(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();manifest={}
            for name in ('root.raw','kernel.Image','initrd','seed.iso'):
                data=('synthetic '+name).encode();(root/name).write_bytes(data)
                manifest['Data/'+name]={'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest()}
            self.assertEqual(verify_boot_files(root,manifest),manifest)
            (root/'kernel.Image').write_bytes(b'changed')
            with self.assertRaises(ValueError):verify_boot_files(root,manifest)
            (root/'kernel.Image').unlink();(root/'kernel.Image').symlink_to(root/'initrd')
            with self.assertRaises((ValueError,OSError)):verify_boot_files(root,manifest)
            with self.assertRaises(ValueError):verify_boot_files(root,{k:v for k,v in manifest.items() if k!='Data/seed.iso'})

    def test_unlisted_saved_state_or_extra_data_never_passes_prestart(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();manifest={}
            for name in ('root.raw','kernel.Image','initrd','seed.iso'):
                (root/name).write_bytes(b'fixture');manifest['Data/'+name]={'bytes':7,'sha256':hashlib.sha256(b'fixture').hexdigest()}
            (root/'saved-state.bin').write_bytes(b'not an accepted boot input')
            with self.assertRaises(ValueError):verify_boot_files(root,manifest)

    def test_host_prestart_is_available_before_any_process_or_pty_binding(self):
        from fort_probe.host import Host
        from test_host import HostFixture,PIN
        kernel=HostFixture();calls=[]
        kernel.prestart=lambda accepted:calls.append('prestart') or {'vm_uuid':PIN['vm_uuid'],'data_files':{'fixture':'only'}}
        host=Host(kernel,dict(PIN,vm_uuid=PIN['vm_uuid'].upper()))
        self.assertEqual(host.prestart({'fixture':'only'})['vm_uuid'],PIN['vm_uuid'])
        self.assertEqual(calls,['prestart']);self.assertIsNone(host.bound)
