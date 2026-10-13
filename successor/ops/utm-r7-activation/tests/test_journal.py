"""Private temporary fixture files only; no actual smoke/output/operation root."""
import os
from pathlib import Path
import sys
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import packet_support


class JournalTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='fort-r7-journal-fixture-', dir='/private/tmp')
        self.path = Path(self.temp.name) / 'smoke'
        self.context = SimpleNamespace(packet_sha='a' * 64, active_sha='b' * 64,
            manifest={'smoke': {'output_directory': str(self.path)}})
        self.guard = patch.object(packet_support, 'require_active')
        self.guard.start()
    def tearDown(self): self.guard.stop(); self.temp.cleanup()

    def test_exclusive_root_and_records_survive_exact_identity_reopen(self):
        journal = packet_support.Journal(self.context, create=True)
        try:
            journal.record('fixture.json', {'value': 1})
            self.assertEqual(journal.read('fixture.json'), {'value': 1})
            with self.assertRaises(FileExistsError): journal.record('fixture.json', {'value': 2})
            with self.assertRaises(FileExistsError): packet_support.Journal(self.context, create=True)
        finally: journal.close()
        reopened = packet_support.Journal(self.context)
        try: self.assertEqual(reopened.read('fixture.json'), {'value': 1})
        finally: reopened.close()

    def test_link_foreign_binding_mode_and_budget_fail(self):
        journal = packet_support.Journal(self.context, create=True)
        try:
            with self.assertRaises(ValueError): journal.record('large.json', {'data': 'x' * 16384})
            for index in range(4): journal.record('part-' + str(index) + '.json', {'data': 'x' * 16000})
            with self.assertRaises(ValueError): journal.record('overflow.json', {'data': 'x' * 2000})
            os.symlink('part-0.json', self.path / 'link.json')
            with self.assertRaises(OSError): journal.read('link.json')
            with self.assertRaises(ValueError): journal.record('after-link.json', {})
            os.unlink(self.path / 'link.json')
            os.chmod(self.path / 'part-0.json', 0o644)
            with self.assertRaises(ValueError): journal.read('part-0.json')
        finally: journal.close()
        foreign = SimpleNamespace(packet_sha='c' * 64, active_sha=self.context.active_sha, manifest=self.context.manifest)
        with self.assertRaises(ValueError): packet_support.Journal(foreign)


if __name__ == '__main__': unittest.main(verbosity=2)
