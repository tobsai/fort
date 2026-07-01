package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/machines"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/cluster"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/exec/gateway"
	"github.com/tobsai/fort/exec/native"
	"github.com/tobsai/fort/exec/remote"
)

// logTracer is the default gateway Tracer — structured logs per model call.
// Swap for an OpenTelemetry adapter in production.
type logTracer struct{}

func (logTracer) Dispatch(agent, runID string, cost float64) {
	slog.Info("model call", "agent", agent, "run", runID, "cost", cost)
}

// app bundles the wired fort-core collaborators. cmd/fort is the composition
// root: it is the only place that imports a concrete runtime (exec/native or
// exec/fake) and injects it into core via the runtime.Runtime interface.
type app struct {
	cfg     config.Config
	store   *store.Store
	router  *router.Router
	engine  *engine.Engine
	rt      runtime.Runtime    // engine's runtime (cluster in multi-machine mode)
	localRT runtime.Runtime    // raw local runtime, for the node exec endpoint
	reg     *machines.Registry // nil in single-machine mode (spec 022)
}

func buildApp() (*app, error) {
	cfg := config.FromEnv(os.Getenv)
	if cfg.NodeName == "" {
		cfg.NodeName, _ = os.Hostname()
	}

	data, err := os.ReadFile(cfg.RulesPath)
	if err != nil {
		// Fall back to the embedded default ruleset only when the user did not
		// override the path — a brew install ships no rules/ directory. An
		// explicit FORT_RULES pointing at a missing file is still an error.
		if os.IsNotExist(err) && cfg.RulesPath == config.Default().RulesPath {
			data = defaultRulesYAML
		} else {
			return nil, fmt.Errorf("read ruleset %s: %w", cfg.RulesPath, err)
		}
	}
	rs, err := rules.Parse(data)
	if err != nil {
		return nil, err
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	// The local execution runtime spawns CLIs on this machine.
	var localRT runtime.Runtime
	if os.Getenv("FORT_FAKE") == "1" {
		localRT = fake.New() // token-free mode for demos/CI
	} else {
		localRT = native.New(cfg.WorkRoot, native.DefaultProviders()...)
	}

	// Multi-machine (spec 022): with a registry, the engine dispatches through a
	// cluster runtime (local + remote peers) and places deterministically.
	rt := localRT
	var reg *machines.Registry
	var placer engine.Placer
	if cfg.MachinesPath != "" {
		r, err := machines.Load(cfg.MachinesPath, cfg.NodeName)
		if err != nil {
			return nil, err
		}
		reg = r
		placer = r
		remotes := map[string]runtime.Runtime{}
		for _, m := range r.Machines {
			if m.Name == r.Local() {
				continue
			}
			remotes[m.Name] = remote.New(m.Name, m.URL, cfg.NodeToken)
		}
		rt = cluster.New(r.Local(), localRT, remotes)
	}

	// Optional gateway: FORT_BUDGET caps spend per process and traces calls. It
	// wraps the engine's runtime so budgets span local + remote dispatch.
	if v := os.Getenv("FORT_BUDGET"); v != "" {
		if limit, err := strconv.ParseFloat(v, 64); err == nil {
			rt = gateway.New(rt, gateway.Options{Limit: limit, DefaultCost: 1, Tracer: logTracer{}})
		}
	}

	r := router.New(rs)
	eng := engine.New(r, rt, st, cfg.WorkRoot)
	if placer != nil {
		eng.UsePlacer(placer)
	}
	return &app{
		cfg:     cfg,
		store:   st,
		router:  r,
		engine:  eng,
		rt:      rt,
		localRT: localRT,
		reg:     reg,
	}, nil
}
