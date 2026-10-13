"""Prepared acquisition activation; running it still requires exact owner authority."""
INSPECTION_ENABLED = True

def require_inspection():
    if not INSPECTION_ENABLED:raise RuntimeError('input inspection disabled pending fixed-source review and exact owner authority')
