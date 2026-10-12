"""Future operation wrapper with a fixed instance and an explicit environment."""
import pathlib
import subprocess
import sys

PACKET = pathlib.Path(__file__).resolve().parent
APP = '/Users/tobiasgunn/Library/Application Support/FortIsolation'
STATE = '/Users/tobiasgunn/Library/FortIsolation/state'
MANAGER = APP + '/tools/lima/2.2.1/bin/limactl'
INSTANCE = 'fort-isolation-probe'
ACTIONS = {
    'validate': ['validate', str(PACKET / 'lima.yaml')],
    'create': ['create', '--name=' + INSTANCE, str(PACKET / 'lima.yaml')],
    'start': ['start', '--timeout=120s', INSTANCE],
    'inspect': ['list', '--json', INSTANCE],
    'stop': ['stop', INSTANCE],
    'force-stop': ['stop', '--force', INSTANCE],
}


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ACTIONS:
        raise SystemExit('Choose one reviewed action: ' + ', '.join(ACTIONS))
    env = {'PATH': APP + '/tools/lima/2.2.1/bin:/usr/bin:/bin:/usr/sbin:/sbin',
           'HOME': APP + '/home', 'LIMA_HOME': STATE, 'TMPDIR': APP + '/tmp',
           'XDG_CONFIG_HOME': APP + '/home/.config', 'XDG_CACHE_HOME': APP + '/cache',
           'LANG': 'C', 'LC_ALL': 'C', 'TZ': 'Etc/UTC'}
    # No token, proxy, agent socket, cloud config, CLI plugin, or QEMU path inherited.
    result = subprocess.run([MANAGER, *ACTIONS[sys.argv[1]]], env=env, close_fds=True, check=False)
    raise SystemExit(result.returncode)


if __name__ == '__main__':
    main()
