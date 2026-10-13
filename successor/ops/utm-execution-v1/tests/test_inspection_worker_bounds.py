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

    def test_closed_pipes_remain_supervised_until_observed_exit(self):
        tree=ast.parse((Path(__file__).parents[1]/'fort_probe/inspection_workers.py').read_text())
        definition=next(n for n in tree.body if isinstance(n,ast.ClassDef) and n.name=='Worker')
        for fault in ('none','rss','rss-unobservable','stage-deadline','input-deadline','missing-result','nonzero'):
            with self.subTest(fault=fault):
                clock=[0.0];effects=[];samples=[];waits=[];streams={}
                class Pipe:
                    def __init__(self,fd):self.fd=fd
                    def fileno(self):return self.fd
                    def close(self):effects.append('close')
                class Child:
                    pid=42;stdout=Pipe(10);stderr=Pipe(11);returncode=1 if fault=='nonzero' else 0
                    def wait(self,**kwargs):effects.append('reap')
                    def poll(self):raise AssertionError('unreaped identity required')
                class Selector:
                    def __enter__(self):return self
                    def __exit__(self,*args):pass
                    def register(self,pipe,event,index):streams[pipe.fd]=SimpleNamespace(fileobj=pipe,data=index)
                    def unregister(self,pipe):del streams[pipe.fd]
                    def get_map(self):return streams
                    def select(self,timeout):
                        clock[0]+=.05
                        return [(key,1) for key in list(streams.values())]
                class Commands:
                    def run(self,*args,**kwargs):
                        samples.append((clock[0],bool(streams)))
                        return (b'999999999' if fault=='rss' and not streams else b'' if fault=='rss-unobservable' and not streams else b'1'),b''
                    def _wait_unreaped(self,child,deadline):
                        waits.append(deadline-clock[0])
                        self_bound=all(value<=.051 for value in waits)
                        if not self_bound:raise AssertionError('exit observation must use short bounded slices')
                        if not streams and fault in ('stage-deadline','input-deadline'):
                            clock[0]=91 if fault=='stage-deadline' else 601
                            raise TimeoutError('synthetic child still live')
                        clock[0]=deadline
                        if clock[0]<.4:raise TimeoutError('synthetic child still live')
                names=['input'+str(i) for i in range(8)]
                operation='registry' if fault=='stage-deadline' else 'acquire'
                result={'operation':operation,'status':'complete','details':{'completed':8}}
                progress=''.join(json.dumps({'progress_input':name})+'\n' for name in names) if operation=='acquire' else ''
                wire=(progress+json.dumps(result)+'\n').encode()
                reads={10:([wire,b''] if fault!='missing-result' else [b'']),11:[b'']}
                pin={'bytes':1,'sha256':hashlib.sha256(b'x').hexdigest()}
                ns={'WorkerOutput':WorkerOutput,'verify_source':lambda value:None,'PROFILES':PROFILES,'worker_command':lambda *a,**k:['/fixture'],
                    'INPUTS':SimpleNamespace(read_bytes=lambda:json.dumps({'downloads':[{'name':name} for name in names],'existing_host_tools':[dict(pin,path='/fixture'),dict(pin,path='/bin/ps')]}).encode()),
                    'Path':lambda p:SimpleNamespace(read_bytes=lambda:b'x'),'hashlib':hashlib,'json':json,'SOURCE':'/fixture-source',
                    'subprocess':SimpleNamespace(Popen=lambda *a,**k:Child(),DEVNULL=None,PIPE=None),'time':SimpleNamespace(monotonic=lambda:clock[0]),
                    'selectors':SimpleNamespace(DefaultSelector=Selector,EVENT_READ=1),
                    'os':SimpleNamespace(set_blocking=lambda *a:None,read=lambda fd,*a:reads[fd].pop(0),killpg=lambda *a:effects.append('cancel')),
                    'signal':SimpleNamespace(SIGKILL=9),'rss_bytes':lambda raw:int(raw)*1024}
                exec(compile(ast.Module(body=[definition],type_ignores=[]),'fixture-worker','exec'),ns)
                worker=ns['Worker'].__new__(ns['Worker']);worker.accepted_source_sha256='a'*64;worker.commands=Commands()
                if fault=='none':self.assertEqual(worker.run(operation,Path('/fixture')),result)
                else:
                    with self.assertRaises((ValueError,TimeoutError)):worker.run(operation,Path('/fixture'))
                self.assertLess(effects.index('cancel'),effects.index('reap'));self.assertEqual(effects.count('reap'),1)
                if fault in ('rss','none'):self.assertTrue(any(not open_pipes for _,open_pipes in samples))
