package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/tobsai/fort/core/config"
)

// cmdTaskBreakdown is a thin loopback client of the running daemon: it posts the
// goal to POST /api/breakdown and prints the visible planner run id. The
// sub-tasks appear in the backlog when that run completes (spec 026). Like `fort
// mesh invite`, it needs `fort serve` running on this machine.
func cmdTaskBreakdown(args []string) error {
	goal := strings.TrimSpace(strings.Join(args, " "))
	if goal == "" {
		return fmt.Errorf(`usage: fort task breakdown "<goal>"`)
	}
	cfg := config.Load(os.Getenv)
	url, err := loopbackURL(cfg, "/api/breakdown")
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{"text": goal})
	resp, err := meshHTTP.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) {
			return errors.New("fort serve is not running on this machine — start it first (breakdown runs in the daemon)")
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("breakdown failed: %s", strings.TrimSpace(string(raw)))
	}
	var res struct {
		RunID string `json:"run_id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	fmt.Printf("planner run %s started — sub-tasks will appear in the backlog\n", res.RunID)
	return nil
}
