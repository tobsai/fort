package requestid

import (
	"context"
	"testing"
)

func TestWithAndFromAcceptOnlyCanonicalUUID(t *testing.T) {
	const id = "018f3f1c-7d3a-7c1d-a176-9c52c606c6e4"
	if got := From(With(context.Background(), id)); got != id {
		t.Fatalf("request id=%q", got)
	}
	if got := From(With(context.Background(), "not-a-request-id")); got != "" {
		t.Fatalf("invalid request id=%q", got)
	}
	if got := New(); !Valid(got) {
		t.Fatalf("generated request id=%q is invalid", got)
	}
}
