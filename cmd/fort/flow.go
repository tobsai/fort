package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/flow"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/scheduler"
)

func flowsDir() string {
	if v := os.Getenv("FORT_FLOWS"); v != "" {
		return v
	}
	return "flows"
}

// fort flow list | run <id> [--input S]
func cmdFlow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: fort flow list | fort flow run <id> [--input S]")
	}
	switch args[0] {
	case "list":
		flows, err := flow.LoadDir(flowsDir())
		if err != nil {
			return err
		}
		for _, f := range flows {
			fmt.Printf("%-16s %s (%d nodes)\n", f.ID, f.Name, len(f.Nodes))
		}
		return nil
	case "run":
		fs := flag.NewFlagSet("flow run", flag.ExitOnError)
		input := fs.String("input", "", "initial payload")
		_ = fs.Parse(args[1:])
		if fs.NArg() < 1 {
			return fmt.Errorf("usage: fort flow run <id> [--input S]")
		}
		return runFlow(fs.Arg(0), *input)
	default:
		return fmt.Errorf("usage: fort flow list | fort flow run <id>")
	}
}

func runFlow(id, input string) error {
	f, err := loadFlowByID(id)
	if err != nil {
		return err
	}
	a, err := buildApp()
	if err != nil {
		return err
	}
	defer a.store.Close()

	ex := graph.NewExecutor(a.rt, a.store)
	runID := uuid.NewString()
	fmt.Printf("flow %s -> run %s\n", id, runID)
	res, err := ex.Start(context.Background(), f, runID, input)
	if err != nil {
		return err
	}
	printFlowResult(runID, res)
	return nil
}

// fort gate list | approve <run> <node> [--edit S] | reject <run> <node>
func cmdGate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: fort gate list | approve <run> <node> | reject <run> <node>")
	}
	a, err := buildApp()
	if err != nil {
		return err
	}
	defer a.store.Close()

	switch args[0] {
	case "list":
		gates, err := a.store.WaitingGates()
		if err != nil {
			return err
		}
		if len(gates) == 0 {
			fmt.Println("(no gates awaiting decision)")
			return nil
		}
		fmt.Printf("%-36s  %-14s\n", "RUN", "GATE")
		for _, g := range gates {
			fmt.Printf("%-36s  %-14s\n", g.RunID, g.NodeID)
		}
		return nil

	case "approve", "reject":
		if len(args) < 3 {
			return fmt.Errorf("usage: fort gate %s <run> <node>", args[0])
		}
		runID, nodeID := args[1], args[2]
		ex := graph.NewExecutor(a.rt, a.store)
		if args[0] == "approve" {
			edit := ""
			if len(args) >= 5 && args[3] == "--edit" {
				edit = args[4]
			}
			if err := ex.Approve(runID, nodeID, edit); err != nil {
				return err
			}
		} else {
			if err := ex.Reject(runID, nodeID); err != nil {
				return err
			}
		}
		// resume the flow
		run, err := a.store.GetRun(runID)
		if err != nil {
			return err
		}
		f, err := loadFlowByID(run.FlowID)
		if err != nil {
			return err
		}
		res, err := ex.Resume(context.Background(), f, runID)
		if err != nil {
			return err
		}
		printFlowResult(runID, res)
		return nil
	default:
		return fmt.Errorf("usage: fort gate list | approve | reject")
	}
}

// fort schedule cron <spec> <flow> | once <duration> <flow>
func cmdSchedule(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: fort schedule cron <spec> <flow> | once <duration> <flow>")
	}
	mode := args[0]
	f, err := loadFlowByID(args[2])
	if err != nil {
		return err
	}
	a, err := buildApp()
	if err != nil {
		return err
	}
	defer a.store.Close()
	ex := graph.NewExecutor(a.rt, a.store)

	fire := func() {
		runID := uuid.NewString()
		fmt.Printf("\n[scheduler] firing %s -> run %s\n", f.ID, runID)
		if _, err := ex.Start(context.Background(), f, runID, ""); err != nil {
			fmt.Fprintln(os.Stderr, "scheduler:", err)
		}
	}

	s := scheduler.New()
	defer s.Stop()
	switch mode {
	case "cron":
		if _, err := s.Cron(args[1], fire); err != nil {
			return err
		}
		s.Start()
		fmt.Printf("scheduled %s on cron %q — Ctrl-C to stop\n", f.ID, args[1])
		select {} // block
	case "once":
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return err
		}
		done := make(chan struct{})
		s.Once(d, func() { fire(); close(done) })
		fmt.Printf("scheduled %s once in %s\n", f.ID, d)
		<-done
		return nil
	default:
		return fmt.Errorf("usage: fort schedule cron|once ...")
	}
}

func loadFlowByID(id string) (graph.Flow, error) {
	flows, err := flow.LoadDir(flowsDir())
	if err != nil {
		return graph.Flow{}, err
	}
	for _, f := range flows {
		if f.ID == id {
			return f, nil
		}
	}
	return graph.Flow{}, fmt.Errorf("flow %q not found in %s", id, flowsDir())
}

func printFlowResult(runID string, res graph.Result) {
	switch res.State {
	case "paused":
		fmt.Printf("run %s PAUSED at gate %q — `fort gate approve %s %s`\n", runID, res.PausedNode, runID, res.PausedNode)
	case "completed":
		fmt.Printf("run %s COMPLETED\n", runID)
	case "failed":
		fmt.Printf("run %s FAILED\n", runID)
	default:
		fmt.Printf("run %s %s\n", runID, res.State)
	}
}
