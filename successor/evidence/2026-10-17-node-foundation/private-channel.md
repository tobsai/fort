# Private core pipe relay

The portable core artifact now exports `./host-channel`, a bounded pipe relay to
one fixed private Unix socket. It forwards the native relative-request contract
through the core's existing authentication and installation checks. It creates no
enrollment, credential, provider, application or startup registration. The caller
owns application close. Governing design: [Spec 059](../../../specs/059-modular-node-v1.md).
Source base: `87d8f02` plus this slice.

Protocol version 1 uses monotonically increasing request IDs, correlated replies
and explicit cancellation. Limits are eight active requests, 2 MiB input frames,
8 MiB decoded response bodies, 12 MiB output frames, 30-second request deadlines
and five seconds to drain an output write. Malformed, stale, oversized and
overloaded input closes the channel. EOF and abort cancel outstanding HTTP calls;
a late cancellation of a completed ID is harmless. The endpoint is pinned to an
owner-only directory/socket in the current working directory. No TCP fallback or
request retry exists. Canceling transport does not cancel an accepted domain Run.

Verification:

- Red: the first public journey failed with the missing relay module.
- Five relay tests passed, zero skips. Source and compiled artifact journeys
  check unauthenticated and foreign-installation rejection, explicit fixture
  enrollment, exact synthetic text, replay, canceled long wait, restart receipt
  recovery without duplicate execution, and socket removal after close.
- Negative controls cover malformed/oversized frames, protocol mismatch, foreign
  routes, invalid base64, future cancel IDs, reused request IDs, excess concurrent
  requests, EOF during waits and blocked stdout. The backpressure case completed
  in about 5.16 seconds with a failing exit and no remaining socket.
- All 836 portable tests passed, zero failures/skips; strict TypeScript passed.
  After adding request-timeout and harmless late-cancel assertions, both source
  and compiled journeys passed again (2/2).
- A fresh core artifact contains 22 files totaling 161,592 bytes, with every
  manifest hash independently checked. Manifest SHA-256:
  `baa7130defa0110cf1ad356a0671bef9045dc401dc0028f42315c77a3f528b46`.
  Retained path: `successor/.build/core-channel-059-20261018`.

The first backpressure run hung after the relay and core had closed: Node was
waiting to flush a failed stdout pipe. Only the identified synthetic fixture/test
process groups were stopped, and their absence was verified. The fixture now
exits with failure **after awaiting core close**; the test closes its owned
readers after observing child exit and bounds the failure wait. Forced cleanup
would fail the test. The original synthetic failure directory remains at
`/tmp/fort-private-channel-rORdMi`, with no socket. A production entrypoint must
preserve this close-before-exit discipline.

This is source/compiled relay evidence. Swift forwarding, signed sandbox bridge,
owned host lifecycle, actual providers and installed/physical acceptance remain
unrun for this slice. The preceding 68 Swift tests and four signed sandbox
scenarios are separate evidence. The relay does not establish executor isolation;
agent-host and computer-tool sandbox boundaries remain distinct.

Logs: `/tmp/fort059-private-channel-red.log`, `-first.log`, `-controls.log`,
`-green.log`, `-portable.log`, `-typecheck-final.log`, `-deadline.log` and
`-artifact.log` under the same prefix. Structural and accepted/staged spec gates
and whitespace checks passed before commit.
