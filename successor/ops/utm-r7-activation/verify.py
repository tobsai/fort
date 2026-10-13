"""Read-only closure/tool/gate verification; never starts smoke or acquisition."""
import json
from pathlib import Path
import sys
sys.path.insert(0, str(Path(__file__).resolve().parent))
from packet_support import check_packet, verify_tools

if len(sys.argv) != 3 or sys.argv[1] not in ('disabled', 'activated'):
    raise SystemExit('Usage: verify.py disabled|activated EXACT_PACKET_SHA256')
context = check_packet(sys.argv[2], sys.argv[1])
verify_tools(context)
print(json.dumps({'packet_sha': context.packet_sha, 'state': sys.argv[1],
                  'source_seal': context.manifest['source_seals'][sys.argv[1]],
                  'closure_and_tool_pins': 'passed', 'live_validation': 'unrun'}, sort_keys=True))
