# Authenticated core transport over a private socket

The core can now explicitly select loopback TCP or a private Unix socket using
`ServiceListener`. Its typed `ServiceEndpoint` distinguishes the result. Private
mode exposes no TCP URL. The same HTTP routes, authentication, installation
scope, Origin restriction, durable acceptance and replay apply to both modes.
This is local source preparation under [Spec 059](../../../specs/059-modular-node-v1.md),
not an installed Mac host or reachable iPhone service.

Private mode requires a short socket filename, the ledger directory as the
process working directory, ownership by the current user and no group/other
directory permissions. The socket is mode 0600. An occupied path is rejected
without removing its contents. A caller's later listener-object mutation cannot
change the selected filename during asynchronous startup. Existing prototype
callers now select loopback explicitly; no historical activation packet or
consumed authorization was modified or run.

Verification used a separate synthetic core process and an HTTP client process:

- Red: the service had no private endpoint and accepted an occupied-path setup
  as ordinary loopback startup.
- Green: two public transport tests passed. They cover absent credentials,
  wrong installation identity, Origin denial, exact Unicode/whitespace text,
  durable reply/replay, mode 0600, normal process exit, socket removal, restart
  and duplicate receipt recovery without another message.
- Negative startup checks preserve an occupied regular-file marker and reject
  traversal, a different working directory and group/other directory access.
- A second red check changed the caller's listener object after starting. It
  reproduced startup failure before selection was copied and passed afterward.
- Final strict TypeScript checking and all 831 portable tests passed, with no
  skips. The compiled Swift HTTP journey also passed, retaining ten synthetic
  executions, six restarts, lost-response recovery and no duplicate/canceled
  dispatch. The structural and accepted spec gates passed.

The separate [signed sandbox proof](local-ipc.md) established a private socket
inside an inherited sandbox with TCP/outside-file denial. These new service
tests are ordinary owned processes. They do not yet establish the compiled core
inside that signed bundle, Swift/XPC peer authorization, a real owned-host
adapter, login/background registration, credential recovery, official runtime
confinement, app update/removal or TestFlight. No provider/account, persistent
service or externally reachable endpoint was activated.

Logs: `/tmp/fort059-private-transport-red.log`,
`/tmp/fort059-private-transport-green.log`,
`/tmp/fort059-private-transport-pin-red.log`,
`/tmp/fort059-private-transport-typecheck.log`,
`/tmp/fort059-private-transport-portable.log`,
`/tmp/fort059-private-transport-native-http.log`.
