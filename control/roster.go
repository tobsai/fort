package control

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tobsai/fort/core/machines"
	"github.com/tobsai/fort/ui"
)

// Roster adapts a live machine registry pointer to ui.MachineLister and tracks
// peer reachability by polling each machine's /health (spec 022). It reads the
// Live pointer on every call, so a registry installed after construction (e.g.
// a mesh enrollment) is visible without a restart (spec 024). The local
// machine is reachable by definition; peers start unreachable until first
// probed.
type Roster struct {
	live   *machines.Live
	client *http.Client

	mu        sync.Mutex
	reachable map[string]bool
}

// NewRoster builds a roster over live.
func NewRoster(live *machines.Live) *Roster {
	return &Roster{
		live:      live,
		client:    &http.Client{Timeout: 3 * time.Second},
		reachable: map[string]bool{},
	}
}

// Machines implements ui.MachineLister.
func (r *Roster) Machines() []ui.MachineStatus {
	reg := r.live.Load()
	if reg == nil {
		// Non-nil empty slice: /api/machines must emit [], never null, so
		// strictly-typed clients (the Swift surfaces) decode cleanly.
		return []ui.MachineStatus{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ui.MachineStatus, 0, len(reg.Machines))
	for _, m := range reg.Machines {
		local := m.Name == reg.Local()
		out = append(out, ui.MachineStatus{
			Name:      m.Name,
			URL:       m.URL,
			Agents:    m.Agents,
			Local:     local,
			Reachable: local || r.reachable[m.Name],
		})
	}
	return out
}

// Poll refreshes peer reachability every interval until ctx is done. Run it in a
// goroutine from the composition root.
func (r *Roster) Poll(ctx context.Context, interval time.Duration) {
	r.probe(ctx)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.probe(ctx)
		}
	}
}

func (r *Roster) probe(ctx context.Context) {
	reg := r.live.Load()
	if reg == nil {
		return
	}
	for _, m := range reg.Machines {
		if m.Name == reg.Local() {
			continue
		}
		ok := r.ping(ctx, m.URL)
		r.mu.Lock()
		r.reachable[m.Name] = ok
		r.mu.Unlock()
	}
}

func (r *Roster) ping(ctx context.Context, base string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
