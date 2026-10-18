# Recorded private-socket recovery

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md). Source base
`2de9928`. This proves established-socket recovery with synthetic data, not every
startup crash window or installed-product recovery.

The red test completed a durable message, killed the exact core process, confirmed
its socket remained, and tried to restart. Startup failed at the occupied path.
The same test now restarts, authenticates with the existing synthetic pairing and
returns the original receipt and exact two-message history without reenrollment.

After binding and before readiness, the core records the selected socket name,
directory device/inode/owner and socket device/inode/owner/change-time identity in
its exclusively locked SQLite ledger. Restart only reclaims that exact recorded
socket. A bounded connection probe must return connection-refused, then identity
and cancellation are checked again before unlinking. A live listener, uncertainty,
changed path or absent/invalid record blocks startup. Confirmed clean shutdown
removes the ownership record after the socket is absent.

The adapter adds no package, external service, credential or idle polling. It
adds a small startup/shutdown metadata write and a 250 ms maximum recovery probe.
Records and error messages contain no credential or message body. Malformed
record errors are generic rather than echoing stored contents.

Passed:

- Thirteen socket tests: exact SIGKILL/restart/receipt replay; clean close/restart;
  active-owner refusal; cancellation during recovery followed by a successful
  retry; regular-file, symlink and replacement-socket preservation; unrecorded
  stale sockets; malformed/future records; existing path/permission controls.
- All 860 portable tests, strict TypeScript and all 84 Swift tests passed. The
  Swift suite used the newly compiled core for messaging, restart and recovery.
- Retained artifact `successor/.build/socket-recovery-059-20261018`: 24 files,
  173,076 bytes; every file size/hash checked. Manifest SHA256:
  `5830dd32598a41fb09608123a48211267960fc4b07337cec55fb46e468268538`.
  The compiled-core test still excludes provider/optional implementations.
- Full/implementation spec gates and whitespace checks passed.

Logs: `/tmp/fort059-socket-recovery-{red,green,first,controls,controls-final,typecheck,artifact,portable,swift}.log`.
The original red and first implementation failure remain. The first implementation
run exited before fixture readiness; constructor parameter properties were replaced
with ordinary fields compatible with the source runner. The fixture now reports
early child exit promptly instead of waiting only for its IPC timeout.

A socket created before its ownership record commits remains unknown and is
preserved. This deliberately requires separate recovery evidence rather than
guessing ownership. Signed XPC abrupt restart is the next proof; ordinary app
builds, installed screenshots/accessibility, providers and TestFlight were not
run for this slice. No real account, persistent startup or provider cleanup changed.
