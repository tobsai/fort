// Command fort is the fort-native CLI (backlog AO-018): route --dry-run, task
// add, runs list, run logs, gate, flow, schedule, and serve (the core daemon).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/core/flow"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/inbox"
	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/server"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/meshjoin"
	"github.com/tobsai/fort/exec/node"
	"github.com/tobsai/fort/exec/relay"
	"github.com/tobsai/fort/exec/relay/secure"
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
  fort task breakdown "<goal>"     plan a goal into backlog sub-tasks (needs fort serve)
  fort runs list                   list runs
  fort run logs <run-id>           tail a run's event stream
  fort gate list                   list paused gates (flows)
  fort gate approve <run> <node>   approve a paused gate
  fort gate reject  <run> <node>   reject a paused gate
  fort flow run <name> [--input k=v]   run a flow
  fort flow list                   list available flows
  fort mesh invite [--ttl 15m] [--advertise URL]   mint a join code (hub must be running)
  fort mesh join <hub-url> --code C [--name N] [--port 4087] [--agents a,b] [--advertise URL]
  fort mesh remove <name>          drop a machine from the mesh
  fort relay join <gateway-url> --code XXXX-XXXX [--name N]   tunnel this machine through a remote gateway (spec 028)
  fort relay status                print the joined gateway + key fingerprint
  fort relay remove                stop tunneling; delete relay.yaml (gateway revocation is authoritative)
  fort service install             install + start the launchd user agent
  fort service start|stop|restart  control the running daemon
  fort service status              report running/stopped + address
  fort service uninstall           stop + remove the launchd agent
  fort version

taskflags:
  --title S  --body S  --label L (repeatable)  --path P (repeatable)
  --repo S   --agent S (force @agent)  --size S  --machine S (pin a host)

