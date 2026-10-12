"""Static packet contract tests. No guest, host settings, network, or credentials."""
import copy
import hashlib
import importlib.util
import json
import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("packet_validator", ROOT / "validate.py")
validator = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(validator)


class PacketContract(unittest.TestCase):
    def setUp(self):
        self.config = json.loads((ROOT / "lima.yaml").read_text())
        self.scope = json.loads((ROOT / "scope.json").read_text())

    def rejected(self, config=None, scope=None):
        self.assertTrue(validator.policy_errors(config or self.config, scope or self.scope))

    def test_exact_packet_has_no_static_policy_errors(self):
        self.assertEqual([], validator.policy_errors(self.config, self.scope))

    def test_host_mount_rejected(self):
        self.config["mounts"] = [{"location": "/Users/tobiasgunn", "writable": True}]
        self.rejected()

    def test_ambient_keys_rejected(self):
        self.config["ssh"]["loadDotSSHPubKeys"] = True
        self.rejected()

    def test_proxy_inheritance_rejected(self):
        self.config["propagateProxyEnv"] = True
        self.rejected()

    def test_guest_package_install_rejected(self):
        self.config["provision"][0]["skipDefaultDependencyResolution"] = False
        self.rejected()

    def test_remote_image_rejected(self):
        self.config["images"][0]["location"] = "https://example.org/image.img"
        self.rejected()

    def test_unlisted_payload_rejected(self):
        self.scope["downloads"].append({"url": "https://example.org/helper", "sha256": "a" * 64})
        self.rejected()

    def test_verifier_proposal_is_exact_and_private(self):
        self.assertEqual('openpgp@6.3.2', self.scope['private_verifier']['package'])
        entry = next(item for item in self.scope['downloads'] if item['name'] == 'openpgp-6.3.2.tgz')
        self.assertEqual(8388608, entry['max_bytes'])
        entry['sha512'] = '0' * 128
        self.rejected()

    def test_verifier_cannot_expand_to_package_manager_install(self):
        self.scope['private_verifier']['install_hooks'] = True
        self.rejected()

    def test_resource_expansion_rejected(self):
        self.config["disk"] = "40GiB"
        self.rejected()

    def test_diagnostic_storage_expansion_rejected(self):
        self.scope['download_diagnostics']['public_key_body_max_bytes'] += 1
        self.rejected()

    def test_diagnostics_cannot_add_automatic_retry(self):
        self.scope['download_diagnostics']['automatic_retry'] = True
        self.rejected()

    def test_approval_cannot_be_implied_by_static_success(self):
        self.scope["execution_authorized"] = True
        self.rejected()

    def test_systemd_lifetime_weakening_rejected(self):
        units = validator.read_units(ROOT)
        units["fort-runtime.service"]["Service"]["KillMode"] = "process"
        self.assertTrue(validator.unit_errors(units))

    def test_tool_management_exposure_rejected(self):
        units = validator.read_units(ROOT)
        units["fort-tool.service"]["Service"]["PrivateNetwork"] = "no"
        self.assertTrue(validator.unit_errors(units))

    def test_changed_content_and_unlisted_file_rejected(self):
        content = {"scope.json": b"old"}
        manifest = {"scope.json": hashlib.sha256(b"old").hexdigest()}
        self.assertEqual([], validator.digest_errors(content, manifest))
        self.assertTrue(validator.digest_errors({"scope.json": b"new"}, manifest))
        self.assertTrue(validator.digest_errors({**content, "extra": b"new"}, manifest))
        self.assertTrue(validator.digest_errors({}, manifest))


if __name__ == "__main__":
    unittest.main()
