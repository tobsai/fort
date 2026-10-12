"""Fixed public downloads with bounded, inert diagnostics; never retry or execute."""
import datetime
import hashlib
import json
import os
import pathlib
import ssl
import stat
import sys
import urllib.error
import urllib.parse
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parent
RECEIPT_MAX_BYTES = 16384
BODY_MAX_BYTES = 65536
MAX_INPUTS = 12
PUBLIC_KEY_NAMES = {'lima.pub', 'ubuntu.pub'}


class DownloadFailure(RuntimeError):
    pass


def private_directory(path, parent=None, create=False, exclusive=False):
    if create:
        try:
            os.mkdir(path, 0o700, dir_fd=parent)
        except FileExistsError:
            if exclusive:
                raise
        else:
            os.fsync(parent)
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
    info = os.fstat(descriptor)
    if info.st_uid != os.getuid() or info.st_mode & 0o077:
        os.close(descriptor)
        raise DownloadFailure('private-task-owned-directory-required')
    return descriptor


def write_new(directory, name, data):
    descriptor = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=directory)
    with os.fdopen(descriptor, 'wb') as output:
        output.write(data)
        output.flush()
        os.fsync(output.fileno())
    os.fsync(directory)


def response_metadata(url, status, headers, original_url):
    parsed = urllib.parse.urlsplit(url)
    # Signed redirect query strings and arbitrary response headers are not evidence.
    safe_url = urllib.parse.urlunsplit((parsed.scheme, parsed.hostname or '', parsed.path, '', ''))
    return {'status': status, 'final_url': (url if url == original_url else safe_url)[:1024],
            'final_url_query_omitted': url != original_url and bool(parsed.query),
            'content_type': headers.get('Content-Type', '')[:256],
            'content_length': headers.get('Content-Length', '')[:64]}


