package main

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/tobsai/fort/control"
	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/core/machines"
	coreruntime "github.com/tobsai/fort/core/runtime"
	execcap "github.com/tobsai/fort/exec/capability"
)

var executionProfileAdapters = []string{
	"profile.claude.native",
	"profile.codex.native",
	"profile.hermes.native",
	"profile.openclaw.main",
}

type capabilitySubsystem struct {
	local       *execcap.Registry
	coordinator *control.CapabilityCoordinator
	runtime     coreruntime.Runtime
	executables *execcap.CommandResolver
}

func capabilityPlanningEnabled(getenv func(string) string) bool {
	return getenv("FORT_CAPABILITY_PLANNING") != "0" && getenv("FORT_FAKE") != "1"
}

func (s *capabilitySubsystem) refresh(ctx context.Context) (uint64, error) {
	_, generation, err := s.coordinator.Refresh(ctx, corecap.RefreshPlanning, executionProfileAdapters)
	return generation, err
}

func (s *capabilitySubsystem) poll(ctx context.Context, interval time.Duration) {
	pollCapabilityRefresh(ctx, interval, s.refresh)
}

func pollCapabilityRefresh(ctx context.Context, interval time.Duration, refresh func(context.Context) (uint64, error)) {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			refreshCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			generation, err := refresh(refreshCtx)
			cancel()
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				slog.Warn("capability inventory refresh failed", "reason", "probe_failed")
			} else {
				slog.Info("capability inventory refreshed", "generation", generation)
			}
			timer.Reset(interval)
		}
	}
}

func (s *capabilitySubsystem) start(ctx context.Context, interval time.Duration) <-chan struct{} {
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		refreshCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		generation, err := s.refresh(refreshCtx)
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("initial capability inventory failed", "reason", "probe_failed")
			}
		} else {
			slog.Info("initial capability inventory ready", "generation", generation)
		}
		s.poll(ctx, interval)
	}()
	return stopped
}

func buildCapabilitySubsystem(
	cfg config.Config,
	live *machines.Live,
	next coreruntime.Runtime,
	revisionKey []byte,
	environment []string,
	platform string,
	token func() string,
) (*capabilitySubsystem, error) {
	return buildCapabilitySubsystemWithVerifier(
		cfg, live, next, revisionKey, environment, platform, token, nil,
	)
}

func buildCapabilitySubsystemWithVerifier(
	cfg config.Config,
	live *machines.Live,
	next coreruntime.Runtime,
	revisionKey []byte,
	environment []string,
	platform string,
	token func() string,
	verifier execcap.CodexContractVerifier,
) (*capabilitySubsystem, error) {
	resolver, err := execcap.NewCommandResolver(execcap.CommandResolverOptions{
		Platform: platform, StageDir: filepath.Join(cfg.DataDir(), "capability-bin"),
		Environment: environment,
	})
	if err != nil {
		return nil, fmt.Errorf("capability wiring: command resolver: %w", err)
	}
	if verifier == nil {
		verifier = execcap.NewCodexSchemaContractVerifier(resolver)
	}
	codexInspector := execcap.NewVerifiedCodexAppServerInspector(
		execcap.ResolverCodexAppServerStarter{Resolver: resolver}, verifier,
	)
	prober := execcap.NewLocalProber(execcap.ResolverExecutor{Resolver: resolver}, codexInspector, nil, nil)
	local, err := execcap.NewRegistry(execcap.RegistryOptions{
		NodeID: localName(live, cfg), Platform: platform,
		RevisionKey: revisionKey, Prober: prober,
	})
	if err != nil {
		return nil, fmt.Errorf("capability wiring: local registry: %w", err)
	}
	coordinator, err := control.NewCapabilityCoordinator(control.CapabilityCoordinatorOptions{
		Live: live, LocalName: localName(live, cfg), Local: local,
		Peers: execcap.NewClientWithToken(token),
	})
	if err != nil {
		return nil, fmt.Errorf("capability wiring: coordinator: %w", err)
	}
	return &capabilitySubsystem{
		local: local, coordinator: coordinator,
		runtime: execcap.NewProfileGate(next, coordinator), executables: resolver,
	}, nil
}
