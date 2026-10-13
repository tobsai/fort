import os
from pathlib import Path
import tempfile
import unittest
from fort_probe.lifecycle import OwnedTree
from support import BINDING

PROCESSES = [{'role': 'app', 'pid': 123, 'start_time': 'fixture-start', 'executable_sha256': 'b' * 64}]


def proof():
    return {'vm_package_uuid': BINDING['vm_package_uuid'], 'expected_processes': PROCESSES,
            'observed_processes': [], 'open_handles': [], 'mounted_devices': [],
            'registration_removed': True, 'vm_stopped': True, 'observation_complete': True}


class LifecycleTests(unittest.TestCase):
    def test_reconcile_mutable_disk_only_after_all_writers_gone_then_delete_exact_tree(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as parent:
            root = Path(parent).resolve() / 'owned'
            with OwnedTree(root, BINDING['vm_package_uuid'], PROCESSES) as owned:
                (root / 'disk.raw').write_bytes(b'initial public fixture')
                (root / 'source').mkdir()
                (root / 'source' / 'fixed').write_bytes(b'fixed public fixture')
                owned.seal(mutable=['disk.raw'])
                (root / 'disk.raw').write_bytes(b'changed by simulated VM')
                owned.reconcile(proof())
                result = owned.cleanup(proof())
                self.assertTrue(result['root_absent'])
                self.assertFalse(root.exists())

    def test_uncertain_writers_or_mounts_block_reconciliation_without_removal(self):
        for fault in ('writer', 'start-identity', 'mount', 'handle', 'incomplete', 'registered'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as parent:
                root = Path(parent).resolve() / 'owned'
                with OwnedTree(root, BINDING['vm_package_uuid'], PROCESSES) as owned:
                    (root / 'disk').write_bytes(b'keep')
                    owned.seal(mutable=['disk'])
                    observation = proof()
                    if fault == 'writer': observation['observed_processes'] = PROCESSES
                    if fault == 'start-identity': observation['expected_processes'] = [dict(PROCESSES[0], start_time='reused-pid')]
                    if fault == 'mount': observation['mounted_devices'] = ['fixture-disk']
                    if fault == 'handle': observation['open_handles'] = ['fixture-handle']
                    if fault == 'incomplete': observation['observation_complete'] = False
                    if fault == 'registered': observation['registration_removed'] = False
                    with self.assertRaises(ValueError): owned.reconcile(observation)
                    self.assertEqual((root / 'disk').read_bytes(), b'keep')

    def test_unknown_link_or_immutable_content_change_preserves_every_file(self):
        for fault in ('unknown', 'symlink', 'hardlink', 'fixed-change', 'after-reconcile'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as parent:
                root = Path(parent).resolve() / 'owned'
                with OwnedTree(root, BINDING['vm_package_uuid'], PROCESSES) as owned:
                    (root / 'fixed').write_bytes(b'original')
                    owned.seal()
                    if fault == 'after-reconcile': owned.reconcile(proof())
                    if fault in ('unknown', 'after-reconcile'): (root / 'unexpected').write_bytes(b'keep')
                    if fault == 'symlink': (root / 'unknown-link').symlink_to(Path(parent) / 'outside')
                    if fault == 'hardlink': os.link(root / 'fixed', Path(parent) / 'alias')
                    if fault == 'fixed-change': (root / 'fixed').write_bytes(b'changed')
                    with self.assertRaises(ValueError):
                        if fault == 'after-reconcile': owned.cleanup(proof())
                        else: owned.reconcile(proof())
                    self.assertTrue((root / 'fixed').exists())

    def test_preexisting_directory_cannot_be_adopted_for_cleanup(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as parent:
            root = Path(parent).resolve() / 'preexisting'
            root.mkdir(mode=0o700)
            (root / 'unrelated').write_text('keep')
            with self.assertRaises(FileExistsError):
                OwnedTree(root, BINDING['vm_package_uuid'], PROCESSES)
            self.assertEqual((root / 'unrelated').read_text(), 'keep')
