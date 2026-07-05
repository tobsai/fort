// Package meshjoin hosts the spec-024 enrollment endpoints on the hub daemon:
//
//	POST   /api/mesh/join              worker enrollment (code-authenticated)
//	POST   /api/mesh/invite            admin: mint invite   (loopback only)
//	DELETE /api/mesh/machines/{name}   admin: remove peer   (loopback only)
//
// Enrollment is plain CRUD — zero model calls. The package touches execution
// only through exec/cluster (hot transport add/remove) and exec/remote (the
// transport it installs); it never imports a provider runtime.
package meshjoin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tobsai/fort/core/machines"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/cluster"
	"github.com/tobsai/fort/exec/remote"
)

// Server wires the enrollment handlers to the daemon's live state. All fields
// are set by the composition root (cmd/fort) before Register.
type Server struct {
	Live         *machines.Live   // shared registry pointer (placer/cluster/roster)
	RegistryPath string           // managed machines.yaml path
	Managed      bool             // false ⇒ FORT_MACHINES set ⇒ refuse writes
	Cluster      *cluster.Runtime // hot Add/Remove of peer transports
	Store        *store.Store     // invites + events
	Tokens       *TokenStore
	NodeName     string                         // hub identity
	Port         string                         // hub bind port (for detectHubURL)
	ProbeAgents  func() []string                // $PATH provider probe, injected
	Resolve      func(string) ([]net.IP, error) // DNS resolver for advertise hostnames; nil ⇒ net.LookupIP
	Now          func() time.Time               // injectable clock
	Log          *slog.Logger

	// mu serializes the entire registry read-modify-write AND, on the join
	// path, the invite check-and-consume. Live's atomic pointer makes reads
	// race-free, but the write sequence must be atomic on two counts: (1) two
	// concurrent joins building on the same base registry would lose one
	// entry, and (2) CheckInvite→consume must be one critical section so a
	// second racer holding the same code cannot slip a token-bearing transport
	// in before the code is marked used.
	mu sync.Mutex
}

// resolve returns the injected resolver, defaulting to net.LookupIP.
func (s *Server) resolve() func(string) ([]net.IP, error) {
	if s.Resolve != nil {
		return s.Resolve
	}
	return net.LookupIP
}

// Register mounts the enrollment routes onto mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/mesh/join", s.handleJoin)
	mux.HandleFunc("POST /api/mesh/invite", s.loopbackOnly(s.handleInvite))
	mux.HandleFunc("DELETE /api/mesh/machines/{name}", s.loopbackOnly(s.handleRemove))
}

// loopbackOnly gates the admin endpoints: local shell access on the hub is the
// admin credential (spec 024 D7), checked against the connection's remote
// address — 127.0.0.0/8 or ::1 only.
func (s *Server) loopbackOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			http.Error(w, "meshjoin: admin endpoint accepts loopback connections only (run this on the hub)", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// refuseUnmanaged writes the 409 for registries owned by the operator. All
// three handlers write machines.yaml, so all three refuse when FORT_MACHINES
// is explicitly set (spec 024: enrollment never touches an operator-managed
// file).
func (s *Server) refuseUnmanaged(w http.ResponseWriter) bool {
	if s.Managed {
		return false
	}
	http.Error(w, "meshjoin: the machine registry is operator-managed via FORT_MACHINES; unset FORT_MACHINES to let fort mesh manage it", http.StatusConflict)
	return true
}

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ" // base32, no I L O U

// mintCode returns a single-use invite code: 5 random bytes (40 bits) as 8
// Crockford-base32 chars, displayed XXXX-XXXX.
func mintCode() (display string, hash string, err error) {
	var buf [5]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", "", err
	}
	v := uint64(buf[0])<<32 | uint64(buf[1])<<24 | uint64(buf[2])<<16 | uint64(buf[3])<<8 | uint64(buf[4])
	out := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		out[i] = crockford[v&31]
		v >>= 5
	}
	code := string(out)
	return code[:4] + "-" + code[4:], hashCode(code), nil
}

