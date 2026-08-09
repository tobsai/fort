package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/transporttrust"
	"github.com/tobsai/fort/ui"
)

func relayRouteResponse(t *testing.T, handler http.Handler, method, path string, trusted bool) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(`{}`))
	request.RemoteAddr = "192.0.2.25:4000"
	request.Host = "127.0.0.1:4087"
	request.Header.Set("Content-Type", "application/json")
	if trusted {
		request = request.WithContext(transporttrust.WithTrusted(request.Context()))
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestNativeRelayCompositionUsesActualPhaseOneModeAndNoLegacySurface(t *testing.T) {
	for _, mode := range []ui.PrimaryChannelsMode{ui.PrimaryChannelsPreview, ui.PrimaryChannelsPrimary} {
		t.Run(string(mode), func(t *testing.T) {
			mux := http.NewServeMux()
			if err := registerNativeRelayRoutes(mux, ui.New(ui.Deps{}), mode); err != nil {
				t.Fatal(err)
			}

			for _, route := range []struct {
				method string
				path   string
			}{
				{http.MethodGet, "/api/settings/primary-agent"},
				{http.MethodPut, "/api/settings/primary-agent"},
				{http.MethodDelete, "/api/settings/primary-agent"},
				{http.MethodPost, "/api/settings/primary-agent/recheck"},
				{http.MethodGet, "/api/channels"},
				{http.MethodPost, "/api/channels"},
				{http.MethodGet, "/api/channels/channel-1"},
				{http.MethodPatch, "/api/channels/channel-1"},
				{http.MethodPost, "/api/channels/channel-1/turns"},
				{http.MethodPost, "/api/channels/channel-1/targets/target-1/retry"},
				{http.MethodPost, "/api/channels/channel-1/targets/target-1/recheck-and-retry"},
				{http.MethodPost, "/api/channels/channel-1/targets/target-1/cancel"},
				{http.MethodGet, "/api/channels/channel-1/events"},
				{http.MethodGet, "/api/needs-you"},
				{http.MethodGet, "/api/schedules"},
				{http.MethodGet, "/api/schedules/schedule-1"},
				{http.MethodGet, "/api/schedules/schedule-1/occurrences"},
			} {
				if response := relayRouteResponse(t, mux, route.method, route.path, true); response.Code == http.StatusNotFound {
					t.Errorf("trusted %s %s was not mounted", route.method, route.path)
				}
			}

			for _, route := range []struct {
				method string
				path   string
			}{
				{http.MethodGet, "/"},
				{http.MethodGet, "/channels-preview"},
				{http.MethodGet, "/shared"},
				{http.MethodGet, "/legacy"},
				{http.MethodGet, "/api/summary"},
				{http.MethodGet, "/api/board"},
				{http.MethodGet, "/api/projects"},
				{http.MethodPost, "/api/chat"},
				{http.MethodPost, "/api/gate"},
				{http.MethodPost, "/api/schedules"},
			} {
				if response := relayRouteResponse(t, mux, route.method, route.path, true); response.Code != http.StatusNotFound {
					t.Errorf("trusted %s %s status=%d, want 404", route.method, route.path, response.Code)
				}
			}

			if response := relayRouteResponse(t, mux, http.MethodGet, "/api/channels", false); response.Code != http.StatusForbidden {
				t.Fatalf("untrusted Primary Channels status=%d, want 403", response.Code)
			}
		})
	}
}

func TestNativeRelayCompositionOffMountsNoPhaseOneRoutes(t *testing.T) {
	mux := http.NewServeMux()
	if err := registerNativeRelayRoutes(mux, ui.New(ui.Deps{}), ui.PrimaryChannelsOff); err != nil {
		t.Fatal(err)
	}
	if response := relayRouteResponse(t, mux, http.MethodGet, "/api/channels", true); response.Code != http.StatusNotFound {
		t.Fatalf("off relay status=%d, want 404", response.Code)
	}
}

func TestNativeRelayCompositionRejectsUnknownMode(t *testing.T) {
	if err := registerNativeRelayRoutes(
		http.NewServeMux(), ui.New(ui.Deps{}), ui.PrimaryChannelsMode("future"),
	); err == nil {
		t.Fatal("unknown native relay mode was accepted")
	}
}
