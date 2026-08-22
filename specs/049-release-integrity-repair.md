# Spec 049 — Release Integrity Repair

## Goal

Restore Fort to one functional native Mac installation and working web sign-in
without weakening immutable Agent bindings or silently substituting a provider,
model, computer, or execution contract.

## Confirmed failures

- The visible `/Applications/Fort.app` is the iPhone/iPad build, while launchd
  is still running the native `1.0.3` daemon from `~/.Trash/FortMac.app`.
- The current ChatGPT-bundled Codex executable is
  `codex-cli 0.149.0-alpha.4.1`; the accepted subscription contract still pins
  `0.147.0-alpha.6.5`, so the selected Agent is
  `setup_required / incompatible_version` and Send correctly fails closed.
- `fort-gateway-preview` has no Google OAuth client variables and emits an
  authorization redirect with `client_id=undefined`.

## Approach

1. Advance the exact Codex subscription contract only after validating the
   installed executable bytes, `exec` flags, disabled-feature inventory, and
   freshly generated normal and experimental app-server schema bundles.
2. Keep the existing fail-closed version, executable, schema, policy, adapter,
   and isolation checks. Existing bindings are never mutated in place; create
   a user-authorized replacement Agent Channel bound to the ready option, then
   archive the drifted channel without deleting its conversation history.
3. Configure the preview gateway with encrypted Google OAuth credentials and a
   registered preview callback. Never expose secret values in source or logs.
4. Produce a new signed and notarized native Mac bundle containing the repaired
   daemon. Stop the daemon running from Trash, move the iOS-on-Mac bundle and
   obsolete backup bundles to recoverable Trash locations, install exactly one
   native bundle in `/Applications`, and preserve the validated launchd
   environment when restarting it.

## Accepted Codex evidence

- ChatGPT app: `26.818.41705` (`6971`), signed, notarized, and
  Gatekeeper-accepted from the official Sparkle update staged on the Mac mini
- Version: `codex-cli 0.149.0-alpha.4.1`
- Executable SHA-256:
  `fa8b41f0e7ae971171d05ca55451a3ffb8b7e74e01837a2f5c177513a5403c5d`
- Normal schema: 291 JSON files,
  `bfa21213f862696b6919e8ddf60c454be5f24e6f432735651fc4fbaa7d2b3919`
- Experimental schema: 401 JSON files,
  `780383c87746e4840e0eaeef83f636030c291ed05b44be2cb233c39e757a144a`

## Acceptance

- The existing contract test fails against the old pin, then passes against
  the exact evidence above.
- Agent enrollment keeps deterministic computer grouping and presents ready
  options before setup, unavailable, and ineligible inventory on each computer.
- `go test ./...`, focused race tests, `go vet ./...`, Swift contract checks,
  and release configuration checks pass.
- Both gateway origins produce Google authorization redirects with a nonempty
  client ID and their exact HTTPS callback URI; a real sign-in reaches Fort.
- Exactly one Fort application bundle is installed in `/Applications`; launchd
  executes the daemon from that bundle, not Trash or a backup.
- The live Agent option and replacement Agent Channel report `ready`, the
  drifted channel is archived with its history intact, Send is enabled with
  nonblank text, and one real smoke turn completes with a persisted
  message/receipt.

## Rollback

- Restore the prior launchd plist and recover the moved application bundles
  from Trash.
- Reinstall the prior notarized DMG if the new native package fails validation.
- Remove only the preview OAuth variables added by this repair if its callback
  cannot be registered.
- Revert the contract revision; existing immutable binding history remains
  intact and will fail closed rather than reroute.

## Authorization

Toby authorized implementation and live repair with “Fix everything” on
2026-08-22.
