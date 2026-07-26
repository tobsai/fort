package main

import (
	"fmt"
	"log/slog"
	"os"
	goruntime "runtime"
	"strconv"
	"time"

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
	"github.com/tobsai/fort/exec/meshjoin"
	"github.com/tobsai/fort/exec/native"
	"github.com/tobsai/fort/exec/remote"
	"github.com/tobsai/fort/exec/watchdog"
)

const runtimeSilenceTimeout = 10 * time.Minute

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
	rt      runtime.Runtime  // engine's runtime (cluster, wrapped by gateway if budgeted)
	localRT runtime.Runtime  // raw local runtime, for the node exec endpoint
	live    *machines.Live   // swappable registry (nil registry = single-machine)
	clus    *cluster.Runtime // hot Add/Remove of peer transports (mesh enrollment)
	caps    *capabilitySubsystem
	tokens  *meshjoin.TokenStore
}

// localName resolves the cluster's local identity: the registry's canonical
// local name when a registry is installed, else the configured NodeName.
func localName(l *machines.Live, cfg config.Config) string {
	if r := l.Load(); r != nil {
		return r.Local()
	}
	return cfg.NodeName
}

func buildApp() (*app, error) {
	cfg := config.Load(os.Getenv)
	if cfg.NodeName == "" {
		cfg.NodeName, _ = os.Hostname()
	}
	tokens := meshjoin.NewTokenStore(cfg.NodeToken, cfg.DataDir(), cfg.NodeName, cfg.Addr)

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
	var localNative *native.Runtime
	if os.Getenv("FORT_FAKE") == "1" {
		localRT = fake.New() // token-free mode for demos/CI
	} else {
		localNative = native.New(cfg.WorkRoot, native.DefaultProviders()...)
		localRT = localNative
	}

	// Multi-machine (spec 022/024): the registry lives behind a Live pointer so
	// mesh enrollment can swap it at runtime. The engine ALWAYS dispatches
	// through a cluster runtime now — with zero remotes it is a pass-through to
	// the local runtime, and enrollment can hot-add peers without re-wiring.
	live := &machines.Live{}
	if cfg.MachinesPath != "" {
		r, err := machines.Load(cfg.MachinesPath, cfg.NodeName)
		if err != nil {
			return nil, err
		}
		live.Store(r)
	}
	clus := cluster.New(localName(live, cfg), localRT, nil)
	if r := live.Load(); r != nil {
		for _, m := range r.Machines {
			if m.Name == r.Local() {
				continue
			}
			clus.Add(m.Name, remote.New(m.Name, m.URL, cfg.NodeToken))
		}
	}
	var rt runtime.Runtime = clus

	// Optional gateway: FORT_BUDGET caps spend per process and traces calls. It
	// wraps the engine's runtime so budgets span local + remote dispatch.
	if v := os.Getenv("FORT_BUDGET"); v != "" {
		if limit, err := strconv.ParseFloat(v, 64); err == nil {
			rt = gateway.New(rt, gateway.Options{Limit: limit, DefaultCost: 1, Tracer: logTracer{}})
		}
	}
	rt = watchdog.New(rt, runtimeSilenceTimeout)
	var caps *capabilitySubsystem
	if capabilityPlanningEnabled(os.Getenv) {
		revisionKey, err := config.LoadOrCreateCapabilityKey(cfg.DataDir())
		if err != nil {
			return nil, err
		}
		caps, err = buildCapabilitySubsystem(
			cfg, live, rt, revisionKey, os.Environ(),
			goruntime.GOOS+"/"+goruntime.GOARCH, tokens.Get,
		)
		if err != nil {
			return nil, err
		}
		localNative.UseVerifiedExecutables(caps.executables)
		rt = caps.runtime
	}

	r := router.New(rs)
	eng := engine.New(r, rt, st, cfg.WorkRoot)
	// The Live placer preserves single-machine semantics when no registry is
	// installed (empty pin ⇒ "",nil), so this is safe to wire unconditionally.
	eng.UsePlacer(live)
	return &app{
		cfg:     cfg,
		store:   st,
		router:  r,
		engine:  eng,
		rt:      rt,
		localRT: localRT,
		live:    live,
		clus:    clus,
		caps:    caps,
		tokens:  tokens,
	}, nil
}
