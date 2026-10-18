import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import time

repo = Path('/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016')
destination = repo / 'successor/.build/performance-native-059-20261018-a'
destination.mkdir(mode=0o700)
source = destination / 'source'
source.mkdir()

def run(args, log=None, timeout=300):
    started = time.perf_counter()
    process = subprocess.Popen(args, cwd=source, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, start_new_session=True)
    try:
        output, _ = process.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGTERM)
        try:
            output, _ = process.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            output, _ = process.communicate(timeout=5)
        if log:
            log.write_text(output)
        raise RuntimeError('Owned build command timed out')
    elapsed = (time.perf_counter() - started) * 1000
    if log:
        log.write_text(output)
    if process.returncode:
        raise RuntimeError('Build command failed; inspect ' + str(log))
    return elapsed, output.strip()

tracked = subprocess.check_output(['git', 'ls-files', 'successor/apple'], cwd=repo, text=True).splitlines()
inputs = []
for name in tracked:
    relative = Path(name).relative_to('successor/apple')
    if not (str(relative).startswith(('Apps/', 'FortKit/Sources/', 'FortKit/Tests/', 'UITests/')) or str(relative) in ('FortKit/Package.swift', 'project.yml')):
        continue
    original = repo / name
    target = source / relative
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(original, target)
    inputs.append({'path': name, 'sha256': hashlib.sha256(original.read_bytes()).hexdigest()})
shutil.copyfile(__file__, destination / 'measure.py')
run(['/opt/homebrew/bin/xcodegen', 'generate', '--no-env', '--spec', str(source / 'project.yml'), '--project', str(source), '--project-root', str(source)], destination / 'generate.log')

def stats(values):
    return {'samplesMs': values, 'p50Ms': sorted(values)[2], 'p95Ms': max(values)}

report = {
    'format': 1,
    'sourceCommit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=repo, text=True).strip(),
    'sourceFiles': inputs,
    'scriptSha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
    'toolchain': {
        'xcode': run(['/usr/bin/xcodebuild', '-version'])[1],
        'swift': run(['/usr/bin/xcrun', 'swift', '--version'])[1],
        'xcodegen': run(['/opt/homebrew/bin/xcodegen', '--version'])[1],
        'macOS': run(['/usr/bin/sw_vers'])[1],
        'hardware': run(['/usr/sbin/sysctl', 'hw.model', 'hw.memsize', 'hw.ncpu'])[1],
    },
    'conditions': {
        'configuration': 'Debug, arm64, CODE_SIGNING_ALLOWED=NO',
        'clean': 'Five fresh DerivedData directories per platform, copied source, OS file caches uncontrolled; no cache purge.',
        'incremental': 'Five window-title string edits in the copied app source, last clean DerivedData reused.',
        'scope': 'Native client apps only. No bundled Node host, real provider, install, launch, archive, upload or TestFlight claim.',
        'percentile': 'Nearest rank; p95 of five samples is the maximum.',
    },
    'platforms': {},
}
editable = source / 'Apps/FortApp.swift'
original = editable.read_text()
marker = 'return "Fort"'
if original.count(marker) != 1:
    raise RuntimeError('Review changed incremental edit marker')
for name, scheme, target, products in [
    ('mac', 'FortMac', 'platform=macOS,arch=arm64', 'Debug'),
    ('iphoneSimulator', 'FortiOS', 'generic/platform=iOS Simulator', 'Debug-iphonesimulator'),
]:
    editable.write_text(original)
    clean, incremental = [], []
    for index in range(5):
        print(json.dumps({'phase': name + '-clean', 'sample': index + 1}), flush=True)
        derived = destination / (name + '-clean-' + str(index + 1))
        args = ['/usr/bin/xcodebuild', '-project', str(source / 'FortSpec058.xcodeproj'), '-scheme', scheme, '-configuration', 'Debug', '-destination', target, '-derivedDataPath', str(derived), '-disableAutomaticPackageResolution', '-skipPackageUpdates', 'CODE_SIGNING_ALLOWED=NO', 'build']
        clean.append(run(args, destination / (name + '-clean-' + str(index + 1) + '.log'))[0])
    app = derived / 'Build/Products' / products / (scheme + '.app')
    preserved = destination / (name + '-clean.app')
    shutil.copytree(app, preserved, symlinks=True)
    for index in range(5):
        print(json.dumps({'phase': name + '-incremental', 'sample': index + 1}), flush=True)
        editable.write_text(original.replace(marker, 'return "Fort ' + str(index + 1) + '"'))
        incremental.append(run(args, destination / (name + '-incremental-' + str(index + 1) + '.log'))[0])
    report['platforms'][name] = {'clean': stats(clean), 'incremental': stats(incremental), 'cleanAppPath': str(preserved),
        'files': [{'path': str(path.relative_to(preserved)), 'bytes': path.stat().st_size, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()} for path in sorted(preserved.rglob('*')) if path.is_file() and not path.is_symlink()],
        'budgets': {'cleanBuild': max(clean) <= 120_000, 'incrementalBuild': max(incremental) <= 20_000}}
    (destination / 'report.json').write_text(json.dumps(report, indent=2) + '\n')
editable.write_text(original)
print(json.dumps({'reportPath': str(destination / 'report.json'), 'budgets': {name: value['budgets'] for name, value in report['platforms'].items()}}), flush=True)
