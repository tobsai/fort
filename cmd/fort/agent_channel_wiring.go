package main

import (
	"fmt"

	"github.com/tobsai/fort/control"
	coreruntime "github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/ui"
)

// channelProducts is the single owner of the shared Primary execution
// lifecycle. Agent Channels wrap that same service; they do not construct a
// second runner or consult the legacy singleton Primary Agent selection.
type channelProducts struct {
	Primary       *control.PrimaryChannelService
	AgentChannels *control.AgentChannelService
	Migration     *store.AgentChannelMigrationReport
}

func (p channelProducts) Close() {
	if p.Primary != nil {
		p.Primary.Close()
	}
}

// wireChannelProducts composes the Agent product over the established Primary
// execution lifecycle. Agent cutover requires Primary preview or primary mode
// so one flag restores the exact prior shell and API without changing scheduler
// ownership or provider configuration.
func wireChannelProducts(
	deps *ui.Deps,
	st *store.Store,
	rt coreruntime.Runtime,
	capabilities control.PrimaryOptionCapabilities,
	mode ui.ProductMode,
	previewObservers ...func(store.AgentChannelMigrationReport) error,
) (channelProducts, error) {
	if deps == nil {
		return channelProducts{}, fmt.Errorf("channel product wiring: dependencies are unavailable")
	}
	if err := validateAgentChannelsCutover(mode); err != nil {
		return channelProducts{}, fmt.Errorf("channel product wiring: %w", err)
	}
	primaryEnabled := mode.PrimaryChannels == ui.PrimaryChannelsPreview || mode.PrimaryChannels == ui.PrimaryChannelsPrimary
	agentEnabled := mode.AgentChannels == ui.AgentChannelsPrimary
	if !primaryEnabled && !agentEnabled {
		return channelProducts{}, nil
	}
	if st == nil {
		return channelProducts{}, fmt.Errorf("channel product wiring: store is unavailable")
	}

	products := channelProducts{
		Primary: control.NewPrimaryChannelService(st, rt, capabilities),
	}
	if agentEnabled {
		preview, err := st.PreviewPrimaryAgentChannelMigration()
		if err != nil {
			products.Close()
			return channelProducts{}, fmt.Errorf("preview Primary Channels to Agent Channels migration: %w", err)
		}
		if len(previewObservers) > 1 {
			products.Close()
			return channelProducts{}, fmt.Errorf("preview Primary Channels to Agent Channels migration: multiple preview observers")
		}
		if len(previewObservers) == 1 && previewObservers[0] != nil {
			if err := previewObservers[0](preview); err != nil {
				products.Close()
				return channelProducts{}, fmt.Errorf("publish Primary Channels to Agent Channels migration preview: %w", err)
			}
		}
		report, err := st.MigratePrimaryAgentChannels()
		if err != nil {
			products.Close()
			return channelProducts{}, fmt.Errorf("migrate Primary Channels to Agent Channels: %w", err)
		}
		products.Migration = &report
		products.AgentChannels = control.NewAgentChannelService(st, products.Primary, nil)
	}

	// Publish ports only after the additive migration has committed. A failed
	// cutover cannot leave a partially wired agent-first surface.
	if primaryEnabled || agentEnabled {
		deps.Primary = products.Primary
	}
	if agentEnabled {
		deps.AgentChannels = products.AgentChannels
	}
	return products, nil
}
