import errno
import os
import socket
import tempfile
import unittest
from fort_probe.ipc import prove_denial


class IPCTests(unittest.TestCase):
    def test_listening_endpoint_and_explicit_denial_pass(self):
        for denied in (errno.EACCES, errno.EPERM, errno.ENOENT):
            with self.subTest(errno=denied), tempfile.TemporaryDirectory(prefix='fort-ipc-', dir='/tmp') as root:
                path = root + '/canary.sock'
                with socket.socket(socket.AF_UNIX) as listener:
                    listener.bind(path)
                    listener.listen(4)
                    result = prove_denial(path, lambda: {'connected': False, 'errno': denied}, hidden=denied == errno.ENOENT)
                    self.assertTrue(result['passed'])
                    self.assertEqual(result['before']['inode'], result['after']['inode'])
                    self.assertTrue(result['before']['connected'])
                    self.assertTrue(result['after']['connected'])

    def test_unlistening_removed_or_unreachable_socket_is_not_permission_proof(self):
        for fault in ('unlistening', 'removed', 'refused', 'timeout', 'success', 'hidden-unproven'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory(prefix='fort-ipc-', dir='/tmp') as root:
                path = root + '/canary.sock'
                with socket.socket(socket.AF_UNIX) as listener:
                    listener.bind(path)
                    if fault != 'unlistening': listener.listen(4)
                    def tool():
                        if fault == 'removed': os.unlink(path)
                        return {'connected': fault == 'success', 'errno': {
                            'refused': errno.ECONNREFUSED, 'timeout': errno.ETIMEDOUT,
                            'hidden-unproven': errno.ENOENT}.get(fault, errno.EACCES)}
                    with self.assertRaises((ValueError, OSError)):
                        prove_denial(path, tool)
