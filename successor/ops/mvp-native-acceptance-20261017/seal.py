"""Seal committed local source and existing native artifacts without activation."""
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[3]
DIRECTORY = Path(__file__).resolve().parent
PRODUCT = ['successor/contracts', 'successor/runner', 'successor/service', 'successor/prototype',
           'successor/apple', 'successor/package.json', 'successor/package-lock.json']
NATIVE_SOURCE = '996e19849f5104e71b26b7c14d201aaa56b31c87'


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def git(*args):
    return subprocess.check_output(['git', *args], cwd=ROOT, text=True).strip()


if __name__ == '__main__':
    assert not git('diff', 'HEAD', '--name-only', '--', *PRODUCT), 'Commit product changes before sealing'
    assert not git('ls-files', '--others', '--exclude-standard', '--', *PRODUCT), 'Untracked product source requires review'
    assert not git('diff', NATIVE_SOURCE, '--name-only', '--', 'successor/apple'), 'Native source changed; build new artifacts'
    builds = json.loads((ROOT / 'successor/evidence/2026-10-17-mvp-memory-review/builds.json').read_text())
    assert builds['sourceCommit'] == NATIVE_SOURCE
    app = next(item for item in builds['artifacts'] if item['path'].endswith('/Debug/FortMac.app'))
    for file in app['files']:
        assert digest(Path(app['path']) / file['path']) == file['sha256'], 'App bytes changed'
    source_files = [{'path': path, 'sha256': digest(ROOT / path)} for path in git('ls-files', '--', *PRODUCT).splitlines()]
    preserved = json.loads((ROOT / 'successor/evidence/2026-10-17-mvp-recovery/preservation.json').read_text())
    preserved_files = []
    for group in preserved['attempts']:
        for file in group['files']:
            path = Path(group['root']) / file['path']
            assert digest(path) == file['sha256'], 'Historical packet changed'
            preserved_files.append({'path': str(path), 'sha256': file['sha256']})
    pins = {'schemaVersion': 1, 'sourceCommit': git('rev-parse', 'HEAD'), 'nativeSourceCommit': NATIVE_SOURCE,
            'scopeSHA256': digest(ROOT / 'successor/ops/mvp-native-test-scope-20261017.json'),
            'appPath': app['path'], 'sourceFiles': source_files, 'appFiles': app['files'],
            'preservedFiles': preserved_files, 'activationAuthorized': False}
    with (DIRECTORY / 'pins.json').open('x') as output:
        output.write(json.dumps(pins, indent=2) + '\n')
    print(json.dumps({'sourceCommit': pins['sourceCommit'], 'sourceFiles': len(source_files),
                      'appPath': pins['appPath'], 'activationAuthorized': False}))
