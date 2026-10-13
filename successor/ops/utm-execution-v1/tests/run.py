"""Run only repository-owned fixture tests with the existing Python standard library."""
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / 'tests'))
if sys.argv[1:]:
    suite = unittest.defaultTestLoader.loadTestsFromNames(sys.argv[1:])
else:
    suite = unittest.defaultTestLoader.discover(str(ROOT / 'tests'), 'test_*.py')
result = unittest.TextTestRunner(verbosity=2).run(suite)
raise SystemExit(0 if result.wasSuccessful() else 1)
