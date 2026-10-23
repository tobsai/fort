# Fort — rules for coding agents

Fort is Toby's personal multi-agent chat app. Active code is in `successor/`:
a Node/TypeScript core plus Swift Mac and iPhone apps in `successor/apple/`.
The top-level Go tree is legacy; leave it alone unless asked.
[ARCHITECTURE.md](ARCHITECTURE.md) covers the design principles in two pages.

## Hard rules

- **No API-billing fallback.** Use Toby's existing Claude and ChatGPT subscriptions
  through the official routes. Never silently switch provider, model, account or
  billing path.
- **Ask first** before adding new credentials, persistent grants or startup
  registration, agreeing to legal terms, or taking any destructive or production
  action (deleting data, cleaning up providers, publishing). Approval covers only
  the action it names.
- Keep credentials and message bodies out of the repo, logs and PRs.
- The agent host and the computer-tool sandbox are separate security boundaries.
  Don't weaken permission checks.
- `foundation/` is a quarantined donor import. Keep its license and provenance.

## How to work

1. Branch, write a failing test at the public interface, then make it pass.
2. Run `npm --prefix successor run typecheck` and `npm --prefix successor test`.
   For Swift changes on a Mac, also run `npm --prefix successor run test:macos`
   and `swift test` (see `successor/README.md`).
3. Open a PR. Say what you tested and what you didn't. CI must be green.
4. Ship betas with `make -C successor beta` (see `successor/README.md`).

Write a short spec in `specs/` only when a change touches billing or provider
path, credentials, persistent access or destructive behaviour. See
[specs/README.md](specs/README.md). Everything else is a normal PR.

Ordinary coding, tests, fixes, CI and requested delivery work need no extra
confirmation. Don't commit evidence logs; results go in the PR description.
