"""Open only an already-attributed PTY; no chmod/chown, discovery or fallback.

The host collector must first prove VM/PID/start identity and absence of competing
consumers. TIOCEXCL prevents future opens; it does not evict pre-existing readers.
"""
import fcntl
import os
import re
import stat
import termios
import tty
from .evidence import exact


def _fact(info):
    return {'device': info.st_dev, 'inode': info.st_ino, 'rdev': info.st_rdev,
            'uid': info.st_uid, 'mode': stat.S_IMODE(info.st_mode), 'kind': stat.S_IFMT(info.st_mode)}


class Kernel:
    def lstat(self, path): return _fact(os.lstat(path))
    def open(self, path): return os.open(path, os.O_RDWR | os.O_NOCTTY | os.O_NONBLOCK | os.O_NOFOLLOW)
    def fstat(self, fd): return _fact(os.fstat(fd))
    def exclusive(self, fd): fcntl.ioctl(fd, termios.TIOCEXCL)
    def settings(self, fd): return termios.tcgetattr(fd)
    def raw(self, fd): tty.setraw(fd, termios.TCSANOW)
    def restore(self, fd, settings): termios.tcsetattr(fd, termios.TCSANOW, settings)
    def close(self, fd): os.close(fd)


class OwnedPTY:
    def __init__(self, path, expected, kernel=None):
        self.kernel = kernel if kernel is not None else Kernel()
        self.path, self.expected = path, dict(expected)
        self.fd, self.settings = None, None
        if (not re.fullmatch(r'/dev/ttys[0-9]{3,}', path) or set(expected) != {'device', 'inode', 'rdev', 'uid', 'mode', 'kind'}
                or any(type(v) is not int for v in expected.values())
                or expected['kind'] != stat.S_IFCHR or expected['uid'] != 501 or expected['mode'] != 0o600):
            raise ValueError('exact owner-501 mode-0600 character PTY required')
        if not exact(self.kernel.lstat(path), self.expected): raise ValueError('PTY path identity mismatch')
        try:
            self.fd = self.kernel.open(path)
            if not exact(self.kernel.fstat(self.fd), self.expected): raise ValueError('PTY swapped during open')
            self.kernel.exclusive(self.fd)
            self.settings = self.kernel.settings(self.fd)
            self.kernel.raw(self.fd)
        except Exception:
            self.close()
            raise

    def __enter__(self): return self
    def __exit__(self, *args): self.close()

    def close(self):
        if self.fd is None: return
        try:
            if self.settings is not None:
                if not exact(self.kernel.fstat(self.fd), self.expected) or not exact(self.kernel.lstat(self.path), self.expected):
                    raise ValueError('PTY identity changed; do not restore another terminal')
                self.kernel.restore(self.fd, self.settings)
        finally:
            self.kernel.close(self.fd)
            self.fd = None
