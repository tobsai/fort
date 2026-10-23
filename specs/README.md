# Specs

Most changes don't need a spec. A normal PR with tests is enough.

Write a short spec, using [TEMPLATE.md](TEMPLATE.md), when a change:

- touches the billing or provider path (which account, model or subscription pays),
- adds or changes how credentials are stored or used,
- adds persistent access (startup items, background services, long-lived grants),
- deletes data or does anything else that can't be undone.

Put it in the same PR as the code, or open it first if you need Toby's decision
before building. Record Toby's decision in the spec in one line, with a date.

[Spec 059](059-modular-node-v1.md) and earlier specs are historical reference.
They describe how Fort got here, not a gate on current work.
[ARCHITECTURE.md](../ARCHITECTURE.md) holds the principles that still apply.
