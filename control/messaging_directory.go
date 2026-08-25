package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/tobsai/fort/core/messaging"
	"github.com/tobsai/fort/ui"
)

var canonicalHermesProfileID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type MessagingDirectoryConfig struct {
	HumanID     string
	HumanName   string
	SourceID    string
	MachineName string
}

type MessagingChannelRegistration struct {
	ConnectionID       string
	CanonicalProfileID string
	DisplayName        string
}

type MessagingChannelRegistrationReceipt struct {
	ChannelID      string
	ConversationID string
}

// MessagingDirectoryService owns the exact profile-scoped messaging
// destinations reported by one authenticated Messaging Source. It reuses the
// proven message/event module for each channel while keeping registration and
// presentation outside that single-peer proof.
type MessagingDirectoryService struct {
	mu      sync.RWMutex
	config  MessagingDirectoryConfig
	byID    map[string]*registeredMessagingChannel
	byConvo map[string]*registeredMessagingChannel
}

type registeredMessagingChannel struct {
	channelID          string
	canonicalProfileID string
	displayName        string
	conversationID     string
	connectionID       string
	relay              *messagingRelaySlot
	service            *MessagingService
}

type messagingRelaySlot struct {
	mu    sync.RWMutex
	relay MessagingRelay
}

func (slot *messagingRelaySlot) SnapshotMessagingRelay() MessagingRelay {
	slot.mu.RLock()
	relay := slot.relay
	slot.mu.RUnlock()
	return relay
}

func (slot *messagingRelaySlot) Connected() bool {
	slot.mu.RLock()
	relay := slot.relay
	slot.mu.RUnlock()
	return relay != nil && relay.Connected()
}

func (slot *messagingRelaySlot) reserved() bool {
	relay := slot.SnapshotMessagingRelay()
	if relay == nil {
		return false
	}
	if reservation, ok := relay.(interface{ Reserved() bool }); ok {
		return reservation.Reserved()
	}
	return relay.Connected()
}

func (slot *messagingRelaySlot) Send(ctx context.Context, messageID, text string) error {
	slot.mu.RLock()
	relay := slot.relay
	slot.mu.RUnlock()
	if relay == nil {
		return errors.New("Hermes platform adapter is offline")
	}
	return relay.Send(ctx, messageID, text)
}

func (slot *messagingRelaySlot) replace(relay MessagingRelay) {
	slot.mu.Lock()
	slot.relay = relay
	slot.mu.Unlock()
}

func NewMessagingDirectoryService(config MessagingDirectoryConfig) (*MessagingDirectoryService, error) {
	for label, value := range map[string]string{
		"human id": config.HumanID, "human name": config.HumanName,
		"Messaging Source id": config.SourceID, "machine name": config.MachineName,
	} {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return nil, fmt.Errorf("messaging directory %s is required and canonical", label)
		}
	}
	return &MessagingDirectoryService{
		config:  config,
		byID:    make(map[string]*registeredMessagingChannel),
		byConvo: make(map[string]*registeredMessagingChannel),
	}, nil
}

func (directory *MessagingDirectoryService) RegisterMessagingChannel(
	ctx context.Context,
	registration MessagingChannelRegistration,
	relay MessagingRelay,
) (MessagingChannelRegistrationReceipt, error) {
	if err := ctx.Err(); err != nil {
		return MessagingChannelRegistrationReceipt{}, err
	}
	if relay == nil || strings.TrimSpace(registration.ConnectionID) == "" ||
		strings.TrimSpace(registration.ConnectionID) != registration.ConnectionID ||
		!validHermesProfileID(registration.CanonicalProfileID) ||
		!validMessagingDisplayName(registration.DisplayName) {
		return MessagingChannelRegistrationReceipt{}, errors.New("Hermes Messaging Channel registration is invalid")
	}
	channelID := messagingChannelID(directory.config.SourceID, registration.CanonicalProfileID)
	conversationID := "conversation:messaging:home:v1:" + strings.TrimPrefix(channelID, "messaging-channel:hermes:v1:")

	directory.mu.Lock()
	defer directory.mu.Unlock()
	if existing := directory.byID[channelID]; existing != nil {
		if existing.connectionID != registration.ConnectionID && existing.relay.reserved() {
			return MessagingChannelRegistrationReceipt{}, errors.New("Hermes Messaging Channel is already connected")
		}
		existing.connectionID = registration.ConnectionID
		existing.displayName = registration.DisplayName
		existing.relay.replace(relay)
		return MessagingChannelRegistrationReceipt{ChannelID: channelID, ConversationID: existing.conversationID}, nil
	}

	slot := &messagingRelaySlot{relay: relay}
	hub, err := messaging.New(messaging.Config{
		HumanID: directory.config.HumanID, PeerID: channelID,
		EndpointID: channelID, ConversationID: conversationID,
	})
	if err != nil {
		return MessagingChannelRegistrationReceipt{}, err
	}
	service, err := NewMessagingService(MessagingConfig{
		HumanID: directory.config.HumanID, PeerID: channelID,
		BotDisplayName: registration.DisplayName, MachineName: directory.config.MachineName,
		ConversationID: conversationID,
	}, hub, slot)
	if err != nil {
		return MessagingChannelRegistrationReceipt{}, err
	}
	channel := &registeredMessagingChannel{
		channelID: channelID, canonicalProfileID: registration.CanonicalProfileID,
		displayName: registration.DisplayName, conversationID: conversationID,
		connectionID: registration.ConnectionID, relay: slot, service: service,
	}
	directory.byID[channelID] = channel
	directory.byConvo[conversationID] = channel
	return MessagingChannelRegistrationReceipt{ChannelID: channelID, ConversationID: conversationID}, nil
}

