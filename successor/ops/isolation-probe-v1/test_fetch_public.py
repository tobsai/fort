"""Downloader behavior over an in-memory HTTP transport: no socket or public fetch."""
import contextlib
import hashlib
import importlib.util
import io
import json
import os
import pathlib
import tempfile
import unittest
import urllib.request
import urllib.response
from email.message import Message
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location('fetch_under_test', ROOT / 'fetch-public.py')
fetch = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(fetch)


class DownloadDiagnostics(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='fort-download-test-')
        self.addCleanup(self.temporary.cleanup)
        self.root = pathlib.Path(self.temporary.name).resolve()
        self.app = self.root / 'app'
        self.app.mkdir(mode=0o700)
        (self.app / 'downloads').mkdir(mode=0o700)
        self.diagnostics = self.app / 'evidence/download-diagnostics'
        self.calls = []
        self.response_hook = lambda: None
        self.stderr = io.StringIO()

    def entry(self, name, body, cap=65536):
        return {'name': name, 'url': 'https://keyserver.ubuntu.com/' + name,
                'sha256': hashlib.sha256(body).hexdigest(), 'max_bytes': cap,
                'kind': 'public-verification-metadata'}

    def run_download(self, entries, responses):
        scope = {'app_root': str(self.app), 'host_https_domains': ['keyserver.ubuntu.com'], 'downloads': entries}
        (self.root / 'scope.json').write_text(json.dumps(scope))
        calls = self.calls
        hook = self.response_hook

        class FixtureHTTPS(urllib.request.HTTPSHandler):
            def https_open(self, request):
                calls.append(request.full_url)
                hook()
                status, fields, body = responses[len(calls) - 1]
                headers = Message()
                for name, value in fields.items():
                    headers[name] = value
                response = urllib.response.addinfourl(io.BytesIO(body), headers, request.full_url, status)
                response.msg = 'Fixture response'
                return response

        with mock.patch.object(fetch, 'ROOT', self.root), \
             mock.patch.object(fetch.urllib.request, 'HTTPSHandler', FixtureHTTPS), \
             contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(self.stderr):
            fetch.main()

    def receipts(self):
        return [json.loads(path.read_text()) for path in sorted(self.diagnostics.glob('*.json'))]

    def test_mismatch_retains_observed_metadata_and_untrusted_body_without_advancing(self):
        original = b'-----BEGIN PGP PUBLIC KEY BLOCK-----\nfixture\n'
        changed = original.replace(b'\n', b'\r\n')
        entries = [self.entry('ubuntu.pub', original), self.entry('next', b'next')]
        with self.assertRaises(RuntimeError):
            self.run_download(entries, [(200, {'Content-Type': 'application/pgp-keys',
                                               'Content-Length': str(len(changed))}, changed)])
        self.assertEqual([entries[0]['url']], self.calls)
        self.assertEqual([], list((self.app / 'downloads').iterdir()))
        receipts = self.receipts()
        self.assertEqual(1, len(receipts))
        receipt = receipts[0]
        self.assertEqual('failed', receipt['outcome'])
        self.assertEqual('pinned-checksum-mismatch', receipt['failure'])
        self.assertEqual(entries[0]['sha256'], receipt['expected_digest'])
        self.assertEqual(hashlib.sha256(changed).hexdigest(), receipt['observed_digest'])
        self.assertEqual(len(changed), receipt['observed_bytes'])
        self.assertEqual(200, receipt['http']['status'])
        self.assertEqual('application/pgp-keys', receipt['http']['content_type'])
        body = self.diagnostics / receipt['untrusted_body']['file']
        self.assertEqual(changed, body.read_bytes())
        self.assertEqual(0o600, body.stat().st_mode & 0o777)
        self.assertFalse(receipt['publisher_signature_verified'])

    def test_exact_success_has_receipt_and_accepted_file(self):
        entry = self.entry('ubuntu.pub', b'public fixture')
        self.run_download([entry], [(200, {'Content-Type': 'application/pgp-keys'}, b'public fixture')])
        self.assertEqual([entry['url']], self.calls)
        self.assertEqual(b'public fixture', (self.app / 'downloads/ubuntu.pub').read_bytes())
        self.assertFalse((self.app / 'downloads/ubuntu.pub.partial').exists())
        self.assertEqual('accepted', self.receipts()[0]['outcome'])
        self.assertTrue(self.receipts()[0]['body_complete'])
        self.assertIsNone(self.receipts()[0]['untrusted_body'])

    def test_later_failure_preserves_earlier_receipt_and_stops(self):
        entries = [self.entry('first', b'one'), self.entry('ubuntu.pub', b'two'), self.entry('third', b'three')]
        with self.assertRaises(RuntimeError):
            self.run_download(entries, [(200, {}, b'one'), (200, {}, b'changed')])
        self.assertEqual(2, len(self.calls))
        self.assertEqual(['accepted', 'failed'], [item['outcome'] for item in self.receipts()])
        self.assertEqual(['first'], [path.name for path in (self.app / 'downloads').iterdir()])

    def test_http_200_html_is_preserved_as_untrusted_failure(self):
        html = b'<html><script>fixture only</script>unavailable</html>'
        with self.assertRaises(RuntimeError):
            self.run_download([self.entry('ubuntu.pub', b'key')], [(200, {'Content-Type': 'text/html'}, html)])
        value = self.receipts()[0]
        self.assertEqual('pinned-checksum-mismatch', value['failure'])
        self.assertEqual('text/html', value['http']['content_type'])
        self.assertEqual(html, (self.diagnostics / value['untrusted_body']['file']).read_bytes())

    def test_truncated_body_records_observed_digest_and_incomplete_status(self):
        with self.assertRaises(RuntimeError):
            self.run_download([self.entry('ubuntu.pub', b'complete')], [(200, {'Content-Length': '8'}, b'part')])
        value = self.receipts()[0]
        self.assertEqual('content-length-mismatch', value['failure'])
        self.assertEqual(4, value['observed_bytes'])
        self.assertEqual(hashlib.sha256(b'part').hexdigest(), value['observed_digest'])
        self.assertFalse(value['body_complete'])

    def test_disallowed_redirect_is_not_followed_and_keeps_response_status(self):
        with self.assertRaises(RuntimeError):
            self.run_download([self.entry('ubuntu.pub', b'key')], [
                (302, {'Content-Type': 'text/html', 'Location': 'https://unapproved.invalid/key'}, b'')])
        self.assertEqual(1, len(self.calls))
        self.assertEqual('unlisted-download-destination', self.receipts()[0]['failure'])
        self.assertEqual(302, self.receipts()[0]['http']['status'])

    def test_allowed_redirect_query_is_not_retained_in_evidence(self):
        self.run_download([self.entry('ubuntu.pub', b'key')], [
            (302, {'Location': 'https://keyserver.ubuntu.com/redirected?token=synthetic-only'}, b''),
            (200, {'Content-Type': 'application/pgp-keys'}, b'key')])
        value = self.receipts()[0]
        self.assertEqual('https://keyserver.ubuntu.com/redirected', value['http']['final_url'])
        self.assertTrue(value['http']['final_url_query_omitted'])
        self.assertNotIn('synthetic-only', json.dumps(value))

    def test_streaming_overflow_never_retains_more_than_the_input_cap(self):
        with self.assertRaises(RuntimeError):
            self.run_download([self.entry('ubuntu.pub', b'key', cap=64)], [(200, {}, b'x' * 200)])
        value = self.receipts()[0]
        self.assertEqual('streaming-byte-cap-exceeded', value['failure'])
        self.assertEqual(65, value['observed_bytes'])
        self.assertFalse(value['body_complete'])
        self.assertEqual(64, (self.diagnostics / value['untrusted_body']['file']).stat().st_size)
        self.assertEqual([], list((self.app / 'downloads').iterdir()))

    def test_declared_overflow_is_rejected_before_body_reads(self):
        with self.assertRaises(RuntimeError):
            self.run_download([self.entry('ubuntu.pub', b'key', cap=64)], [(200, {'Content-Length': '200'}, b'x' * 200)])
        value = self.receipts()[0]
        self.assertEqual('declared-byte-cap-exceeded', value['failure'])
        self.assertEqual(0, value['observed_bytes'])
        self.assertIsNone(value['untrusted_body'])

    def test_http_error_records_status_type_and_bounded_public_body(self):
        with self.assertRaises(RuntimeError):
            self.run_download([self.entry('ubuntu.pub', b'key')], [(503, {'Content-Type': 'text/plain'}, b'unavailable')])
        value = self.receipts()[0]
        self.assertEqual('unexpected-http-status', value['failure'])
        self.assertEqual(503, value['http']['status'])
        self.assertEqual(b'unavailable', (self.diagnostics / value['untrusted_body']['file']).read_bytes())

    def test_non_key_failure_retains_metadata_without_payload_copy(self):
        entry = self.entry('archive.tgz', b'archive')
        entry['kind'] = 'executable-archive'
        with self.assertRaises(RuntimeError):
            self.run_download([entry], [(200, {}, b'changed')])
        self.assertIsNone(self.receipts()[0]['untrusted_body'])
        self.assertEqual([], list(self.diagnostics.glob('*.bin')))

    def test_sha512_pin_still_applies(self):
        entry = self.entry('verifier.tgz', b'archive')
        del entry['sha256']
        entry['sha512'] = hashlib.sha512(b'archive').hexdigest()
        self.run_download([entry], [(200, {}, b'archive')])
        self.assertEqual('sha512', self.receipts()[0]['algorithm'])
        self.assertEqual(entry['sha512'], self.receipts()[0]['observed_digest'])

    def test_existing_download_or_partial_is_never_replaced(self):
        for name in ('ubuntu.pub', 'ubuntu.pub.partial'):
            with self.subTest(name=name):
                path = self.app / 'downloads' / name
                path.write_bytes(b'existing')
                with self.assertRaises(RuntimeError):
                    self.run_download([self.entry('ubuntu.pub', b'key')], [])
                self.assertEqual(b'existing', path.read_bytes())
                path.unlink()
        self.assertEqual([], self.calls)

    def test_existing_diagnostic_directory_blocks_before_network(self):
        self.diagnostics.mkdir(parents=True, mode=0o700)
        (self.diagnostics.parent).chmod(0o700)
        sentinel = self.diagnostics / 'existing.json'
        sentinel.write_bytes(b'existing')
        with self.assertRaises(FileExistsError):
            self.run_download([self.entry('ubuntu.pub', b'key')], [])
        self.assertEqual(b'existing', sentinel.read_bytes())
        self.assertEqual([], self.calls)

    def test_evidence_parent_symlink_is_never_followed(self):
        outside = self.root / 'outside'
        outside.mkdir(mode=0o700)
        (self.app / 'evidence').symlink_to(outside, target_is_directory=True)
        with self.assertRaises(OSError):
            self.run_download([self.entry('ubuntu.pub', b'key')], [])
        self.assertEqual([], list(outside.iterdir()))
        self.assertEqual([], self.calls)

    def test_receipt_write_failure_removes_promoted_input_and_never_advances(self):
        outside = self.root / 'existing-receipt'
        outside.write_bytes(b'existing')
        self.response_hook = lambda: (self.diagnostics / '01-ubuntu.pub.json').symlink_to(outside)
        with self.assertRaisesRegex(RuntimeError, 'diagnostic write failed'):
            self.run_download([self.entry('ubuntu.pub', b'key'), self.entry('next', b'next')], [(200, {}, b'key')])
        self.assertEqual(1, len(self.calls))
        self.assertEqual([], list((self.app / 'downloads').iterdir()))
        self.assertEqual(b'existing', outside.read_bytes())
        self.assertIn('"outcome": "failed"', self.stderr.getvalue())

    def test_large_public_key_failure_prefix_is_at_most_64_kib(self):
        with self.assertRaises(RuntimeError):
            self.run_download([self.entry('ubuntu.pub', b'key')], [(200, {}, b'x' * 70000)])
        value = self.receipts()[0]
        self.assertEqual(65536, value['untrusted_body']['bytes'])
        self.assertTrue(value['untrusted_body']['prefix_only'])
        self.assertLessEqual(sum(path.stat().st_size for path in self.diagnostics.iterdir()), 262144)

    def test_cleanup_failure_does_not_discard_mismatch_evidence_or_continue(self):
        unlink = os.unlink

        def deny_partial(path, *args, **kwargs):
            if path == 'ubuntu.pub.partial':
                raise PermissionError('synthetic cleanup failure')
            return unlink(path, *args, **kwargs)

        with mock.patch.object(fetch.os, 'unlink', side_effect=deny_partial):
            with self.assertRaises(RuntimeError):
                self.run_download([self.entry('ubuntu.pub', b'key'), self.entry('next', b'next')], [(200, {}, b'changed')])
        self.assertEqual(1, len(self.calls))
        value = self.receipts()[0]
        self.assertEqual('pinned-checksum-mismatch', value['failure'])
        self.assertEqual(hashlib.sha256(b'changed').hexdigest(), value['observed_digest'])
        self.assertTrue(value['cleanup_errors'])
        self.assertTrue((self.app / 'downloads/ubuntu.pub.partial').exists())
        self.assertFalse((self.app / 'downloads/ubuntu.pub').exists())


if __name__ == '__main__':
    unittest.main()
