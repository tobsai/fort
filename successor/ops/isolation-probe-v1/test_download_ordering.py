"""Faults at persistence boundaries over the existing in-memory HTTP fixture."""
import json
import os
import unittest
from unittest import mock

import test_fetch_public as fixture

fetch = fixture.fetch


class DownloadOrdering(unittest.TestCase):
    def setUp(self):
        self.case = fixture.DownloadDiagnostics()
        self.case.setUp()
        self.addCleanup(self.case.doCleanups)

    def run_input(self, body=b'key'):
        case = self.case
        case.run_download([case.entry('ubuntu.pub', b'key'), case.entry('next', b'next')],
                          [(200, {}, body)])

    def test_mismatch_evidence_survives_interruption_before_first_evidence_write(self):
        with mock.patch.object(fetch, 'write_new', side_effect=SystemExit('synthetic abrupt stop')):
            with self.assertRaises(SystemExit):
                self.run_input(b'changed')
        self.assertEqual(1, len(self.case.calls))
        self.assertEqual(b'changed', (self.case.app / 'downloads/ubuntu.pub.partial').read_bytes())
        self.assertFalse((self.case.app / 'downloads/ubuntu.pub').exists())

    def test_target_fsync_failure_does_not_leave_apparent_acceptance(self):
        target = (self.case.app / 'downloads').stat()
        fsync = os.fsync
        injected = False

        def fail_once(descriptor):
            nonlocal injected
            info = os.fstat(descriptor)
            if not injected and (info.st_dev, info.st_ino) == (target.st_dev, target.st_ino):
                injected = True
                raise OSError('synthetic publication sync failure')
            return fsync(descriptor)

        with mock.patch.object(fetch.os, 'fsync', side_effect=fail_once):
            with self.assertRaises(RuntimeError):
                self.run_input()
        self.assertTrue(injected)
        self.assert_no_accepted_receipt_or_final_input()
        self.assertEqual('failed', self.case.receipts()[0]['outcome'])

    def test_successful_input_cleanup_failure_has_durable_failed_receipt(self):
        unlink = os.unlink

        def deny_partial(path, *args, **kwargs):
            if path == 'ubuntu.pub.partial':
                raise PermissionError('synthetic cleanup failure after valid download')
            return unlink(path, *args, **kwargs)

        with mock.patch.object(fetch.os, 'unlink', side_effect=deny_partial):
            with self.assertRaises(RuntimeError):
                self.run_input()
        self.assert_no_accepted_receipt_or_final_input()
        self.assertEqual('failed', self.case.receipts()[0]['outcome'])
        self.assertTrue(self.case.receipts()[0]['cleanup_errors'])

    def assert_no_accepted_receipt_or_final_input(self):
        self.assertEqual(1, len(self.case.calls))
        self.assertFalse((self.case.app / 'downloads/ubuntu.pub').exists())
        for path in self.case.diagnostics.glob('*.json'):
            self.assertNotEqual('accepted', json.loads(path.read_text())['outcome'])

    def test_mismatch_evidence_storage_failure_preserves_raw_response(self):
        with mock.patch.object(fetch, 'write_new', side_effect=OSError('synthetic storage failure')):
            with self.assertRaises(RuntimeError):
                self.run_input(b'changed')
        self.assert_no_accepted_receipt_or_final_input()
        self.assertEqual(b'changed', (self.case.app / 'downloads/ubuntu.pub.partial').read_bytes())

    def test_acceptance_storage_failure_quarantines_raw_response(self):
        with mock.patch.object(fetch, 'write_new', side_effect=OSError('synthetic storage failure')):
            with self.assertRaises(RuntimeError):
                self.run_input()
        self.assert_no_accepted_receipt_or_final_input()
        self.assertEqual(b'key', (self.case.app / 'downloads/ubuntu.pub.partial').read_bytes())

    def test_acceptance_file_fsync_failure_leaves_only_failed_receipts(self):
        fsync = os.fsync
        pending = self.case.diagnostics / '01-ubuntu.pub.json.pending'
        injected = False

        def fail_once(descriptor):
            nonlocal injected
            if not injected and pending.exists():
                info, expected = os.fstat(descriptor), pending.stat()
                if (info.st_dev, info.st_ino) == (expected.st_dev, expected.st_ino):
                    injected = True
                    raise OSError('synthetic acceptance file sync failure')
            return fsync(descriptor)

        with mock.patch.object(fetch.os, 'fsync', side_effect=fail_once):
            with self.assertRaises(RuntimeError):
                self.run_input()
        self.assertTrue(injected)
        self.assert_no_accepted_receipt_or_final_input()
        self.assertEqual('failed', self.case.receipts()[0]['outcome'])

    def test_acceptance_directory_fsync_failure_leaves_only_failed_receipts(self):
        fsync = os.fsync
        terminal = self.case.diagnostics / '01-ubuntu.pub.json'
        injected = False

        def fail_once(descriptor):
            nonlocal injected
            if not injected and terminal.exists():
                info, expected = os.fstat(descriptor), self.case.diagnostics.stat()
                if (info.st_dev, info.st_ino) == (expected.st_dev, expected.st_ino):
                    injected = True
                    raise OSError('synthetic acceptance directory sync failure')
            return fsync(descriptor)

        with mock.patch.object(fetch.os, 'fsync', side_effect=fail_once):
            with self.assertRaises(RuntimeError):
                self.run_input()
        self.assertTrue(injected)
        self.assert_no_accepted_receipt_or_final_input()
        self.assertEqual('failed', self.case.receipts()[0]['outcome'])

    def test_interruption_after_body_before_observation_keeps_raw_response(self):
        write_new = fetch.write_new

        def interrupt_observation(directory, name, data):
            if name.endswith('.failure.json'):
                raise SystemExit('synthetic stop before observation')
            return write_new(directory, name, data)

        with mock.patch.object(fetch, 'write_new', side_effect=interrupt_observation):
            with self.assertRaises(SystemExit):
                self.run_input(b'changed')
        self.assert_no_accepted_receipt_or_final_input()
        self.assertEqual(b'changed', (self.case.app / 'downloads/ubuntu.pub.partial').read_bytes())
        self.assertEqual(b'changed', (self.case.diagnostics / '01-ubuntu.pub.untrusted.bin').read_bytes())

    def test_interruption_before_failure_cleanup_has_durable_observation_and_body(self):
        unlink = os.unlink

        def interrupt_cleanup(path, *args, **kwargs):
            if path == 'ubuntu.pub.partial':
                raise SystemExit('synthetic stop before failure cleanup')
            return unlink(path, *args, **kwargs)

        with mock.patch.object(fetch.os, 'unlink', side_effect=interrupt_cleanup):
            with self.assertRaises(SystemExit):
                self.run_input(b'changed')
        self.assert_no_accepted_receipt_or_final_input()
        value = self.case.receipts()[0]
        self.assertEqual('failure-observed', value['phase'])
        self.assertEqual('pending', value['cleanup_status'])
        self.assertEqual(b'changed', (self.case.diagnostics / value['untrusted_body']['file']).read_bytes())
        self.assertEqual(b'changed', (self.case.app / 'downloads/ubuntu.pub.partial').read_bytes())

    def test_interruption_before_failure_terminal_record_preserves_observation(self):
        write_new = fetch.write_new

        def interrupt_terminal(directory, name, data):
            if name == '01-ubuntu.pub.json':
                raise SystemExit('synthetic stop before failure terminal record')
            return write_new(directory, name, data)

        with mock.patch.object(fetch, 'write_new', side_effect=interrupt_terminal):
            with self.assertRaises(SystemExit):
                self.run_input(b'changed')
        self.assert_no_accepted_receipt_or_final_input()
        self.assertEqual([], list((self.case.app / 'downloads').iterdir()))
        value = self.case.receipts()[0]
        self.assertEqual('failure-observed', value['phase'])
        self.assertEqual(b'changed', (self.case.diagnostics / value['untrusted_body']['file']).read_bytes())

    def test_twelfth_input_failure_keeps_two_phase_evidence_within_original_budget(self):
        entries = [self.case.entry('first-%02d' % index, b'x') for index in range(11)]
        entries.append(self.case.entry('ubuntu.pub', b'key'))
        responses = [(200, {}, b'x')] * 11 + [(200, {}, b'y' * 65537)]
        with self.assertRaises(RuntimeError):
            self.case.run_download(entries, responses)
        self.assertEqual(12, len(self.case.calls))
        self.assertEqual(13, len(list(self.case.diagnostics.glob('*.json'))))
        self.assertEqual(12, len(self.case.receipts()))
        self.assertEqual('failed', self.case.receipts()[-1]['outcome'])
        self.assertEqual(65536, self.case.receipts()[-1]['untrusted_body']['bytes'])
        self.assertLessEqual(sum(path.stat().st_size for path in self.case.diagnostics.iterdir()), 262144)
        self.assertEqual([], list(self.case.diagnostics.glob('*.pending')))


if __name__ == '__main__':
    unittest.main()
