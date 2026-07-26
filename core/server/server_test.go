package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tobsai/fort/core/requestid"
)

func TestHealthEndpoint(t *testing.T) {
	s := New(Deps{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %v, want ok", body["status"])
	}
}

func TestHandlerCorrelatesRequestWithoutLoggingPayload(t *testing.T) {
	var observed RequestEvent
	s := New(Deps{
		RequestObserver: func(event RequestEvent) { observed = event },
		Mount: func(mux *http.ServeMux) {
			mux.HandleFunc("POST /api/check", func(w http.ResponseWriter, r *http.Request) {
				if requestid.From(r.Context()) == "" {
					t.Error("handler context has no request id")
				}
				w.WriteHeader(http.StatusAccepted)
			})
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/check?secret=do-not-log", nil)
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted || !requestid.Valid(rec.Header().Get(requestid.Header)) {
		t.Fatalf("status=%d request-id=%q", rec.Code, rec.Header().Get(requestid.Header))
	}
	if observed.ID != rec.Header().Get(requestid.Header) || observed.Method != http.MethodPost ||
		observed.Path != "/api/check" || observed.Status != http.StatusAccepted {
		t.Fatalf("observed=%+v", observed)
	}
	if observed.Duration < 0 {
		t.Fatalf("duration=%v", observed.Duration)
	}
}

func TestUnknownRoute404(t *testing.T) {
	s := New(Deps{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
