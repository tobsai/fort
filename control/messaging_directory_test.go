package control_test

import (
	"context"
	"testing"

	"github.com/tobsai/fort/control"
)

type reservedDirectoryRelay struct{}

func (*reservedDirectoryRelay) Connected() bool { return false }
func (*reservedDirectoryRelay) Reserved() bool  { return true }
func (*reservedDirectoryRelay) Send(context.Context, string, string) error {
	return nil
}

func TestMessagingDirectoryRegistersProfileScopedChannelsWithHermesNames(t *testing.T) {
	ctx := context.Background()
	directory, err := control.NewMessagingDirectoryService(control.MessagingDirectoryConfig{
		HumanID:     "human:toby",
		HumanName:   "Toby",
		SourceID:    "messaging-source:macbook",
		MachineName: "tobiass.macbook.pro.lan",
	})
	if err != nil {
		t.Fatal(err)
	}

	defaultReceipt, err := directory.RegisterMessagingChannel(ctx, control.MessagingChannelRegistration{
		ConnectionID:       "connection:default:one",
		CanonicalProfileID: "default",
		DisplayName:        "Lewis",
	}, &proofRelay{connected: true})
	if err != nil {
		t.Fatal(err)
	}
	writerReceipt, err := directory.RegisterMessagingChannel(ctx, control.MessagingChannelRegistration{
		ConnectionID:       "connection:writer:one",
		CanonicalProfileID: "writer",
		DisplayName:        "Pascal",
	}, &proofRelay{connected: true})
	if err != nil {
		t.Fatal(err)
	}

	channels, err := directory.MessagingChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 2 {
		t.Fatalf("channels = %+v, want two profile-scoped channels", channels)
	}
	if defaultReceipt.ChannelID != "messaging-channel:hermes:v1:c73b902b8d7931414b7d0f89fba86f1c468f34f39475773e88ad3ac2c544445e" {
		t.Fatalf("default channel id = %q", defaultReceipt.ChannelID)
	}
	if defaultReceipt.ChannelID == writerReceipt.ChannelID || defaultReceipt.ConversationID == writerReceipt.ConversationID {
		t.Fatalf("distinct profiles collided: default=%+v writer=%+v", defaultReceipt, writerReceipt)
	}
	if channels[0].CanonicalProfileID != "default" || channels[0].DisplayName != "Lewis" ||
		channels[0].MachineName != "tobiass.macbook.pro.lan" || channels[0].State != "connected" {
		t.Fatalf("default channel = %+v", channels[0])
	}
	if channels[1].CanonicalProfileID != "writer" || channels[1].DisplayName != "Pascal" {
		t.Fatalf("writer channel = %+v", channels[1])
	}

	renamed, err := directory.RegisterMessagingChannel(ctx, control.MessagingChannelRegistration{
		ConnectionID:       "connection:default:one",
		CanonicalProfileID: "default",
		DisplayName:        "Lewis Prime",
	}, &proofRelay{connected: true})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ChannelID != defaultReceipt.ChannelID || renamed.ConversationID != defaultReceipt.ConversationID {
		t.Fatalf("presentation rename changed identity: before=%+v after=%+v", defaultReceipt, renamed)
	}
	channels, err = directory.MessagingChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if channels[0].DisplayName != "Lewis Prime" {
		t.Fatalf("Hermes rename did not update presentation: %+v", channels[0])
	}
}

func TestMessagingDirectoryRejectsDuplicateSocketWhileFirstAwaitsAcknowledgement(t *testing.T) {
	ctx := context.Background()
	directory, err := control.NewMessagingDirectoryService(control.MessagingDirectoryConfig{
		HumanID:     "human:toby",
		HumanName:   "Toby",
		SourceID:    "messaging-source:macbook",
		MachineName: "tobiass.macbook.pro.lan",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directory.RegisterMessagingChannel(ctx, control.MessagingChannelRegistration{
		ConnectionID:       "connection:default:first",
		CanonicalProfileID: "default",
		DisplayName:        "Lewis",
	}, &reservedDirectoryRelay{}); err != nil {
		t.Fatal(err)
	}
	channels, err := directory.MessagingChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].State != "offline" {
		t.Fatalf("pre-ack channel = %+v, want one Offline reservation", channels)
	}
	if _, err := directory.RegisterMessagingChannel(ctx, control.MessagingChannelRegistration{
		ConnectionID:       "connection:default:second",
		CanonicalProfileID: "default",
		DisplayName:        "Lewis",
	}, &proofRelay{connected: true}); err == nil {
		t.Fatal("second socket replaced the first registration reservation")
	}
}
