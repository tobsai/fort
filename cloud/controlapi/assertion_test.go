package controlapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
)

func TestServiceAssertionBindsAccountRouteAudienceDigestAndNonce(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_787_331_600, 0).UTC()
	digestBytes := sha256.Sum256([]byte(`{"message":"hello"}`))
	digest := hex.EncodeToString(digestBytes[:])
	key := []byte("0123456789abcdef0123456789abcdef")
	token, err := controlapi.IssueServiceAssertion(key, controlapi.ServiceAssertion{
		KeyID:         "service-2026-08",
		AccountID:     "4af424a4-d81a-47d5-a495-400868883b86",
		RouteClass:    "owner.commands.create",
		Audience:      "fort-control",
		RequestDigest: digest,
		IssuedAt:      now.Add(-time.Second),
		ExpiresAt:     now.Add(30 * time.Second),
		Nonce:         "908b3b526cf8472e91b1e6f71fb8df99",
	})
	if err != nil {
		t.Fatalf("issue assertion: %v", err)
	}

	nonces := &memoryNonceClaimer{seen: make(map[string]struct{})}
	verifier := controlapi.ServiceAssertionVerifier{
		Audience: "fort-control",
		Keys: map[string][]byte{
			"service-2026-08": key,
		},
		Clock:     func() time.Time { return now },
		Nonces:    nonces,
		MaxTTL:    time.Minute,
		ClockSkew: 5 * time.Second,
	}
	assertion, err := verifier.Verify(
		context.Background(),
		token,
		"owner.commands.create",
		digest,
	)
	if err != nil {
		t.Fatalf("verify assertion: %v", err)
	}
	if assertion.AccountID != "4af424a4-d81a-47d5-a495-400868883b86" {
		t.Fatalf("account ID = %q", assertion.AccountID)
	}
	if nonces.lastAccountID != assertion.AccountID {
		t.Fatalf("nonce claim account = %q, want signed account %q", nonces.lastAccountID, assertion.AccountID)
	}

	_, err = verifier.Verify(context.Background(), token, "owner.commands.create", digest)
	if !errors.Is(err, controlapi.ErrAssertionReplay) {
		t.Fatalf("second verify error = %v, want replay", err)
	}
}

func TestServiceAssertionMatchesGatewayCrossLanguageVector(t *testing.T) {
	t.Parallel()

	body := []byte(`{"after_cursor":"cursor-9"}`)
	digestBytes := sha256.Sum256(body)
	token, err := controlapi.IssueServiceAssertion(
		[]byte("0123456789abcdef0123456789abcdef"),
		controlapi.ServiceAssertion{
			KeyID:         "service-2026-08",
			AccountID:     "4af424a4-d81a-47d5-a495-400868883b86",
			RouteClass:    "owner.events.read",
			Audience:      "fort-control",
			RequestDigest: hex.EncodeToString(digestBytes[:]),
			IssuedAt:      time.Unix(1_787_331_600, 0).UTC(),
			ExpiresAt:     time.Unix(1_787_331_630, 0).UTC(),
			Nonce:         "908b3b526cf8472e91b1e6f71fb8df99",
		},
	)
	if err != nil {
		t.Fatalf("issue assertion: %v", err)
	}

	const gatewayToken = "eyJhbGciOiJIUzI1NiIsImtpZCI6InNlcnZpY2UtMjAyNi0wOCJ9.eyJhY2NvdW50X2lkIjoiNGFmNDI0YTQtZDgxYS00N2Q1LWE0OTUtNDAwODY4ODgzYjg2Iiwicm91dGVfY2xhc3MiOiJvd25lci5ldmVudHMucmVhZCIsImF1ZCI6ImZvcnQtY29udHJvbCIsInJlcXVlc3RfZGlnZXN0IjoiYWY2YjJiODkyNmQ0ZWM1ZTM5ODliZWEzODRlNzIwOTVlMWUzYmI5NTE5ZjE3ZTkzZjU1MjE3NjE4MGY0ZmExYyIsImlhdCI6MTc4NzMzMTYwMCwiZXhwIjoxNzg3MzMxNjMwLCJub25jZSI6IjkwOGIzYjUyNmNmODQ3MmU5MWIxZTZmNzFmYjhkZjk5In0.BNSYUAo-6B19UVvQRcqVTaAbTXV73ZMe5Ul3qpcIXog"
	if token != gatewayToken {
		t.Fatalf("token = %q, want gateway vector %q", token, gatewayToken)
	}
}

