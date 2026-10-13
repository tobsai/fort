"""Finite command runner for future live adapters. Construction remains disabled."""
import os
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
                process.wait(timeout=max(.001,deadline-time.monotonic()))
            if process.returncode != 0 and not (allow_empty and process.returncode==1 and not any(output)):
                raise ValueError('owned command failed: ' + os.path.basename(argv[0]))
            return bytes(output[0]), bytes(output[1])
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=1)
            process.stdout.close(); process.stderr.close()
