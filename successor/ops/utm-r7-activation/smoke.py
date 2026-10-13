"""Fixed network-free synthetic smoke. No acquisition entrypoint or operation-root access."""
import ast
import json
import os
from pathlib import Path
import resource
import selectors
import signal
import subprocess
import sys
import time
sys.path.insert(0, str(Path(__file__).resolve().parent))
from packet_support import (HERE, SOURCE, ENV, LAUNCHER_KEYS, Journal, check_packet,
                            inspect_environment, require_active, source_module, verify_tools)

LIMITS = {'core': resource.RLIMIT_CORE, 'cpu': resource.RLIMIT_CPU,
          'file': resource.RLIMIT_FSIZE, 'descriptors': resource.RLIMIT_NOFILE}
PERMISSIONS = {'read_allowed': True, 'read_denied': True, 'global_read': False,
               'write': False, 'child': False, 'worker': False}


def expected_limits(case):
    return {'core': [0, 0], 'cpu': [case['cpu'], case['cpu'] + 1],
            'file': [case['file'], case['file']], 'descriptors': [64, 64]}


def validate_result(result, case, identities):
    if (result.get('case') != case['name'] or result.get('runtime') != case['runtime'] or
            result.get('limits') != expected_limits(case) or result.get('environment') != ENV or
            result.get('runtime_identity') != identities[case['runtime']] or
            result.get('permissions') != (PERMISSIONS if case['runtime'] == 'node' else None)):
        raise ValueError('runtime setup evidence differs')
    keys = result.get('launcher_keys')
    if not isinstance(keys, list) or keys != sorted(set(keys)) or set(keys) - (LAUNCHER_KEYS if case['runtime'] == 'python' else set()):
        raise ValueError('runtime environment evidence differs')
    if (type(result.get('pid')) is not int or result['pid'] <= 1 or
            type(result.get('rss_samples')) is not int or not 2 <= result['rss_samples'] <= 50 or
            type(result.get('max_rss_bytes')) is not int or not 0 < result['max_rss_bytes'] <= case['rss'] or
            result.get('returncode') != -signal.SIGKILL or
            any(result.get(k) is not True for k in ('rss_after_eof', 'cancel_sent', 'reaped', 'child_absent', 'group_absent'))):
        raise ValueError('RSS observation or owned cleanup incomplete')


def run_smoke(host, journal, context, clock=time.monotonic):
    started = clock(); before = host.limits(); results = []
    try:
        for case in context.cases:
            if clock() - started >= 30: raise TimeoutError('total smoke deadline')
            result = host.child(case)
            validate_result(result, case, context.identities)
            journal.record(case['name'].replace('_', '-') + '.json', result)
            results.append(result)
        after = host.limits()
        if before != after: raise ValueError('parent limits changed')
        if clock() - started >= 30: raise TimeoutError('total smoke deadline')
        receipt = {'packet_sha': context.packet_sha, 'source_seal': context.active_sha,
                   'cases': [r['case'] for r in results], 'parent_before': before, 'parent_after': after,
                   'parent_limits_unchanged': True, 'status': 'passed',
                   'completed_monotonic': clock(), 'completed_epoch': time.time()}
        journal.record('complete.json', receipt)
        return receipt
    except BaseException as error:
        after = host.limits()
        journal.record('failure.json', {'type': type(error).__name__, 'details': getattr(error, 'details', None),
                                      'completed': [r['case'] for r in results], 'parent_before': before,
                                      'parent_after': after, 'parent_limits_unchanged': before == after})
        raise


class SmokeFailure(RuntimeError):
    def __init__(self, details):
        super().__init__('synthetic child failed; see bounded evidence')
        self.details = details


