package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEntrypointReadsOnlyNamedPublicMetadata(t *testing.T) {
	values := map[string]string{
		"VERCEL_GIT_COMMIT_SHA": "712ae4694bbf9eb4a9cac8aeccfb1902e4994d30",
		"FORT_SCHEMA_VERSION":   "20260821203821",
		"FORT_API_MIN_VERSION":  "2",
		"FORT_API_MAX_VERSION":  "2",
		"FORT_AUTHORITY_EPOCH":  "7",
		"FORT_AUTHORITY_MODE":   "legacy_v1_write",
		"DATABASE_URL":          "postgres://secret",
	}
	handler := healthHandler(func(name string) string { return values[name] })
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Body.String(); got == "" || contains(got, "postgres://secret") || contains(got, "DATABASE_URL") {
		t.Fatalf("health leaked secret-bearing configuration: %q", got)
	}
}

func TestHealthEntrypointUsesExplicitCommitForArchivedCLIDeployments(t *testing.T) {
	values := map[string]string{
		"FORT_BUILD_COMMIT":    "8acfeada5f25eceb5751c87770c6b3fd528465cb",
		"FORT_SCHEMA_VERSION":  "20260821203821",
		"FORT_API_MIN_VERSION": "2",
		"FORT_API_MAX_VERSION": "2",
		"FORT_AUTHORITY_EPOCH": "7",
		"FORT_AUTHORITY_MODE":  "legacy_v1_write",
	}
	handler := healthHandler(func(name string) string { return values[name] })
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Body.String(); !contains(got, values["FORT_BUILD_COMMIT"]) {
		t.Fatalf("health commit = %q, want explicit archive commit", got)
	}
}

func TestHealthEntrypointRejectsMalformedNumericMetadata(t *testing.T) {
	values := map[string]string{
		"VERCEL_GIT_COMMIT_SHA": "712ae4694bbf9eb4a9cac8aeccfb1902e4994d30",
		"FORT_SCHEMA_VERSION":   "20260821203821",
		"FORT_API_MIN_VERSION":  "two",
		"FORT_API_MAX_VERSION":  "2",
		"FORT_AUTHORITY_EPOCH":  "7",
		"FORT_AUTHORITY_MODE":   "legacy_v1_write",
	}
	handler := healthHandler(func(name string) string { return values[name] })
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
