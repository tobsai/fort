// Command hermes-relay-poc runs the disposable local connector from Spec 051.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/tobsai/fort/exec/hermesrelaypoc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	flags := flag.NewFlagSet("hermes-relay-poc", flag.ContinueOnError)
	flags.SetOutput(output)
	listenAddress := flags.String("listen", "127.0.0.1:4191", "local connector listen address")
	gatewayID := flags.String("gateway-id", "", "exact Hermes relay gateway ID")
	bindingID := flags.String("binding-id", "", "exact Fort binding ID")
	profileID := flags.String("profile-id", "", "canonical Hermes profile ID")
	botID := flags.String("bot-id", "", "exact Hermes relay bot ID")
	botName := flags.String("bot-name", "", "Hermes bot display name")
	conversationID := flags.String("conversation-id", "", "one allowed Fort Home Conversation ID")
	senderID := flags.String("sender-id", "", "stable Fort sender ID")
	senderName := flags.String("sender-name", "", "Fort sender display name")
	if err := flags.Parse(args); err != nil {
		return err
	}

	var deliverySequence atomic.Uint64
	connector, err := hermesrelaypoc.New(hermesrelaypoc.Config{
		GatewayID:             *gatewayID,
		SharedSecret:          os.Getenv("FORT_HERMES_RELAY_SECRET"),
		BindingID:             *bindingID,
		CanonicalProfileID:    *profileID,
		BotID:                 *botID,
		BotDisplayName:        *botName,
		AllowedConversationID: *conversationID,
		SenderID:              *senderID,
		SenderName:            *senderName,
		ObserveAccepted:       true,
		Deliver: func(context.Context, hermesrelaypoc.Message) (string, error) {
			return fmt.Sprintf("fort-hermes-poc:received:%d", deliverySequence.Add(1)), nil
		},
	})
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", *listenAddress)
	if err != nil {
		return fmt.Errorf("listen for Hermes relay: %w", err)
	}
	server := &http.Server{
		Handler:           connector.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(listener)
	}()

	bot := connector.Bot()
	fmt.Fprintf(output, "%s — waiting for Hermes\n", bot.Label)
	fmt.Fprintf(output, "Hermes relay endpoint: ws://%s/relay\n", listener.Addr())
	fmt.Fprintln(output, "Enter a line to send it after Hermes connects. End input to stop.")

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		for {
			message, err := connector.Receive(runCtx)
			if err != nil {
				return
			}
			fmt.Fprintf(output, "%s: %s\n", bot.Label, message.Text)
		}
	}()

	var sequence atomic.Uint64
	type inputEvent struct {
		text string
		err  error
		done bool
	}
	inputEvents := make(chan inputEvent)
	go func() {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			select {
			case inputEvents <- inputEvent{text: scanner.Text()}:
			case <-runCtx.Done():
				return
			}
		}
		select {
		case inputEvents <- inputEvent{err: scanner.Err(), done: true}:
		case <-runCtx.Done():
		}
	}()

	var inputErr error
	statusTicker := time.NewTicker(100 * time.Millisecond)
	defer statusTicker.Stop()
	wasConnected := bot.Connected
readInput:
	for {
		select {
		case event := <-inputEvents:
			if event.done {
				inputErr = event.err
				break readInput
			}
			if event.text == "" {
				continue
			}
			messageID := fmt.Sprintf("fort-hermes-poc:%d:%d", time.Now().UnixMilli(), sequence.Add(1))
			if err := connector.Send(runCtx, messageID, event.text); err != nil {
				fmt.Fprintf(output, "not sent: %v\n", err)
			}
		case <-ctx.Done():
			break readInput
		case <-statusTicker.C:
			connected := connector.Bot().Connected
			if connected != wasConnected {
				state := "disconnected"
				if connected {
					state = "connected"
				}
				fmt.Fprintf(output, "%s — %s\n", bot.Label, state)
				wasConnected = connected
			}
		}
	}
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("stop Hermes relay: %w", err)
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve Hermes relay: %w", err)
	}
	return inputErr
}
