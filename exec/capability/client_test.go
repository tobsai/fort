package capability

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	corecap "github.com/tobsai/fort/core/capability"
)

func TestClientReadsLiveTokenForEveryRequest(t *testing.T) {
	var token atomic.Value
	token.Store("first-token")
	headers := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(corecap.NodeInventory{
			ProtocolVersion: corecap.ProtocolVersion, CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
			NodeID: "node", State: corecap.MachineUnknown, Reason: corecap.ReasonStale,
			Profiles: []corecap.ProfileOffer{}, Offers: []corecap.LogicalOffer{},
			Bindings: []corecap.ExecutionBindingOffer{},
		})
	}))
	t.Cleanup(server.Close)

	client := NewClientWithToken(func() string { return token.Load().(string) })
	if _, err := client.Get(context.Background(), server.URL, "node"); err != nil {
		t.Fatal(err)
	}
	token.Store("enrolled-token")
	if _, err := client.Get(context.Background(), server.URL, "node"); err != nil {
		t.Fatal(err)
	}
	if first, second := <-headers, <-headers; first != "Bearer first-token" || second != "Bearer enrolled-token" {
		t.Fatalf("authorization headers = %q, %q", first, second)
	}
}

func TestClientRefreshBindsAuthenticatedNodeIdentity(t *testing.T) {
	inventory := corecap.NodeInventory{
		ProtocolVersion: corecap.ProtocolVersion, CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
		NodeID: "enrolled-node", ObservedAt: time.Unix(1, 0).UTC(),
		State: corecap.MachinePartial, Reason: corecap.ReasonAuthRequired,
		Profiles: []corecap.ProfileOffer{}, Offers: []corecap.LogicalOffer{},
		Bindings: []corecap.ExecutionBindingOffer{},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/node/capabilities/recheck" || r.Header.Get("Authorization") != "Bearer mesh-token" {
			http.Error(w, "bad request", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(inventory)
	}))
	t.Cleanup(server.Close)

	client := NewClient("mesh-token")
	got, err := client.Refresh(context.Background(), server.URL, "enrolled-node", corecap.RecheckRequest{
		ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
		MaxAgeSeconds: 60, Adapters: []string{"profile.codex.native"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.NodeID != "enrolled-node" {
		t.Fatalf("node id = %q", got.NodeID)
	}
}

func TestClientRejectsNodeIdentityMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(corecap.NodeInventory{
			ProtocolVersion: corecap.ProtocolVersion, CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
			NodeID: "other", State: corecap.MachineUnknown, Reason: corecap.ReasonStale,
			Profiles: []corecap.ProfileOffer{}, Offers: []corecap.LogicalOffer{},
			Bindings: []corecap.ExecutionBindingOffer{},
		})
	}))
	t.Cleanup(server.Close)

	_, err := NewClient("token").Get(context.Background(), server.URL, "expected")
	var discovery *DiscoveryError
	if !errors.As(err, &discovery) || discovery.Reason != corecap.ReasonCommandContractChanged {
		t.Fatalf("error = %#v", err)
	}
}

func TestClientClassifiesOldNode404WithoutParsingBody(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)

	_, err := NewClient("token").Get(context.Background(), server.URL, "node")
	var discovery *DiscoveryError
	if !errors.As(err, &discovery) || discovery.Reason != corecap.ReasonOldNode {
		t.Fatalf("error = %#v", err)
	}
}

func TestClientRejectsOversizedInventory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"padding":"` + strings.Repeat("x", 512*1024) + `"}`))
	}))
	t.Cleanup(server.Close)

	if _, err := NewClient("token").Get(context.Background(), server.URL, "node"); err == nil {
		t.Fatal("expected oversized response to fail")
	}
}
