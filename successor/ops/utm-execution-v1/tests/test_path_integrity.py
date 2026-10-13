"""Temporary-file races only; no live adapter or acquired input is used."""
import hashlib
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from fort_probe.acquisition import LocalStore
from fort_probe.boot import verify_boot_files
from test_review_r4_regressions import boot_files


class PathIntegrityTests(unittest.TestCase):
    def test_parent_replacement_during_creation_never_writes_an_attempt_marker(self):
        with tempfile.TemporaryDirectory() as temporary:
            base=Path(temporary).resolve();parent=base/'parent';parent.mkdir();root=parent/'attempt'
            mkdir=os.mkdir;swapped=False
            def replace(path,*args,**kwargs):
                nonlocal swapped
                result=mkdir(path,*args,**kwargs)
                if not swapped and Path(path).name=='attempt':
                    swapped=True;parent.rename(base/'detached');parent.mkdir()
                    (parent/'sentinel').write_bytes(b'unowned')
                return result
            with patch.object(os,'mkdir',replace),self.assertRaises((OSError,ValueError)):LocalStore(root)
            self.assertFalse(any(base.rglob('attempt.json')))
            self.assertEqual((parent/'sentinel').read_bytes(),b'unowned')

    def test_new_leaf_replacement_between_identity_capture_and_open_is_not_adopted(self):
        with tempfile.TemporaryDirectory() as temporary:
            base=Path(temporary).resolve();root=base/'attempt';real_open=os.open;swapped=False
            def replace(path,flags,*args,**kwargs):
                nonlocal swapped
                if not swapped and Path(path).name=='attempt':
                    swapped=True;root.rename(base/'detached');root.mkdir(mode=0o700)
                    (root/'sentinel').write_bytes(b'unowned')
                return real_open(path,flags,*args,**kwargs)
            with patch.object(os,'open',replace),self.assertRaises((OSError,ValueError)):LocalStore(root)
            self.assertFalse(any(base.rglob('attempt.json')))
            self.assertEqual((root/'sentinel').read_bytes(),b'unowned')

    def test_ancestor_swap_during_record_is_detected_before_success(self):
        with tempfile.TemporaryDirectory() as temporary:
            base=Path(temporary).resolve();parent=base/'parent';parent.mkdir();store=LocalStore(parent/'attempt')
            fsync=os.fsync;swapped=False
            def replace(fd):
                nonlocal swapped
                result=fsync(fd)
                if not swapped:
                    swapped=True;parent.rename(base/'detached');parent.mkdir()
                return result
            try:
                with patch.object(os,'fsync',replace),self.assertRaises((OSError,ValueError)):
                    store.record('complete.json',{'status':'complete'})
            finally:store.close()
            self.assertFalse((parent/'attempt').exists())

    def test_publication_never_removes_a_replacement_partial(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()/'attempt';store=LocalStore(root);fd=store.begin('input');os.write(fd,b'fixture')
            link=os.link
            def replace(*args,**kwargs):
                result=link(*args,**kwargs)
                (root/'input.partial').unlink();(root/'input.partial').write_bytes(b'unowned')
                return result
            try:
                with patch.object(os,'link',replace),self.assertRaises((OSError,ValueError)):store.publish('input',fd,7)
                self.assertEqual((root/'input.partial').read_bytes(),b'unowned')
            finally:os.close(fd);store.close()

    def test_package_ancestor_replacement_while_hashing_cannot_pass(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();package=root/'package';package.mkdir();manifest=boot_files(package/'Data')
            read=os.read;swapped=False
            def replace(fd,size):
                nonlocal swapped
                if not swapped:
                    swapped=True;package.rename(root/'detached');package.mkdir();boot_files(package/'Data')
                return read(fd,size)
            with patch.object(os,'read',replace),self.assertRaises((OSError,ValueError)):
                verify_boot_files(package/'Data',manifest)

    def test_same_bytes_do_not_hide_leaf_replacement_after_open(self):
        with tempfile.TemporaryDirectory() as temporary:
            data=Path(temporary).resolve()/'Data';manifest=boot_files(data);read=os.read;swapped=False
            def replace(fd,size):
                nonlocal swapped
                if not swapped:
                    swapped=True;(data/'root.raw').rename(data/'detached');(data/'root.raw').write_bytes(b'approved')
                return read(fd,size)
            with patch.object(os,'read',replace),self.assertRaises((OSError,ValueError)):verify_boot_files(data,manifest)

    def test_boot_rejects_symlinked_ancestors_and_hardlinks(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();manifest=boot_files(root/'Data');(root/'alias').symlink_to(root/'Data')
            with self.assertRaises((OSError,ValueError)):verify_boot_files(root/'alias',manifest)
            os.link(root/'Data/root.raw',root/'external-link')
            with self.assertRaises((OSError,ValueError)):verify_boot_files(root/'Data',manifest)

    def test_prestart_keeps_data_binding_through_final_configuration_readback(self):
        from fort_probe.host import MacKernel
        from fort_probe.config import candidate
        from fort_probe.utm_schema import pinned_schema
        from support import BINDING
        from test_config import IMAGE
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();package=root/'vm/fort-utm-isolation-probe.utm';package.mkdir(parents=True)
            data=package/'Data';manifest=boot_files(data);config=candidate(BINDING,IMAGE,pinned_schema(BINDING))['config_plist']
            kernel=MacKernel.__new__(MacKernel);kernel.root=root
            from fort_probe.boot import validate_config
            import plistlib
            kernel.config=lambda:validate_config(plistlib.loads(config))
            reads=0
            def read(*args):
                nonlocal reads
                reads+=1
                if reads==2:
                    data.rename(package/'Detached');boot_files(data)
                return config
            with patch('fort_probe.host.bounded_read',read),self.assertRaises((OSError,ValueError)):
                kernel.prestart({'config_plist':config,'data_files':manifest})

    def test_record_replacement_during_sync_cannot_be_accepted(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()/'attempt';store=LocalStore(root);fsync=os.fsync;swapped=False
            def replace(fd):
                nonlocal swapped
                result=fsync(fd)
                if not swapped:
                    swapped=True;(root/'receipt.json').unlink();(root/'receipt.json').write_bytes(b'unowned')
                return result
            try:
                with patch.object(os,'fsync',replace),self.assertRaises((OSError,ValueError)):store.record('receipt.json',{'status':'accepted'})
                self.assertEqual((root/'receipt.json').read_bytes(),b'unowned')
            finally:store.close()

    def test_archive_reader_never_opens_a_fifo_in_blocking_mode(self):
        from fort_probe.inspection_worker import regular
        with tempfile.TemporaryDirectory() as temporary:
            fifo=Path(temporary).resolve()/'archive';os.mkfifo(fifo,0o600);real_open=os.open
            def checked_open(path,flags,*args,**kwargs):
                if Path(path).name=='archive':self.assertTrue(flags & os.O_NONBLOCK,'validate the type before any blocking read')
                return real_open(path,flags,*args,**kwargs)
            with patch.object(os,'open',checked_open),self.assertRaises((OSError,ValueError)):
                with regular(fifo,32):pass

    def test_existing_node_fixture_rejects_fifo_without_a_writer(self):
        import subprocess
        with tempfile.TemporaryDirectory() as temporary:
            fifo=Path(temporary).resolve()/'input';os.mkfifo(fifo,0o600)
            source=Path(__file__).resolve().parents[1]/'inspection-verifier.mjs'
            script="import {hashRegular} from "+repr(source.as_uri())+";import assert from 'node:assert/strict';assert.throws(()=>hashRegular(process.argv[1],'sha256',32),/regular input/);"
            result=subprocess.run(['/opt/homebrew/opt/node@26/bin/node','--input-type=module','--eval',script,str(fifo)],
                env={'PATH':'/usr/bin:/bin'},stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=3)
            self.assertEqual(result.returncode,0,result.stderr.decode())


    def test_archive_read_retains_and_rechecks_ancestor_identity(self):
        from fort_probe.inspection_worker import regular
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();parent=root/'parent';parent.mkdir();(parent/'input').write_bytes(b'fixture')
            (root/'alias').symlink_to(parent)
            with self.assertRaises((OSError,ValueError)):
                with regular(root/'alias/input',32):pass
            with self.assertRaises((OSError,ValueError)):
                with regular(parent/'input',32) as (stream,unused):
                    self.assertEqual(stream.read(),b'fixture')
                    parent.rename(root/'detached');parent.mkdir();(parent/'input').write_bytes(b'fixture')

    def test_verifier_install_refuses_symlinked_parent_before_output(self):
        from fort_probe.inspection_worker import install_verifier
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();target=root/'target';target.mkdir();alias=root/'alias';alias.symlink_to(target)
            result={'files':{'package/LICENSE':b'fixture'},'entry':'package/LICENSE','manifest':{},'archive_sha512':'a'*128}
            with self.assertRaises((OSError,ValueError)):install_verifier(alias,result)
            self.assertEqual(list(target.iterdir()),[])

    def test_verifier_install_writes_only_selected_files_and_rejects_existing_target(self):
        from fort_probe.inspection_worker import install_verifier
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()
            files={'package/LICENSE':b'fixture','package/dist/a.mjs':b'fixture bytes only','package/dist/b.mjs':b'fixture bytes only'}
            manifest={name:{'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest()} for name,data in files.items()}
            result={'files':files,'entry':'package/dist/a.mjs','manifest':manifest,'archive_sha512':'a'*128}
            installed=install_verifier(root,result)
            self.assertEqual(installed['entry'],result['entry'])
            self.assertEqual({p.relative_to(root/'verifier').as_posix():p.read_bytes() for p in (root/'verifier').rglob('*') if p.is_file()},files)
            with self.assertRaises(FileExistsError):install_verifier(root,result)


    def test_record_leaf_replacement_at_directory_sync_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()/'attempt';store=LocalStore(root);fsync=os.fsync;calls=0
            def replace(fd):
                nonlocal calls
                result=fsync(fd);calls+=1
                if calls==2:
                    (root/'receipt.json').unlink();(root/'receipt.json').write_bytes(b'unowned')
                return result
            try:
                with patch.object(os,'fsync',replace),self.assertRaises((OSError,ValueError)):store.record('receipt.json',{'status':'accepted'})
                self.assertEqual((root/'receipt.json').read_bytes(),b'unowned')
            finally:store.close()

    def test_published_leaf_replacement_at_directory_sync_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve()/'attempt';store=LocalStore(root);fd=store.begin('input');os.write(fd,b'fixture')
            fsync=os.fsync;calls=0
            def replace(handle):
                nonlocal calls
                result=fsync(handle);calls+=1
                if calls==2:
                    (root/'input').unlink();(root/'input').write_bytes(b'unowned')
                return result
            try:
                with patch.object(os,'fsync',replace),self.assertRaises((OSError,ValueError)):store.publish('input',fd,7)
                self.assertEqual((root/'input').read_bytes(),b'unowned')
            finally:os.close(fd);store.close()

    def test_directory_observation_failure_closes_every_opened_descriptor(self):
        from fort_probe.lifecycle import DirectoryChain
        opened=[];closed=[];real_open=os.open;real_close=os.close
        def open_directory(*args,**kwargs):
            fd=real_open(*args,**kwargs);opened.append(fd);return fd
        def close_directory(fd):closed.append(fd);return real_close(fd)
        with patch.object(os,'open',open_directory),patch.object(os,'close',close_directory),patch.object(os,'fstat',side_effect=OSError('fixture observation failure')):
            with self.assertRaises(OSError):DirectoryChain(Path('/'))
        # Close a leaked fixture descriptor even when the assertion will fail.
        for fd in set(opened)-set(closed):real_close(fd)
        self.assertEqual(opened,closed)
