from copy import deepcopy
import stat
import unittest
from fort_probe.pty import OwnedPTY

FACT = {'device': 3, 'inode': 1234, 'rdev': 5678, 'uid': 501, 'mode': 0o600, 'kind': stat.S_IFCHR}


class DeviceFixture:
    """PTY kernel boundary double; no real /dev entry is touched."""
    def __init__(self, fault=None):
        self.fact = deepcopy(FACT)
        self.fault = fault
        self.opened = self.closed = self.restored = self.raw_set = False
    def lstat(self, path):
        result = deepcopy(self.fact)
        if self.fault == 'symlink': result['kind'] = stat.S_IFLNK
        if self.fault == 'owner': result['uid'] = 502
        if self.fault == 'mode': result['mode'] = 0o620
        return result
    def open(self, path): self.opened = True; return 7
    def fstat(self, fd):
        result = deepcopy(self.fact)
        if self.fault == 'swap': result['rdev'] += 1
        return result
    def exclusive(self, fd):
        if self.fault == 'busy': raise OSError('fixture competitor')
    def settings(self, fd): return ['fixture-settings']
    def raw(self, fd): self.raw_set = True
    def restore(self, fd, settings): self.restored = True
    def close(self, fd): self.closed = True


class PtyTests(unittest.TestCase):
    def test_owned_device_is_exclusive_and_settings_restored_before_close(self):
        device = DeviceFixture()
        with OwnedPTY('/dev/ttys999', FACT, device) as pty:
            self.assertEqual(pty.fd, 7)
            self.assertTrue(device.raw_set)
        self.assertTrue(device.restored)
        self.assertTrue(device.closed)

    def test_symlink_owner_mode_rdev_swap_or_competing_consumer_stops_before_raw(self):
        for fault in ('symlink', 'owner', 'mode', 'swap', 'busy'):
            with self.subTest(fault=fault):
                device = DeviceFixture(fault)
                with self.assertRaises((ValueError, OSError)): OwnedPTY('/dev/ttys999', FACT, device)
                self.assertFalse(device.raw_set)
                if device.opened: self.assertTrue(device.closed)

    def test_replaced_device_is_closed_without_restoring_settings_on_wrong_identity(self):
        device = DeviceFixture()
        pty = OwnedPTY('/dev/ttys999', FACT, device)
        device.fact['inode'] += 1
        with self.assertRaises(ValueError): pty.close()
        self.assertFalse(device.restored)
        self.assertTrue(device.closed)
