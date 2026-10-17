# Owner-demo startup repair

The first operator invocation passed read-only preparation and native exit checks,
then failed with a suppressed exception. The exact session root never existed.
The fixed startup order writes the session directory/state before cache mutation,
service creation, runtime launch or enrollment, and never deletes that directory.
Independent inspection found no root, lock, ledger, pairing or native connection;
all 418 preserved files were unchanged. No session clock or credential lifetime
began. The existing approved action is still available; no consumed packet is reset.

The original exception cannot be recovered from the generic output. The operator's
workspace differs from Fort's root, making a sandbox write refusal plausible.
The new --check-writes operation tests that exact root with a temporary inert file,
then removes only its own probe. It starts no provider, timer, app or enrollment.
The manual now calls for the supported require_escalated execution path for both
that preflight and the attended launcher. No OS permission or security setting changes.

The launcher reports a fixed startup stage, allowlisted errno and presence-based
attempt status, never a raw exception, path, stack or provider payload. EACCES/EPERM
before a durable session produces actionable same-approval retry guidance. Existing
session directory/lock or unreadable evidence still blocks retry. The original scope
and owner approval remain byte-identical; initial execution pins are archived here.

Tests cover pre-session failure classification and redaction, existing-state retry
refusal and actual temporary write-probe cleanup. No live demo activation is part
of repair validation. See diagnosis.json and checks.json for exact results.
