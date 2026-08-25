package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRunShowsTheEnrolledHermesBotAndRelayEndpoint(t *testing.T) {
	t.Setenv("FORT_HERMES_RELAY_SECRET", "test-shared-secret")
	var output bytes.Buffer
	err := run(context.Background(), []string{
		"-listen", "127.0.0.1:0",
		"-gateway-id", "hermes-gateway-one",
		"-binding-id", "binding-one",
		"-profile-id", "profile-one",
		"-bot-id", "hermes-bot-one",
		"-bot-name", "Scout",
		"-conversation-id", "conversation-home-one",
		"-sender-id", "fort-user-one",
		"-sender-name", "Toby",
	}, strings.NewReader(""), &output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Hermes & Scout") {
		t.Fatalf("terminal did not show the enrolled bot: %q", output.String())
	}
	if !strings.Contains(output.String(), "ws://127.0.0.1:") ||
		!strings.Contains(output.String(), "/relay") {
		t.Fatalf("terminal did not show the relay endpoint: %q", output.String())
	}
}

func TestRunStopsWhenItsContextIsCanceled(t *testing.T) {
	t.Setenv("FORT_HERMES_RELAY_SECRET", "test-shared-secret")
	input, inputWriter := io.Pipe()
	defer input.Close()
	defer inputWriter.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{
			"-listen", "127.0.0.1:0",
			"-gateway-id", "hermes-gateway-one",
			"-binding-id", "binding-one",
			"-profile-id", "profile-one",
			"-bot-id", "hermes-bot-one",
			"-bot-name", "Scout",
			"-conversation-id", "conversation-home-one",
			"-sender-id", "fort-user-one",
		}, input, io.Discard)
	}()
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		_ = inputWriter.Close()
		<-done
		t.Fatal("terminal proof ignored context cancellation while waiting for input")
	}
}
