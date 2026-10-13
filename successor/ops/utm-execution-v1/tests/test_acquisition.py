import hashlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from fort_probe.acquisition import Acquisition,LocalStore,fixed_plan
from support import Clock

class Response:
    def __init__(self,body=b'fixture',status=200,headers=None):
        self.status=status;self.headers=headers or [];self.body=io.BytesIO(body);self.closed=False
    def set_timeout(self,seconds):self.timeout=seconds
    def read(self,n):return self.body.read(n)
    def close(self):self.closed=True
class Transport:
    def __init__(self,responses):self.responses=list(responses);self.calls=[]
    def request(self,url,timeout):
        self.calls.append((url,timeout))
        item=self.responses.pop(0)
        if isinstance(item,Exception):raise item
        return item

def entry(name='first',data=b'fixture',cap=32):
    return {'name':name,'url':'https://registry.npmjs.org/'+name,'sha256':hashlib.sha256(data).hexdigest(),'max_bytes':cap}
POLICY={'allowed_hosts':['registry.npmjs.org'],'maximum_redirects':4,'socket_timeout_seconds':30,
        'per_input_total_timeout_seconds':600,'automatic_retry':False}

class AcquisitionTests(unittest.TestCase):
    def run_case(self,responses,entries=None,clock=None):
        temporary=tempfile.TemporaryDirectory();self.addCleanup(temporary.cleanup)
        root=Path(temporary.name).resolve()/'owned';store=LocalStore(root)
        transport=Transport(responses)
        acquire=Acquisition(entries or [entry()],POLICY,transport,store,clock or Clock())
        return acquire,root,transport
    def test_fixed_inventory_excludes_utm_and_retains_exact_caps(self):
        entries,policy=fixed_plan();self.assertEqual(len(entries),8)
        self.assertNotIn('UTM.dmg',[x['name'] for x in entries]);self.assertEqual(sum(x['max_bytes'] for x in entries),814088192)
    def test_pinned_bytes_are_published_only_after_eof_hash_and_receipt(self):
        response=Response(headers=[('Content-Length','7')]);acquire,root,transport=self.run_case([response])
        result=acquire.run();self.assertEqual((root/'first').read_bytes(),b'fixture')
        self.assertTrue(response.closed);self.assertEqual(result['completed'],1)
        self.assertEqual(json.loads((root/'complete.json').read_bytes())['status'],'complete')
        with self.assertRaises(FileExistsError):LocalStore(root)
        with self.assertRaises(ValueError):acquire.run()
    def test_redirects_stay_bounded_https_public_and_do_not_forward_credentials(self):
        acquire,root,transport=self.run_case([Response(status=302,headers=[('Location','/next?opaque=public')]),Response()])
        acquire.run();self.assertEqual(len(transport.calls),2)
        self.assertNotIn('opaque',(root/'receipt-first.json').read_text())
        for url in ('http://registry.npmjs.org/x','https://evil.example/x','https://user:secret@registry.npmjs.org/x','https://registry.npmjs.org:444/x'):
            acquire,root,transport=self.run_case([Response(status=302,headers=[('Location',url)])])
            with self.subTest(url=url),self.assertRaises(ValueError):acquire.run()
            self.assertEqual(len(transport.calls),1);self.assertFalse((root/'complete.json').exists())
    def test_failed_http_hash_length_or_cap_vetoes_acceptance_and_never_fetches_next(self):
        cases=[Response(status=503),Response(b'bad'),Response(headers=[('Content-Length','100')]),
               Response(headers=[('Content-Length','8')]),Response(headers=[('Content-Encoding','gzip')]),
               Response(headers=[('Content-Length','7'),('Content-Length','7')]),Response(b'x'*33),TimeoutError('sensitive upstream text')]
        for response in cases:
            acquire,root,transport=self.run_case([response,Response()],entries=[entry(),entry('second')])
            with self.subTest(response=response),self.assertRaises((ValueError,TimeoutError)):acquire.run()
            self.assertEqual(len(transport.calls),1);self.assertFalse((root/'first').exists());self.assertFalse((root/'complete.json').exists())
            self.assertNotIn('sensitive',(root/'failure.json').read_text())
    def test_slow_stream_and_redirect_loops_stop_with_no_retry(self):
        clock=Clock()
        class Slow(Response):
            def read(self,n):clock.advance(601);return super().read(n)
        for responses in ([Slow()],[Response(status=302,headers=[('Location','/first')])]):
            acquire,root,transport=self.run_case(responses,clock=clock)
            with self.assertRaises((ValueError,TimeoutError)):acquire.run()
            self.assertEqual(len(transport.calls),1);self.assertFalse((root/'complete.json').exists())

    def test_receipt_failure_preserves_failure_veto_and_stops_after_first_input(self):
        acquire,root,transport=self.run_case([Response(),Response()],entries=[entry(),entry('second')])
        record=acquire.store.record
        def fail_receipt(name,value,**kwargs):
            if name.startswith('receipt-'):raise OSError('synthetic disk fault')
            return record(name,value,**kwargs)
        acquire.store.record=fail_receipt
        with self.assertRaises(OSError):acquire.run()
        self.assertEqual(len(transport.calls),1);self.assertTrue((root/'failure.json').exists())
        self.assertFalse((root/'complete.json').exists())

    def test_replaced_partial_is_preserved_and_never_published(self):
        acquire,root,transport=self.run_case([])
        class Changed(Response):
            def read(self,n):
                data=super().read(n)
                if not data:
                    (root/'first.partial').unlink();(root/'first.partial').write_bytes(b'unowned replacement')
                return data
        transport.responses=[Changed()]
        with self.assertRaises(ValueError):acquire.run()
        self.assertEqual((root/'first.partial').read_bytes(),b'unowned replacement')
        self.assertFalse((root/'first').exists());self.assertFalse((root/'complete.json').exists())