def download(entry, index, target, evidence, allowed, context):
    name = entry['name']
    partial = name + '.partial'
    algorithm = 'sha512' if 'sha512' in entry else 'sha256'
    digest = hashlib.new(algorithm)
    size = 0
    complete = False
    http = None
    body = bytearray()
    retain_body = name in PUBLIC_KEY_NAMES and entry['kind'] == 'public-verification-metadata'
    receipt_name = '%02d-%s.json' % (index, name)
    owned_identity = None
    promoted = False

    def check_url(url):
        parsed = urllib.parse.urlsplit(url)
        if parsed.scheme != 'https' or parsed.hostname not in allowed or parsed.port not in (None, 443):
            raise DownloadFailure('unlisted-download-destination')
        if parsed.username or parsed.password or parsed.fragment:
            raise DownloadFailure('authenticated-or-ambiguous-url')

    class Redirects(urllib.request.HTTPRedirectHandler):
        count = 0

        def redirect_request(self, request, response, code, message, headers, newurl):
            nonlocal http
            http = response_metadata(request.full_url, code, headers, entry['url'])
            self.count += 1
            try:
                if self.count > 4:
                    raise DownloadFailure('redirect-limit-exceeded')
                check_url(newurl)
                return super().redirect_request(request, response, code, message, headers, newurl)
            except Exception:
                response.close()
                raise

    def receipt(outcome, failure=None):
        return {'schema': 1, 'input_name': name, 'input_url': entry['url'],
                'outcome': outcome, 'failure': failure,
                'algorithm': algorithm, 'expected_digest': entry[algorithm],
                'observed_digest': digest.hexdigest(), 'observed_bytes': size,
                'body_complete': complete, 'max_bytes': entry['max_bytes'], 'http': http,
                'untrusted_body': None, 'publisher_signature_verified': False,
                'recorded_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat()}

    def record(value):
        data = (json.dumps(value, indent=2) + '\n').encode()
        if len(data) > RECEIPT_MAX_BYTES:
            raise DownloadFailure('receipt-byte-cap-exceeded')
        write_new(evidence, receipt_name, data)

    def remove_owned(filename):
        info = os.stat(filename, dir_fd=target, follow_symlinks=False)
        if not stat.S_ISREG(info.st_mode) or (info.st_dev, info.st_ino) != owned_identity:
            raise DownloadFailure('download-ownership-changed')
        os.unlink(filename, dir_fd=target)

    try:
        check_url(entry['url'])
        descriptor = os.open(partial, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=target)
        with os.fdopen(descriptor, 'wb') as output:
            info = os.fstat(output.fileno())
            owned_identity = (info.st_dev, info.st_ino)
            opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), Redirects(),
                                                 urllib.request.HTTPSHandler(context=context))
            opener.addheaders = [('User-Agent', 'Fort-static-isolation-probe/1')]
            try:
                response = opener.open(entry['url'], timeout=30)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                http = response_metadata(response.url, response.code, response.headers, entry['url'])
                check_url(response.url)
                length_text = response.headers.get('Content-Length')
                length = int(length_text) if length_text is not None else None
                if length is not None and (length < 0 or length > entry['max_bytes']):
                    raise DownloadFailure('declared-byte-cap-exceeded')
                if response.code != 200 and not retain_body:
                    raise DownloadFailure('unexpected-http-status')
                while True:
                    # At most one extra byte diagnoses overflow; no oversized chunk is written.
                    chunk = response.read(min(65536, entry['max_bytes'] - size + 1))
                    if not chunk:
                        complete = True
                        break
                    size += len(chunk)
                    digest.update(chunk)
                    if retain_body:
                        body.extend(chunk[:max(0, min(BODY_MAX_BYTES, entry['max_bytes']) - len(body))])
                    if size > entry['max_bytes']:
                        raise DownloadFailure('streaming-byte-cap-exceeded')
                    output.write(chunk)
                if response.code != 200:
                    raise DownloadFailure('unexpected-http-status')
                if length is not None and size != length:
                    complete = False
                    raise DownloadFailure('content-length-mismatch')
            if digest.hexdigest() != entry[algorithm]:
                raise DownloadFailure('pinned-checksum-mismatch')
            output.flush()
            os.fsync(output.fileno())
        # No replacement of an existing file, including a racing symlink.
        os.link(partial, name, src_dir_fd=target, dst_dir_fd=target, follow_symlinks=False)
        promoted = True
        record(receipt('accepted'))
        remove_owned(partial)
        owned_identity = None
        os.fsync(target)
    except Exception as error:
        failure = str(error) if isinstance(error, DownloadFailure) else type(error).__name__
        value = receipt('failed', failure)
        value['cleanup_errors'] = []
        if owned_identity is not None:
            for filename in ([name] if promoted else []) + [partial]:
                try:
                    remove_owned(filename)
                except Exception as cleanup_error:
                    value['cleanup_errors'].append({'file': filename, 'error': type(cleanup_error).__name__})
            try:
                os.fsync(target)
            except Exception as cleanup_error:
                value['cleanup_errors'].append({'file': 'downloads/', 'error': type(cleanup_error).__name__})
        try:
            if body:
                body_name = '%02d-%s.untrusted.bin' % (index, name)
                write_new(evidence, body_name, bytes(body))
                value['untrusted_body'] = {'file': body_name, 'bytes': len(body),
                    'sha256': hashlib.sha256(body).hexdigest(),
                    'prefix_only': not complete or len(body) != size,
                    'trusted': False, 'executable': False}
            record(value)
        except Exception as evidence_error:
            # Do not hide available metadata if storage fails. Never print the body.
            print(json.dumps(value), file=sys.stderr, flush=True)
            raise DownloadFailure('Download stopped: %s; diagnostic write failed (%s)' %
                                  (name, type(evidence_error).__name__)) from None
        raise DownloadFailure('Download stopped: %s (%s); cleanup errors: %d' %
                              (name, failure, len(value['cleanup_errors']))) from None


def main():
    scope = json.loads((ROOT / 'scope.json').read_text())
    entries = scope['downloads']
    if not 1 <= len(entries) <= MAX_INPUTS or len({entry['name'] for entry in entries}) != len(entries):
        raise DownloadFailure('invalid-fixed-download-list')
    for entry in entries:
        name = entry['name']
        if not name or name in ('.', '..') or len(name) > 128 or any(character not in
                'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-' for character in name):
            raise DownloadFailure('invalid-fixed-filename')
    os.umask(0o077)
    descriptors = []
    try:
        app = private_directory(scope['app_root'])
        descriptors.append(app)
        target = private_directory('downloads', app)
        descriptors.append(target)
        for entry in entries:
            for name in (entry['name'], entry['name'] + '.partial'):
                try:
                    os.stat(name, dir_fd=target, follow_symlinks=False)
                except FileNotFoundError:
                    continue
                raise DownloadFailure('no-overwrite-or-resume-reconcile-existing-download')
        parent = private_directory('evidence', app, create=True)
        descriptors.append(parent)
        evidence = private_directory('download-diagnostics', parent, create=True, exclusive=True)
        descriptors.append(evidence)
        # At most 12 x 16 KiB receipts plus one failed 64 KiB public-key prefix = 256 KiB.
        # This is part of the existing 128 MiB raw/copied-evidence allowance.
        context = ssl.create_default_context(cafile='/etc/ssl/cert.pem')
        for index, entry in enumerate(entries, 1):
            download(entry, index, target, evidence, set(scope['host_https_domains']), context)
        print(json.dumps({'downloads_completed': len(entries), 'publisher_signatures_verified': False}))
    finally:
        for descriptor in reversed(descriptors):
            os.close(descriptor)


if __name__ == '__main__':
    main()
