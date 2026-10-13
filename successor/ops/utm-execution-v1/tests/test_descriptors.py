from copy import deepcopy
import unittest
from fort_probe.linux_kernel import decode_process
from fort_probe.evidence import process,TOOL_CGROUP
from test_linux_kernel import proc


def journal_proofs():
    return {'1':{'inode':2,'family':1,'type':1,'peer_path':'/run/systemd/journal/stdout',
        'peer_uid':0,'peer_gid':0,'peer_pid':1,'peer_start_ticks':10,
        'peer_executable':'/usr/lib/systemd/systemd','peer_cgroup':'/init.scope',
        'path_uid':0,'path_is_socket':True},
        '2':{'inode':2,'family':1,'type':1,'peer_path':'/run/systemd/journal/stdout',
        'peer_uid':0,'peer_gid':0,'peer_pid':1,'peer_start_ticks':10,
        'peer_executable':'/usr/lib/systemd/systemd','peer_cgroup':'/init.scope',
        'path_uid':0,'path_is_socket':True}}

class DescriptorTests(unittest.TestCase):
    def test_only_independently_attributed_journal_connections_pass(self):
        status='Uid:\t61184 61184 61184 61184\nNoNewPrivs:\t1\n'+''.join(k+':\t0000000000000000\n' for k in ('CapInh','CapPrm','CapEff','CapBnd','CapAmb'))
        args=[proc(),status,'0::'+TOOL_CGROUP+'\n',{'net':1,'mnt':2,'ipc':3},{'0':'/dev/null','1':'socket:[2]','2':'socket:[2]'},proc()]
        facts=decode_process(*args,journal=journal_proofs());process(facts,TOOL_CGROUP)
        self.assertEqual(facts['fd_classes'],['stdio','journal'])
        for fault in ('missing','management','inode','peer_uid','peer_pid','peer_executable','peer_cgroup','path_uid','path_is_socket','type'):
            proof=deepcopy(journal_proofs())
            if fault=='missing':proof.pop('2')
            elif fault=='management':proof['1']['peer_path']='/run/fort-probe/session/root.sock'
            elif fault in ('inode','peer_uid','peer_pid','path_uid','type'):proof['1'][fault]=9999
            elif fault=='path_is_socket':proof['1'][fault]=False
            else:proof['1'][fault]='unattributed'
            with self.subTest(fault=fault),self.assertRaises(ValueError):decode_process(*args,journal=proof)
        args[4]={'0':'/dev/null','1':'/dev/null','2':'/dev/null'}
        self.assertEqual(decode_process(*args)['fd_classes'],['stdio'])

    def test_pidfd_collector_uses_kernel_socket_facts_and_closes_duplicates(self):
        import ast
        from pathlib import Path
        import re
        import socket
        import stat
        import struct
        from types import SimpleNamespace
        from fort_probe.descriptors import validate_journal,JOURNAL
        from fort_probe.linux import parse_stat
        effects=[]
        class Handle:
            family=socket.AF_UNIX
            def __enter__(self):return self
            def __exit__(self,*args):effects.append('duplicate-closed')
            def fileno(self):return 11
            def getsockopt(self,level,option,*args):
                return struct.pack('3i',1,0,0) if option==17 else int(socket.SOCK_STREAM)
            def getpeername(self):return JOURNAL
        class Syscall:
            def __call__(self,*args):effects.append('pidfd_getfd');return 11
        syscalls=Syscall()
        ns={'require_live':lambda:None,'platform':SimpleNamespace(system=lambda:'Linux',machine=lambda:'aarch64'),
            'os':SimpleNamespace(pidfd_open=lambda *a:10,close=lambda fd:effects.append(('closed',fd)),
                fstat=lambda fd:SimpleNamespace(st_mode=stat.S_IFSOCK,st_ino=2),
                lstat=lambda p:SimpleNamespace(st_mode=stat.S_IFSOCK,st_uid=0),
                readlink=lambda p:'/usr/lib/systemd/systemd'),
            'ctypes':SimpleNamespace(CDLL=lambda *a,**kw:SimpleNamespace(syscall=syscalls),c_long=int,c_int=int,c_uint=int),
            'socket':SimpleNamespace(socket=lambda **kw:Handle(),SOL_SOCKET=socket.SOL_SOCKET,SO_TYPE=socket.SO_TYPE,SO_PEERCRED=17),
            're':re,'stat':stat,'struct':struct,'JOURNAL':JOURNAL,'validate_journal':validate_journal}
        def read(path,cap):
            if path.endswith('/stat'):return proc(pid=1,start=10).encode()
            if path.endswith('/status'):return b'Uid:\t0 0 0 0\nGid:\t0 0 0 0\n'
            if path.endswith('/cgroup'):return b'0::/init.scope\n'
            raise AssertionError(path)
        tree=ast.parse((Path(__file__).parents[1]/'fort_probe/descriptors.py').read_text())
        node=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=='observe_journal')
        exec(compile(ast.Module(body=[node],type_ignores=[]),'fixture-only-pidfd','exec'),ns)
        result=ns['observe_journal'](42,{'0':'/dev/null','1':'socket:[2]','2':'socket:[2]'},read,parse_stat)
        self.assertEqual(result,journal_proofs())
        self.assertEqual(effects.count('pidfd_getfd'),2)
        self.assertEqual(effects.count('duplicate-closed'),2)
        self.assertEqual(effects[-1],('closed',10))
