package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/cloud/controlapi"
	coreruntime "github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/cloudworker"
)

type workerModeFile struct {
	Endpoint             string   `json:"endpoint"`
	AccountID            string   `json:"account_id"`
	WorkerID             string   `json:"worker_id"`
	MachineID            string   `json:"machine_id"`
	TokenEnv             string   `json:"token_env"`
	CapabilityRevisionID string   `json:"capability_revision_id"`
	CapabilityRevision   int      `json:"capability_revision"`
	ReadinessCommand     []string `json:"readiness_command"`
	PollInterval         string   `json:"poll_interval,omitempty"`
	HeartbeatInterval    string   `json:"heartbeat_interval,omitempty"`
}

type loadedWorkerModeConfig struct {
	file              workerModeFile
	token             string
	pollInterval      time.Duration
	heartbeatInterval time.Duration
}

func loadWorkerModeConfig(path string, getenv func(string) string) (loadedWorkerModeConfig, error) {
	if strings.TrimSpace(path) == "" || getenv == nil {
		return loadedWorkerModeConfig{}, errors.New("worker: --config is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return loadedWorkerModeConfig{}, fmt.Errorf("worker: open config: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 4<<20))
	decoder.DisallowUnknownFields()
	var config workerModeFile
	if err := decoder.Decode(&config); err != nil {
		return loadedWorkerModeConfig{}, fmt.Errorf("worker: decode config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return loadedWorkerModeConfig{}, errors.New("worker: config must contain one JSON object")
	}
	if config.Endpoint == "" || config.AccountID == "" || config.WorkerID == "" || config.MachineID == "" ||
		config.TokenEnv == "" || strings.ContainsAny(config.TokenEnv, "= \t\r\n") ||
		config.CapabilityRevisionID == "" || config.CapabilityRevision < 1 || len(config.ReadinessCommand) == 0 {
		return loadedWorkerModeConfig{}, errors.New("worker: config identity and readiness fields are required")
	}
	token := getenv(config.TokenEnv)
	if token == "" {
		return loadedWorkerModeConfig{}, fmt.Errorf("worker: machine token environment %s is empty", config.TokenEnv)
	}
	pollInterval, err := workerDuration(config.PollInterval, 5*time.Second, time.Second, time.Minute)
	if err != nil {
		return loadedWorkerModeConfig{}, fmt.Errorf("worker: poll interval: %w", err)
	}
	heartbeatInterval, err := workerDuration(config.HeartbeatInterval, controlDefaultHeartbeat(), 5*time.Second, time.Minute)
	if err != nil {
		return loadedWorkerModeConfig{}, fmt.Errorf("worker: heartbeat interval: %w", err)
	}
	return loadedWorkerModeConfig{file: config, token: token,
		pollInterval: pollInterval, heartbeatInterval: heartbeatInterval}, nil
}

func workerDuration(raw string, fallback, minimum, maximum time.Duration) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, errors.New("duration is outside its bounded range")
	}
	return value, nil
}

func controlDefaultHeartbeat() time.Duration { return 40 * time.Second }

func cmdWorker(args []string) error {
	flags := flag.NewFlagSet("worker", flag.ContinueOnError)
	configPath := flags.String("config", "", "path to enrolled worker JSON config")
	once := flags.Bool("once", false, "claim at most one target, then exit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: fort worker --config PATH [--once]")
	}
	config, err := loadWorkerModeConfig(*configPath, os.Getenv)
	if err != nil {
		return err
	}
	identity := cloudworker.Identity{AccountID: config.file.AccountID, WorkerID: config.file.WorkerID, MachineID: config.file.MachineID}
	client, err := cloudworker.NewHTTPClient(cloudworker.HTTPConfig{
		Endpoint: config.file.Endpoint, Identity: identity, Token: config.token,
	})
	if err != nil {
		return fmt.Errorf("worker: control transport: %w", err)
	}
	readiness, err := cloudworker.NewCommandReadiness(config.file.CapabilityRevisionID,
		config.file.CapabilityRevision, config.file.ReadinessCommand, nil)
	if err != nil {
		return fmt.Errorf("worker: readiness: %w", err)
	}
	runtime, adapters := workerModeExecution()
	worker := &cloudworker.Worker{
		Identity: identity, Control: client, Runtime: runtime, Readiness: readiness, Adapters: adapters,
		Clock: time.Now, IDs: randomWorkerIDs{}, HeartbeatInterval: config.heartbeatInterval,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	for {
		claimed, err := worker.RunOne(ctx)
		if err != nil {
			return fmt.Errorf("worker: %w", err)
		}
		if *once {
			return nil
		}
		if claimed {
			continue
		}
		timer := time.NewTimer(config.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

type randomWorkerIDs struct{}

func (randomWorkerIDs) New(kind string) string { return kind + ":" + uuid.NewString() }

// The shipped worker deliberately authorizes no real provider adapter. Its
// authenticated transport and durable lifecycle are operational, but a future
// provider can execute only after a separately approved exact adapter contract
// replaces both of these fail-closed components.
func workerModeExecution() (coreruntime.Runtime, cloudworker.AdapterRegistry) {
	return unauthorizedWorkerRuntime{}, unauthorizedWorkerAdapters{}
}

type unauthorizedWorkerAdapters struct{}

func (unauthorizedWorkerAdapters) Prepare(controlapi.WorkerAssignment, cloudworker.ExecutionContext) (coreruntime.RunSpec, error) {
	return coreruntime.RunSpec{}, cloudworker.ErrAdapterNotApproved
}

type unauthorizedWorkerRuntime struct{}

func (unauthorizedWorkerRuntime) Name() string { return "unauthorized" }
func (unauthorizedWorkerRuntime) Dispatch(context.Context, coreruntime.RunSpec) (coreruntime.Run, error) {
	return nil, cloudworker.ErrAdapterNotApproved
}
