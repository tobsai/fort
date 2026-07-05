// Package cluster composes a machine's local runtime with remote runtimes for
// its peers (spec 022). It implements core/runtime.Runtime by dispatching each
// RunSpec to the transport for spec.Machine: the local name (or empty) runs on
// the local runtime; any other name runs on that machine's remote runtime.
//
// The machine is chosen upstream by deterministic placement (core/machines);
// cluster only carries the run to the resolved transport, so core/engine stays
// unaware of local-vs-remote.
package cluster

import (
	"context"
	"fmt"
	"sync"

	"github.com/tobsai/fort/core/runtime"
)

// Runtime is the per-machine composite runtime.
type Runtime struct {
	local   string
	localRT runtime.Runtime

	mu      sync.RWMutex
	remotes map[string]runtime.Runtime
}

// New builds a cluster runtime: localName identifies this machine, localRT runs
// its work, and remotes maps peer machine names to their runtimes.
func New(localName string, localRT runtime.Runtime, remotes map[string]runtime.Runtime) *Runtime {
	m := make(map[string]runtime.Runtime, len(remotes))
	for k, v := range remotes {
		m[k] = v
	}
	return &Runtime{local: localName, localRT: localRT, remotes: m}
}

// Name implements runtime.Runtime.
func (r *Runtime) Name() string { return "cluster:" + r.local }

// Dispatch routes spec to the transport for spec.Machine.
func (r *Runtime) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	if spec.Machine == "" || spec.Machine == r.local {
		return r.localRT.Dispatch(ctx, spec)
	}
	r.mu.RLock()
	rt, ok := r.remotes[spec.Machine]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("cluster %s: no route to machine %q", r.local, spec.Machine)
	}
	return rt.Dispatch(ctx, spec)
}

// Add installs (or replaces) the transport for a peer machine. Used by mesh
// enrollment (spec 024) to apply a join without restarting the daemon.
func (r *Runtime) Add(name string, rt runtime.Runtime) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remotes[name] = rt
}

// Remove drops the transport for a peer machine.
func (r *Runtime) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.remotes, name)
}
