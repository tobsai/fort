"""Finite command runner for future live adapters. Construction remains disabled."""
import os
import errno
import select
import threading
import selectors
import signal
import subprocess
import time

LIVE_EXECUTION_ENABLED = False


def require_live():
    if not LIVE_EXECUTION_ENABLED:
        raise RuntimeError('live adapters disabled in source-only revision')


class Commands:
    def __init__(self):
        require_live()
        if threading.active_count()!=1 or threading.current_thread() is not threading.main_thread() or signal.getsignal(signal.SIGCHLD)!=signal.SIG_DFL:
            raise RuntimeError('runner requires sole child-reaping ownership and default SIGCHLD')
        if not hasattr(os,'waitid') and not hasattr(select,'kqueue'):
            raise RuntimeError('non-reaping child observation unavailable')

    def _wait_unreaped(self, process, deadline):
        # The unreaped direct child reserves its PID/PGID through cancellation.
        if hasattr(os,'waitid'):
            while os.waitid(os.P_PID,process.pid,os.WEXITED|os.WNOHANG|os.WNOWAIT) is None:
                if time.monotonic()>=deadline: raise TimeoutError('owned command exit deadline')
                time.sleep(.001)
        else:
            with select.kqueue() as queue:
                event=select.kevent(process.pid,filter=select.KQ_FILTER_PROC,
                                    flags=select.KQ_EV_ADD|select.KQ_EV_ONESHOT,fflags=select.KQ_NOTE_EXIT)
                try: observed=queue.control([event],1,max(0,deadline-time.monotonic()))
                except ProcessLookupError: return  # Already exited, still owned and unreaped.
                if observed and observed[0].flags & select.KQ_EV_ERROR:
                    if observed[0].data==errno.ESRCH:return  # Exited child still reserves the identity.
                    raise RuntimeError('owned command exit observation failed')
                if not observed or not observed[0].fflags & select.KQ_NOTE_EXIT:
                    raise TimeoutError('owned command exit observation incomplete')

    def run(self, argv, timeout=.25, cap=65536, allow_empty=False):
        if not argv or not argv[0].startswith('/') or not 0 < timeout <= 5 or not 0 < cap <= 1048576:
            raise ValueError('bounded absolute command required')
        env = {'PATH':'/usr/bin:/bin:/usr/sbin:/sbin','LANG':'C','LC_ALL':'C'}
        process = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, env=env, close_fds=True, start_new_session=True)
        output = [bytearray(), bytearray()]
        deadline = time.monotonic() + timeout
        try:
            with selectors.DefaultSelector() as selector:
                for index, stream in enumerate((process.stdout, process.stderr)):
                    os.set_blocking(stream.fileno(), False)
                    selector.register(stream, selectors.EVENT_READ, index)
                while selector.get_map():
                    remaining = deadline - time.monotonic()
                    if remaining <= 0: raise TimeoutError('owned command deadline')
                    for key, unused in selector.select(remaining):
                        chunk = os.read(key.fileobj.fileno(), 4096)
                        if not chunk: selector.unregister(key.fileobj)
                        else:
                            output[key.data].extend(chunk)
                            if sum(map(len,output)) > cap: raise ValueError('owned command output exceeded cap')
                self._wait_unreaped(process,deadline)
        finally:
            try:
                try: os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError: pass
                finally: process.wait(timeout=1)
            finally:
                process.stdout.close(); process.stderr.close()
        if process.returncode != 0 and not (allow_empty and process.returncode==1 and not any(output)):
            raise ValueError('owned command failed: ' + os.path.basename(argv[0]))
        return bytes(output[0]), bytes(output[1])
