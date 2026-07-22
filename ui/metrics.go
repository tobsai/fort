package ui

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tobsai/fort/core/store"
)

// Metrics (spec 033): per-agent scorecards derived entirely from run rows and
// the append-only event log — no new state. Assignments are routed runs plus
// flow task-node executions (attributed via `started` events); sign-off numbers
// come from `gate` decision events; cost comes from engine `result` lines that
// happen to be parseable (claude's stream-json today). Anything unknown is
// reported as absent, never estimated.

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 365 {
			days = n
		}
	}
	lane := r.URL.Query().Get("lane")
	runs, err := s.d.Store.ListRuns()
	if err != nil {
		httpError(w, err)
		return
	}
	resp := computeMetrics(assignmentRuns(runs), s.d.Store.Events, time.Now().UTC(), days, lane)
	writeJSON(w, http.StatusOK, resp)
}

type laneStat struct{ total, ok int }

type agentAgg struct {
	assignments int
	decided     int
	firstPass   int
	accepted    int
	redirects   int
	cost        float64
	costKnown   bool
	decisions   []gateOutcome // one per decided sign-off, for trend + spark
	lanes       map[string]*laneStat
}

type gateOutcome struct {
	firstPass bool
	at        time.Time
}

func computeMetrics(runs []store.Run, events func(string) ([]store.Event, error), now time.Time, days int, lane string) MetricsResponse {
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	aggs := map[string]*agentAgg{}
	get := func(agent string) *agentAgg {
		a, ok := aggs[agent]
		if !ok {
			a = &agentAgg{lanes: map[string]*laneStat{}}
			aggs[agent] = a
		}
		return a
	}
	laneSet := map[string]bool{}

	for _, r := range runs {
		if r.CreatedAt.Before(cutoff) {
			continue
		}
		if !strings.HasPrefix(r.Agent, "flow:") {
			if r.Agent == "" || r.Agent == "unassigned" {
				continue
			}
			if r.MatchedRule != "" {
				laneSet[r.MatchedRule] = true
			}
			if lane != "" && r.MatchedRule != lane {
				continue
			}
			a := get(r.Agent)
			a.assignments++
			if r.MatchedRule != "" && isTerminalStatus(r.Status) {
				ls, ok := a.lanes[r.MatchedRule]
				if !ok {
					ls = &laneStat{}
					a.lanes[r.MatchedRule] = ls
				}
				ls.total++
				if r.Status == "succeeded" {
					ls.ok++
				}
			}
			if evs, err := events(r.ID); err == nil {
				for _, e := range evs {
					if c, ok := parseCost(e); ok {
						a.cost += c
						a.costKnown = true
					}
				}
			}
			continue
		}

		// Flow run: attribute node executions, sign-offs, and cost to the agents
		// that produced them, via the event log. Flow work carries no routing
		// lane, so a lane filter excludes it.
		if lane != "" {
			continue
		}
		evs, err := events(r.ID)
		if err != nil {
			continue
		}
		lastStarted := ""
		nodeAgent := map[string]string{} // task node -> agent that ran it
		type gateState struct {
			agent     string
			decisions int
			firstPass bool
			approved  bool
			last      time.Time
		}
		gates := map[string]*gateState{}
		for _, e := range evs {
			switch e.Type {
			case "started":
				if e.Data != "" {
					lastStarted = e.Data
					if e.NodeID != "" {
						if _, seen := nodeAgent[e.NodeID]; !seen {
							nodeAgent[e.NodeID] = e.Data
							get(e.Data).assignments++
						}
					}
				}
			case "gate":
				var d struct{ Decision, Note string }
				if json.Unmarshal([]byte(e.Data), &d) != nil || lastStarted == "" {
					continue
				}
				g, ok := gates[e.NodeID]
				if !ok {
					g = &gateState{agent: lastStarted}
					gates[e.NodeID] = g
				}
				g.decisions++
				if g.decisions == 1 {
					g.firstPass = d.Decision == "approved" && d.Note == ""
				}
				g.approved = d.Decision == "approved"
				g.last = e.CreatedAt
				if d.Decision == "rejected" || (d.Decision == "approved" && d.Note != "") {
					get(g.agent).redirects++
				}
			case "stdout":
				if c, ok := parseCost(e); ok {
					agent := nodeAgent[e.NodeID]
					if agent == "" {
						agent = lastStarted
					}
					if agent != "" {
						a := get(agent)
						a.cost += c
						a.costKnown = true
					}
				}
			}
		}
		for _, g := range gates {
			a := get(g.agent)
			a.decided++
			if g.firstPass {
				a.firstPass++
			}
			if g.approved {
				a.accepted++
			}
			a.decisions = append(a.decisions, gateOutcome{firstPass: g.firstPass, at: g.last})
		}
	}

	resp := MetricsResponse{WindowDays: days, Agents: []AgentMetrics{}, Lanes: []string{}}
	for l := range laneSet {
		resp.Lanes = append(resp.Lanes, l)
	}
	sort.Strings(resp.Lanes)
	for agent, a := range aggs {
		m := AgentMetrics{
			Agent: agent, Assignments: a.assignments, Decided: a.decided,
			FirstPass: a.firstPass, Accepted: a.accepted, Redirects: a.redirects,
			CostUSD: a.cost, CostKnown: a.costKnown,
			Spark: spark(a.decisions, cutoff, now), Best: []string{}, Weak: []string{},
		}
		if a.decided > 0 {
			m.FirstPassPct = 100 * float64(a.firstPass) / float64(a.decided)
		}
		if a.assignments > 0 {
			m.RedirectsPer = float64(a.redirects) / float64(a.assignments)
		}
		if a.costKnown && a.accepted > 0 {
			m.CostPerAccept = a.cost / float64(a.accepted)
		}
		m.Trend, m.TrendDelta = trend(a.decisions, cutoff, now)
		m.Best, m.Weak = laneChips(a.lanes)
		resp.Agents = append(resp.Agents, m)
		resp.Assignments += a.assignments
	}
	sort.Slice(resp.Agents, func(i, j int) bool {
		if resp.Agents[i].Assignments != resp.Agents[j].Assignments {
			return resp.Agents[i].Assignments > resp.Agents[j].Assignments
		}
		return resp.Agents[i].Agent < resp.Agents[j].Agent
	})
	return resp
}

