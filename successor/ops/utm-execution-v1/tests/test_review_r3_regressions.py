"""Expected-behavior regressions against fixed R3; no live adapter is constructed.

Only MacKernel.config's unchanged AST is loaded. Its file and registry reads are
replaced with in-memory data; Host uses supplied signature/process/PTY fixtures.
Run: /usr/bin/python3 -I -B reviewer_regressions.py
"""
import ast
from copy import deepcopy
import hashlib
import json
from pathlib import Path
import plistlib
import sys
from types import SimpleNamespace
import unittest

HERE = Path(__file__).resolve().parent
SOURCE = HERE.parent
sys.path[:0] = [str(SOURCE), str(SOURCE / 'tests')]
from fort_probe.config import candidate
from fort_probe.evidence import exact
from fort_probe.host import Host
from fort_probe.utm_schema import pinned_schema, registry_snapshot
from support import BINDING
from test_config import IMAGE
from test_host import HostFixture, PIN, PROCESS

host_source = (SOURCE / 'fort_probe/host.py').read_bytes()
tree = ast.parse(host_source)
mac = next(n for n in tree.body if isinstance(n, ast.ClassDef) and n.name == 'MacKernel')
method = next(n for n in mac.body if isinstance(n, ast.FunctionDef) and n.name == 'config')
module = ast.Module(body=[method], type_ignores=[])
observations = {'commit': 'd965abb76d20a17879925ae0e3fedd504ecc6a57',
                'host_source_sha256': hashlib.sha256(host_source).hexdigest(),
                'boundaries': 'in-memory config/registry, fixture signature/process/PTY; no live constructor',
                'boot_mutations': {}}


class ReadbackTests(unittest.TestCase):
    def setUp(self):
        self.config = plistlib.loads(candidate(BINDING, IMAGE, pinned_schema(BINDING))['config_plist'])
        registry = {PIN['vm_uuid'].upper(): {
            'Name': 'fort-utm-isolation-probe', 'UUID': PIN['vm_uuid'].upper(),
            'Package': {'Path': '/fixture/vm/fort-utm-isolation-probe.utm', 'Bookmark': b'', 'ReadOnly': False},
            'Suspended': False, 'ExternalDrives': {}, 'SharedDirectories': [], 'WindowSettings': {}}}
        readback = SimpleNamespace(root=Path('/fixture'), registry_source=lambda: {
            'persisted': deepcopy(registry), 'in_memory': deepcopy(registry), 'bookmark_resolution': None})
        namespace = {'plistlib': plistlib, 'pinned_schema': pinned_schema,
                     'registry_snapshot': registry_snapshot, 'exact': exact,
                     'bounded_read': lambda *_args: plistlib.dumps(self.config)}
        exec(compile(module, str(SOURCE / 'fort_probe/host.py'), 'exec'), namespace)
        self.kernel = HostFixture()
        self.kernel.config = lambda: namespace['config'](readback)

    def bind(self):
        return Host(self.kernel, PIN).bind(42, PROCESS['start_time'], ['/dev/ttys001', '/dev/ttys002'])

    def test_approved_configuration_binds(self):
        self.assertEqual(self.bind()['vm_uuid'], PIN['vm_uuid'])
        observations['approved_configuration_binds'] = True

    def test_equivalent_uppercase_uuid_binds(self):
        self.config['Information']['UUID'] = PIN['vm_uuid'].upper()
        observations['uppercase_config_readback'] = self.kernel.config()['uuid']
        error = None
        try:
            self.bind()
        except ValueError as rejected:
            error = str(rejected)
        observations['uppercase_binding_error'] = error
        self.assertIsNone(error, 'equivalent UUID spelling must retain canonical pinned identity')

    def test_changed_boot_inputs_are_rejected(self):
        original = deepcopy(self.config['System']['Boot'])
        for key, value in (
            ('OperatingSystem', 'None'),
            ('LinuxKernelPath', '../../outside.Image'),
            ('LinuxInitialRamdiskPath', '../../outside.initrd'),
            ('LinuxCommandLine', 'init=/bin/sh'),
        ):
            with self.subTest(field=key):
                self.config['System']['Boot'] = dict(original, **{key: value})
                error = None
                try:
                    self.bind()
                except (ValueError, KeyError) as rejected:
                    error = str(rejected)
                observations['boot_mutations'][key] = {'value': value, 'rejected': error is not None, 'error': error}
                self.assertIsNotNone(error, 'changed accepted boot input must be rejected before Start')