func TestServiceAssertionRejectsChangedTrustInputs(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_787_331_600, 0).UTC()
	digestBytes := sha256.Sum256([]byte("body"))
	digest := hex.EncodeToString(digestBytes[:])
	key := []byte("0123456789abcdef0123456789abcdef")
	base := controlapi.ServiceAssertion{
		KeyID:         "service-2026-08",
		AccountID:     "4af424a4-d81a-47d5-a495-400868883b86",
		RouteClass:    "owner.commands.create",
		Audience:      "fort-control",
		RequestDigest: digest,
		IssuedAt:      now.Add(-time.Second),
		ExpiresAt:     now.Add(30 * time.Second),
		Nonce:         "908b3b526cf8472e91b1e6f71fb8df99",
	}
	token, err := controlapi.IssueServiceAssertion(key, base)
	if err != nil {
		t.Fatalf("issue assertion: %v", err)
	}
	tampered := []byte(token)
	signatureStart := len(tampered) - 43
	if tampered[signatureStart] == 'A' {
		tampered[signatureStart] = 'B'
	} else {
		tampered[signatureStart] = 'A'
	}

	tests := []struct {
		name       string
		token      string
		audience   string
		routeClass string
		digest     string
		now        time.Time
		want       error
	}{
		{name: "wrong audience", token: token, audience: "other-control", routeClass: base.RouteClass, digest: digest, now: now, want: controlapi.ErrAssertionAudience},
		{name: "wrong route", token: token, audience: base.Audience, routeClass: "owner.agents.list", digest: digest, now: now, want: controlapi.ErrAssertionRoute},
		{name: "wrong digest", token: token, audience: base.Audience, routeClass: base.RouteClass, digest: hex.EncodeToString(make([]byte, sha256.Size)), now: now, want: controlapi.ErrAssertionDigest},
		{name: "expired", token: token, audience: base.Audience, routeClass: base.RouteClass, digest: digest, now: now.Add(time.Minute), want: controlapi.ErrAssertionExpired},
		{name: "tampered", token: string(tampered), audience: base.Audience, routeClass: base.RouteClass, digest: digest, now: now, want: controlapi.ErrAssertionSignature},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			verifier := controlapi.ServiceAssertionVerifier{
				Audience: test.audience,
				Keys:     map[string][]byte{"service-2026-08": key},
				Clock:    func() time.Time { return test.now },
				Nonces:   &memoryNonceClaimer{seen: make(map[string]struct{})},
				MaxTTL:   time.Minute,
			}
			_, err := verifier.Verify(context.Background(), test.token, test.routeClass, test.digest)
			if !errors.Is(err, test.want) {
				t.Fatalf("Verify error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestServiceAssertionFailsClosedWithoutDurableNonceClaimer(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_787_331_600, 0).UTC()
	digestBytes := sha256.Sum256(nil)
	digest := hex.EncodeToString(digestBytes[:])
	key := []byte("0123456789abcdef0123456789abcdef")
	token, err := controlapi.IssueServiceAssertion(key, controlapi.ServiceAssertion{
		KeyID:         "service-2026-08",
		AccountID:     "4af424a4-d81a-47d5-a495-400868883b86",
		RouteClass:    "owner.agents.list",
		Audience:      "fort-control",
		RequestDigest: digest,
		IssuedAt:      now,
		ExpiresAt:     now.Add(30 * time.Second),
		Nonce:         "908b3b526cf8472e91b1e6f71fb8df99",
	})
	if err != nil {
		t.Fatalf("issue assertion: %v", err)
	}

	verifier := controlapi.ServiceAssertionVerifier{
		Audience: "fort-control",
		Keys:     map[string][]byte{"service-2026-08": key},
		Clock:    func() time.Time { return now },
		MaxTTL:   time.Minute,
	}
	_, err = verifier.Verify(context.Background(), token, "owner.agents.list", digest)
	if !errors.Is(err, controlapi.ErrAssertionNonceStore) {
		t.Fatalf("Verify error = %v, want nonce-store failure", err)
	}
}

func TestIssueServiceAssertionRejectsNonCanonicalSecurityFields(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_787_331_600, 0).UTC()
	valid := controlapi.ServiceAssertion{
		KeyID:         "service-2026-08",
		AccountID:     "4af424a4-d81a-47d5-a495-400868883b86",
		RouteClass:    "owner.events.read",
		Audience:      "fort-control",
		RequestDigest: "af6b2b8926d4ec5e3989bea384e72095e1e3bb9519f17e93f552176180f4fa1c",
		IssuedAt:      now,
		ExpiresAt:     now.Add(30 * time.Second),
		Nonce:         "908b3b526cf8472e91b1e6f71fb8df99",
	}
	tests := []struct {
		name   string
		mutate func(*controlapi.ServiceAssertion)
	}{
		{name: "noncanonical account", mutate: func(value *controlapi.ServiceAssertion) { value.AccountID = strings.ToUpper(value.AccountID) }},
		{name: "key id whitespace", mutate: func(value *controlapi.ServiceAssertion) { value.KeyID = " service" }},
		{name: "route whitespace", mutate: func(value *controlapi.ServiceAssertion) { value.RouteClass = "owner events" }},
		{name: "audience whitespace", mutate: func(value *controlapi.ServiceAssertion) { value.Audience = "fort control" }},
		{name: "uppercase digest", mutate: func(value *controlapi.ServiceAssertion) { value.RequestDigest = strings.ToUpper(value.RequestDigest) }},
		{name: "short nonce", mutate: func(value *controlapi.ServiceAssertion) { value.Nonce = "short" }},
		{name: "nonce padding", mutate: func(value *controlapi.ServiceAssertion) { value.Nonce += "=" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertion := valid
			test.mutate(&assertion)
			if _, err := controlapi.IssueServiceAssertion([]byte("0123456789abcdef0123456789abcdef"), assertion); !errors.Is(err, controlapi.ErrAssertionInvalid) {
				t.Fatalf("IssueServiceAssertion error = %v, want %v", err, controlapi.ErrAssertionInvalid)
			}
		})
	}
}

type memoryNonceClaimer struct {
	seen          map[string]struct{}
	lastAccountID string
}

func (claimer *memoryNonceClaimer) Claim(_ context.Context, accountID, keyID, nonce string, _ time.Time) (bool, error) {
	claimer.lastAccountID = accountID
	key := accountID + ":" + keyID + ":" + nonce
	if _, ok := claimer.seen[key]; ok {
		return false, nil
	}
	claimer.seen[key] = struct{}{}
	return true, nil
}