class LimitRecorder:
    """Every reviewed setter is immediately read back; all other resources are rejected."""
    def __init__(self): self.observed = {}; self.current = None
    def __getattr__(self, name): return getattr(resource, name)
    def setrlimit(self, key, value):
        names = [name for name, number in LIMITS.items() if number == key]
        if len(names) != 1: raise ValueError('unexpected resource setter')
        self.current = names[0]
        if self.current in self.observed: raise ValueError('repeated resource setter')
        resource.setrlimit(key, value)
        got = resource.getrlimit(key)
        if tuple(value) != got: raise ValueError('resource readback differs')
        self.observed[self.current] = list(got)


def reviewed_setter(recorder):
    tree = ast.parse((SOURCE / 'fort_probe/inspection_workers.py').read_bytes())
    nodes = [n for n in tree.body if
             (isinstance(n, ast.FunctionDef) and n.name == 'child_limits') or
             (isinstance(n, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'PROFILES' for t in n.targets))]
    if len(nodes) != 2: raise ValueError('reviewed setter extraction differs')
    scope = {'resource': recorder, 'sys': sys}
    exec(compile(ast.Module(body=nodes, type_ignores=[]), 'reviewed-child-limits', 'exec'), scope)
    return scope['child_limits'], scope['PROFILES']


def setup_child(fd, case):
    signal.signal(signal.SIGALRM, signal.SIG_DFL)
    signal.alarm(5)
    recorder = LimitRecorder()
    try:
        setter, profiles = reviewed_setter(recorder)
        profile = profiles[case['name']]
        if any(profile[k] != case[k] for k in ('cpu', 'file', 'rss')): raise ValueError('profile differs')
        setter(profile)
        if recorder.observed != expected_limits(case): raise ValueError('set/readback incomplete')
        value = {'limits': recorder.observed}
    except BaseException as error:
        value = {'setup_failure': {'setter': recorder.current, 'type': type(error).__name__,
                                   'errno': getattr(error, 'errno', None)}, 'limits': recorder.observed}
        os.write(fd, (json.dumps(value, separators=(',', ':')) + '\n').encode())
        os.close(fd)
        raise
    os.write(fd, (json.dumps(value, separators=(',', ':')) + '\n').encode())
    os.close(fd)


def cancel_owned(child):
    """No wait/poll before SIGKILL. After reap, only zero-signal existence probes."""
    result = {'cancel_sent': False, 'reaped': False, 'child_absent': False, 'group_absent': False}
    try:
        os.killpg(child.pid, signal.SIGKILL)
        result['cancel_sent'] = True
    except ProcessLookupError:
        pass
    finally:
        result['returncode'] = child.wait(timeout=1)
        result['reaped'] = True
    for key, probe in (('child_absent', os.kill), ('group_absent', os.killpg)):
        try: probe(child.pid, 0)
        except ProcessLookupError: result[key] = True
    return result


