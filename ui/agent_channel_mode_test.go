package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentProductModePreservesPrimaryChannelsContractDuringCutover(t *testing.T) {
	legacyMux := http.NewServeMux()
	if err := New(Deps{}).RegisterMode(legacyMux, PrimaryChannelsPreview); err != nil {
		t.Fatal(err)
	}
	productMux := http.NewServeMux()
	if err := New(Deps{}).RegisterProductMode(productMux, ProductMode{
		PrimaryChannels: PrimaryChannelsPreview,
		AgentChannels:   AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}

	request := func(handler http.Handler) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
		req.RemoteAddr = "127.0.0.1:4000"
		req.Host = "127.0.0.1:4087"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	legacy, product := request(legacyMux), request(productMux)
	if product.Code != legacy.Code || product.Body.String() != legacy.Body.String() ||
		product.Header().Get("Content-Type") != legacy.Header().Get("Content-Type") {
		t.Fatalf("product /api/channels = status %d content-type %q body %q; want unchanged status %d content-type %q body %q",
			product.Code, product.Header().Get("Content-Type"), product.Body.String(),
			legacy.Code, legacy.Header().Get("Content-Type"), legacy.Body.String())
	}
}

func TestAgentChannelsOffTombstonesTheWholeAgentFirstContract(t *testing.T) {
	mux := http.NewServeMux()
	if err := New(Deps{}).RegisterProductMode(mux, ProductMode{
		PrimaryChannels: PrimaryChannelsOff,
		AgentChannels:   AgentChannelsOff,
	}); err != nil {
		t.Fatal(err)
	}

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/agent-options"},
		{http.MethodPost, "/api/agent-options/recheck"},
		{http.MethodGet, "/api/agent-needs-you"},
		{http.MethodGet, "/api/agent-channels"},
		{http.MethodPost, "/api/agent-channels"},
		{http.MethodGet, "/api/agent-channels/channel-1"},
		{http.MethodPatch, "/api/agent-channels/channel-1"},
		{http.MethodGet, "/api/agent-channels/channel-1/conversations"},
		{http.MethodPost, "/api/agent-channels/channel-1/conversations"},
		{http.MethodGet, "/api/agent-channels/channel-1/conversations/conversation-1"},
		{http.MethodPatch, "/api/agent-channels/channel-1/conversations/conversation-1"},
		{http.MethodPost, "/api/agent-channels/channel-1/conversations/conversation-1/turns"},
		{http.MethodPost, "/api/agent-channels/channel-1/conversations/conversation-1/targets/target-1/retry"},
		{http.MethodPost, "/api/agent-channels/channel-1/conversations/conversation-1/targets/target-1/cancel"},
		{http.MethodGet, "/api/agent-channels/channel-1/conversations/conversation-1/events"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status=%d want=%d body=%s", response.Code, http.StatusNotFound, response.Body.String())
			}
			if strings.Contains(strings.ToLower(response.Body.String()), "<!doctype html") {
				t.Fatalf("disabled Agent Channels route returned the legacy HTML shell: %s", response.Body.String())
			}
		})
	}
}

func TestNativeProductModePreservesPrimaryChannelsContractDuringCutover(t *testing.T) {
	legacyMux := http.NewServeMux()
	if err := New(Deps{}).RegisterNativeRelayRoutes(legacyMux, PrimaryChannelsPreview); err != nil {
		t.Fatal(err)
	}
	productMux := http.NewServeMux()
	if err := New(Deps{}).RegisterNativeProductRoutes(productMux, ProductMode{
		PrimaryChannels: PrimaryChannelsPreview,
		AgentChannels:   AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}

	request := func(handler http.Handler) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
		req.RemoteAddr = "192.0.2.25:4000"
		req.Host = "127.0.0.1:4087"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	legacy, product := request(legacyMux), request(productMux)
	if product.Code != legacy.Code || product.Body.String() != legacy.Body.String() {
		t.Fatalf("native product /api/channels = status %d body %q; want unchanged status %d body %q",
			product.Code, product.Body.String(), legacy.Code, legacy.Body.String())
	}
}
