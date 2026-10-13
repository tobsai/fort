"""C06 positive controls on one root-owned listener. No data or commands are sent."""
import errno
import os
import socket
import stat
import time


def _connect(path):
    before = os.lstat(path)
    if not stat.S_ISSOCK(before.st_mode) or before.st_uid != os.getuid():
        raise ValueError('expected owned Unix socket')
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
        client.settimeout(.15)
        client.connect(path)
    after = os.lstat(path)
    if (before.st_dev, before.st_ino) != (after.st_dev, after.st_ino):
        raise ValueError('listener replaced during root connection')
    return {'device': before.st_dev, 'inode': before.st_ino, 'uid': before.st_uid, 'connected': True}


def prove_denial(path, tool_probe, hidden=False):
    """The trusted broker holds this listener open throughout the bounded callback."""
    started = time.monotonic()
    before = _connect(path)
    result = tool_probe()
    after = _connect(path)
    if time.monotonic() - started >= .5:
        raise ValueError('IPC observation deadline exceeded')
    accept_denial(before, after, result, hidden)
    return {'passed': True, 'before': before, 'after': after, 'tool': result, 'deliberately_hidden': hidden}


def accept_denial(before, after, result, hidden=False):
    allowed = (errno.EACCES, errno.EPERM, errno.ENOENT) if hidden else (errno.EACCES, errno.EPERM)
    if (set(before) != {'device','inode','uid','connected'} or before['connected'] is not True
            or before != after or not isinstance(result, dict)
            or set(result) != {'connected', 'errno'} or result['connected'] is not False
            or type(result['errno']) is not int or result['errno'] not in allowed):
        raise ValueError('IPC permission denial not proven')
