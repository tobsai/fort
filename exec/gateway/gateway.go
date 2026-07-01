// Package gateway is an optional runtime.Runtime decorator (backlog AO-042) that
// puts spend caps, tracing, and failover in front of any underlying runtime —
// the role agentgateway/plano play in front of providers. Wire one gateway per
// flow to enforce a per-flow budget; the Tracer hook is where OpenTelemetry
// plugs in so every model call is traced.
package gateway

import (
	"context"
	"errors"
	"sync"

	"github.com/tobsai/fort/core/runtime"
)

// ErrBudgetExceeded is returned when a dispatch would exceed the spend cap.
var ErrBudgetExceeded = errors.New("gateway: per-flow spend cap exceeded")

// Tracer observes every admitted model call (implement with OTel in production).
type Tracer interface {
	Dispatch(agent, runID string, cost float64)
}

// Options configures a gateway.
type Options struct {
	Limit       float64            // spend cap; 0 = unlimited
	DefaultCost float64            // cost charged per dispatch when no per-agent cost
	Costs       map[string]float64 // per-agent cost overrides
	Tracer      Tracer             // call tracer (OTel adapter, etc.)
	Failover    map[string]string  // agent -> fallback agent on dispatch error
}

// Gateway wraps an underlying runtime.
type Gateway struct {
	under runtime.Runtime
	opts  Options

	mu    sync.Mutex
	spent float64
}

// New builds a gateway over under.
func New(under runtime.Runtime, opts Options) *Gateway {
	if opts.Tracer == nil {
		opts.Tracer = nopTracer{}
	}
	return &Gateway{under: under, opts: opts}
}

// Name implements runtime.Runtime.
func (g *Gateway) Name() string { return "gateway(" + g.under.Name() + ")" }

func (g *Gateway) cost(agent string) float64 {
	if c, ok := g.opts.Costs[agent]; ok {
		return c
	}
	return g.opts.DefaultCost
}

// Spent returns the total charged so far.
func (g *Gateway) Spent() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.spent
}

// Dispatch enforces the budget, traces the call, delegates, and fails over.
func (g *Gateway) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	cost := g.cost(spec.Agent)

	g.mu.Lock()
	if g.opts.Limit > 0 && g.spent+cost > g.opts.Limit {
		g.mu.Unlock()
		return nil, ErrBudgetExceeded
	}
	g.spent += cost
	g.mu.Unlock()

	g.opts.Tracer.Dispatch(spec.Agent, spec.RunID, cost)
	run, err := g.under.Dispatch(ctx, spec)
	if err != nil {
		if fb, ok := g.opts.Failover[spec.Agent]; ok {
			spec.Agent = fb
			g.opts.Tracer.Dispatch(fb, spec.RunID, 0)
			return g.under.Dispatch(ctx, spec)
		}
		return nil, err
	}
	return run, nil
}

type nopTracer struct{}

func (nopTracer) Dispatch(string, string, float64) {}
