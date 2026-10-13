"""Every command effect is doubled; no child is launched."""
import ast
from pathlib import Path
from types import SimpleNamespace
import unittest


class CommandTests(unittest.TestCase):
    def test_leader_stays_unreaped_until_group_cancellation_on_every_exit(self):
        for fault in ('timeout','cap','success','kill-failure'):
            with self.subTest(fault=fault):
                effects=[]
                class Pipe:
                    def fileno(self):return 9
                    def close(self):effects.append('close')
                class Child:
                    pid=42; stdout=Pipe();stderr=Pipe();returncode=0
                    def poll(self):raise AssertionError('poll reaps the group identity anchor')
                    def wait(self,**kwargs):effects.append('reap')
                class Selector:
                    def __enter__(self):return self
                    def __exit__(self,*args):pass
                    def register(self,*args):pass
                    def get_map(self):return fault!='success'
                    def select(self,remaining):return [(SimpleNamespace(fileobj=Pipe(),data=0),1)]
                def kill(*args):
                    effects.append('cancel-group')
                    if fault=='kill-failure':raise PermissionError('synthetic failure')
                clock=iter((0,2 if fault in ('timeout','kill-failure') else .1,.2,.3))
                ns={'subprocess':SimpleNamespace(Popen=lambda *a,**k:Child(),DEVNULL=None,PIPE=None),
                    'selectors':SimpleNamespace(DefaultSelector=Selector,EVENT_READ=1),
                    'signal':SimpleNamespace(SIGKILL=9), 'time':SimpleNamespace(monotonic=lambda:next(clock)),
                    'os':SimpleNamespace(set_blocking=lambda *a:None,read=lambda *a:b'xx',killpg=kill)}
                tree=ast.parse((Path(__file__).parents[1]/'fort_probe/bounded.py').read_text())
                node=next(n for n in tree.body if isinstance(n,ast.ClassDef) and n.name=='Commands')
                exec(compile(ast.Module(body=[node],type_ignores=[]),'fixture-only','exec'),ns)
                command=ns['Commands'].__new__(ns['Commands'])
                command._wait_unreaped=lambda *args:effects.append('observe-unreaped')
                if fault=='success':self.assertEqual(command.run(['/fixture'],timeout=1,cap=1),(b'',b''))
                else:
                    with self.assertRaises((TimeoutError,ValueError,PermissionError,RuntimeError)):
                        command.run(['/fixture'],timeout=1,cap=1)
                self.assertEqual(effects.count('cancel-group'),1)
                self.assertLess(effects.index('cancel-group'),effects.index('reap'))
                self.assertEqual(effects[-2:],['close','close'])

    def test_nonreaping_linux_observation_keeps_waitid_wnowait(self):
        from unittest.mock import patch
        from fort_probe import bounded
        calls=[];ticks=iter((None,SimpleNamespace(si_pid=42)))
        fake=SimpleNamespace(P_PID=1,WEXITED=4,WNOHANG=1,WNOWAIT=8,
            waitid=lambda *args:calls.append(args) or next(ticks))
        command=bounded.Commands.__new__(bounded.Commands)
        with patch.object(bounded,'os',fake),patch.object(bounded.time,'monotonic',return_value=0),patch.object(bounded.time,'sleep'):
            command._wait_unreaped(SimpleNamespace(pid=42),1)
        self.assertEqual(calls,[(1,42,13),(1,42,13)])

    def test_mac_exit_or_exited_zombie_is_observed_without_reaping(self):
        import errno
        from unittest.mock import patch
        from fort_probe import bounded
        for case in ('exit','zombie','timeout','unknown-error'):
            event=SimpleNamespace(flags=0,fflags=8,data=0)
            if case in ('zombie','unknown-error'):
                event.flags=16;event.fflags=0;event.data=errno.ESRCH if case=='zombie' else errno.EPERM
            class Queue:
                def __enter__(self):return self
                def __exit__(self,*args):pass
                def control(self,changes,count,timeout):return [] if case=='timeout' else [event]
            fake=SimpleNamespace(kqueue=Queue,kevent=lambda *a,**kw:kw,KQ_FILTER_PROC=-5,
                KQ_EV_ADD=1,KQ_EV_ONESHOT=2,KQ_NOTE_EXIT=8,KQ_EV_ERROR=16)
            command=bounded.Commands.__new__(bounded.Commands)
            with self.subTest(case=case),patch.object(bounded,'os',SimpleNamespace()),patch.object(bounded,'select',fake),patch.object(bounded.time,'monotonic',return_value=0):
                if case in ('exit','zombie'):command._wait_unreaped(SimpleNamespace(pid=42),1)
                else:
                    with self.assertRaises((TimeoutError,RuntimeError)):command._wait_unreaped(SimpleNamespace(pid=42),1)
