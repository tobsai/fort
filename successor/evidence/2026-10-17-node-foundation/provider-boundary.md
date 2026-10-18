# Explicit provider selection in core and native contracts

[Spec 059](../../../specs/059-modular-node-v1.md) governs this checkpoint. The
application constructor now requires an explicit runtime descriptor. Composition
supplies Codex metadata from its adapter directory; the core does not choose that
provider. Bindings use the supplied provider, host and runtime version. The ledger
pins that identity and rejects silent replacement. A future provider/version/host
change requires explicit binding migration; this checkpoint does not implement it.

The existing `service/openai.ts` filename remains because preserved demo policies
refer to it. Its active constructor is `createApplication`; all active callers
were migrated and the old export removed. Historical packet pins, approvals and
consumed session ledgers were not changed or replayed.

The public API test first substituted Codex for an explicitly supplied stateless
fixture. It now preserves that fixture's binding and exact Unicode text through
send, completion and restart. Model discovery reports unavailable for that
fixture. Further regressions first permitted foreign provider reopening and
foreign model catalog evidence; both now fail closed.

Swift accepts local installations and available models from the selected runtime.
Settings display its actual name. Skill compatibility takes an explicit provider.
The native regression first rejected a non-Codex available model and local mode;
it now passes. A full run caught inconsistent QA fixture metadata: its binding
was simulated but its model catalog said Codex. Correcting the fixture identity
preserved strict matching. The remaining native-check caller was migrated too.

Passed: the current registered Node suite, 752 tests; the separately invoked
service/settings/pairing/shutdown suite, 66 tests before the final catalog
regression; the final catalog regression; 57 Swift tests; strict TypeScript
checking. All reported runs had zero skips. Some registered suites overlap; these
counts are not additive. The npm command still omits some source test files and
needs the planned CI/test-discovery work. No real provider, credential or app
installation was used.

This is a partial independence result. The common behavior suite across
independently shaped sessionful/stateless providers, an artifact build with the
Codex implementation absent, full optional-module capability enforcement and real
official runtime acceptance remain open. No new Claude integration is claimed.
