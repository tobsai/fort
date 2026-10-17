import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('readiness', Path(__file__).with_name('check.py'))
readiness = importlib.util.module_from_spec(spec)
spec.loader.exec_module(readiness)


class PreparationCheckTests(unittest.TestCase):
    def test_pending_scope_is_inspectable_but_changes_do_not_become_authority(self):
        scope = json.loads(readiness.SCOPE.read_text())
        readiness.verify_scope(scope)
        for field, value in [('status', 'approved'), ('maximumPlainTextTurnsPerSession', 13),
                             ('maximumEnrollmentsTotal', 4), ('workSeconds', 600),
                             ('renewExistingCredential', True)]:
            changed = copy.deepcopy(scope)
            changed[field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                readiness.verify_scope(changed)

    def test_changed_artifacts_and_path_escape_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            (base / 'app').write_bytes(b'prepared')
            record = {'path': 'app', 'bytes': 8, 'sha256': hashlib.sha256(b'prepared').hexdigest()}
            readiness.verify_file(base, record)
            (base / 'app').write_bytes(b'changed!')
            with self.assertRaises(ValueError):
                readiness.verify_file(base, record)
            with self.assertRaises(ValueError):
                readiness.verify_file(base, dict(record, path='../escape'))

    def test_no_start_action_is_exposed(self):
        result = subprocess.run([sys.executable, str(Path(__file__).with_name('check.py')), '--start'], capture_output=True)
        self.assertEqual(result.returncode, 2)
        self.assertIn(b'unrecognized arguments', result.stderr)


if __name__ == '__main__':
    unittest.main()
