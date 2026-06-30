// Package core is the umbrella for Fort's deterministic orchestration modules:
// rules, router, runtime (the executor interface), task, store, graph, inbox,
// flow, scheduler, event, and server.
//
// Seam rules (enforced by arch_test.go):
//   - core must NOT import ui or any exec concrete package.
//   - core reaches execution only through the runtime.Runtime interface;
//     cmd/fort wires the concrete exec/native runtime in.
package core
