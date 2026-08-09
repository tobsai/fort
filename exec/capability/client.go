package capability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
)

const maxNodeInventoryBytes = 512 * 1024

type DiscoveryError struct {
	Reason corecap.Reason
}

func (e *DiscoveryError) Error() string {
	return "capability discovery failed: " + string(e.Reason)
}

type Client struct {
	token  func() string
	client *http.Client
}

func NewClient(token string) *Client {
	return NewClientWithToken(func() string { return token })
}

// NewClientWithToken reads the mesh token immediately before every request so
// a token minted by live mesh enrollment takes effect without a daemon restart.
func NewClientWithToken(token func() string) *Client {
	if token == nil {
		token = func() string { return "" }
	}
	return &Client{
		token:  token,
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) Get(ctx context.Context, baseURL, expectedNodeID string) (corecap.NodeInventory, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/node/capabilities", nil)
	if err != nil {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonUnavailable}
	}
	return c.do(request, expectedNodeID)
}

func (c *Client) Refresh(ctx context.Context, baseURL, expectedNodeID string, refresh corecap.RecheckRequest) (corecap.NodeInventory, error) {
	body, err := json.Marshal(refresh)
	if err != nil {
		return corecap.NodeInventory{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/node/capabilities/recheck", bytes.NewReader(body))
	if err != nil {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonUnavailable}
	}
	request.Header.Set("Content-Type", "application/json")
	return c.do(request, expectedNodeID)
}

func (c *Client) do(request *http.Request, expectedNodeID string) (corecap.NodeInventory, error) {
	request.Header.Set("Authorization", "Bearer "+c.token())
	response, err := c.client.Do(request)
	if err != nil {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonUnavailable}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonOldNode}
	}
	if response.StatusCode != http.StatusOK {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonUnavailable}
	}
	limited := io.LimitReader(response.Body, maxNodeInventoryBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil || len(raw) > maxNodeInventoryBytes {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonCommandContractChanged}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var inventory corecap.NodeInventory
	if err := decoder.Decode(&inventory); err != nil {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonCommandContractChanged}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonCommandContractChanged}
	}
	if inventory.NodeID != expectedNodeID {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonCommandContractChanged}
	}
	if inventory.ProtocolVersion != corecap.ProtocolVersion ||
		inventory.CatalogVersion != corecap.CatalogVersion ||
		inventory.ProfileMappingVersion != corecap.ProfileMappingVersion {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonOldNode}
	}
	if inventory.Profiles == nil || inventory.Offers == nil || inventory.Bindings == nil || inventory.TextOnlyOptions == nil {
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonCommandContractChanged}
	}
	// Reuse the core's exact offer/predicate validator. Coordinator-owned fields
	// are synthetic and discarded after validation.
	_, err = corecap.NormalizeSnapshot(corecap.Snapshot{
		CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
		LocalMachine: expectedNodeID,
		Machines: []corecap.MachineInventory{{
			Name: expectedNodeID, Local: true, RegistryRank: 0, Reachable: true,
			ProtocolVersion: inventory.ProtocolVersion, CatalogVersion: inventory.CatalogVersion,
			ProfileMappingVersion: inventory.ProfileMappingVersion,
			State:                 inventory.State, Reason: inventory.Reason, ObservedAt: inventory.ObservedAt,
			Profiles: inventory.Profiles, Offers: inventory.Offers, Bindings: inventory.Bindings,
			TextOnlyOptions: inventory.TextOnlyOptions,
		}},
	})
	if err != nil {
		_ = fmt.Sprintf("%v", err) // private validation detail is intentionally discarded
		return corecap.NodeInventory{}, &DiscoveryError{Reason: corecap.ReasonCommandContractChanged}
	}
	return inventory, nil
}