multi-machine (spec 024): the easy path is ` + "`fort mesh invite`" + ` on the hub, then
  paste the printed ` + "`fort mesh join …`" + ` line on each new machine — Fort mints the
  shared token and manages machines.yaml for you. Manual alternative (spec 022):
  set FORT_MACHINES=machines.yaml, FORT_NODE_NAME, and FORT_NODE_TOKEN (shared)
  yourself. Either way, expose the API on the LAN with FORT_ADDR=0.0.0.0:4087.
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
	case "mesh":
		err = cmdMesh(os.Args[2:])
	case "relay":
		err = cmdRelay(os.Args[2:])
	case "service":
		err = cmdService(os.Args[2:])
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
	return task.Task{
		ID:        fmt.Sprintf("t-%d", time.Now().UnixNano()),
		Title:     title,
		Body:      tf.body,
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

	primaryMode, err := primaryChannelsMode(os.Getenv)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	agentMode, err := agentChannelsMode(os.Getenv)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	productMode := ui.ProductMode{
		PrimaryChannels: primaryMode,
		AgentChannels:   agentMode,
	}
	if err := validateAgentChannelsCutover(productMode); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	todayLocation, err := config.Load(os.Getenv).DisplayLocation()
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	a, err := buildApp()
	if err != nil {
		return err
	}
	defer a.store.Close()
	const interruptedRunReason = "interrupted when the Fort daemon stopped"
	if reconciled, err := a.store.FailInterruptedDirectRuns(interruptedRunReason); err != nil {
		return fmt.Errorf("reconcile interrupted runs: %w", err)
	} else if reconciled > 0 {
		slog.Warn("reconciled interrupted direct runs", "count", reconciled)
	}
	if reconciled, err := a.store.FailInterruptedConversationTargets(interruptedRunReason); err != nil {
		return fmt.Errorf("reconcile interrupted conversation targets: %w", err)
	} else if reconciled > 0 {
		slog.Warn("reconciled interrupted conversation targets", "count", reconciled)
	}

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
	graphExecutor := graph.NewExecutor(a.rt, a.store)
	defer graphExecutor.Wait()
	graphExecutor.UsePlacer(a.live)
	flowRunner := control.NewFlowExecutor(graphExecutor, flows)
	deps := wirePlaybooks(ui.Deps{
		Dispatcher: control.NewEngineDispatcher(a.engine),
		Store:      a.store,
		FlowIDs:    ids,
	}, a.store, &flowRunner)
	var conversationSeats control.ConversationSeatSource = control.SnapshotConversationSeats{}
	if a.caps != nil {
		conversationSeats = control.SnapshotConversationSeats{Source: a.caps.coordinator}
	} else if os.Getenv("FORT_FAKE") == "1" {
		conversationSeats = control.FakeConversationSeats(localName(a.live, a.cfg))
	}
	conversationService := control.NewConversationService(a.store, a.rt, conversationSeats, a.cfg.WorkRoot)
	defer conversationService.Close()
	durableSchedules := scheduler.NewDurableScheduler(a.store, func(scheduleCtx context.Context, definition scheduler.Definition, occurrence scheduler.Occurrence) error {
		var selected *graph.Flow
		for i := range flows {
			if flows[i].ID == definition.FlowID {
				selected = &flows[i]
				break
			}
		}
		if selected == nil {
			return fmt.Errorf("scheduled flow %q is unavailable", definition.FlowID)
		}
		result, err := graphExecutor.Start(scheduleCtx, *selected, occurrence.RunID, "")
		if err != nil {
			return err
		}
		if result.State == "failed" {
			return fmt.Errorf("scheduled flow %q failed", definition.FlowID)
		}
		return nil
	}, todayLocation)
	deps.Conversations = conversationService
	deps.Today = control.NewTodayService(a.store, conversationService)
	deps.TodayLocation = todayLocation
	deps.Schedules = control.NewScheduleService(durableSchedules, ids)
	var (
		flowDigests       map[string]string
		inventory         ui.ScheduleInventory
		inventoryErr      error
		acceptedInventory string
	)
	if primaryMode != ui.PrimaryChannelsOff {
		var digestErr error
		flowDigests, digestErr = control.FlowDefinitionDigests(flows)
		if digestErr != nil {
			return fmt.Errorf("Phase 1 flow inventory: %w", digestErr)
		}
		preflightScheduleRead := control.NewScheduleReadAdapter(control.NewScheduleReadService(
			a.store, control.SchedulerOwnershipInactive, flowDigests,
		))
		acceptedInventory = os.Getenv("FORT_ACCEPTED_SCHEDULE_INVENTORY")
		inventory, inventoryErr = preflightScheduleRead.Inventory(ctx, acceptedInventory)
	}
	var primaryCapabilities control.PrimaryOptionCapabilities
	if a.caps != nil {
		primaryCapabilities = a.caps.coordinator
	}
	channelProducts, err := wireChannelProducts(&deps, a.store, a.rt, primaryCapabilities, productMode,
		func(preview store.AgentChannelMigrationReport) error {
			slog.Info("Agent Channel migration preview", "report", preview)
			return nil
		})
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	defer channelProducts.Close()
	if channelProducts.Migration != nil {
		slog.Info("migrated Primary Channels into Agent Channels",
			"channels", len(channelProducts.Migration.Channels),
			"conversations", len(channelProducts.Migration.Conversations),
			"pins", len(channelProducts.Migration.Pins),
			"skipped", len(channelProducts.Migration.Skipped))
	}
	var primaryRecheck primaryAgentRecheck
	if channelProducts.Primary != nil {
		primaryRecheck = channelProducts.Primary.RecheckPrimaryAgent
	}
	if err := startDurableSchedulerAfterPrimaryPromotion(
		ctx, primaryMode, inventory, inventoryErr, primaryRecheck, durableSchedules.Start,
	); err != nil {
		return err
	}
	defer durableSchedules.Stop()
	if primaryMode != ui.PrimaryChannelsOff {
		scheduleRead := control.NewScheduleReadAdapter(control.NewScheduleReadService(
			a.store, control.SchedulerOwnershipActive, flowDigests,
		))
		deps.ScheduleRead = scheduleRead
		deps.ScheduleInventory = scheduleRead
		deps.AcceptedScheduleInventory = acceptedInventory
	}
	// Task breakdown (spec 026): a planner agent decomposes a goal into backlog
	// sub-tasks. FORT_PLANNER selects the agent (default claude). Only wired in
	// serve — breakdown is a real agent run, so it 409s in control-only mode.
	planner := os.Getenv("FORT_PLANNER")
	if planner == "" {
		planner = "claude"
	}
	deps.Planner = control.NewPlanner(a.engine, a.store, planner)
	// Multi-machine (spec 022/024): expose the peer roster over the shared live
	// registry + poll reachability. Always wired — an empty registry reports an
	// empty roster, and hot joins/removes are reflected without a restart.
	roster := control.NewRoster(a.live)
	go roster.Poll(ctx, 10*time.Second)
	deps.Machines = roster
	if a.caps != nil {
		deps.Capabilities = a.caps.coordinator
		deps.SeatRechecker = a.caps.coordinator
		a.caps.start(ctx, time.Minute)
	}
	uiSrv := ui.New(deps)

	// Mesh enrollment (spec 024): the token store holds the durable mesh token
	// (minted on first `mesh invite`, persisted to node.yaml) and feeds both the
	// node exec endpoint and the join server's outbound transports.
	tokens := a.tokens
	_, port, err := net.SplitHostPort(a.cfg.Addr)
	if err != nil {
		return fmt.Errorf("serve: invalid bind address FORT_ADDR %q: %w", a.cfg.Addr, err)
	}
	meshSrv := &meshjoin.Server{
		Live:         a.live,
		RegistryPath: managedRegistryPath(a.cfg),
		Managed:      a.cfg.MachinesManaged || a.cfg.MachinesPath == "",
		Cluster:      a.clus,
		Store:        a.store,
		Tokens:       tokens,
		NodeName:     a.cfg.NodeName,
		Port:         port,
		ProbeAgents:  probeAgents,
		Now:          time.Now,
		Log:          slog.Default(),
	}

	// Node exec endpoint: let peer Forts dispatch runs to this machine when a
	// shared token is set. It serves the raw local runtime (never re-routes).
	// Always mounted; the token is read fresh per request via tokens.Get, so a
	// `mesh invite` minted after startup takes effect without a restart. An empty
	// token still 403s every request (same "disabled" behavior as before).
	nodeSrv := node.New(a.localRT, tokens.Get)
	if a.caps != nil {
		nodeSrv.UseCapabilities(a.caps.local)
	}
	mountMode := func(mux *http.ServeMux, mode ui.ProductMode) {
		_ = uiSrv.RegisterProductMode(mux, mode)
		nodeSrv.Register(mux)
		meshSrv.Register(mux)
	}
	mount := func(mux *http.ServeMux) { mountMode(mux, productMode) }

	// Remote gateway (spec 028): when this machine has joined a gateway,
	// maintain the outbound tunnel with only the native Phase 1 API contract.
	// The transport moves sealed bytes and never imports ui; cmd/fort remains the
	// composition root that selects the closed route set.
	if rc, err := config.LoadRelay(a.cfg.DataDir()); err == nil {
		rmux := http.NewServeMux()
		if err := registerNativeProductRoutes(rmux, uiSrv, productMode); err != nil {
			return fmt.Errorf("serve: register native relay routes: %w", err)
		}
		relayHandler := server.ObserveRequests(rmux, func(event server.RequestEvent) {
			slog.Info("fort relay request", "request_id", event.ID, "method", event.Method,
				"path", event.Path, "status", event.Status, "duration", event.Duration)
		})
		tr := relay.New(relayHandler, relay.Config{
			URL:   rc.GatewayURL + "/tunnel",
			Token: rc.DeviceToken,
			Key:   secure.Keypair{Private: rc.PrivateKey, Public: rc.PublicKey},
			OnConnectionEvent: func(event relay.ConnectionEvent) {
				attributes := []any{"state", event.State}
				if event.Err != nil {
					attributes = append(attributes, "error", event.Err)
				}
				if event.RetryIn > 0 {
					attributes = append(attributes, "retry_in", event.RetryIn)
				}
				slog.Info("fort relay connection", attributes...)
			},
		})
		go func() { _ = tr.Run(ctx) }()
		fmt.Printf("fort relay: tunnel to %s (machine %s, fingerprint %s)\n",
			rc.GatewayURL, rc.MachineID, secure.FingerprintOf(rc.PublicKey))
	}

	srv := server.New(server.Deps{Config: a.cfg, Engine: a.engine, Store: a.store, Mount: mount})
	fmt.Printf("fort-core on http://%s  (runtime=%s · node=%s)\n", a.cfg.Addr, a.rt.Name(), a.cfg.NodeName)
	fmt.Printf("fort-ui   on http://%s/  (%s)\n", a.cfg.Addr, productUISurfaceLabel(productMode))
	if reg := a.live.Load(); reg != nil {
		exec := "off"
		if a.cfg.NodeToken != "" {
			exec = "on"
		}
		fmt.Printf("fort mesh : %d machines (%s) · accept remote exec: %s\n", len(reg.Machines), a.cfg.MachinesPath, exec)
		peers := 0
		for _, m := range reg.Machines {
			if m.Name != reg.Local() {
				peers++
			}
		}
		if peers > 0 && a.cfg.NodeToken == "" {
			fmt.Println("  warning: FORT_NODE_TOKEN is empty — outbound dispatch to peer machines will fail auth")
		}
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
	if len(args) > 0 && args[0] == "breakdown" {
		return cmdTaskBreakdown(args[1:])
	}
	if len(args) == 0 || args[0] != "add" {
		return fmt.Errorf("usage: fort task <add|breakdown> ...")
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Restore default handling immediately after the first signal so a second
	// Ctrl-C can force-exit if a provider refuses its cancellation contract.
	go func() {
		<-ctx.Done()
		stop()
	}()
	runID, err := a.engine.Submit(ctx, t)
	if err != nil {
		return err
	}
	return streamRunCancelable(ctx, a, runID)
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
