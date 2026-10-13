"""Integrated fixture only: a socketpair joins the real codec and controller.

GuestFixture supplies all Linux facts. This cannot prove guest confinement.
"""
import json
from pathlib import Path
import socket
import tempfile
import threading
import unittest
from fort_probe.controller import Controller
from fort_probe.lifecycle import OwnedTree
from fort_probe.seed import preview
from fort_probe.serial import Exchange, Frames, encode
from support import BINDING, Clock, GuestFixture
from test_lifecycle import PROCESSES, proof


class SessionTests(unittest.TestCase):
    def test_seed_control_live_evidence_stop_and_exact_cleanup_in_one_fixture(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as parent:
            root = Path(parent).resolve() / 'owned'
            owned = OwnedTree(root, BINDING['vm_package_uuid'], PROCESSES)
            self.addCleanup(owned.close)
            seed = root / 'seed-preview'
            seed.mkdir()
            for name, data in preview().items(): (seed / name).write_bytes(data)
            state = root / 'controller'
            state.mkdir(mode=0o700)
            left, right = socket.socketpair()
            guest, clock, errors = GuestFixture(), Clock(), []
            def broker_fixture():
                try:
                    with Controller(state, BINDING, guest, clock) as controller:
                        frames = Frames()
                        while True:
                            chunk = right.recv(4096)
                            if not chunk: break
                            for message in frames.feed(chunk):
                                receipt = controller.handle(message)
                                if message['op'] == 'heartbeat' and message['sequence'] == 20:
                                    controller.capture_live()
                                right.sendall(encode(receipt))
                                if message['op'] == 'stop':
                                    right.sendall(encode(controller.poll_stop()))
                                else: clock.advance(.25)
                except Exception as error:
                    errors.append(repr(error))
            worker = threading.Thread(target=broker_fixture)
            worker.start()
            try:
                channel = Exchange(left.fileno())
                prepared = channel.request({'op': 'prepare', 'sequence': 0}, lambda r: r.get('event') == 'prepared')
                identity = prepared['identity']
                for seq in range(1, 21):
                    message = {'op': 'grant' if seq == 1 else 'heartbeat', 'sequence': seq, 'identity': identity}
                    channel.request(message, lambda r: r == {'event': 'accepted', 'sequence': seq, 'identity': identity})
                terminal = channel.stop({'op': 'stop', 'sequence': 21, 'identity': identity}, terminal_timeout=.1)
                self.assertTrue(terminal['live_evidence_proven'])
                self.assertTrue(guest.current()['empty'])
                snapshot = json.loads((state / 'live.json').read_bytes())
                self.assertEqual(snapshot['identity'], identity)
                self.assertEqual(snapshot['sequence'], 20)
                self.assertEqual(len(snapshot['processes']), 3)
            finally:
                left.shutdown(socket.SHUT_RDWR)
                worker.join(2)
                left.close()
                right.close()
            self.assertFalse(worker.is_alive())
            self.assertEqual(errors, [])
            owned.seal()
            owned.reconcile(proof())
            self.assertTrue(owned.cleanup(proof())['root_absent'])