// hashCode normalizes a code (uppercase, hyphens stripped) and returns its
// SHA-256 hex — the only form ever stored or compared.
func hashCode(code string) string {
	norm := strings.ToUpper(strings.ReplaceAll(code, "-", ""))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// registryMachine looks name up in reg, tolerating a nil registry.
func registryMachine(reg *machines.Registry, name string) (machines.Machine, bool) {
	if reg == nil {
		return machines.Machine{}, false
	}
	return reg.Machine(name)
}

// saveRegistry writes reg atomically; on failure it logs and writes the 500,
// returning false so the caller returns. Callers hold s.mu.
func (s *Server) saveRegistry(w http.ResponseWriter, reg *machines.Registry) bool {
	if err := machines.Save(s.RegistryPath, reg); err != nil {
		s.Log.Error("meshjoin: registry write failed", "path", s.RegistryPath, "err", err)
		http.Error(w, "meshjoin: failed to write the machine registry", http.StatusInternalServerError)
		return false
	}
	return true
}

// appendMachineEvent appends a run-less machine_joined/machine_removed event
// (its payload is the registry entry). A failed append is logged, not fatal —
// the registry change already committed.
func (s *Server) appendMachineEvent(typ string, m machines.Machine) {
	data, _ := json.Marshal(m)
	if _, err := s.Store.AppendEvent(store.Event{RunID: "", Type: typ, Data: string(data)}); err != nil {
		s.Log.Error("meshjoin: event append failed", "type", typ, "err", err)
	}
}

// --- invite ---

type inviteReq struct {
	TTL       string `json:"ttl"`
	Advertise string `json:"advertise"`
}

type inviteResp struct {
	Code    string `json:"code"`
	HubURL  string `json:"hub_url"`
	JoinCmd string `json:"join_cmd"`
	Minted  bool   `json:"minted"`
}

func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	var req inviteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "meshjoin: bad request body", http.StatusBadRequest)
		return
	}

	// Refuse an operator-managed registry before doing anything else (mint,
	// write): enrollment never touches a FORT_MACHINES-owned file.
	if s.refuseUnmanaged(w) {
		return
	}

	ttl := 15 * time.Minute
	if req.TTL != "" {
		d, err := time.ParseDuration(req.TTL)
		if err != nil || d <= 0 || d > time.Hour {
			http.Error(w, fmt.Sprintf("meshjoin: invalid ttl %q: must be a positive duration capped at 1h (default 15m)", req.TTL), http.StatusBadRequest)
			return
		}
		ttl = d
	}

	_, minted, err := s.Tokens.Ensure()
	if err != nil {
		s.Log.Error("meshjoin: token mint failed", "err", err)
		http.Error(w, "meshjoin: failed to mint the mesh token", http.StatusInternalServerError)
		return
	}

	hubURL := req.Advertise
	if hubURL == "" {
		hubURL, err = detectHubURL(s.Port, net.InterfaceAddrs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Hub self-entry (spec 024): register the hub in the managed registry if
	// absent. A hub that runs no agents claims no entry — the registry
	// validator requires ≥1 agent per machine — so an agent-less hub defers
	// registry creation to the first join (the worker becomes the sole, and
	// therefore first-placed, entry until the hub offers agents).
	s.mu.Lock()
	if _, ok := registryMachine(s.Live.Load(), s.NodeName); !ok {
		if agents := s.ProbeAgents(); len(agents) > 0 {
			base := s.Live.Load()
			if base == nil {
				base = &machines.Registry{Version: 1}
			}
			reg := base.WithMachine(machines.Machine{Name: s.NodeName, URL: hubURL, Agents: agents})
			if !s.saveRegistry(w, reg) {
				s.mu.Unlock()
				return
			}
			s.Live.Store(reg)
		}
	}
	s.mu.Unlock()

	code, hash, err := mintCode()
	if err != nil {
		http.Error(w, "meshjoin: failed to mint an invite code", http.StatusInternalServerError)
		return
	}
	if err := s.Store.CreateInvite(hash, s.Now().Add(ttl)); err != nil {
		s.Log.Error("meshjoin: invite store failed", "err", err)
		http.Error(w, "meshjoin: failed to store the invite", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, inviteResp{
		Code:    code,
		HubURL:  hubURL,
		JoinCmd: "fort mesh join " + hubURL + " --code " + code,
		Minted:  minted,
	})
}

// --- join ---

type joinReq struct {
	Code         string   `json:"code"`
	Port         int      `json:"port"`
	Name         string   `json:"name"`
	Agents       []string `json:"agents"`
	AdvertiseURL string   `json:"advertise_url"`
}

type joinResp struct {
	Token string `json:"token"`
	Name  string `json:"name"`
	URL   string `json:"url"`
}

// handleJoin admits a worker into the mesh. Every response below 200 must
// never contain the mesh token. Input is validated (pure) up front; then the
// invite check-and-consume plus all token-bearing side effects run in ONE
// critical section under s.mu, so a second racer holding the same code blocks,
// re-checks, and 401s having installed nothing — no TOCTOU token exfiltration.
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req joinReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "meshjoin: bad request body", http.StatusBadRequest)
		return
	}

	if s.refuseUnmanaged(w) {
		return
	}

	// --- pure input validation (no state touched, no lock held) ---
	if req.Name == "" {
		http.Error(w, "meshjoin: name is required", http.StatusBadRequest)
		return
	}
	if strings.EqualFold(req.Name, s.NodeName) {
		http.Error(w, fmt.Sprintf("meshjoin: %q is this hub — a machine cannot join itself", req.Name), http.StatusBadRequest)
		return
	}
	if len(req.Agents) == 0 {
		http.Error(w, "meshjoin: agents list is empty", http.StatusBadRequest)
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		http.Error(w, fmt.Sprintf("meshjoin: port %d out of range", req.Port), http.StatusBadRequest)
		return
	}

	// Worker URL: an explicit advertise_url wins, else the observed source IP
	// plus the advertised port (spec 024 D2). Either way it must name a
	// private/tailnet address — the cleartext token stays on the trusted
	// network. validateWorkerURL returns the URL to register, pinned to the
	// validated IP for hostnames (DNS-rebinding defense).
	rawURL := req.AdvertiseURL
	if rawURL == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "meshjoin: cannot determine the caller's address; pass advertise_url", http.StatusBadRequest)
			return
		}
		rawURL = "http://" + net.JoinHostPort(host, strconv.Itoa(req.Port))
	}
	workerURL, err := validateWorkerURL(rawURL, s.resolve())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// --- atomic check-and-consume + side effects ---
	hash := hashCode(req.Code)
	s.mu.Lock()
	defer s.mu.Unlock()

	// (1) Invite check INSIDE the lock: the loser of a same-code race blocks
	// here until the winner's MarkInviteUsed lands, then sees it consumed.
	switch err := s.Store.CheckInvite(hash, s.Now()); {
	case errors.Is(err, store.ErrInviteExpired):
		http.Error(w, "meshjoin: invite code expired — mint a new one with `fort mesh invite`", http.StatusGone)
		return
	case errors.Is(err, store.ErrInviteInvalid):
		http.Error(w, "meshjoin: invite code invalid or already used", http.StatusUnauthorized)
		return
	case err != nil:
		s.Log.Error("meshjoin: invite check failed", "err", err)
		http.Error(w, "meshjoin: invite check failed", http.StatusInternalServerError)
		return
	}

	// (2) Write the registry FIRST (spec-024 ordering); on failure the code
	// stays valid. prior is retained for rollback if the consume fails.
	prior := s.Live.Load()
	base := prior
	if base == nil {
		// No registry yet (agent-less hub deferred its self-entry): the worker
		// alone seeds the managed registry.
		base = &machines.Registry{Version: 1}
	}
	reg := base.WithMachine(machines.Machine{Name: req.Name, URL: workerURL, Agents: req.Agents})
	m, _ := reg.Machine(req.Name) // canonical casing of an existing entry wins
	if !s.saveRegistry(w, reg) {
		return
	}

	// (3) Consume the code. If this fails, the join did NOT win: roll the Save
	// back (restore the prior registry file, or remove it if there was none)
	// and return WITHOUT installing any token-bearing transport or Live.Store.
	if err := s.Store.MarkInviteUsed(hash, s.Now()); err != nil {
		s.rollbackSave(prior)
		if errors.Is(err, store.ErrInviteInvalid) {
			s.Log.Warn("meshjoin: invite consumed concurrently — rolled back", "name", m.Name)
			http.Error(w, "meshjoin: invite code invalid or already used", http.StatusUnauthorized)
			return
		}
		s.Log.Error("meshjoin: mark invite used failed", "err", err)
		http.Error(w, "meshjoin: failed to consume the invite", http.StatusInternalServerError)
		return
	}

	// (4) Only now — after the code is consumed — install side effects.
	s.Live.Store(reg)
	// A valid code implies a prior `mesh invite` already called Tokens.Ensure,
	// so Get() is non-empty here.
	s.Cluster.Add(m.Name, remote.New(m.Name, m.URL, s.Tokens.Get()))
	s.appendMachineEvent("machine_joined", m)
	s.Log.Info("meshjoin: machine joined", "name", m.Name, "url", m.URL, "agents", m.Agents)

	writeJSON(w, http.StatusOK, joinResp{Token: s.Tokens.Get(), Name: m.Name, URL: m.URL})
}

