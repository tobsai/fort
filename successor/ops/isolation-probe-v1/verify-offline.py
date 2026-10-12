"""Future approved verifier invocation. No library is installed or invoked now."""
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent
APP = '/Users/tobiasgunn/Library/Application Support/FortIsolation'
NODE = '/opt/homebrew/opt/node@26/bin/node'


def main():
    if sys.argv[1:] not in ([], ['--registry-only']):
        raise SystemExit('Only the optional --registry-only mode is accepted')
    args = [NODE, '--permission', '--no-addons', '--max-old-space-size=256',
            '--allow-fs-read=' + str(ROOT), '--allow-fs-read=' + APP + '/downloads',
            '--allow-fs-read=' + APP + '/tools/openpgp/6.3.2', str(ROOT / 'verify-inputs.mjs'), *sys.argv[1:]]
    env = {'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C',
           'HOME': APP + '/home', 'TMPDIR': APP + '/tmp'}
    result = subprocess.run(args, env=env, close_fds=True, stdin=subprocess.DEVNULL,
                            timeout=120, check=False)
    raise SystemExit(result.returncode)


if __name__ == '__main__':
    main()
