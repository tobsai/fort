// Command fort is the fort-native CLI (backlog AO-018): route --dry-run, task
// add, runs list, run logs, gate, flow, schedule, and serve (the core daemon).
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/flow"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/inbox"
	"github.com/tobsai/fort/core/server"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/node"
	"github.com/tobsai/fort/ui"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `fort — deterministic agent orchestration

usage:
  fort serve [--inbox DIR]         boot the full plane (control + execution)
  fort control [--inbox DIR]       boot the CONTROL PLANE ONLY (board, chat,
                                   scheduler, gate inbox) — no router/runtime/DAG,
                                   no agent CLIs needed; tasks are boarded as queued
  fort route --dry-run [taskflags] print the matched rule + target agent
  fort task add [taskflags]        route + dispatch a task natively
  fort runs list                   list runs
  fort run logs <run-id>           tail a run's event stream
  fort gate list                   list paused gates (flows)
  fort gate approve <run> <node>   approve a paused gate
  fort gate reject  <run> <node>   reject a paused gate
  fort flow run <name> [--input k=v]   run a flow
  fort flow list                   list available flows
  fort version

taskflags:
  --title S  --body S  --label L (repeatable)  --path P (repeatable)
  --repo S   --agent S (force @agent)  --size S  --machine S (pin a host)

multi-machine (spec 022): set FORT_MACHINES=machines.yaml to route across hosts,
  FORT_NODE_NAME to identify this host, and FORT_NODE_TOKEN (shared) to accept
  remote exec. Expose the API on the LAN with FORT_ADDR=0.0.0.0:4087.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = cmdServe(os.Args[2:])
	case "control":
		err = cmdControl(os.Args[2:])
	case "route":
		err = cmdRoute(os.Args[2:])
	case "task":
		err = cmdTask(os.Args[2:])
	case "runs":
		err = cmdRuns(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "gate":
		err = cmdGate(os.Args[2:])
	case "flow":
		err = cmdFlow(os.Args[2:])
	case "schedule":
		err = cmdSchedule(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("fort %s (fort-native)\n", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// --- task flags ---

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type taskFlags struct {
	title, body, repo, agent, size, machine string
	labels, paths                           stringSlice
}

func addTaskFlags(fs *flag.FlagSet) *taskFlags {
	tf := &taskFlags{}
	fs.StringVar(&tf.title, "title", "", "task title")
	fs.StringVar(&tf.body, "body", "", "task body (the prompt)")
	fs.StringVar(&tf.repo, "repo", "", "repo signal")
	fs.StringVar(&tf.agent, "agent", "", "force @agent")
	fs.StringVar(&tf.size, "size", "", "size: S|M|L|XL")
	fs.StringVar(&tf.machine, "machine", "", "pin a target host (spec 022)")
	fs.Var(&tf.labels, "label", "label (repeatable)")
	fs.Var(&tf.paths, "path", "touched path (repeatable)")
	return tf
}

func (tf *taskFlags) toTask(args []string) task.Task {
	title := tf.title
	if title == "" && len(args) > 0 {
		title = strings.Join(args, " ")
	}
	body := tf.body
	if body == "" {
		body = title
	}
	return task.Task{
		ID:        fmt.Sprintf("t-%d", time.Now().UnixNano()),
		Title:     title,
		Body:      body,
		Labels:    tf.labels,
		Paths:     tf.paths,
		Repo:      tf.repo,
		Agent:     tf.agent,
		Size:      tf.size,
		Machine:   tf.machine,
		CreatedAt: time.Now(),
	}
}

// --- serve ---

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	inboxDir := fs.String("inbox", ".fort-native/inbox", "task inbox directory to watch")
	_ = fs.Parse(args)

	a, err := buildApp()
	if err != nil {
		return err
	}
	defer a.store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	in := inbox.NewDir(*inboxDir, a.engine)
	go func() {
		fmt.Printf("fort: watching inbox %s\n", *inboxDir)
		_ = in.Watch(ctx, time.Second)
	}()

	// Mount the fort-ui module with a full execution plane wired in.
	flows, err := flow.LoadDir(flowsDir())
	if err != nil {
		return err
	}
	ids := make([]string, len(flows))
	for i, f := range flows {
		ids[i] = f.ID
	}
	deps := ui.Deps{
		Dispatcher: control.NewEngineDispatcher(a.engine),
		Runner:     control.NewFlowExecutor(graph.NewExecutor(a.rt, a.store), flows),
		Store:      a.store,
		FlowIDs:    ids,
	}
	// Multi-machine (spec 022): expose the peer roster + poll reachability.
	if a.reg != nil {
		roster := control.NewRoster(a.reg)
		go roster.Poll(ctx, 10*time.Second)
		deps.Machines = roster
	}
	uiSrv := ui.New(deps)

	// Node exec endpoint: let peer Forts dispatch runs to this machine when a
	// shared token is set. It serves the raw local runtime (never re-routes).
	mount := uiSrv.Register
	if a.cfg.NodeToken != "" {
		nodeSrv := node.New(a.localRT, a.cfg.NodeToken)
		mount = func(mux *http.ServeMux) {
			uiSrv.Register(mux)
			nodeSrv.Register(mux)
		}
	}

	srv := server.New(server.Deps{Config: a.cfg, Engine: a.engine, Store: a.store, Mount: mount})
	fmt.Printf("fort-core on http://%s  (runtime=%s · node=%s)\n", a.cfg.Addr, a.rt.Name(), a.cfg.NodeName)
	fmt.Printf("fort-ui   on http://%s/  (board · feed · gates · chat · execution)\n", a.cfg.Addr)
	if a.reg != nil {
		exec := "off"
		if a.cfg.NodeToken != "" {
			exec = "on"
		}
		fmt.Printf("fort mesh : %d machines (%s) · accept remote exec: %s\n", len(a.reg.Machines), a.cfg.MachinesPath, exec)
	}
	return srv.Run(ctx)
}

// --- route --dry-run ---

func cmdRoute(args []string) error {
	fs := flag.NewFlagSet("route", flag.ExitOnError)
	dry := fs.Bool("dry-run", false, "print the route without dispatching")
	tf := addTaskFlags(fs)
	_ = fs.Parse(args)

	a, err := buildApp()
	if err != nil {
		return err
	}
	defer a.store.Close()

	t := tf.toTask(fs.Args())
	d := a.router.Route(t)
	rule := d.MatchedRule
	if d.Default {
		rule = "(default)"
	}
	fmt.Printf("task   : %s\n", t.Title)
	fmt.Printf("labels : %v\n", []string(tf.labels))
	fmt.Printf("rule   : %s\n", rule)
	fmt.Printf("route  : %s\n", d.Route)
	if !*dry {
		fmt.Println("(use `fort task add` to dispatch)")
	}
	return nil
}

// --- task add ---

func cmdTask(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return fmt.Errorf("usage: fort task add [taskflags]")
	}
	fs := flag.NewFlagSet("task add", flag.ExitOnError)
	tf := addTaskFlags(fs)
	_ = fs.Parse(args[1:])

	a, err := buildApp()
	if err != nil {
		return err
	}
	defer a.store.Close()

	t := tf.toTask(fs.Args())
	d := a.router.Route(t)
	fmt.Printf("routing %q -> %s (%s)\n", t.Title, d.Route, ruleLabel(d))

	runID, err := a.engine.Submit(context.Background(), t)
	if err != nil {
		return err
	}
	return streamRun(a, runID)
}

// --- runs list / run logs ---

func cmdRuns(args []string) error {
	if len(args) == 0 || args[0] != "list" {
		return fmt.Errorf("usage: fort runs list")
	}
	a, err := buildApp()
	if err != nil {
		return err
	}
	defer a.store.Close()

	runs, err := a.store.ListRuns()
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		fmt.Println("(no runs yet)")
		return nil
	}
	fmt.Printf("%-36s  %-8s  %-10s  %s\n", "RUN", "AGENT", "STATUS", "TITLE")
	for _, r := range runs {
		fmt.Printf("%-36s  %-8s  %-10s  %s\n", r.ID, r.Agent, r.Status, r.Title)
	}
	return nil
}

func cmdRun(args []string) error {
	if len(args) < 2 || args[0] != "logs" {
		return fmt.Errorf("usage: fort run logs <run-id>")
	}
	a, err := buildApp()
	if err != nil {
		return err
	}
	defer a.store.Close()
	return tailRun(a, args[1])
}
