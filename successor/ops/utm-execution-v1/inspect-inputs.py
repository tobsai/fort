"""Prepared input-inspection CLI; do not run before exact owner approval of its seal."""
import json
import sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parent))
from fort_probe.inspection_guard import INSPECTION_ENABLED

if not INSPECTION_ENABLED:
    sys.stderr.write('Input inspection disabled.\n')
    raise SystemExit(78)
from fort_probe.inspection import main
if len(sys.argv)!=2:raise SystemExit('exact accepted source seal SHA-256 required')
print(json.dumps(main(sys.argv[1]),sort_keys=True))
