"""Independent input-inspection gate. Changing this never enables guest execution."""
INSPECTION_ENABLED = False

def require_inspection():
    if not INSPECTION_ENABLED:raise RuntimeError('input inspection disabled pending fixed-source review and exact owner authority')
