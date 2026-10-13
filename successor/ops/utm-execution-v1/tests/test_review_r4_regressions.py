"""Expected-behavior R4 regressions; fixed snapshot, fixtures only.

No network, actual supervised child, live adapter, guest or downloaded module.
The FIFO fixture runs one local thread and always connects its own cleanup peer.
"""
import ast
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
import threading
from types import SimpleNamespace
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
SOURCE = HERE.parent
sys.path.insert(0, str(SOURCE))
from fort_probe.acquisition import LocalStore
from fort_probe.boot import verify_boot_files
from fort_probe.inspection_workers import PROFILES, WorkerOutput

observations = {'commit': 'd379e6fe6ffadecc32e3282226d0a7859c9a9495'}


def boot_files(data):
    data.mkdir()
    manifest = {}
    for name in ('root.raw', 'kernel.Image', 'initrd', 'seed.iso'):
        content = b'approved'
        (data / name).write_bytes(content)
        manifest['Data/' + name] = {'bytes': len(content), 'sha256': hashlib.sha256(content).hexdigest()}
    return manifest


class R4Regressions(unittest.TestCase):
    def test_acquisition_rejects_symlinked_ancestor_before_creating_state(self):
        with tempfile.TemporaryDirectory(dir=HERE) as temp:
            root = Path(temp).resolve()
            (root / 'target').mkdir()
            (root / 'alias').symlink_to(root / 'target', target_is_directory=True)
            rejected = False
            store = None
            try:
                store = LocalStore(root / 'alias' / 'attempt')
            except (ValueError, OSError):
                rejected = True
            finally:
                if store is not None:
                    store.close()
            redirected = (root / 'target/attempt/attempt.json').exists()
            observations['ancestor'] = {'rejected': rejected, 'redirected_state_created': redirected}
            self.assertTrue(rejected and not redirected, 'symlinked ancestor must fail before creating state outside the declared path')

    def test_prestart_rejects_replaced_data_path(self):
        with tempfile.TemporaryDirectory(dir=HERE) as temp:
            package = Path(temp).resolve()
            data = package / 'Data'
            manifest = boot_files(data)
            real_read = os.read
            swapped = False
            def read(fd, size):
                nonlocal swapped
                if not swapped:
                    swapped = True
                    data.rename(package / 'Detached-Data')
                    data.mkdir()
                    for name in ('root.raw', 'kernel.Image', 'initrd', 'seed.iso'):
                        (data / name).write_bytes(b'replaced')
                return real_read(fd, size)
            rejected = False
            result = None
            with patch.object(os, 'read', read):
                try:
                    result = verify_boot_files(data, manifest)
                except (ValueError, OSError):
                    rejected = True
            observations['detached_data'] = {'rejected': rejected, 'accepted_original_manifest': result == manifest,
                                             'launch_kernel': (data / 'kernel.Image').read_text()}
            self.assertTrue(rejected, 'checked directory must remain bound to the actual launch pathname')

    def test_prestart_rejects_fifo_without_waiting_for_a_writer(self):
        with tempfile.TemporaryDirectory(dir=HERE) as temp:
            data = Path(temp).resolve() / 'Data'
            manifest = boot_files(data)
            fifo = data / 'root.raw'
            fifo.unlink()
            os.mkfifo(fifo, 0o600)
            opened = threading.Event()
            done = threading.Event()
            errors = []
            real_open = os.open
            def observed_open(path, flags, *args, **kwargs):
                if path == 'root.raw':
                    opened.set()
                return real_open(path, flags, *args, **kwargs)
            def verify():
                try:
                    verify_boot_files(data, manifest)
                except Exception as error:
                    errors.append(type(error).__name__)
                finally:
                    done.set()
            with patch.object(os, 'open', observed_open):
                thread = threading.Thread(target=verify, daemon=True)
                thread.start()
                self.assertTrue(opened.wait(1), 'fixture must reach the expected leaf')
                rejected_without_peer = done.wait(.05)
                # Always release a blocked reader through a nonblocking local peer.
                cleanup_peer = real_open(fifo, os.O_RDWR | os.O_NONBLOCK | os.O_NOFOLLOW)
                try:
                    thread.join(1)
                finally:
                    os.close(cleanup_peer)
                self.assertFalse(thread.is_alive(), 'fixture must leave no blocked reader')
            observations['fifo'] = {'rejected_without_peer': rejected_without_peer,
                                    'error_after_peer_connection': errors, 'reader_joined': not thread.is_alive()}
            self.assertTrue(rejected_without_peer and errors, 'a FIFO must be rejected before a writer connects')

    def test_pipe_eof_does_not_end_worker_resource_supervision(self):
        clock = [0.0]
        effects = []
        class Pipe:
            def __init__(self, fd): self.fd = fd
            def fileno(self): return self.fd
            def close(self): effects.append('pipe_closed')
        class Child:
            pid = 42
            returncode = 0
            stdout, stderr = Pipe(10), Pipe(11)
            def wait(self, **kwargs): effects.append('reaped')
            def poll(self): raise AssertionError('do not reap before cancellation')
        class Selector:
            def __init__(self): self.streams = {}
            def __enter__(self): return self
            def __exit__(self, *args): pass
            def register(self, stream, event, index): self.streams[stream.fd] = SimpleNamespace(fileobj=stream, data=index)
            def unregister(self, stream): del self.streams[stream.fd]
            def get_map(self): return self.streams
            def select(self, *args): return [(key, 1) for key in list(self.streams.values())]
        class Commands:
            def run(self, *args, **kwargs):
                effects.append('rss_sample')
                return b'1', b''
            def _wait_unreaped(self, child, deadline):
                effects.append('unmonitored_wait')
                clock[0] = 650.0
        names = ['input' + str(i) for i in range(8)]
        terminal = {'operation': 'acquire', 'status': 'complete', 'details': {'completed': 8}}
        wire = (''.join(json.dumps({'progress_input': n}) + '\n' for n in names) + json.dumps(terminal) + '\n').encode()
        reads = {10: [wire, b''], 11: [b'']}
        pin = {'bytes': 1, 'sha256': hashlib.sha256(b'x').hexdigest()}
        namespace = {
            'WorkerOutput': WorkerOutput, 'PROFILES': PROFILES, 'verify_source': lambda value: None,
            'worker_command': lambda *args, **kwargs: ['/fixture-worker'], 'SOURCE': '/fixture-source',
            'INPUTS': SimpleNamespace(read_bytes=lambda: json.dumps({'downloads': [{'name': n} for n in names],
                'existing_host_tools': [dict(pin, path='/fixture-worker'), dict(pin, path='/bin/ps')]}).encode()),
            'Path': lambda path: SimpleNamespace(read_bytes=lambda: b'x'), 'hashlib': hashlib, 'json': json,
            'subprocess': SimpleNamespace(Popen=lambda *args, **kwargs: Child(), DEVNULL=None, PIPE=None),
            'time': SimpleNamespace(monotonic=lambda: clock[0]),
            'selectors': SimpleNamespace(DefaultSelector=Selector, EVENT_READ=1),
            'os': SimpleNamespace(set_blocking=lambda *args: None, read=lambda fd, count: reads[fd].pop(0),
                                  killpg=lambda *args: effects.append('cancelled')),
            'signal': SimpleNamespace(SIGKILL=9), 'rss_bytes': lambda raw: 1024,
        }
        tree = ast.parse((SOURCE / 'fort_probe/inspection_workers.py').read_bytes())
        worker_class = next(n for n in tree.body if isinstance(n, ast.ClassDef) and n.name == 'Worker')
        exec(compile(ast.Module(body=[worker_class], type_ignores=[]), 'fixed-R4-worker', 'exec'), namespace)
        worker = namespace['Worker'].__new__(namespace['Worker'])
        worker.accepted_source_sha256 = 'a' * 64
        worker.commands = Commands()
        rejected = False
        result = None
        try:
            result = worker.run('acquire', Path('/fixture-root'))
        except (ValueError, TimeoutError):
            rejected = True
        observations['pipe_eof'] = {'rejected': rejected, 'at_seconds': clock[0], 'input_deadline_seconds': 600,
                                     'accepted_result': result, 'effects': effects}
        self.assertTrue(rejected, 'EOF must not disable deadline/RSS enforcement while the child remains live')
