package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tobsai/fort/control"
	fortconfig "github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/core/messaging"
	"github.com/tobsai/fort/exec/hermesplatform"
	"github.com/tobsai/fort/exec/hermesrelaypoc"
	"github.com/tobsai/fort/ui"
)

const hermesMessagingConfigName = "hermes-messaging.json"
const hermesPlatformConfigName = "hermes-platform.json"

type hermesMessagingFileConfig struct {
	GatewayID          string `json:"gateway_id"`
	SharedSecret       string `json:"shared_secret"`
	BindingID          string `json:"binding_id"`
	CanonicalProfileID string `json:"canonical_profile_id"`
	BotID              string `json:"bot_id"`
	BotDisplayName     string `json:"bot_display_name"`
	ConversationID     string `json:"conversation_id"`
	HumanID            string `json:"human_id"`
	HumanName          string `json:"human_name"`
	PeerID             string `json:"peer_id"`
	EndpointID         string `json:"endpoint_id"`
}

type hermesMessagingProducts struct {
	Service    ui.MessagingPort
	LocalRelay http.Handler
	LocalPath  string
}

func wireHermesMessaging(dataDir, machineName string) (hermesMessagingProducts, error) {
	platformPath := filepath.Join(dataDir, hermesPlatformConfigName)
	if _, err := os.Stat(platformPath); err == nil {
		if _, legacyErr := os.Stat(filepath.Join(dataDir, hermesMessagingConfigName)); legacyErr == nil {
			return hermesMessagingProducts{}, errors.New("Hermes platform and completed proof configs cannot both be active")
		} else if !errors.Is(legacyErr, os.ErrNotExist) {
			return hermesMessagingProducts{}, fmt.Errorf("Hermes proof config: %w", legacyErr)
		}
		return wireHermesPlatform(platformPath, machineName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return hermesMessagingProducts{}, fmt.Errorf("Hermes platform config: %w", err)
	}

	configPath := filepath.Join(dataDir, hermesMessagingConfigName)
	info, err := os.Stat(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return hermesMessagingProducts{}, nil
	}
	if err != nil {
		return hermesMessagingProducts{}, fmt.Errorf("Hermes messaging config: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return hermesMessagingProducts{}, errors.New("Hermes messaging config must be a regular file readable only by its owner")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return hermesMessagingProducts{}, fmt.Errorf("read Hermes messaging config: %w", err)
	}
	var config hermesMessagingFileConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return hermesMessagingProducts{}, fmt.Errorf("decode Hermes messaging config: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return hermesMessagingProducts{}, errors.New("Hermes messaging config must contain one JSON object")
	}
	for label, value := range map[string]string{
		"gateway id": config.GatewayID, "shared secret": config.SharedSecret,
		"binding id": config.BindingID, "canonical profile id": config.CanonicalProfileID,
		"bot id": config.BotID, "bot display name": config.BotDisplayName,
		"Conversation id": config.ConversationID, "human id": config.HumanID,
		"human name": config.HumanName, "peer id": config.PeerID, "endpoint id": config.EndpointID,
	} {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return hermesMessagingProducts{}, fmt.Errorf("Hermes messaging %s is required and canonical", label)
		}
	}

	hub, err := messaging.New(messaging.Config{
		HumanID: config.HumanID, PeerID: config.PeerID,
		EndpointID: config.EndpointID, ConversationID: config.ConversationID,
	})
	if err != nil {
		return hermesMessagingProducts{}, err
	}
	var service *control.MessagingService
	connector, err := hermesrelaypoc.New(hermesrelaypoc.Config{
		GatewayID: config.GatewayID, SharedSecret: config.SharedSecret,
		BindingID: config.BindingID, CanonicalProfileID: config.CanonicalProfileID,
		BotID: config.BotID, BotDisplayName: config.BotDisplayName,
		AllowedConversationID: config.ConversationID,
		SenderID:              config.HumanID, SenderName: config.HumanName,
		Deliver: func(ctx context.Context, message hermesrelaypoc.Message) (string, error) {
			if service == nil {
				return "", errors.New("Fort messaging service is unavailable")
			}
			return service.AcceptPeerMessage(ctx, control.PeerMessage{
				RequestID: message.RequestID, ConversationID: message.ConversationID,
				Text: message.Text, InReplyToMessageID: message.InReplyToMessageID,
			})
		},
	})
	if err != nil {
		return hermesMessagingProducts{}, err
	}
	service, err = control.NewMessagingService(control.MessagingConfig{
		HumanID: config.HumanID, PeerID: config.PeerID,
		BotDisplayName: config.BotDisplayName,
		MachineName:    machineName,
		ConversationID: config.ConversationID,
	}, hub, hermesRelayControlAdapter{connector: connector})
	if err != nil {
		return hermesMessagingProducts{}, err
	}
	return hermesMessagingProducts{Service: service, LocalRelay: connector.Handler(), LocalPath: "/relay"}, nil
}

type hermesPlatformFileConfig struct {
	ProfileTokenKey string `json:"profile_token_key"`
	HumanID         string `json:"human_id"`
	HumanName       string `json:"human_name"`
}

func wireHermesPlatform(configPath, machineName string) (hermesMessagingProducts, error) {
	info, err := os.Stat(configPath)
	if err != nil {
		return hermesMessagingProducts{}, fmt.Errorf("Hermes platform config: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return hermesMessagingProducts{}, errors.New("Hermes platform config must be a regular file readable only by its owner")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return hermesMessagingProducts{}, fmt.Errorf("read Hermes platform config: %w", err)
	}
	var platformConfig hermesPlatformFileConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&platformConfig); err != nil {
		return hermesMessagingProducts{}, fmt.Errorf("decode Hermes platform config: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return hermesMessagingProducts{}, errors.New("Hermes platform config must contain one JSON object")
	}
	for label, value := range map[string]string{
		"profile token key": platformConfig.ProfileTokenKey,
		"human id":          platformConfig.HumanID,
		"human name":        platformConfig.HumanName,
	} {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return hermesMessagingProducts{}, fmt.Errorf("Hermes platform %s is required and canonical", label)
		}
	}
	relayConfig, err := fortconfig.LoadRelay(filepath.Dir(configPath))
	if err != nil {
		return hermesMessagingProducts{}, fmt.Errorf("Hermes platform machine identity: %w", err)
	}
	if strings.TrimSpace(relayConfig.MachineID) == "" || strings.TrimSpace(relayConfig.MachineID) != relayConfig.MachineID ||
		strings.ContainsAny(relayConfig.MachineID, "\r\n") {
		return hermesMessagingProducts{}, errors.New("Hermes platform machine id is required and canonical")
	}
	sourceID := "messaging-source:fort-machine:v1:" + relayConfig.MachineID

	directory, err := control.NewMessagingDirectoryService(control.MessagingDirectoryConfig{
		HumanID: platformConfig.HumanID, HumanName: platformConfig.HumanName,
		SourceID: sourceID, MachineName: machineName,
	})
	if err != nil {
		return hermesMessagingProducts{}, err
	}
	connector, err := hermesplatform.New(hermesplatform.Config{
		ProfileTokenKey: platformConfig.ProfileTokenKey,
		SenderID:        platformConfig.HumanID,
		SenderName:      platformConfig.HumanName,
		Register: func(ctx context.Context, registration hermesplatform.Registration, sender hermesplatform.Sender) (hermesplatform.RegistrationReceipt, error) {
			receipt, err := directory.RegisterMessagingChannel(ctx, control.MessagingChannelRegistration{
				ConnectionID: registration.ConnectionID, CanonicalProfileID: registration.CanonicalProfileID,
				DisplayName: registration.DisplayName,
			}, sender)
			return hermesplatform.RegistrationReceipt{
				ChannelID: receipt.ChannelID, ConversationID: receipt.ConversationID,
			}, err
		},
		Deliver: func(ctx context.Context, channelID string, message hermesplatform.Message) (string, error) {
			return directory.AcceptPeerMessage(ctx, channelID, control.PeerMessage{
				RequestID: message.RequestID, ConversationID: message.ConversationID,
				Text: message.Text, InReplyToMessageID: message.InReplyToMessageID,
			})
		},
	})
	if err != nil {
		return hermesMessagingProducts{}, err
	}
	return hermesMessagingProducts{
		Service: directory, LocalRelay: connector.Handler(), LocalPath: "/platforms/hermes",
	}, nil
}

type hermesRelayControlAdapter struct {
	connector *hermesrelaypoc.Connector
}

func (adapter hermesRelayControlAdapter) Connected() bool {
	return adapter.connector != nil && adapter.connector.Bot().Connected
}

func (adapter hermesRelayControlAdapter) Send(ctx context.Context, messageID, text string) error {
	if adapter.connector == nil {
		return errors.New("Hermes relay is unavailable")
	}
	return adapter.connector.Send(ctx, messageID, text)
}
