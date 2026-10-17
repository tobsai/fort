"""Read only cleanup metadata from an already checkpointed, stopped ledger."""
import hashlib
import json
from pathlib import Path
import sqlite3
import sys


def inspect(path):
    if not path.is_absolute() or path.resolve() != path:
        raise ValueError('Use the exact ledger path')
    files = [path, Path(str(path) + '-wal'), Path(str(path) + '-shm')]

    def snapshot():
        result = []
        for file in files:
            if file.is_symlink():
                raise ValueError('Ledger symlink refused')
            if file.exists():
                stat = file.stat()
                result.append((file.name, stat.st_size, stat.st_mtime_ns,
                               hashlib.sha256(file.read_bytes()).hexdigest()))
        return result

    before = snapshot()
    if not path.is_file() or (files[1].exists() and files[1].stat().st_size != 0):
        raise ValueError('Ledger must already be checkpointed; do not modify it')
    connection = sqlite3.connect(path.as_uri() + '?mode=ro&immutable=1', uri=True)
    try:
        installation = connection.execute("SELECT json_extract(data, '$.installationId') FROM metadata WHERE key='installation'").fetchone()[0]
        deadline = connection.execute("SELECT json_extract(data, '$') FROM metadata WHERE key='credentialDeadline'").fetchone()[0]
        result = {'installationId': installation, 'credentialDeadline': deadline,
                  'devices': connection.execute('SELECT COUNT(*) FROM devices').fetchone()[0],
                  'enrollments': connection.execute('SELECT COUNT(*) FROM enrollment').fetchone()[0],
                  'runs': [{'runId': row[0], 'state': row[1]} for row in connection.execute(
                      "SELECT json_extract(data, '$.runId'), json_extract(data, '$.state') FROM entities WHERE kind='runs'")]}
    finally:
        connection.close()
    if snapshot() != before:
        raise ValueError('Ledger changed during read-only inspection')
    return result


if __name__ == '__main__':
    try:
        print(json.dumps(inspect(Path(sys.argv[1]))))
    except Exception:
        sys.exit('Read-only ledger cleanup inspection refused')