func (directory *MessagingDirectoryService) MessagingChannels(context.Context) ([]ui.MessagingPeer, error) {
	directory.mu.RLock()
	channels := make([]*registeredMessagingChannel, 0, len(directory.byID))
	for _, channel := range directory.byID {
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].canonicalProfileID < channels[j].canonicalProfileID
	})
	result := make([]ui.MessagingPeer, 0, len(channels))
	for _, channel := range channels {
		state := "offline"
		if channel.relay.Connected() {
			state = "connected"
		}
		result = append(result, ui.MessagingPeer{
			ID: channel.channelID, SourceID: directory.config.SourceID,
			CanonicalProfileID: channel.canonicalProfileID,
			DisplayName:        channel.displayName, MachineName: directory.config.MachineName,
			ConversationID: channel.conversationID, State: state,
		})
	}
	directory.mu.RUnlock()
	return result, nil
}

// MessagingPeers keeps the completed Spec 052 endpoint readable while new
// clients use MessagingChannels and /api/messaging/channels.
func (directory *MessagingDirectoryService) MessagingPeers(ctx context.Context) ([]ui.MessagingPeer, error) {
	return directory.MessagingChannels(ctx)
}

func (directory *MessagingDirectoryService) MessagingEvents(ctx context.Context, conversationID string, after int64) (ui.MessagingEventPage, error) {
	channel := directory.channelForConversation(conversationID)
	if channel == nil {
		return ui.MessagingEventPage{}, codedMessagingError{code: MessagingRecipientDenied, err: messaging.ErrRecipientNotAllowed}
	}
	return channel.service.MessagingEvents(ctx, conversationID, after)
}

func (directory *MessagingDirectoryService) PostMessagingMessage(ctx context.Context, conversationID, clientMessageID, text string) (ui.MessagingPostReceipt, error) {
	channel := directory.channelForConversation(conversationID)
	if channel == nil {
		return ui.MessagingPostReceipt{}, codedMessagingError{code: MessagingRecipientDenied, err: messaging.ErrRecipientNotAllowed}
	}
	return channel.service.PostMessagingMessage(ctx, conversationID, clientMessageID, text)
}

func (directory *MessagingDirectoryService) AcceptPeerMessage(ctx context.Context, channelID string, incoming PeerMessage) (string, error) {
	directory.mu.RLock()
	channel := directory.byID[channelID]
	directory.mu.RUnlock()
	if channel == nil {
		return "", codedMessagingError{code: MessagingRecipientDenied, err: messaging.ErrRecipientNotAllowed}
	}
	incoming.ConversationID = channel.conversationID
	return channel.service.AcceptPeerMessage(ctx, incoming)
}

func (directory *MessagingDirectoryService) channelForConversation(conversationID string) *registeredMessagingChannel {
	directory.mu.RLock()
	channel := directory.byConvo[conversationID]
	directory.mu.RUnlock()
	return channel
}

func validHermesProfileID(value string) bool {
	if !canonicalHermesProfileID.MatchString(value) {
		return false
	}
	switch value {
	case "hermes", "test", "tmp", "root", "sudo":
		return false
	default:
		return true
	}
}

func validMessagingDisplayName(value string) bool {
	return utf8.ValidString(value) && strings.TrimSpace(value) != "" &&
		strings.TrimSpace(value) == value && utf8.RuneCountInString(value) <= 64
}

func messagingChannelID(sourceID, profileID string) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte("fort-hermes-messaging-channel:v1\n"))
	for _, value := range []string{sourceID, profileID} {
		_, _ = digest.Write([]byte(strconv.Itoa(len([]byte(value)))))
		_, _ = digest.Write([]byte{':'})
		_, _ = digest.Write([]byte(value))
	}
	return "messaging-channel:hermes:v1:" + hex.EncodeToString(digest.Sum(nil))
}

var _ ui.MessagingPort = (*MessagingDirectoryService)(nil)
