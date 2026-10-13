"""Disabled R7 preparation; review and exact owner authority precede activation."""
INSPECTION_ENABLED = False

def require_inspection():
    if not INSPECTION_ENABLED:raise RuntimeError('input inspection disabled pending fixed-source review and exact owner authority')
