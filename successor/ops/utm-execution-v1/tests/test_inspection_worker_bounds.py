"""The supervisor runs only against process, OS, clock, selector and command doubles."""
import ast
import hashlib
import json
from pathlib import Path
from types import SimpleNamespace
import unittest
from fort_probe.inspection_workers import PROFILES,WorkerOutput

class WorkerBoundTests(unittest.TestCase):
    def test_deadline_rss_and_output_failures_cancel_before_reap(self):
        tree=ast.parse((Path(__file__).parents[1]/'fort_probe/inspection_workers.py').read_text())
        definition=next(n for n in tree.body if isinstance(n,ast.ClassDef) and n.name=='Worker')
        for fault in ('deadline','rss','output'):
            effects=[]
            class Pipe:
                def fileno(self):return 9
                def close(self):effects.append('close')
            class Process:
                pid=42;stdout=Pipe();stderr=Pipe();returncode=0
                def wait(self,**kwargs):effects.append('reap')
                def poll(self):raise AssertionError('identity anchor must not be reaped early')
            class Selector:
                def __enter__(self):return self
                def __exit__(self,*args):pass
                def register(self,*args):pass
                def get_map(self):return True
                def select(self,*args):return [(SimpleNamespace(fileobj=Pipe(),data=0),1)]
            ticks=iter((0,901))
            pin={'bytes':1,'sha256':hashlib.sha256(b'x').hexdigest()}
            ns={'WorkerOutput':WorkerOutput,'verify_source':lambda value:None,'PROFILES':PROFILES,'worker_command':lambda *a,**k:['/fixture'],
                'INPUTS':SimpleNamespace(read_bytes=lambda:json.dumps({'downloads':[],'existing_host_tools':[dict(pin,path='/fixture'),dict(pin,path='/bin/ps')]}).encode()),
                'Path':lambda p:SimpleNamespace(read_bytes=lambda:b'x'),'hashlib':hashlib,'json':json,'SOURCE':'/fixture-source',
                'subprocess':SimpleNamespace(Popen=lambda *a,**k:Process(),DEVNULL=None,PIPE=None),
                'time':SimpleNamespace(monotonic=lambda:next(ticks) if fault=='deadline' else 0),
                'selectors':SimpleNamespace(DefaultSelector=Selector,EVENT_READ=1),
                'os':SimpleNamespace(set_blocking=lambda *a:None,read=lambda *a:b'x'*65537,killpg=lambda *a:effects.append('cancel')),
                'signal':SimpleNamespace(SIGKILL=9),'rss_bytes':lambda raw:4*1024**3 if fault=='rss' else 0}
            exec(compile(ast.Module(body=[definition],type_ignores=[]),'fixture-worker','exec'),ns)
            worker=ns['Worker'].__new__(ns['Worker'])
            worker.accepted_source_sha256='a'*64
            worker.commands=SimpleNamespace(run=lambda *a,**k:(b'1',b''))
            with self.subTest(fault=fault),self.assertRaises((ValueError,TimeoutError)):worker.run('acquire',Path('/fixture'))
            self.assertLess(effects.index('cancel'),effects.index('reap'));self.assertEqual(effects[-2:],['close','close'])

    def test_input_progress_cannot_extend_expired_deadlines_or_skip_fixed_inputs(self):
        from fort_probe.inspection_workers import WorkerOutput
        parser=WorkerOutput('acquire',['first','second'],0)
        parser.feed(b'{"progress_input":"first"}\n',.1)
        parser.feed(b'{"progress_input":"second"}\n',1)
        parser.feed(b'{"operation":"acquire","status":"complete","details":{}}\n',2)
        self.assertEqual(parser.finish()['operation'],'acquire')
        for data,now in ((b'{"progress_input":"second"}\n',1),(b'{"progress_input":"first"}\n',601),
                         (b'{"operation":"acquire"}\n',1)):
            parser=WorkerOutput('acquire',['first','second'],0)
            with self.assertRaises((ValueError,TimeoutError)):
                parser.feed(data,now);parser.finish()
