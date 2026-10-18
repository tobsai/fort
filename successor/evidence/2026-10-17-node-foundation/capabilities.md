# Enforce runtime context capabilities

Design: [Spec 059](../../../specs/059-modular-node-v1.md).

The application now checks memory and skill requirements before durable
acceptance and again before dispatch. Unsupported context returns the explicit
`capabilityUnavailable` error. A previously accepted routine that loses support
fails before calling the runtime. The service never removes selected instructions
or saved note contents to make a request appear supported.

Historical receipts, messages, notes and bundle revisions remain readable when
capabilities are removed. Repeating an accepted command still returns its exact
receipt. Restoring support allows a new request to use the same saved note.
Library authoring, review and export remain available independently of invocation.
An owner can explicitly clear a note and send plain text without memory content.
Service API capabilities still describe these storage/editing APIs; the runtime
descriptor separately describes execution support.

Two public HTTP regressions initially accepted unsupported memory and skills.
Both now reject them before writing a receipt. An extension of the skill journey
reproduced a queued routine completing after skill support was removed across
restart. Dispatch now fails that routine with no runtime call. Ordinary messages
and routine invocations share the acceptance check; future scheduled admission
uses that same routine path.

On October 18, 2026, the affected memory/skill journeys passed, strict TypeScript
passed, and the full discovered portable suite passed 818 tests with no failures
or skips. These use synthetic runtimes and data. No provider, credentials or
persistent service was activated. Native installed presentation of an unsupported
capability remains unverified. Optional implementation removal is a separate
module-boundary requirement; this slice alone does not establish it.
