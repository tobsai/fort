package node

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tobsai/fort/exec/fake"
)

// TestTokenBecomesLiveWithoutRestart is the regression guard for spec 024: the
// first `mesh invite` mints the mesh token inside the running daemon, so the
// node server must observe it without a restart. The token is read per-request
// via a func() string rather than captured once at construction.
func TestTokenBecomesLiveWithoutRestart(t *testing.T) {
	tok := ""
	srv := New(fake.New(), func() string { return tok })
	mux := http.NewServeMux()
	srv.Register(mux)
	req := httptest.NewRequest("POST", "/api/exec", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer later")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("empty token: %d, want 403", rec.Code)
	}
	tok = "later" // first `mesh invite` minted the token — no restart
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/exec", strings.NewReader("{}")))
	// wrong/missing header still 401
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no header: %d, want 401", rec.Code)
	}

	// A request WITH the now-valid token must pass auth and reach handleExec.
	req = httptest.NewRequest("POST", "/api/exec", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer later")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authed: %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("content-type = %q, want application/x-ndjson", ct)
	}
}
