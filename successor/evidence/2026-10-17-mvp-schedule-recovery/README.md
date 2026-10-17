# Schedule recovery — October 17, 2026

Source `ced1cc6ad74c6866c9fee353c8ce42b27e1ef1c6` fixes a one-time schedule
that was offline for more than 366 days. Previously the recurring-calendar cap
replaced that exact occurrence with a range summary, bypassing the owner's
one-catch-up choice. One-time recovery now resolves its single dated occurrence
directly, preserving its intended instant and either skipping or accepting it
according to the saved policy. Duplicate ticks and restarts preserve one occurrence.

Recurring calendars retain the bounded 366-day enumeration plus an older-range
summary. A synthetic four-year outage proves skip yields no run and catch-up yields
at most the latest one. The old synthetic device credential expires as expected;
test-only reenrollment is used to inspect recovery. No actual runtime or live
credential is involved in those schedule tests.

The new tests first showed one pass and one failure: the old one-time occurrence
returned a null intended instant. After the fix, all **43 actual-service synthetic
tests pass**, with zero failures/skips; strict typechecking passes. The Swift HTTP
journey passes against the changed service: ten synthetic executions, six restarts,
two discarded acceptances, one discarded cancellation, no duplicate/canceled dispatch
and an empty pending queue. Schedule consent still produces no runtime call.

The prior broad checkpoint has 654 TypeScript and 33 Swift passes and fresh Mac /
iPhone Simulator builds; see [its evidence](../2026-10-17-mvp-pending/README.md).
Those broad suites and builds were not repeated for this service-only fix. Git
comparison confirms native source is unchanged; the exact previously verified Swift
binary was reused for HTTP conformance. Native build source remains `8a877ff`.
Persistent raw logs and hashes are in checks.json. The initial failing log is kept.

All six consumed packets, the native cache and main checkout baseline still match.
No installed app, Simulator UI, physical device, provider session, startup service,
remote publication or deployment ran. The live enrollment proposal remains pending.
Real sleep/reboot, installed-provider acceptance, host isolation, ownership recovery
and release remain open. This closes the synthetic long-downtime calendar gap only.
