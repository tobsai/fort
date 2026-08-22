package controlapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tobsai/fort/cloud/controlapi"
)

func TestHealthReportsExactDeploymentCompatibility(t *testing.T) {
	t.Parallel()

	handler := controlapi.HealthHandler(controlapi.BuildInfo{
		Commit:         "712ae4694bbf9eb4a9cac8aeccfb1902e4994d30",
		SchemaVersion:  "20260821203821",
		APIMinVersion:  2,
		APIMaxVersion:  2,
		AuthorityEpoch: 7,
		AuthorityMode:  "legacy_v1_write",
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	var response struct {
		Status         string `json:"status"`
		Commit         string `json:"commit"`
		SchemaVersion  string `json:"schema_version"`
		APIMinVersion  int    `json:"api_min_version"`
		APIMaxVersion  int    `json:"api_max_version"`
		AuthorityEpoch int64  `json:"authority_epoch"`
		AuthorityMode  string `json:"authority_mode"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" ||
		response.Commit != "712ae4694bbf9eb4a9cac8aeccfb1902e4994d30" ||
		response.SchemaVersion != "20260821203821" ||
		response.APIMinVersion != 2 ||
		response.APIMaxVersion != 2 ||
		response.AuthorityEpoch != 7 ||
		response.AuthorityMode != "legacy_v1_write" {
		t.Fatalf("unexpected health response: %+v", response)
	}
}

func TestHealthFailsClosedWhenBuildMetadataIsIncomplete(t *testing.T) {
	t.Parallel()

	handler := controlapi.HealthHandler(controlapi.BuildInfo{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if got := recorder.Body.String(); got != "{\"code\":\"deployment_metadata_unavailable\"}\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestHealthRejectsMutationMethods(t *testing.T) {
	t.Parallel()

	handler := controlapi.HealthHandler(controlapi.BuildInfo{
		Commit:         "712ae4694bbf9eb4a9cac8aeccfb1902e4994d30",
		SchemaVersion:  "20260821203821",
		APIMinVersion:  2,
		APIMaxVersion:  2,
		AuthorityEpoch: 7,
		AuthorityMode:  "legacy_v1_write",
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/health", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if got := recorder.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", got)
	}
}
