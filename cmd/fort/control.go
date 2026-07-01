package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/core/inbox"
	"github.com/tobsai/fort/core/server"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/ui"
)

// cmdControl boots the CONTROL PLANE ONLY: board, chat, scheduler, gate inbox,
// and the live feed — with no deterministic components (router, native runtime,
// DAG engine) and no agent CLIs. Submitted tasks are boarded as "queued"; an
// execution plane can pick them up later, or you run the full `fort serve`.
func cmdControl(args []string) error {
	fs := flag.NewFlagSet("control", flag.ExitOnError)
	inboxDir := fs.String("inbox", ".fort-native/inbox", "task inbox directory to watch")
	_ = fs.Parse(args)

	cfg := config.FromEnv(os.Getenv)
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Control-only: board tasks via the queue dispatcher, no execution plane.
	dispatcher := control.NewQueueDispatcher(st)
	uiSrv := ui.New(ui.Deps{Dispatcher: dispatcher, Runner: nil, Store: st})

	in := inbox.NewDir(*inboxDir, queueInbox{dispatcher})
	go func() {
		fmt.Printf("fort: watching inbox %s\n", *inboxDir)
		_ = in.Watch(ctx, time.Second)
	}()

	srv := server.New(server.Deps{Config: cfg, Store: st, Mount: uiSrv.Register})
	fmt.Printf("fort CONTROL PLANE on http://%s/  (board · chat · scheduler · gate inbox)\n", cfg.Addr)
	fmt.Println("  execution plane: none — tasks are boarded as queued")
	return srv.Run(ctx)
}

// queueInbox adapts the control-plane dispatcher to the inbox.Submitter port
// (which returns a plain run id).
type queueInbox struct{ d control.QueueDispatcher }

func (q queueInbox) Submit(ctx context.Context, t task.Task) (string, error) {
	ref, err := q.d.Submit(ctx, t)
	return ref.RunID, err
}
