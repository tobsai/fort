# Candidate B — pending-start cancellation fix

Source checkpoint: **`e3305265eb733e6ae6dd0bcb8780ec05bfe30702`** on
`codex/fort-openai-rebuild-20261016`, worktree
`/Users/tobiasgunn/dev/fort/.local/worktrees/fort-openai-rebuild-20261016`.
The exact [source/app pins](pins.json) replace the previous packet's current pins.
Earlier evidence remains intact. Activation remains disabled. This source is
ready for the requested focused recheck; independent acceptance of this fix is
not claimed.

## Review and correction

The [independent review](independent-review.md) and
[original reproduction](original-reproduction.mjs) are byte-for-byte copies of
the requested `/tmp` artifacts. Their hashes are in `pins.json`. Before editing,
the reproduction confirmed zero aborts while startup was pending and readiness
printed after SIGINT; its [output is retained](original-reproduction.log).
That script deliberately asserts the old defect and is preserved as historical
evidence. The executable regression is now in `prototype/manual.test.ts`.

The manual controller owns an AbortController before awaiting a session start.
SIGINT, SIGTERM and terminal closure abort that controller. `CandidatePacket.start`
propagates cancellation into preflight and begins disposal immediately. Waiting
for preflight races confirmed session closure, so an uncooperative late preflight
cannot delay the controller until the work deadline. Cancellation saves a blocked
packet with its consumed start; neither late success nor another call permits a
retry.

The same signal covers listener startup using Node's native server cancellation.
Startup rejects before publishing its address, and the controller checks again
after awaiting the listener and before issuing enrollment. The manual loop also
checks after awaiting startup, before writing an enrollment file or readiness.
A ready session remains bound to that cancellation handle. The service refuses
reopening after closure.

Production changes are limited to `prototype/manual.ts`, `prototype/session.ts`
and the listener lifecycle in `service/openai.ts`. Their existing test files,
the prototype README and the packet's current pin path are the other source
changes. Sample text, admission, restart rules, runtime settings, five-minute
budget, ten-minute credential deadline and Apple source/app are unchanged.

## Validation

| Check | Result |
| --- | --- |
| Red regressions before each fix | Expected failures: pending SIGINT cancellation, blocked-preflight disposal, and cancellation before/during listener startup; `manual-red.log`, `session-red.log`, `listener-red.log` |
| Affected manual/controller/service suites | **29 passed, 0 failed**; [output](affected-tests.log) |
| Existing combined service/runtime/prototype suites | **162 passed, 0 failed**; [output](regression-tests.log) |
| Strict TypeScript | Passed, exit 0; `node successor/node_modules/typescript/bin/tsc -p successor/tsconfig.json`; empty [diagnostic log](typecheck.log) |
| Swift FortClient/PendingStore HTTP conformance | Five synthetic turns, retained receipts and conditional restart passed; [output](native-http.log) |
| Source and app verification | All **452 source files and five app files** match; [verification](verification.json) |
| Whitespace/error check | `git diff --check` passed |

Node checks used `/opt/homebrew/opt/node@26/bin/node` 26.5.0, temporary SQLite and
loopback services, synthetic runtime transports and owned Node actors. The manual
regression evaluates the actual controller source in an inert VM with fake I/O,
an inert packet and no launcher; the public production activation guard remains
unchanged and its refusal test still passes. Session tests exercise the actual
controller with a blocked synthetic preflight. They prove cancellation and
disposal occur before releasing that preflight, no listener port is published,
and restart remains blocked. Listener tests confirm the reserved loopback port
cannot be reached after cancellation. No real Keychain or provider store is used.

The Swift check uses the existing `FortNativeCheck` executable against a temporary
service. It is HTTP/storage conformance, not rendered UI or real-provider evidence.
Swift model/unit tests and the Mac build were not rerun because Apple code and all
five app files are unchanged.

## Remaining gates and preserved limits

No official provider process, real credential creation/access/deletion, local
provider configuration read, app launch, settings change, startup service,
deployment or remote publication occurred. The actual candidate session directory
is still absent. The main checkout's status is byte-identical to its retained
baseline. The activation guard and official launcher are byte-identical to the
reviewed source.

The focused recheck is still pending. Live preflight, real replies, native UI and
actual enrollment cleanup remain unrun and need the separate exact execution
decision already described by the review. The host still runs with the owner's
HOME/UID before it can answer `config/read`; startup effects cannot be excluded
by the later protocol check. Process-group cleanup cannot contain escaped
descendants or attest remote cancellation. This fix adds no confinement claim
and changes none of those authority boundaries.

Manifest hashes use SHA-256 over UTF-8 JSON arrays with sorted object keys and
compact `,`/`:` separators. Evidence is excluded from the source list to avoid
self-reference. The source canonical hash is
`de7495118494286a978b5806a67c31a2890179f7dd3afe31a2136d77130e222f`;
the unchanged app canonical hash is
`9c32247c3c77b1618515aec7faad3944826ec9e11d4ecc0d58483c8ecd23ad2b`.
