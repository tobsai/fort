import select
import socket
import tempfile
import threading
import unittest
from fort_probe.broker import Broker
from fort_probe.controller import Controller
from fort_probe.serial import Exchange,encode
from fort_probe.host_session import run
from support import BINDING,Clock,GuestFixture


class DriverTests(unittest.TestCase):
    def test_host_driver_completes_one_grant_and_both_stop_phases_over_two_channels(self):
        with tempfile.TemporaryDirectory(prefix='fort-utm-fixture-') as root:
            clock,guest=Clock(),GuestFixture();errors=[]
            pairs={name:socket.socketpair() for name in ('control','inspection')}
            def guest_side():
                try:
                    with Controller(root,BINDING,guest,clock) as controller:
                        broker=Broker(controller,clock)
                        while True:
                            readers,_,_=select.select([pair[1] for pair in pairs.values()],[],[],.01)
                            for name,pair in pairs.items():
                                if pair[1] in readers:
                                    data=pair[1].recv(4096)
                                    if not data:return
                                    for channel,response in broker.receive(name,data):pairs[channel][1].sendall(encode(response))
                            for channel,response in broker.tick():pairs[channel][1].sendall(encode(response))
                except Exception as error:errors.append(repr(error))
            worker=threading.Thread(target=guest_side);worker.start()
            try:
                result=run(Exchange(pairs['control'][0].fileno()),Exchange(pairs['inspection'][0].fileno()),BINDING,clock,clock.advance)
                self.assertTrue(result['stop']['live_evidence_proven'])
                self.assertTrue(guest.current()['empty'])
            finally:
                for left,right in pairs.values():left.shutdown(socket.SHUT_RDWR)
                worker.join(2)
                for left,right in pairs.values():left.close();right.close()
            self.assertFalse(worker.is_alive());self.assertEqual(errors,[])
