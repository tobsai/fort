What this repo does: Fort is a single-owner native messenger with a durable Node
core and optional runtimes. This slice prepares one explicit provider sign-in
operation while keeping ordinary startup, browser and credential behavior inert.

## Semantic review

Codex self-review, October 18, 2026, base 326eff2. Used the pinned Ponytail method
and retained the whole-repository baseline. Read the new operation and every test,
its executor export, artifact compiler/export/declaration checks, existing protocol
request/disposal semantics, control verification and the pinned official account
schema. Traced every await, early return, cancellation path, challenge callback,
completion event and cleanup outcome. Checked current host/private launch code to
separate this protocol seam from unimplemented native ownership. Reviewed final
portable/typecheck logs, isolated measurements and independently verified resources.
No user credential, provider response or historical message body was inspected.

## Must fix

1. **Unsolicited completion could establish success — fixed.**
   - What this is: An account notification may arrive before a login request.
   - Problem: The initial fixture showed success from such a notification before
     this operation had started a login.
   - Fix: Require an active login attempt before accepting completion, then match
     its returned login identifier and independently read account state.
   - If skipped: Unrelated protocol state could be reported as this attempt's result.

2. **Late server refusal could leave a success result — fixed.**
   - What this is: A server request can arrive immediately after an account reply.
   - Problem: Both existing-account and new-account fixtures reproduced success
     after an unsupported tool request had already closed the connection.
   - Fix: Confirm the connection is still open when accepting account state.
   - If skipped: The caller would receive success after the control boundary failed.

3. **Native and private-process ownership remain incomplete — open.**
   - What this is: This operation consumes an explicitly owned connection.
   - Problem: Ordinary host/service/native startup does not compose the private
     authentication process, primary-owner action or challenge/cancel UI yet.
   - Fix: Wire those seams with durable lost-response handling and guarded cleanup;
     preserve provider/network/startup authorization and exact identities.
   - If skipped: This package would remain a protocol preparation, not usable sign-in.

4. **Actual activation still requires verified external prerequisites — open.**
   - What this is: The Mac package needs matching Apple identities and provider access.
   - Problem: Existing Apple access remains at sign-in; final profiles, helper network
     authority, real authentication, installed UI and TestFlight are unverified.
   - Fix: Continue code work while awaiting the existing-account access prerequisite,
     then request only concrete missing grants with the package ready to review.
   - If skipped: Engineering fixtures could be mistaken for usable delivery.

## Should fix

5. **The operation was absent from compiled output — fixed.**
   - What this is: The optional executor compiles only dependencies of its entry.
   - Problem: Source tests passed while the first artifact check could not find
     the new account module.
   - Fix: Export it through that entry and check inert import and typed consumers.
   - If skipped: Native composition could not call the tested source operation.

Challenge display is bounded and uses one exact official URL. The operation never
opens it, retries login, substitutes an API key, sends tokens or grants server tool
requests. Cancellation is followed by account inspection, so a completed login is
not mislabeled as sign-out. Disposal failure prevents a success return. One active
operation per connection, one bounded attempt timer and no polling keep the seam
small. No new dependencies, persistence schema or broad authentication framework
were introduced. No dead-code claim or fresh whole-tree audit is made.

Verdict: The optional protocol operation is verified synthetically and packaged;
private/native composition and actual provider/Apple acceptance remain open.
Not checked: Real Codex login, exact process ownership for this new operation,
credentials, browser, network entitlement, installed UI, archive/upload/TestFlight
or physical devices. Prior native/QA artifacts remain separately labeled.

```fort-review
{
  "format": 1,
  "reviewedCommit": "326eff2468f8c3fb60843438c7ceb77ef136c7bb",
  "reviewer": "Codex",
  "scope": "Optional explicit Codex device-sign-in protocol, compiled export/declarations, synthetic behavior, spec and evidence.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Final portable: 1234 passed, zero failed, ten documented skips.",
    "Final account and packaging: 47 passed, zero failed, two opt-in budget skips.",
    "Strict TypeScript passed. Existing guarded Mac executor regressions: 31 passed, three opt-in skips.",
    "Twenty isolated synthetic journeys after five warmups: p95 0.495792 ms under 25 ms.",
    "100-file host resources independently verified; optional compiled export/import/declaration consumer passed.",
    "Observed stub, unsolicited completion, missing compiled export and late forbidden-request regressions fixed.",
    "No actual Codex/account/browser/credential/startup/provider/installed UI/TestFlight execution."
  ],
  "exceptions": [
    "Proportionate semantic review; prior whole-repository baseline retained, not replaced by a new full audit.",
    "Operation has no ordinary host/native caller yet. Production launch still retains its own 120-second ceiling and guarded cleanup.",
    "No source authorization state or literal runtime version claims actual signed provider acceptance.",
    "Native and 190-file review app are unchanged; the later host resource artifact is recorded separately."
  ],
  "findings": [
    {
      "id": "ACCOUNT-UNSOLICITED",
      "detail": "An unsolicited completion before login initially could establish signed-in success. Completion now requires a started attempt; failing public regression passes.",
      "disposition": "fixed"
    },
    {
      "id": "ACCOUNT-LATE-REQUEST",
      "detail": "A forbidden server request immediately after an account reply initially left a success result. Closed connection now prevents confirmation; existing/new account regressions pass.",
      "disposition": "fixed"
    },
    {
      "id": "ACCOUNT-PACKAGE",
      "detail": "The public account operation initially was missing from compiled output. Export, inert import and declaration consumer now pass.",
      "disposition": "fixed"
    },
    {
      "id": "ACCOUNT-COMPOSITION",
      "detail": "Owned private process, primary-owner service lifecycle and native challenge/cancel/status flow are still required before usable sign-in. No ordinary caller is enabled.",
      "disposition": "open"
    },
    {
      "id": "ACCOUNT-ACTIVATION",
      "detail": "Apple records/profiles, helper network authority and explicit provider/startup grants remain unverified or unperformed. The fixture is not provider or TestFlight acceptance.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "886152f824f35143f8420128299e2ac7c290a6aad075bc49db53769dccf8e906",
    "successor/V1-ACCEPTANCE.md": "2fa0177cfffb60e7f95366e0aa86f9de81fa833e3e9e140b36769fcb6772281e",
    "successor/evidence/2026-10-17-node-foundation/codex-account-sign-in.md": "299a53afad0abb3df153d20df9573339043650f6db56fe2b42c241bd1fd0af18",
    "successor/packaging/codex-executor-artifact.test.ts": "507661ad3dc78e5f9b9041eec7aa84fa7fe6330af41a019be02e2f6881801848",
    "successor/runner/codex-account.test.ts": "7d37996002754a5d084aa48a1efbdebdb4f08d8dcfd2a6edaa4ed08529cebcdc",
    "successor/runner/codex-account.ts": "806e241982f0c4c978064e596c7a9a9231674ab6af1d0a43f0f1f6231d3f97a1",
    "successor/runner/codex-executor.ts": "882b1422df4bd83a137495c0ee1193f29dc2d656e1638ce104b47a68e9659826"
  }
}
```
