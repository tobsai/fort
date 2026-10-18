# Explicit provider sign-in protocol preparation — October 18, 2026

Governing [Spec 059](../../../specs/059-modular-node-v1.md), decision 059. Base
`326eff2` on `codex/fort-openai-rebuild-20261016`. This prepares an optional adapter
operation; it is not a working native sign-in flow or an account authorization.
No actual Codex, credential store, browser, provider, startup or network grant was used.

## Behavior and source boundary

`signInCodexAccount` accepts one explicitly owned Codex connection, the exact
0.157.1 version, an absolute workspace, cancellation signal, bounded attempt duration
and challenge callback. It initializes and verifies existing runtime controls before
account access. An existing ChatGPT account is retained; API-key/other account kinds
are refused. Signed-out state sends exactly one `chatgptDeviceCode` request. The
callback receives only a validated login ID, bounded display code and exact official
verification URL. No token, email, provider error or extra response field is returned.

The official [pinned account protocol](https://raw.githubusercontent.com/openai/codex/36650394c5b38c2990ccf2a3457165ca3e9d9726/codex-rs/app-server-protocol/src/protocol/v2/account.rs)
defines the device challenge and `canceled`/`notFound` cancellation responses.
The [official App Server guide](https://learn.chatgpt.com/docs/app-server#3b-log-in-with-chatgpt-device-code-flow)
describes caller-owned presentation and completion notifications. The adapter uses
the managed device-code operation, with no callback listener, automatic browser
opening, external token injection or paid API-key fallback. Current documentation
is supplementary; the selected runtime and source revision remain pinned.

Completion must match this attempt and is followed by `account/read` with
`refreshToken:false`; a success notification alone cannot establish authentication.
Identical duplicate completion is harmless. Foreign, contradictory, malformed or
unsolicited completion closes the connection. Server tool/permission/token requests
receive a fixed refusal and close; no request authority is granted.

Cancellation while waiting sends the exact login ID, then reads account state.
A login that completed during cancellation remains signed in. `notFound` with no
confirmed account stays unconfirmed. The operation never logs out or claims account
revocation. Pre-canceled input performs no protocol work. Lost replies, invalid
controls/challenges, connection loss or a failed presentation return unconfirmed,
with no retry. One connection cannot host concurrent attempts. The callback receives
a copy so it cannot replace the captured login identity.

The caller supplies a work duration of 1–600,000 ms. Waiting is event-driven with
one attempt timer; cancellation RPCs share a separate three-second cleanup limit.
Owned connection disposal must complete before the operation returns. Disposal
failure throws only a fixed cleanup error. The later production launcher must retain
its own process termination bound; this operation does not extend the existing
guarded launcher's 120-second ceiling or prove OS process absence by itself.

The optional executor exports this operation and contains its declarations and
compiled dependencies. Import is inert. Core remains free of the adapter, and
ordinary host/native composition does not call it yet. No dependency, schema,
credential file or entitlement was added.

## Validation

| Check | Observed result |
| --- | --- |
| Initial public journey | Failed against inert stub, then passed through synthetic protocol |
| Unsolicited completion regression | Failed by returning signed in before login; now refused before login request |
| Late forbidden server request | Existing/new account replies both reproduced false success; now closed connection prevents confirmation |
| Compiled package seam | Initially failed because the account module was not exported; export, inert import and public declaration consumer now pass |
| Final account + packaging checks | 47 passed, zero failed, two opt-in budgets skipped |
| Final complete portable suite | 1,234 passed, zero failed, ten skips; 1,244 total, 34.838 seconds |
| Strict TypeScript | Passed; earlier test `deepEqual([], ...)` narrowed a later mutable call log to `never[]`; a length assertion fixed the test without casts |
| Affected guarded Mac executor regressions | 31 passed, zero failed, three optional budgets skipped; no actual provider executable |
| Current host resources | 100 files / 420,483 bytes, every file hash and exact membership independently verified |
| Provider, current installed UI, Apple provisioning, startup, archive/upload/TestFlight | Unrun; no acceptance inferred |

Portable skips are nine opt-in timing checks and the unavailable alternate-loopback
source-address case. The new account timing check was run separately. Five warmups
plus twenty serial synthetic initialize/control/account/challenge/completion/disposal
journeys: p50 **0.201541 ms**, p95 **0.495792 ms**, max **0.622084 ms**, below the
25 ms p95 budget. Node 26.5.0, TypeScript 6.0.3, macOS 27.2; no concurrent task build
or test during measurement. Prior hardware inventory is MacBookPro18,2, ten CPUs,
64 GiB; a fresh sandboxed sysctl read was unavailable. This measures warm protocol
handling only, excluding process launch, human sign-in and provider/network time.

Retained resources: `/tmp/fort059-account-host-resources`. Manifest SHA256:
`216978dc222b82ad90aa6946a66c688812e8d2947bc9ac6240186e7b5c1c40b7`.
The earlier 190-file Mac review app remains its own unchanged artifact; it was not
relabelled as containing this later operation. Native source is unchanged.

Logs and hashes:

- `/tmp/fort059-account-complete-portable.log`:
  `5e57eb7fae6ca7b81ec47a03244894424761085c9ad919a03aed82207f6d5dd3`.
- `/tmp/fort059-account-complete-focused.log`:
  `36af263fa0e79b214ec78cddc4f1893cea868737238fbe6c9159ad92132f98ad`.
- `/tmp/fort059-account-complete-typecheck.log`: empty successful log,
  `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- `/tmp/fort059-account-macos.log`:
  `177a03d6422d53cc17a5746cac2b6f190a1602fd2a75f3d33ded68e5e65bdb53`.
- `/tmp/fort059-account-final-samples.json`:
  `0adafa3fadffcae726d21be006d8c7b01ff61f23e5f10fa65eed5a822d6ef942`.
- Initial failures: `/tmp/fort059-account-red.log`,
  `...-controls-red.log`, `...-package-red.log`, `...-late-request-red.log` and
  `...-typecheck.log`. Final checks above supersede them; none is hidden as a pass.

## Remaining Mac path

Compose the exact private guarded process, primary-owner account operation and
native challenge/cancel/status UI. Preserve durable lost-response behavior and
exclude companion authority. A successful adapter fixture does not fill these seams.
Then verify signed App Group/native lifecycle and obtain the concrete provider and
startup grants before an actual sign-in and Mac chat demonstration. TestFlight
delivery remains required. Existing Apple sign-in is still the smallest external
prerequisite for the read-only team/client/helper/group check; code work continues
independently. No historical demo grant or credential was reused.
