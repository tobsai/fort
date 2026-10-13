import unittest
import socket
import threading
from fort_probe.serial import Frames, Exchange


class SerialTests(unittest.TestCase):
    def test_fragmented_utf8_and_coalesced_frames_preserve_exact_messages(self):
        stream = Frames()
        self.assertEqual(stream.feed(b'{"text":"\xe2'), [])
        self.assertEqual(stream.feed(b'\x98\x83"}\n{"op":"stop"}\n'),
                         [{'text': '\u2603'}, {'op': 'stop'}])

    def test_ambiguous_or_unbounded_input_poison_the_channel(self):
        for payload in (b'{"op":"grant","op":"stop"}\n', b'{"n":NaN}\n', b'{"n":1e999}\n', b'[]\n',
                        b'\xff\n', b'{"x":"' + b'x' * 4096, b'{}\n' * 65):
            with self.subTest(payload=payload[:30]):
                stream = Frames()
                with self.assertRaises(ValueError):
                    stream.feed(payload)
                with self.assertRaises(ValueError):
                    stream.feed(b'{}\n')

    def test_pipe_exchange_delivers_one_request_and_never_resends_lost_ack(self):
        left, right = socket.socketpair()
        try:
            client = Exchange(left.fileno(), timeout=0.02)
            with self.assertRaises(TimeoutError):
                client.request({'op': 'grant'}, lambda x: x == {'event': 'accepted'})
            right.settimeout(0.1)
            self.assertEqual(right.recv(4096), b'{"op":"grant"}\n')
            with self.assertRaises(ValueError):
                client.request({'op': 'grant'}, lambda x: True)
            right.settimeout(0.02)
            with self.assertRaises(socket.timeout):
                right.recv(4096)
        finally:
            left.close()
            right.close()

    def test_channel_stops_before_sending_more_than_64_requests(self):
        left, right = socket.socketpair()
        requests = []
        errors = []
        def peer():
            try:
                reader = right.makefile('rb')
                for line in reader:
                    requests.append(line)
                    right.sendall(b'{"ok":true}\n')
            except OSError as error:
                errors.append(type(error).__name__)
        worker = threading.Thread(target=peer)
        worker.start()
        try:
            client = Exchange(left.fileno())
            for unused in range(64):
                self.assertEqual(client.request({'op': 'inspect'}, lambda x: x == {'ok': True}), {'ok': True})
            with self.assertRaises(ValueError):
                client.request({'op': 'inspect'}, lambda x: True)
        finally:
            left.shutdown(socket.SHUT_RDWR)
            left.close()
            worker.join(1)
            right.close()
        self.assertFalse(worker.is_alive())
        self.assertEqual(len(requests), 64)

    def test_unsolicited_receipt_cannot_authorize_a_new_request(self):
        left, right = socket.socketpair()
        try:
            right.sendall(b'{"event":"accepted"}\n')
            client = Exchange(left.fileno())
            with self.assertRaises(ValueError):
                client.request({'op': 'grant'}, lambda x: x == {'event': 'accepted'})
            right.settimeout(0.02)
            with self.assertRaises(socket.timeout):
                right.recv(4096)
        finally:
            left.close()
            right.close()
