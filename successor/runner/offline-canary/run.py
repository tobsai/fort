"""One-shot offline experiment; no provider API, enrollment, socket listener in IP space, or live HOME.

Run with Python 3.14. Compile hooks and package managers are not used. The only
non-system executable started is the hash-pinned official Codex CLI, via its
sandbox diagnostic command. All outputs live in a fresh /private/tmp root.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import resource
import shutil
import signal
import subprocess
import tempfile
import time

EXECUTABLE = Path('/opt/homebrew/Caskroom/codex/0.157.1/bin/codex')
EXPECTED = '27ceb5f9b957b43a519efe4eaa3816a0bffb0a531a2c89af18840c0a3c016a7d'
PROFILE = 'fort_candidate_b_no_files'
DECLARATION = 'permissions.fort_candidate_b_no_files={filesystem={"/**"="deny"},network={enabled=false}}'
SOURCE = Path(__file__).resolve().parent
ROOT = Path(tempfile.mkdtemp(prefix='fort-offline-permissions-', dir='/tmp')).resolve()
os.chmod(ROOT, 0o700)
RESULTS = []
VALIDATION = {'providerAttempt': False, 'actualParser': 'unrun', 'fullGeneratedPolicy': 'unrun',
              'hostIsolationProven': False, 'ipTrafficProbe': 'unrun', 'cleaned': False}


def limits():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    resource.setrlimit(resource.RLIMIT_FSIZE, (4 * 1024 * 1024, 4 * 1024 * 1024))


def run(name, args, env, cwd, timeout=15):
    with (ROOT / (name + '.stdout')).open('wb') as stdout, (ROOT / (name + '.stderr')).open('wb') as stderr:
        child = subprocess.Popen(args, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                 stdout=stdout, stderr=stderr, close_fds=True,
                                 start_new_session=True, preexec_fn=limits)
        timed_out = False
        try:
            code = child.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            timed_out = True
            os.killpg(child.pid, signal.SIGKILL)
            code = child.wait(timeout=3)
        # Kill/reap descendants still in this owned group, even after root exit.
        try:
            os.killpg(child.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        absent = False
        for _ in range(100):
            try:
                os.killpg(child.pid, 0)
            except ProcessLookupError:
                absent = True
                break
            time.sleep(0.01)
    result = {'name': name, 'args': args, 'pid': child.pid, 'exitCode': code,
              'timedOut': timed_out, 'ownedGroupAbsent': absent}
    RESULTS.append(result)
    print(json.dumps(result), flush=True)
    (ROOT / 'results.json').write_text(json.dumps(RESULTS, indent=2) + '\n')
    if not absent:
        raise RuntimeError('Owned process group remains; inspect before any further invocation')
    return result


def fixture(name):
    path = ROOT / name
    path.mkdir()
    (path / 'workspace').mkdir()
    for file in ['ordinary', 'workspace/inside', 'unicode-λ', 'line\nbreak', 'rename-from', 'descriptor-write']:
        (path / file).write_text('synthetic-canary\n')
    (path / 'workspace/escape').symlink_to(path / 'ordinary')
    os.link(path / 'ordinary', path / 'hardlink')
    return path


def observations(name):
    text = (ROOT / (name + '.stdout')).read_text()
    return {key: {'result': int(value), 'errno': int(error)}
            for key, value, error in re.findall(r'^(\w+) result=(-?\d+) errno=(\d+)$', text, re.M)}


def main():
    if os.uname().sysname != 'Darwin':
        raise RuntimeError('macOS only')
    if hashlib.sha256(EXECUTABLE.read_bytes()).hexdigest() != EXPECTED:
        raise RuntimeError('Runtime pin mismatch')
    for name in ['home', 'codex', 'tmp', 'workspace', 'xdg-config', 'xdg-data', 'xdg-cache']:
        (ROOT / name).mkdir()
    env = {'HOME': str(ROOT / 'home'), 'CODEX_HOME': str(ROOT / 'codex'),
           'TMPDIR': str(ROOT / 'tmp'), 'XDG_CONFIG_HOME': str(ROOT / 'xdg-config'),
           'XDG_DATA_HOME': str(ROOT / 'xdg-data'), 'XDG_CACHE_HOME': str(ROOT / 'xdg-cache'),
           'PATH': '/usr/bin:/bin', 'LANG': 'en_US.UTF-8', 'TZ': 'UTC'}
    (ROOT / 'environment.json').write_text(json.dumps(env, indent=2) + '\n')
    binary = str(ROOT / 'probe')
    compiled = run('compile', ['/usr/bin/clang', '-Wall', '-Wextra', '-Werror',
                             '-Wno-deprecated-declarations', str(SOURCE / 'probe.c'), '-o', binary], env, ROOT)
    if compiled['exitCode'] != 0:
        return
    control = run('control', [binary, 'probe', 'control', str(fixture('control-fixture'))], env, ROOT)
    expected_reads = ['ordinary', 'workspace', 'symlink', 'hardlink', 'unicode', 'newline',
                      'preopened_read', 'preopened_write', 'preopened_unix_pair', 'preopened_pipe']
    observed = observations('control')
    if control['exitCode'] or any(observed.get(k, {}).get('result') != 1 for k in expected_reads):
        raise RuntimeError('Positive controls failed; do not run official runtime')
    if any(observed.get(k, {}).get('result') != 0 for k in ['create', 'rename', 'unix_path_connect',
                                                        'network_outbound_query', 'network_inbound_query']):
        raise RuntimeError('Positive syscall/query controls failed')

    # Isolate the source-derived deny rule after the C probe has loaded. This
    # is not the full Codex-generated policy and does not exercise its parser.
    policy = '(version 1)\n(allow default)\n(deny network*)\n'
    policy += '(deny file-read* (regex #"^/.*$"))\n(deny file-write* (regex #"^/.*$"))\n'
    policy_path = ROOT / 'deny-glob-rule.sb'
    policy_path.write_text(policy)
    run('deny-glob-rule', [binary, 'probe', str(policy_path), str(fixture('deny-fixture'))], env, ROOT)
    denied = observations('deny-glob-rule')
    (ROOT / 'observations.json').write_text(json.dumps({'control': observed, 'deny-glob-rule': denied}, indent=2) + '\n')
    for key in ['ordinary', 'workspace', 'symlink', 'hardlink', 'unicode', 'newline', 'create', 'rename', 'unix_path_connect']:
        if denied.get(key) != {'result': -1, 'errno': 1}:
            raise RuntimeError('Expected denial missing: ' + key)
    for key in ['network_outbound_query', 'network_inbound_query']:
        if denied.get(key, {}).get('result') != 1:
            raise RuntimeError('Kernel network permission query did not deny: ' + key)
    for key in ['preopened_read', 'preopened_write', 'preopened_unix_pair', 'preopened_pipe']:
        if denied.get(key, {}).get('result') != 1:
            raise RuntimeError('Descriptor limitation changed; inspect rather than assume confinement: ' + key)
    VALIDATION['denyGlobRule'] = '9 syscall denials and 2 kernel network denial queries passed'
    VALIDATION['descriptorRevocation'] = 'not provided: 4 preopened capabilities remained usable'

    # Safety wrapper for trusted CLI diagnostic only, not Fort's production policy.
    # Deny data outside system runtime files, the exact pinned executable and this root.
    # No provider credentials/configuration are permitted, even if the CLI seeks them.
    reads = ['(literal "/")', f'(subpath {json.dumps(str(ROOT))})',
             f'(literal {json.dumps(str(EXECUTABLE))})',
             '(subpath "/System/Library")', '(subpath "/usr/lib")',
             '(subpath "/usr/share")', '(subpath "/usr/bin")', '(subpath "/bin")',
             '(literal "/dev/null")', '(literal "/dev/random")', '(literal "/dev/urandom")']
    guard = '(version 1)\n(allow default)\n(deny network*)\n(deny mach*)\n(deny ipc-posix*)\n'
    guard += '(deny file-read* (subpath "/Users") (subpath "/Volumes"))\n'
    guard += '(deny file-read-data (require-not (require-any ' + ' '.join(reads) + ')))\n'
    guard += '(deny file-write* (require-not (subpath ' + json.dumps(str(ROOT)) + ')))\n'
    guard += '(deny process-exec (require-not (require-any '
    guard += ' '.join('(literal ' + json.dumps(p) + ')' for p in [str(EXECUTABLE), binary, '/usr/bin/true', '/usr/bin/sandbox-exec'])
    guard += ')))\n'
    guard_path = ROOT / 'safety-wrapper.sb'
    guard_path.write_text(guard)
    safety = run('safety-wrapper', [binary, 'guard', str(guard_path), '/usr/bin/true'], env, ROOT)
    if safety['exitCode']:
        VALIDATION['actualParser'] = 'blocked: safety wrapper could not exec /usr/bin/true'
        return

    # Each actual CLI invocation has a fresh empty configuration/home scope.
    settings = ['analytics.enabled=false', 'check_for_update_on_startup=false',
                'cli_auth_credentials_store="file"', 'mcp_oauth_credentials_store="file"',
                'cloud.skills.enabled=false', 'skills.bundled.enabled=false',
                'skills.include_instructions=false', 'project_doc_max_bytes=0', 'notify=[]',
                'otel.exporter="none"', 'otel.trace_exporter="none"']
    base = [binary, 'guard', str(guard_path), str(EXECUTABLE)]
    for name, declaration, selection in [
        ('parser-invalid', DECLARATION.replace('"deny"', '"invalid-canary"'), PROFILE),
        ('parser-missing', DECLARATION, 'fort_absent_canary'),
        ('parser-exact', DECLARATION, PROFILE),
    ]:
        case = ROOT / name
        case.mkdir()
        for subdir in ['home', 'codex', 'tmp', 'workspace']:
            (case / subdir).mkdir()
        case_env = {**env, 'HOME': str(case / 'home'), 'CODEX_HOME': str(case / 'codex'), 'TMPDIR': str(case / 'tmp')}
        args = base + [item for setting in settings + [declaration] for item in ['-c', setting]]
        args += ['sandbox', '-P', selection, '-C', str(case / 'workspace'), '--', '/usr/bin/true']
        result = run(name, args, case_env, case / 'workspace')
        error = (ROOT / (name + '.stderr')).read_text()
        if name == 'parser-invalid':
            if result['exitCode'] != 1 or 'FilesystemPermissionToml' not in error:
                raise RuntimeError('Actual invalid-value parser control did not reject as expected')
        elif name == 'parser-missing':
            if result['exitCode'] != 1 or 'undefined profile `fort_absent_canary`' not in error:
                raise RuntimeError('Actual missing-profile control did not reject as expected')
        elif result['exitCode'] == 71 and 'sandbox-exec: sandbox_apply: Operation not permitted' in error:
            VALIDATION['actualParser'] = 'accepted exact profile through sandbox launcher; both negative controls passed'
            VALIDATION['fullGeneratedPolicy'] = 'blocked: macOS refused nested sandbox application'
        else:
            raise RuntimeError('Unexpected exact-profile outcome; inspect actual results')

try:
    print('Evidence root: ' + str(ROOT), flush=True)
    main()
finally:
    (ROOT / 'results.json').write_text(json.dumps(RESULTS, indent=2) + '\n')
    for path in ROOT.iterdir():
        if path.is_dir():
            shutil.rmtree(path)
    (ROOT / 'probe').unlink(missing_ok=True)
    VALIDATION['cleaned'] = True
    (ROOT / 'validation.json').write_text(json.dumps(VALIDATION, indent=2) + '\n')
    # Retain logs and inert policy text only; all synthetic HOME/config/fixtures
    # and compiled executables have been deleted. No registration was made.
    print('Retained synthetic evidence: ' + str(ROOT), flush=True)
