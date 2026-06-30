package main

import (
	"fmt"
	"os"

	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/exec/native"
)

// app bundles the wired fort-core collaborators. cmd/fort is the composition
// root: it is the only place that imports a concrete runtime (exec/native or
// exec/fake) and injects it into core via the runtime.Runtime interface.
type app struct {
	cfg    config.Config
	store  *store.Store
	router *router.Router
	engine *engine.Engine
	rt     runtime.Runtime
}

func buildApp() (*app, error) {
	cfg := config.FromEnv(os.Getenv)

	data, err := os.ReadFile(cfg.RulesPath)
	if err != nil {
		return nil, fmt.Errorf("read ruleset %s: %w", cfg.RulesPath, err)
	}
	rs, err := rules.Parse(data)
	if err != nil {
		return nil, err
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	var rt runtime.Runtime
	if os.Getenv("FORT_FAKE") == "1" {
		rt = fake.New() // token-free mode for demos/CI
	} else {
		rt = native.New(cfg.WorkRoot, native.DefaultProviders()...)
	}

	r := router.New(rs)
	return &app{
		cfg:    cfg,
		store:  st,
		router: r,
		engine: engine.New(r, rt, st, cfg.WorkRoot),
		rt:     rt,
	}, nil
}
