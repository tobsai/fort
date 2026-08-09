package ui

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/transporttrust"
)

func modeResponse(t *testing.T, handler http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	request.RemoteAddr = "127.0.0.1:4000"
	request.Host = "127.0.0.1:4087"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func modeMux(t *testing.T, mode PrimaryChannelsMode) *http.ServeMux {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	mux := http.NewServeMux()
	if err := New(Deps{Store: st}).RegisterMode(mux, mode); err != nil {
		t.Fatal(err)
	}
	return mux
}

func TestPrimaryChannelsOffFailsClosedAndKeepsSharedRoot(t *testing.T) {
	mux := modeMux(t, PrimaryChannelsOff)
	if response := modeResponse(t, mux, http.MethodGet, "/"); response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "Fort · Conversations") {
		t.Fatalf("off root status=%d body=%s", response.Code, response.Body.String())
	}
	for _, path := range []string{
		"/channels-preview", "/api/settings/primary-agent", "/api/channels",
		"/api/channels/anything", "/api/needs-you", "/api/schedules", "/api/schedules/anything",
		"/channels-preview/stale", "/api/settings/primary-agent/stale",
		"/api/channels/anything/targets/target/stale", "/api/needs-you/stale",
		"/api/schedules/anything/occurrences/stale",
	} {
		if response := modeResponse(t, mux, http.MethodGet, path); response.Code != http.StatusNotFound {
			t.Errorf("GET %s status=%d, want 404", path, response.Code)
		}
	}
}

func TestPrimaryChannelsPreviewMountsOnlyPhaseOneShellAndNarrowAPIs(t *testing.T) {
	mux := modeMux(t, PrimaryChannelsPreview)
	if response := modeResponse(t, mux, http.MethodGet, "/"); response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "Fort · Private Channels") {
		t.Fatalf("preview root status=%d body=%s", response.Code, response.Body.String())
	}
	if response := modeResponse(t, mux, http.MethodGet, "/channels-preview"); response.Code != http.StatusTemporaryRedirect ||
		response.Header().Get("Location") != "/" {
		t.Fatalf("preview redirect status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
	for _, path := range []string{"/api/settings/primary-agent", "/api/channels", "/api/needs-you", "/api/schedules"} {
		if response := modeResponse(t, mux, http.MethodGet, path); response.Code == http.StatusNotFound {
			t.Errorf("GET %s was not mounted in preview", path)
		}
	}
	assertLegacyRoutesAbsent(t, mux)
}

func TestPrimaryChannelsPrimaryMountsOnlyPhaseOneShellAndNarrowAPIs(t *testing.T) {
	mux := modeMux(t, PrimaryChannelsPrimary)
	if response := modeResponse(t, mux, http.MethodGet, "/"); response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "Fort · Private Channels") {
		t.Fatalf("primary root status=%d body=%s", response.Code, response.Body.String())
	}
	if response := modeResponse(t, mux, http.MethodGet, "/channels-preview"); response.Code != http.StatusTemporaryRedirect ||
		response.Header().Get("Location") != "/" {
		t.Fatalf("preview redirect status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
	assertLegacyRoutesAbsent(t, mux)
}

func assertLegacyRoutesAbsent(t *testing.T, mux http.Handler) {
	t.Helper()
	for _, test := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/shared"},
		{http.MethodGet, "/legacy"},
		{http.MethodGet, "/api/summary"},
		{http.MethodGet, "/api/board"},
		{http.MethodGet, "/api/projects"},
		{http.MethodGet, "/api/playbooks"},
		{http.MethodGet, "/api/conversations"},
		{http.MethodPost, "/api/chat"},
		{http.MethodPost, "/api/gate"},
		{http.MethodPost, "/api/backlog"},
		{http.MethodPost, "/api/schedules"},
	} {
		if response := modeResponse(t, mux, test.method, test.path); response.Code != http.StatusNotFound {
			t.Errorf("%s %s status=%d, want 404", test.method, test.path, response.Code)
		}
	}
}

func TestPrimaryShellRequiresLoopbackPeerAndHostAndHidesLegacyPaths(t *testing.T) {
	for _, test := range []struct {
		name       string
		mode       PrimaryChannelsMode
		path       string
		remoteAddr string
		host       string
		want       int
	}{
		{name: "remote preview", mode: PrimaryChannelsPreview, path: "/channels-preview", remoteAddr: "192.0.2.25:4000", host: "127.0.0.1:4087", want: http.StatusForbidden},
		{name: "LAN preview host", mode: PrimaryChannelsPreview, path: "/channels-preview", remoteAddr: "127.0.0.1:4000", host: "192.168.1.25:4087", want: http.StatusForbidden},
		{name: "remote primary", mode: PrimaryChannelsPrimary, path: "/", remoteAddr: "192.0.2.25:4000", host: "127.0.0.1:4087", want: http.StatusForbidden},
		{name: "LAN primary host", mode: PrimaryChannelsPrimary, path: "/", remoteAddr: "127.0.0.1:4000", host: "192.168.1.25:4087", want: http.StatusForbidden},
		{name: "remote primary preview redirect", mode: PrimaryChannelsPrimary, path: "/channels-preview", remoteAddr: "192.0.2.25:4000", host: "127.0.0.1:4087", want: http.StatusForbidden},
		{name: "remote shared", mode: PrimaryChannelsPrimary, path: "/shared", remoteAddr: "192.0.2.25:4000", host: "192.168.1.25:4087", want: http.StatusNotFound},
		{name: "remote legacy", mode: PrimaryChannelsPrimary, path: "/legacy", remoteAddr: "192.0.2.25:4000", host: "192.168.1.25:4087", want: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			request.RemoteAddr, request.Host = test.remoteAddr, test.host
			result := httptest.NewRecorder()
			modeMux(t, test.mode).ServeHTTP(result, request)
			if result.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", result.Code, test.want, result.Body.String())
			}
		})
	}
}

func TestPrimaryHTMLShellRejectsAuthenticatedRelayContext(t *testing.T) {
	for _, test := range []struct {
		mode PrimaryChannelsMode
		path string
	}{
		{mode: PrimaryChannelsPreview, path: "/channels-preview"},
		{mode: PrimaryChannelsPrimary, path: "/"},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.RemoteAddr, request.Host = "192.0.2.25:4000", "127.0.0.1:4087"
		request = request.WithContext(transporttrust.WithTrusted(request.Context()))
		result := httptest.NewRecorder()
		modeMux(t, test.mode).ServeHTTP(result, request)
		if result.Code != http.StatusForbidden {
			t.Errorf("%s shell status=%d want=%d", test.path, result.Code, http.StatusForbidden)
		}
	}
}

func TestPrimaryChannelsModeRejectsUnknownValue(t *testing.T) {
	if err := New(Deps{}).RegisterMode(http.NewServeMux(), PrimaryChannelsMode("future")); err == nil {
		t.Fatal("unknown Primary Channels mode was accepted")
	}
}
