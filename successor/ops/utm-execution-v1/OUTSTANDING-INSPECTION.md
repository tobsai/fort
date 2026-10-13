# Separate remaining gates

R5 disabled source is accepted with zero findings. R6 is a concrete activation
candidate for independent review. The two inspection gates are prepared; guest
execution remains false. Neither activation entrypoint has run.

Read-only host/tool/capacity prerequisites are recorded in `evidence/r6/` and
summarized in [APPROVAL.md](APPROVAL.md). Exact pins, sources, limits and exclusions
are in [activation-operation.json](activation-operation.json). These are snapshots,
not reservations; recheck changing prerequisites before an approved run. Real Node
permissions, resource-limit enforcement, TLS, signatures and archive compatibility
cannot be established by the safe inspections performed here. They remain unrun.

Before any acquisition effect: accept the exact activation commit/seal, obtain
explicit owner authority for one consumed attempt and its narrow execution
allowance. The operation would acquire only eight non-UTM inputs, authenticate and
import the checked OpenPGP entry in the restricted Node worker, then inventory the
Ubuntu archive. It must end with `image_bound: false` and `runtime_authorized: false`.
Unsupported behavior stops; no fallback, silent pin/tool change or retry is included.
Successful artifacts remain for a recorded ownership/evidence handoff; uncertain
state remains quarantined. Existing roots are not adopted or broadly removed.

A later VM decision still requires real member/kernel/initrd/label and seed/ISO
binding, fresh registry/bookmark capture, app/helper/PTY attribution, owned cleanup
and exact attended runtime authority. No UTM.dmg, installation, mount/conversion,
guest extraction, provider/account/credential action, startup service or global
security change belongs to this input operation. T07, real canaries, Stop/quiescence
and installed-product/provider acceptance remain open. No provider cleanup is
implicitly authorized.
