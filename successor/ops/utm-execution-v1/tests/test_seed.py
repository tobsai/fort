import json
import unittest
from fort_probe.seed import preview, package_bytes


class SeedTests(unittest.TestCase):
    def test_preview_has_only_three_synthetic_files_and_no_login_or_autostart(self):
        seed = preview()
        self.assertEqual(set(seed), {'meta-data', 'user-data', 'network-config'})
        config = json.loads(seed['user-data'].decode().split('\n', 1)[1])
        self.assertEqual(config['datasource_list'], ['NoCloud'])
        self.assertEqual(config['ssh_genkeytypes'], [])
        self.assertFalse(config['ssh_pwauth'])
        self.assertEqual(config['growpart']['mode'], 'off')
        self.assertFalse(config['resize_rootfs'])
        self.assertNotIn('packages', config)
        self.assertEqual(config['cloud_init_modules'], ['users-groups', 'write-files'])
        self.assertEqual(config['cloud_config_modules'], [])
        self.assertEqual(config['cloud_final_modules'], [])
        self.assertNotIn('runcmd', config)
        for user in config['users']:
            self.assertTrue(user['lock_passwd'])
            self.assertEqual(user['shell'], '/usr/sbin/nologin')
            self.assertEqual(user['sudo'], False)
        for file in config['write_files']:
            self.assertEqual(file['owner'], 'root:root')
            if file['path'].endswith('.service'):
                self.assertNotIn('WantedBy=', file['content'])
                self.assertIn('/opt/fort-probe/disabled.py', file['content'])
        network = json.loads(seed['network-config'])
        self.assertEqual(network, {'version': 2, 'ethernets': {}})
        self.assertLess(sum(map(len, seed.values())), 8388608)

    def test_no_actual_vm_package_can_be_rendered_with_missing_archive_facts(self):
        with self.assertRaisesRegex(RuntimeError, 'disabled'):
            package_bytes()