// parseCost extracts an engine-reported cost from a raw stdout event: claude's
// terminal stream-json `result` line carries total_cost_usd. Anything that
// doesn't parse is simply not cost data.
func parseCost(e store.Event) (float64, bool) {
	if e.Type != "stdout" || !strings.Contains(e.Data, "total_cost_usd") {
		return 0, false
	}
	var d struct {
		Type string  `json:"type"`
		Cost float64 `json:"total_cost_usd"`
	}
	if json.Unmarshal([]byte(e.Data), &d) != nil || d.Type != "result" || d.Cost <= 0 {
		return 0, false
	}
	return d.Cost, true
}

// trend compares first-pass acceptance between the two halves of the window;
// under 3 decisions per half it stays "steady" — the sample is too small to
// call a direction.
func trend(ds []gateOutcome, cutoff, now time.Time) (string, float64) {
	mid := cutoff.Add(now.Sub(cutoff) / 2)
	var n1, fp1, n2, fp2 int
	for _, d := range ds {
		if d.at.Before(mid) {
			n1++
			if d.firstPass {
				fp1++
			}
		} else {
			n2++
			if d.firstPass {
				fp2++
			}
		}
	}
	if n1 < 3 || n2 < 3 {
		return "steady", 0
	}
	delta := 100*float64(fp2)/float64(n2) - 100*float64(fp1)/float64(n1)
	switch {
	case delta >= 5:
		return "improving", delta
	case delta <= -5:
		return "slipping", delta
	}
	return "steady", delta
}

// spark buckets first-pass % into 7 equal slices of the window, carrying the
// previous value across empty buckets.
func spark(ds []gateOutcome, cutoff, now time.Time) []float64 {
	const buckets = 7
	span := now.Sub(cutoff)
	n := make([]int, buckets)
	fp := make([]int, buckets)
	for _, d := range ds {
		i := int(float64(d.at.Sub(cutoff)) / float64(span) * buckets)
		if i < 0 {
			i = 0
		}
		if i >= buckets {
			i = buckets - 1
		}
		n[i]++
		if d.firstPass {
			fp[i]++
		}
	}
	out := make([]float64, buckets)
	last := 0.0
	for i := 0; i < buckets; i++ {
		if n[i] > 0 {
			last = 100 * float64(fp[i]) / float64(n[i])
		}
		out[i] = last
	}
	return out
}

// laneChips picks the strongest and weakest routing lanes with a meaningful
// sample (≥3 terminal runs).
func laneChips(lanes map[string]*laneStat) (best, weak []string) {
	best, weak = []string{}, []string{}
	type scored struct {
		lane  string
		score float64
	}
	var ss []scored
	for l, st := range lanes {
		if st.total >= 3 {
			ss = append(ss, scored{humanizeLane(l), float64(st.ok) / float64(st.total)})
		}
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].score > ss[j].score })
	for _, s := range ss {
		if s.score >= 0.7 && len(best) < 2 {
			best = append(best, s.lane)
		}
	}
	if len(ss) > 0 {
		if worst := ss[len(ss)-1]; worst.score < 0.6 {
			weak = append(weak, worst.lane)
		}
	}
	return best, weak
}

func humanizeLane(l string) string {
	return strings.NewReplacer("-", " ", "_", " ").Replace(l)
}

func isTerminalStatus(s string) bool {
	return s == "succeeded" || s == "failed" || s == "canceled"
}
