"""R2 expected-behavior regressions: pure parsers, IO doubles and in-memory archives.

No live adapter is constructed. No process, listener, PTY, network request or
guest syscall is created. The command-runner class executes in a namespace in
which every OS/process/clock/selector dependency is a test double.
"""
import ast
import gzip
import hashlib
import io
import json
from pathlib import Path
import sys
import tarfile
from types import SimpleNamespace
import unittest

HERE = Path(__file__).resolve().parent
SOURCE = HERE.parent
sys.path[:0] = [str(SOURCE), str(SOURCE / 'tests')]
from fort_probe.archive import inspect_archive
from fort_probe.evidence import process, TOOL_CGROUP
from fort_probe.linux import LinuxGuest
from fort_probe.linux_kernel import decode_process
from support import BINDING, Clock
from test_archive import fixture_archive
from review_r2_support import LinuxFixture
from test_linux_kernel import proc

OBSERVATIONS = []
NAMES = {'raw': 'fixture/root.raw', 'kernel': 'fixture/Image', 'initrd': 'fixture/initrd'}


class PrefixZeros:
    """Stream a small prefix and logical zeros, without allocating a whole member."""
    def __init__(self, prefix, size):
        self.prefix, self.size, self.offset = prefix, size, 0
    def read(self, count):
        count = min(count, self.size - self.offset)
        leading = self.prefix[self.offset:self.offset + count]
        self.offset += count
        return leading + b'\0' * (count - len(leading))


class R2Regressions(unittest.TestCase):
    def test_exited_leader_does_not_skip_owned_group_cancellation(self):
        tree = ast.parse((SOURCE / 'fort_probe/bounded.py').read_bytes())
        definition = next(n for n in tree.body if isinstance(n, ast.ClassDef) and n.name == 'Commands')
        effects = []
        class Pipe:
            def fileno(self): return 9
            def close(self): effects.append('pipe-closed')
        class Process:
            pid = 7654321
            stdout, stderr = Pipe(), Pipe()
            def poll(self): return 0
            def wait(self, **kwargs): effects.append('wait')
        class Selector:
            def __enter__(self): return self
            def __exit__(self, *args): pass
            def register(self, *args): pass
            def get_map(self): return {'descendant-open-pipe': True}
        clock = iter((0, 2))
        namespace = {
            'subprocess': SimpleNamespace(Popen=lambda *a, **kw: Process(), DEVNULL=None, PIPE=None),
            'selectors': SimpleNamespace(DefaultSelector=Selector, EVENT_READ=1),
            'time': SimpleNamespace(monotonic=lambda: next(clock)),
            'os': SimpleNamespace(set_blocking=lambda *a: None, killpg=lambda *a: effects.append('group-cancelled')),
            'signal': SimpleNamespace(SIGKILL=9),
        }
        exec(compile(ast.Module(body=[definition], type_ignores=[]), 'fixed-R2/Commands', 'exec'), namespace)
        command = namespace['Commands'].__new__(namespace['Commands'])
        with self.assertRaises(TimeoutError):
            command.run(['/synthetic/command'], timeout=1)
        OBSERVATIONS.append({'case': 'leader-exited-descendant-pipe-open', 'effects': effects})
        self.assertIn('group-cancelled', effects)

    def test_global_pax_headers_cannot_bypass_the_64_entry_limit(self):
        header = tarfile.TarInfo('metadata')
        header.type, header.size = tarfile.XGLTYPE, 0
        raw = header.tobuf(format=tarfile.USTAR_FORMAT) * 65 + gzip.decompress(fixture_archive())
        data = gzip.compress(raw)
        accepted = None
        try:
            accepted = inspect_archive(io.BytesIO(data), hashlib.sha256(data).hexdigest(), NAMES)
        except ValueError:
            pass
        OBSERVATIONS.append({'case': 'hidden-tar-metadata', 'global_pax_headers': 65,
                             'selected_members': 3, 'compressed_bytes': len(data),
                             'accepted': accepted is not None})
        self.assertIsNone(accepted, 'forbidden metadata and excess physical entries must fail before preprocessing')

    def test_auxiliary_members_must_fit_the_combined_192_mib_budget(self):
        raw = bytearray(4096)
        raw[1080:1082] = b'\x53\xef'
        raw[1144:1160] = b'cloudimg-rootfs'.ljust(16, b'\0')
        kernel = bytearray(64)
        kernel[56:60] = b'ARM\x64'
        output = io.BytesIO()
        members = [('fixture/root.raw', bytes(raw), len(raw)),
                   ('fixture/Image', bytes(kernel), 1024 ** 2),
                   ('fixture/initrd', b'070701', 192 * 1024 ** 2)]
        with tarfile.open(fileobj=output, mode='w:gz') as archive:
            for name, prefix, size in members:
                member = tarfile.TarInfo(name)
                member.size = size
                archive.addfile(member, PrefixZeros(prefix, size))
        data = output.getvalue()
        accepted = None
        try:
            accepted = inspect_archive(io.BytesIO(data), hashlib.sha256(data).hexdigest(), NAMES)
        except ValueError:
            pass
        OBSERVATIONS.append({'case': 'combined-auxiliary-budget', 'kernel_bytes': 1024 ** 2,
                             'initrd_bytes': 192 * 1024 ** 2, 'raw_bytes': len(raw),
                             'compressed_bytes': len(data), 'accepted': accepted is not None})
        self.assertIsNone(accepted, 'individual member limits must not replace the combined budget')

    def test_unattributed_stdout_socket_cannot_be_proven_as_journal(self):
        status = 'Uid:\t61184 61184 61184 61184\nNoNewPrivs:\t1\n'
        status += ''.join(k + ':\t0000000000000000\n' for k in ('CapInh', 'CapPrm', 'CapEff', 'CapBnd', 'CapAmb'))
        fds = {'0': '/dev/null', '1': 'socket:[424242]', '2': 'socket:[999999]'}
        accepted = None
        try:
            facts = decode_process(proc(), status, '0::' + TOOL_CGROUP + '\n',
                                   {'net': 1, 'mnt': 2, 'ipc': 3}, fds, proc())
            process(facts, TOOL_CGROUP)
            accepted = facts
        except ValueError:
            pass
        OBSERVATIONS.append({'case': 'unattributed-sockets', 'descriptors': fds,
                             'journal_peer_evidence_supplied': False, 'accepted': accepted})
        self.assertIsNone(accepted, 'socket inode syntax alone is not journal endpoint evidence')

    def test_prepared_runtime_requires_its_own_serial_denial_evidence(self):
        kernel, clock = LinuxFixture(), Clock()
        guest = LinuxGuest(kernel, clock)
        identity = guest.prepare(BINDING)
        accepted = None
        try:
            accepted = guest.prepared_controls(identity)
        except ValueError:
            pass
        OBSERVATIONS.append({'case': 'missing-runtime-C05', 'runtime_report': kernel.runtime_report(),
                             'accepted': accepted is not None})
        self.assertIsNone(accepted, 'provider readiness without runtime serial denial must not authorize grant')
