# Single-account default and Mac composer — October 20

Scope: [Spec 059](../../../specs/059-modular-node-v1.md#single-account-default-and-mac-composer-usability--october-20), owner messages
Sentinel_b7cb112f432c8191830ab311ce06c704 and
Sentinel_7b17481acd5c8191a9c5300d4d37ab8b. Work remains on
`codex/fort-openai-rebuild-20261016`, based on `3f84c154`.

## Source behavior

The first fresh, valid, single identity is saved atomically as the default. Existing
choices require an exact account/label, provider, host, installation and recovery
match. A malformed/stale choice or retained run history without a choice cannot be
treated as first use. A legacy selected identity retains its original recovery
scope. Canceled, expired, missing and multiple-identity results cannot select.
Explicit replacement remains available; queued requests keep their account pins.
Existing Agent settings and selected models are preserved.

Native sign-in immediately discovers models for the selected account, including
an immediate completion or retry of the same request. Reconciliation does not
repeat discovery for unchanged status. Advanced account management hides workspace
detail, replacement and further sign-in. Fort still has one active runtime account;
this does not introduce a multi-account credential store. Provider ambiguity stays
unresolved until the official runtime supplies one verified identity.

Mac Return sends; Shift-Return inserts a line. Marked text stays with native input
handling, and empty/disabled/repeated input cannot duplicate a request. The native
text view measures draft size without changing its live frame; final scroll-view
layout owns document width. The existing Send button and iOS composer remain.

The reported image control was the local PNG/JPEG picker, not an attachment.
The signed Mac client lacks user-selected-file access. Its inactive picker is
removed without adding entitlement or permission; stored local image previews and
removal remain. iOS import and the labeled synthetic QA sample remain.

## Verification trail

Raw local evidence: `/tmp/fort059-single-account-20261020/`.

- Service red: first valid account remained unselected. Green: 43 passes, 2 optional
  benchmark skips. Covers first/existing identity, replacement, mismatch, malformed
  scopes, interrupted/re-paired legacy state, retained history, cancellation/expiry.
- Native account red: immediate completion and saved-request retry did not discover
  models. The focused account cases pass with the corrected transition handling.
- Full portable suite: 1,423 passed, 16 skipped, zero failed.
- Native full pass before the final layout correction: 349 passed, 16 skipped.
  Two earlier failures asserted obsolete copy; updated assertions retain the
  selected/unverified state checks. Final post-layout suite: 350 passed, 16 skipped,
  zero failed (366 reported tests). Final Release Mac and Debug QA builds pass.
  Strict TypeScript checking and the spec implementation gate pass.
- Direct AppKit tests cover Return, Shift-Return, empty/disabled input, duplicate
  submission and marked-text delegation. A hosted native document test covers
  visible dimensions and focus. These do not attest a human-operated IME session.
- Rendered QA exposed an inert DisclosureGroup interaction. Its explicit native
  expand/collapse button now reveals replacement and account-change controls.
  A synthetic mismatch remains blocked until the explicit replacement action.
- Rendered QA also exposed zero-width text caused by mutating the live document
  during speculative sizing. Size measurement is now side-effect-free. Actual
  screenshots show both draft lines. Return stored one synthetic request; a second
  Return stored no extra request. The cleared composer disabled Send. The sample
  image displayed, removal worked, and the unsupported picker was absent.
- QA accessibility, screenshots and synthetic stored-request verification are
  separate from installed-product/provider acceptance. No real prompt was sent.

Historical real prompt use remains **2/3**; no new sign-in or provider request is
authorized or performed here. The earlier selected `gpt-6.1-sol` runs remain
completed, with effective model unreported. Retained history is not replaced.

## Remaining acceptance

Signed installation and exact installed disconnected checks passed as recorded below. New-source real-provider sign-in/catalog/inference, an actual
IME session, physical iPhone, TestFlight/G2 and provider-side closure remain unrun.
This increment neither activates the host nor creates credentials or startup jobs.
No remote push, merge, deployment, visibility change or provider cleanup occurs.

Intermediate checks failed before corrections: the first-account and immediate
discovery red tests, obsolete copy assertions, the rendered disclosure and
zero-width composer checks, and local Swift scaffold/API spelling errors. All
final required checks above pass. Early QA reads also mistook a tooltip window
for the main window and omitted combined accessibility message elements; the
final send result is proven by the exact synthetic stored request and main-window
screenshot, not those early reads. No uncertain provider send was retried.

Inspected screenshot SHA256 values (local files, synthetic data only):
- `qa-first-connected.png`: `2b4b142b2084656cdd6a941da7bd236e0b6f71c0083736801685c9699159fad0`.
- `qa-mismatch-advanced.png`: `c7e5ca1755dc39dcc60c9134dc67998d589de7da9c82d8d8b195dbcefcb3c9f6`.
- `qa-composer-multiline.png`: `e53ce22e2d01e6eb92d0fd729089f8cb919407e16f7306bca859acfd48fee29c`.
- `qa-composer-sent.png`: `8fcc878355ab27a6c60a0e27028df5e84c1af32ef416e8de3bc2c08e06902cad`.

## Signed installed result

Source commit: `310b06a936aa8bb4bcbb8ea99e84167a4f0e0b6f`. Installed app:
`/Users/tobiasgunn/Applications/Fort.app`. Previous verified app is retained at
`/tmp/fort059-single-account-20261020/previous-installed.app`. The update used an
atomic bundle swap after checking both payloads and the disconnected state.

The existing signing identity, profiles, entitlements, app IDs, helper, guardian,
Node and Codex 0.163.0 pins are unchanged. All 208 payload files/modes/hashes and
30 Mach-O signatures/certificates passed verification; the native bundle inspector
passed. The exact installed client opened disconnected, showed local host not
enabled, disabled Send, no provider sign-in and no historical message elements.
Its screenshot was inspected and normal Command-Q succeeded. All 208 files still
matched the candidate afterward.

Signed report SHA256: `b9802e94dc45c4554cc23c3998c47cf99bc3d3238845533025b1c0de9d80e89a`.
Payload SHA256: `fdfeaa5f28d6b0ff953fadba2ded734d8986889a1f85c27b7c7293e6861e8cfe`.
Installed screenshot SHA256: `2b82d9f5bc49978f28f607d3b11fa4c49a600b0ec062e440082214f23a37c170`.

Final closed-ledger and absence checks: one Agent, 12 messages, six runs retained;
zero devices, enrollment and outbox rows. No Fort process, startup job, provider
auth file or client connection remains. Original pre-migration messages/runs are
unchanged; the complete ledger hash remains
`e8005f96248cb27d48a74c446ab5f15f22baaf48e468dffc1f9b9ca59e68c561`.
No new provider request or prompt was made; historical prompt use stays 2/3.
The installed check is disconnected smoke, not live first-sign-in or inference
acceptance. No merge, push, deployment, remote visibility change or provider cleanup.
