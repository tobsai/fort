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

// Roster adapts a machine registry to ui.MachineLister and tracks peer
// reachability by polling each machine's /health (spec 022). The local machine
// is reachable by definition; peers start unreachable until first probed.
type Roster struct {
	reg    *machines.Registry
	client *http.Client

	mu        sync.Mutex
	reachable map[string]bool
}

// NewRoster builds a roster over reg.
func NewRoster(reg *machines.Registry) *Roster {
	return &Roster{
		reg:       reg,
		client:    &http.Client{Timeout: 3 * time.Second},
		reachable: map[string]bool{},
	}
}

// Machines implements ui.MachineLister.
func (r *Roster) Machines() []ui.MachineStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ui.MachineStatus, 0, len(r.reg.Machines))
	for _, m := range r.reg.Machines {
		local := m.Name == r.reg.Local()
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
	for _, m := range r.reg.Machines {
		if m.Name == r.reg.Local() {
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
