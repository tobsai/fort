"""Read-only preparation check. This program cannot activate a test session."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
NATIVE_SOURCE = '996e19849f5104e71b26b7c14d201aaa56b31c87'
PIN_FILE = Path(__file__).with_name('pins.json')
EVIDENCE = ROOT / 'successor/evidence/2026-10-17-mvp-memory-review'
SCOPE = ROOT / 'successor/ops/mvp-native-test-scope-20261017.json'
PRODUCT_PATHS = ['successor/contracts', 'successor/runner', 'successor/service',
                 'successor/prototype', 'successor/apple']


def require(condition, reason):
    if not condition:
        raise ValueError(reason)


def verify_file(base, item):
    relative = Path(item['path'])
    require(not relative.is_absolute() and '..' not in relative.parts, 'Unsafe recorded path')
    path = base / relative
    require(path.is_file() and not path.is_symlink(), 'Missing or replaced file: ' + str(path))
    require(path.stat().st_size == item['bytes'], 'File size changed: ' + str(path))
    require(hashlib.sha256(path.read_bytes()).hexdigest() == item['sha256'], 'File bytes changed: ' + str(path))


def verify_scope(scope):
    require(scope['scopeId'] == 'mvp-native-tests-20261017', 'Unexpected scope identity')
    for field, expected in {'maximumSessions': 3, 'sessionSeconds': 600, 'workSeconds': 540,
                            'cleanupReserveSeconds': 60, 'maximumFreshMacEnrollmentsPerSession': 1,
                            'maximumEnrollmentsTotal': 3, 'maximumPlainTextTurnsPerSession': 12,
                            'maximumPlainTextTurnsTotal': 36, 'renewExistingCredential': False,
                            'reenrollmentOfConsumedInstallation': False}.items():
        require(scope[field] == expected, 'Scope changed: ' + field)
    require(scope['status'] == 'proposed-awaiting-owner-approval', 'Reconcile changed approval state before using this preparation record')
    require(scope['provider']['selectedModel'] == 'gpt-6-astra', 'Model changed')
    require(scope['provider']['runtimeVersion'] == '0.157.1', 'Runtime version changed')
    require(scope['permissions']['profile'] == 'fort_candidate_b_no_files' and scope['permissions']['tools'] is False, 'Permission profile changed')
    require(scope['restartBounds'] == {'appRelaunch': True, 'explicitServiceRuntimeRestartsPerSession': 1,
            'ownedRuntimeLaunchesPerSession': 3, 'secondsPerOwnedRuntimeLaunch': 300,
            'sharedAbsoluteSessionDeadline': True}, 'Restart scope changed')
    expected = [f'/Users/tobiasgunn/dev/fort/.local/mvp-native-tests-20261017/session-{n}' for n in range(1, 4)]
    require(scope['preservation']['newRoots'] == expected, 'Session roots changed')


def check():
    require(hashlib.sha256(SCOPE.read_bytes()).hexdigest() == '24232f4c96da97a3fa2b64620bedcc4752332d17e111bc8c7d583eac26bdf4f6', 'Scope bytes changed; reconcile the proposal before acceptance')
    scope = json.loads(SCOPE.read_text())
    pins = json.loads(PIN_FILE.read_text())
    source = pins['sourceCommit']
    verify_scope(scope)
    builds = json.loads((EVIDENCE / 'builds.json').read_text())
    require(builds['sourceCommit'] == NATIVE_SOURCE and builds['appsLaunched'] is False, 'Unexpected build evidence')
    subprocess.run(['git', 'merge-base', '--is-ancestor', source, 'HEAD'], cwd=ROOT, check=True, capture_output=True)
    changed = subprocess.check_output(['git', 'diff', '--name-only', source, '--', *PRODUCT_PATHS], cwd=ROOT, text=True)
    require(not changed.strip(), 'Product source differs from the tested build; reseal before acceptance')
    untracked = subprocess.check_output(['git', 'ls-files', '--others', '--exclude-standard', '--', *PRODUCT_PATHS], cwd=ROOT, text=True)
    require(not untracked.strip(), 'Untracked product files require review')
    for artifact in builds['artifacts']:
        base = Path(artifact['path'])
        require(base.is_dir() and not base.is_symlink(), 'Missing build artifact')
        for item in artifact['files']:
            verify_file(base, item)
        actual = sorted(str(p.relative_to(base)) for p in base.rglob('*') if p.is_file())
        require(actual == sorted(item['path'] for item in artifact['files']), 'Artifact file set changed')
    baseline = json.loads((ROOT / 'successor/evidence/2026-10-17-mvp-recovery/preservation.json').read_text())
    for group in baseline['attempts'] + [baseline['nativeCache']]:
        base = Path(group['root'])
        for item in group['files']:
            verify_file(base, item)
            require((base / item['path']).stat().st_mtime_ns == item['mtimeNS'], 'Historical file modification time changed')
    cache = baseline['nativeCache']
    actual_cache = sorted(str(p.relative_to(cache['root'])) for p in Path(cache['root']).rglob('*') if p.is_file())
    require(actual_cache == sorted(item['path'] for item in cache['files']), 'Native cache file set changed')
    for path in scope['preservation']['newRoots']:
        require(not Path(path).exists() and not Path(path).is_symlink(), 'Proposed session root already exists; never reset it')
    runtime = Path(scope['provider']['runtimePath'])
    require(runtime.is_file(), 'Pinned runtime file is missing')
    require(hashlib.sha256(runtime.read_bytes()).hexdigest() == scope['provider']['runtimeSHA256'], 'Pinned runtime bytes changed')
    subprocess.run(['/opt/homebrew/opt/node@26/bin/node', 'successor/prototype/mvp-manual.ts', '--check'], cwd=ROOT, check=True, capture_output=True)
    return {'preparationChecks': 'passed', 'sourceCommit': source, 'artifactsVerified': len(builds['artifacts']),
            'activation': 'blocked', 'blockers': ['Owner approval for the proposed temporary enrollment batch is pending.',
            'The actual host may require platform permission prompts for approved execution.'],
            'acceptanceUnrun': ['installed accessibility/screenshots', 'current real-provider behavior', 'physical iPhone'],
            'liveActionsPerformed': False, 'credentialsReadOrCreated': False, 'launcher': 'implemented and sealed; no activation',
            'checklist': str(Path(__file__).with_name('PLAN.md'))}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='Verify prepared bytes and preserved state; never launch')
    parser.parse_args()
    try:
        print(json.dumps(check(), indent=2))
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print(json.dumps({'preparationChecks': 'failed', 'activation': 'blocked', 'reason': str(error)}))
        sys.exit(2)
