// Source preparation only. No environment flag enables this archive.
export function foundationLaunch() {
  throw new Error("Fort foundation is source-only: runtime extraction and independence review required");
}
export function foundationRouteAllowed(_path) { return false; }
