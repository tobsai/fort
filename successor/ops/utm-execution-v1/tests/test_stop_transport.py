import socket
import threading
import time
import unittest
from fort_probe.serial import Exchange, encode
from support import BINDING


class StopTransportTests(unittest.TestCase):
    def exchange(self, fault=None):
        left, right = socket.socketpair()
        self.addCleanup(left.close)
        self.addCleanup(right.close)
        identity = dict(BINDING, guest_boot_id='fixture', invocation_id='1' * 32,
                        runtime_cgroup='/fort.slice/fort-attempt.slice/fort-runtime.service')
        stop = {'op': 'stop', 'sequence': 21, 'identity': identity}
        received = []
        def peer():
            received.append(right.recv(4096))
            ack = {'event': 'stopping', 'sequence': 21, 'identity': identity}
            terminal = {'event': 'owned-attempt-empty', 'sequence': 21, 'identity': identity,
                        'stop_elapsed_seconds': .02, 'live_evidence_proven': True}
            if fault == 'mismatch': terminal['sequence'] = 22
            if fault == 'coalesced':
                right.sendall(encode(ack) + encode(terminal))
                return
            right.sendall(encode(ack))
            if fault == 'missing': return
            time.sleep(.15 if fault == 'late' else .02)
            right.sendall(encode(terminal))
            if fault == 'duplicate': right.sendall(encode(terminal))
        worker = threading.Thread(target=peer)
        worker.start()
        channel = Exchange(left.fileno(), timeout=.05)
        return channel, stop, received, worker, right

    def test_two_stop_phases_accept_delayed_or_coalesced_proof_and_send_once(self):
        for fault in (None, 'coalesced'):
            with self.subTest(fault=fault):
                channel, stop, received, worker, right = self.exchange(fault)
                terminal = channel.stop(stop, terminal_timeout=.1)
                worker.join()
                self.assertEqual(terminal['event'], 'owned-attempt-empty')
                self.assertEqual(received, [encode(stop)])
                with self.assertRaises(ValueError): channel.request({'op': 'grant'}, lambda r: True)

    def test_missing_wrong_late_duplicate_terminal_quarantines_without_resend(self):
        for fault in ('missing', 'mismatch', 'late', 'duplicate'):
            with self.subTest(fault=fault):
                channel, stop, received, worker, right = self.exchange(fault)
                with self.assertRaises((TimeoutError, ValueError)):
                    channel.stop(stop, terminal_timeout=.1)
                worker.join()
                with self.assertRaises(ValueError): channel.stop(stop, terminal_timeout=.1)
                right.settimeout(.01)
                with self.assertRaises(socket.timeout): right.recv(4096)
                self.assertEqual(received, [encode(stop)])
                self.assertEqual(channel.outcome, 'quarantined-unknown')
