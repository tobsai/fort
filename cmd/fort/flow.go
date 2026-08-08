package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/config"
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
			if err := ex.Reject(runID, nodeID, ""); err != nil {
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
	cfg := config.Load(os.Getenv)
	displayLocation, err := cfg.DisplayLocation()
	if err != nil {
		return fmt.Errorf("schedule: %w", err)
	}
	definition := scheduler.Definition{ID: uuid.NewString(), FlowID: args[2], Timezone: displayLocation.String(), Enabled: true}
	switch args[0] {
	case "cron":
		definition.Kind, definition.Expression = scheduler.KindCron, args[1]
	case "once":
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return err
		}
		definition.Kind = scheduler.KindOnce
		definition.Expression = time.Now().Add(d).Format(time.RFC3339)
	default:
		return fmt.Errorf("usage: fort schedule cron|once ...")
	}
	body, err := json.Marshal(struct {
		ID         string         `json:"id"`
		Kind       scheduler.Kind `json:"kind"`
		Expression string         `json:"expression"`
		FlowID     string         `json:"flow_id"`
		Timezone   string         `json:"timezone"`
	}{definition.ID, definition.Kind, definition.Expression, definition.FlowID, definition.Timezone})
	if err != nil {
		return err
	}
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return fmt.Errorf("invalid Fort address %q: %w", cfg.Addr, err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	endpoint := "http://" + net.JoinHostPort(host, port) + "/api/schedules"
	response, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("contact running Fort daemon at %s (start Fort with `fort serve` first): %w", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		var message bytes.Buffer
		_, _ = message.ReadFrom(response.Body)
		return fmt.Errorf("Fort daemon rejected schedule: %s", strings.TrimSpace(message.String()))
	}
	fmt.Printf("scheduled %s (%s) in the running Fort daemon\n", definition.FlowID, definition.ID)
	return nil
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
