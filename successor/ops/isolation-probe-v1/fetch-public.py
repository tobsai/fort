"""Future approved downloads only: fixed URLs, redirect hosts, byte caps and digests."""
import hashlib
import json
import os
import pathlib
import ssl
import stat
import urllib.parse
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parent


def main():
    scope = json.loads((ROOT / 'scope.json').read_text())
    target = pathlib.Path(scope['app_root']) / 'downloads'
    allowed = set(scope['host_https_domains'])
    for path in (target, target.parent):
        info = path.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid() or info.st_mode & 0o077:
            raise RuntimeError('Fresh private task-owned directory required')
    # Fixed system public CA bundle, not an ambient SSL_CERT_FILE or Keychain identity.
    context = ssl.create_default_context(cafile='/etc/ssl/cert.pem')

    def check_url(url):
        parsed = urllib.parse.urlsplit(url)
        if parsed.scheme != 'https' or parsed.hostname not in allowed or parsed.port not in (None, 443):
            raise RuntimeError('Unlisted download destination')
        if parsed.username or parsed.password or parsed.fragment:
            raise RuntimeError('Authenticated or ambiguous URL refused')

    class Redirects(urllib.request.HTTPRedirectHandler):
        count = 0

        def redirect_request(self, request, response, code, message, headers, newurl):
            self.count += 1
            if self.count > 4:
                raise RuntimeError('Redirect limit exceeded')
            check_url(newurl)
            return super().redirect_request(request, response, code, message, headers, newurl)

    os.umask(0o077)
    receipts = []
    for entry in scope['downloads']:
        name = entry['name']
        if pathlib.PurePath(name).name != name:
            raise RuntimeError('Invalid fixed filename')
        final = target / name
        partial = target / (name + '.partial')
        if final.exists() or final.is_symlink() or partial.exists() or partial.is_symlink():
            raise RuntimeError('No overwrite/resume: reconcile existing download')
        check_url(entry['url'])
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), Redirects(),
                                             urllib.request.HTTPSHandler(context=context))
        opener.addheaders = [('User-Agent', 'Fort-static-isolation-probe/1')]
        algorithm = 'sha512' if 'sha512' in entry else 'sha256'
        digest = hashlib.new(algorithm)
        size = 0
        with partial.open('xb') as output:
            try:
                with opener.open(entry['url'], timeout=30) as response:
                    check_url(response.url)
                    length = response.headers.get('Content-Length')
                    if length and int(length) > entry['max_bytes']:
                        raise RuntimeError('Declared byte cap exceeded')
                    while True:
                        chunk = response.read(65536)
                        if not chunk:
                            break
                        size += len(chunk)
                        if size > entry['max_bytes']:
                            raise RuntimeError('Streaming byte cap exceeded')
                        output.write(chunk)
                        digest.update(chunk)
                    final_host = urllib.parse.urlsplit(response.url).hostname
                if digest.hexdigest() != entry[algorithm]:
                    raise RuntimeError('Pinned checksum mismatch')
                output.flush()
                os.fsync(output.fileno())
            except Exception:
                partial.unlink()
                raise
        os.rename(partial, final)
        receipts.append({'name': name, 'bytes': size, algorithm: digest.hexdigest(),
                         'final_host': final_host, 'publisher_signature_verified': False})
    print(json.dumps(receipts, indent=2))


if __name__ == '__main__':
    main()