// rollbackSave undoes a machines.Save when the join fails after the write:
// re-save the prior registry, or remove the file if there was none. Best
// effort — a failure here is logged; the registry pointer (Live) was never
// swapped, so in-memory state is already correct.
func (s *Server) rollbackSave(prior *machines.Registry) {
	if prior == nil {
		if err := os.Remove(s.RegistryPath); err != nil && !os.IsNotExist(err) {
			s.Log.Error("meshjoin: rollback remove failed", "path", s.RegistryPath, "err", err)
		}
		return
	}
	if err := machines.Save(s.RegistryPath, prior); err != nil {
		s.Log.Error("meshjoin: rollback re-save failed", "path", s.RegistryPath, "err", err)
	}
}

// --- remove ---

func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	if s.refuseUnmanaged(w) {
		return
	}
	name := r.PathValue("name")
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.Live.Load()
	m, ok := registryMachine(cur, name)
	if !ok {
		http.Error(w, fmt.Sprintf("meshjoin: machine %q is not in the registry", name), http.StatusNotFound)
		return
	}
	if strings.EqualFold(m.Name, s.NodeName) {
		http.Error(w, "meshjoin: cannot remove the hub's own entry", http.StatusBadRequest)
		return
	}
	reg := cur.Without(m.Name)
	if !s.saveRegistry(w, reg) {
		return
	}
	s.Live.Store(reg)
	s.Cluster.Remove(m.Name)
	s.appendMachineEvent("machine_removed", m)
	s.Log.Info("meshjoin: machine removed", "name", m.Name)

	// Roster-only removal (spec 024 D6): the machine keeps the shared token.
	writeJSON(w, http.StatusOK, map[string]string{
		"warning": m.Name + " still holds the mesh token; rotate it to revoke access (see docs/notes/threat-model.md)",
	})
}
