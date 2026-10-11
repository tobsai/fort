// Source preparation only. Unlocking runtime is a separately reviewed change.
export function foundationLaunch() {
  throw new Error('Fort foundation is source-only: runtime extraction and independence review required');
}
export function foundationRouteAllowed(path) {
  return !(/^\/api\/(?:cloud|hosted|pair|auth|composio|boat|voice|live)(?:\/|$)/.test(path)
    || /^\/api\/bots\/[^/]+\/(?:messages\/guarded|tasks|requests|queue)(?:\/|$)/.test(path)
    || /^\/api\/threads\/[^/]+\/respond$/.test(path));
}