class Host:
    def __init__(self, context):
        require_active()
        if sys.platform != 'darwin': raise ValueError('fixed macOS smoke only')
        self.context = context
        self.commands = source_module('bounded').Commands(guard=require_active)

    def limits(self):
        return {name: list(resource.getrlimit(number)) for name, number in
                dict(LIMITS, rss=resource.RLIMIT_RSS, address_space=resource.RLIMIT_AS).items()}

    def child(self, case):
        check_packet(self.context.packet_sha, 'activated')
        verify_tools(self.context)
        read_fd, write_fd = os.pipe()
        os.set_blocking(read_fd, False)
        child = None; failure = None; result = None; cleanup = {}; setup = bytearray()
        output = [bytearray(), bytearray()]; samples = 0; maximum = 0; after_eof = False
        deadline = time.monotonic() + 5
        try:
            try:
                child = subprocess.Popen(case['argv'], stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                         stderr=subprocess.PIPE, cwd=str(HERE), env=ENV, close_fds=True,
                                         pass_fds=(write_fd,), start_new_session=True,
                                         preexec_fn=lambda: setup_child(write_fd, case))
            finally:
                os.close(write_fd)
            with selectors.DefaultSelector() as selector:
                selector.register(read_fd, selectors.EVENT_READ, 2)
                for index, stream in enumerate((child.stdout, child.stderr)):
                    os.set_blocking(stream.fileno(), False)
                    selector.register(stream, selectors.EVENT_READ, index)
                next_rss = 0
                while True:
                    if time.monotonic() >= deadline: raise TimeoutError('synthetic child deadline')
                    for key, unused in selector.select(min(.05, max(0, deadline - time.monotonic()))):
                        fd = key.fileobj if isinstance(key.fileobj, int) else key.fileobj.fileno()
                        chunk = os.read(fd, 4096)
                        if not chunk: selector.unregister(key.fileobj)
                        else:
                            (setup if key.data == 2 else output[key.data]).extend(chunk)
                            if len(setup) + sum(map(len, output)) > 8192: raise ValueError('synthetic output cap')
                    if time.monotonic() >= next_rss:
                        if samples >= 50: raise ValueError('RSS observation count')
                        raw, errors = self.commands.run(['/bin/ps', '-o', 'rss=', '-p', str(child.pid)],
                                                        timeout=.25, cap=128, allow_empty=True)
                        if errors or not raw: raise ValueError('owned RSS observation absent')
                        # Same strict parser as the reviewed worker, without importing its acquisition module.
                        import re
                        text = raw.decode().strip()
                        if not re.fullmatch('[0-9]{1,12}', text): raise ValueError('malformed owned RSS')
                        rss = int(text) * 1024
                        if not 0 < rss <= case['rss']: raise ValueError('RSS observation outside threshold')
                        samples += 1; maximum = max(maximum, rss)
                        after_eof = after_eof or not selector.get_map()
                        next_rss = time.monotonic() + .1
                    try: self.commands._wait_unreaped(child, min(deadline, time.monotonic() + .001))
                    except TimeoutError: pass
                    else: raise ValueError('synthetic child exited before owned cancellation')
                    if time.monotonic() >= deadline: raise TimeoutError('synthetic child completion deadline')
                    if not selector.get_map() and samples >= 2 and after_eof:
                        if output[1]: raise ValueError('synthetic stderr')
                        setup_value = json.loads(setup)
                        ready = json.loads(output[0])
                        result = dict(ready, limits=setup_value['limits'], pid=child.pid, rss_samples=samples,
                                      rss_after_eof=after_eof, max_rss_bytes=maximum)
                        break
        except BaseException as error:
            failure = {'type': type(error).__name__}
            if child is None:
                try: setup.extend(os.read(read_fd, 8193))
                except BlockingIOError: pass
            if setup:
                try: failure['setup'] = json.loads(setup)
                except (ValueError, UnicodeError): failure['setup'] = 'unreadable'
        finally:
            if child is not None:
                try: cleanup = cancel_owned(child)
                except BaseException as error:
                    failure = dict(failure or {}, cleanup_error=type(error).__name__)
                finally: child.stdout.close(); child.stderr.close()
            os.close(read_fd)
        if failure is not None: raise SmokeFailure(dict(failure, cleanup=cleanup, case=case['name']))
        result.update(cleanup)
        validate_result(result, case, self.context.identities)
        return result


def main():
    if len(sys.argv) != 3 or sys.argv[1] != '--run-once':
        sys.stderr.write('Inert packet: exact approved --run-once packet digest required.\n'); return 78
    context = check_packet(sys.argv[2], 'activated')
    require_active(); verify_tools(context)
    inspect_environment(os.environ, 'python')
    host = Host(context)
    journal = Journal(context, create=True)
    try:
        receipt = run_smoke(host, journal, context)
        print(json.dumps(receipt, sort_keys=True))
    finally: journal.close()
    return 0


if __name__ == '__main__':
    try: raise SystemExit(main())
    except Exception as error:
        sys.stderr.write('Smoke refused or failed: ' + type(error).__name__ + '; no continuation.\n')
        raise SystemExit(1)
