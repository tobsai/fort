# Private native owner recovery

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md), under
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
Source base `dc23178`; October 18, 2026. This is source, compiled-artifact and
synthetic process evidence. No installed lifecycle or provider acceptance is claimed.

The native process transport can send a separate `ownerRecovery` frame with exact
installation/device IDs. The relay accepts it only when local composition supplies
the core recovery callback; ordinary compositions reject it. It is never translated
to an HTTP request, and no bearer-token exception or public recovery route exists.
The same monotonic correlation, eight-call admission, request deadline (at most
30 seconds), cancellation and bounded output apply. The relay validates identity
and confirmations before responding. Timed-out/canceled recovery may finish its
durable stop/revocation, but cannot return a success confirmation to that caller.

Swift independently checks status, exact returned identity, runtime disposal and
revocation. `MacCorePairingRecovery` proves those facts only; callers must still
close the process and establish owned-group absence. The synthetic native journey
does so before reopening the same ledger. It proves expired HTTP credentials are
rejected, foreign recovery cannot act, exact recovery succeeds, fresh explicit
resume preserves message history and a superseded device cannot stop the new one.
Synthetic setup data lives only in temporary fixture directories and is removed
after the owned processes exit; no credential values or message bodies are evidence.

Passed:

- Nine private-channel tests, including four recovery controls: exact recovery,
  disabled/malformed input, timeout/cancellation and foreign callback proof.
- All 848 portable tests, zero failures/skips, and strict TypeScript checking.
- 81 Swift tests against source and again against the compiled core. These include
  the real native recovery journey and five negative proof cases (installation,
  device, runtime confirmation, revocation confirmation and response status).
- All 22 artifact file hashes verified. Artifact:
  `successor/.build/core-recovery-059-20261018`; 168,124 listed bytes; manifest
  SHA256 `4bdc218107386e8592dbaf0e3b2fb59e66ff9d01d73f24a485a83098e9cc249b`.

The initial Node regression failed with a missing recovery response because that
frame was unsupported. The first Swift test build failed because the native API
did not yet exist. Both are preserved as red-stage logs, not validation passes.
Logs: `/tmp/fort059-private-recovery-{red,controls,portable,typecheck,swift-red,swift,swift-compiled,artifact}.log`.

The fresh signed XPC compatibility attempt did **not** pass. Its 15-second startup
deadline expired before the aggregate report, leaving cleanup unconfirmed in the
original result. The system log places process launch at 23:48:09.864 local time,
macOS signing evaluation near 23:48:23.842, the first XPC call at 23:48:23.985 and
forced parent exit at 23:48:24.867. The harness counts XPC work against the same
startup deadline until its later Node-child marker. Separating bounded startup
and execution phases is the next harness repair; these logs do not establish a
new product startup performance result.

Retained failure: `/tmp/fort059-sandbox-xpc-test-fog2wt/proof/result.json` and its
signed bundle. Independent cleanup verified parent 10508, its group, observed
helpers 10525/10526/10529 and all exact bundle executable paths absent. Only the
four generated `io.fort.foundation.probe.3f582c9b-6ab4-4602-9097-0058e7691386`
containers were then removed. The original failure record is unchanged.
Logs: `/tmp/fort059-private-recovery-xpc.log`,
`/tmp/fort059-private-recovery-xpc-system.log`,
`/tmp/fort059-private-recovery-xpc-cleanup.json`.

The current ordinary XPC interface still carries only authenticated Fort requests;
it does not expose this pipe command. A pinned owner lifecycle composition and
ordinary app setup remain to be implemented and separately verified. Startup
unregistration, installed removal, real provider behavior and TestFlight remain
unrun. This slice adds no dependency, network listener, credential access or
persistent service activation.
