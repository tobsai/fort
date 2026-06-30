package main

import (
	"fmt"
	"time"

	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/store"
)

func ruleLabel(d router.RouteDecision) string {
	if d.Default {
		return "default"
	}
	return d.MatchedRule
}

// streamRun blocks until the run terminates, printing events live. Used by
// `fort task add` so it doubles as a live log.
func streamRun(a *app, runID string) error {
	var cursor int64
	for {
		evs, err := a.store.Events(runID)
		if err != nil {
			return err
		}
		for _, e := range evs {
			if e.ID > cursor {
				printEvent(e)
				cursor = e.ID
			}
		}
		r, err := a.store.GetRun(runID)
		if err != nil {
			return err
		}
		if terminal(r.Status) {
			fmt.Printf("\nrun %s %s (exit %d)\n", runID, r.Status, r.ExitCode)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// tailRun prints existing events then follows new ones until the run finishes.
func tailRun(a *app, runID string) error {
	if _, err := a.store.GetRun(runID); err != nil {
		return fmt.Errorf("run %s: %w", runID, err)
	}
	return streamRun(a, runID)
}

func printEvent(e store.Event) {
	switch e.Type {
	case "message":
		fmt.Printf("  ▸ %s\n", e.Data)
	case "stderr":
		fmt.Printf("  ! %s\n", e.Data)
	case "exited":
		// shown by the summary line
	case "started":
		fmt.Printf("  · started (%s)\n", e.Data)
	default:
		if e.Data != "" {
			fmt.Printf("  %s\n", e.Data)
		}
	}
}

func terminal(status string) bool {
	return status == "succeeded" || status == "failed" || status == "canceled"
}
