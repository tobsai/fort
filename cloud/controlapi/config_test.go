package controlapi_test

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
)

func TestServiceAssertionVerifierFromEnvironmentLoadsRotationKeyring(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"` + base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")) + `","service-2026-09":"` + base64.RawURLEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789")) + `"}`,
	}
	nonces := &memoryNonceClaimer{seen: make(map[string]struct{})}
	verifier, err := controlapi.ServiceAssertionVerifierFromEnvironment(func(key string) string {
		return values[key]
	}, nonces)
	if err != nil {
		t.Fatalf("load verifier: %v", err)
	}
	if verifier.Audience != "fort-control" || verifier.MaxTTL != time.Minute || verifier.ClockSkew != 5*time.Second {
		t.Fatalf("verifier contract = %+v", verifier)
	}
	if len(verifier.Keys) != 2 || string(verifier.Keys["service-2026-08"]) != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("decoded verifier keys = %#v", verifier.Keys)
	}
}

func TestServiceAssertionVerifierFromEnvironmentFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "missing", raw: ""},
		{name: "unknown field shape", raw: `[]`},
		{name: "empty keyring", raw: `{}`},
		{name: "short key", raw: `{"service":"c2hvcnQ"}`},
		{name: "padded base64", raw: `{"service":"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="}`},
		{name: "blank key id", raw: `{"":"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := controlapi.ServiceAssertionVerifierFromEnvironment(func(key string) string {
				if key == "FORT_CONTROL_ASSERTION_KEYS_JSON" {
					return test.raw
				}
				return ""
			}, &memoryNonceClaimer{seen: make(map[string]struct{})})
			if !errors.Is(err, controlapi.ErrAssertionConfiguration) {
				t.Fatalf("error = %v, want %v", err, controlapi.ErrAssertionConfiguration)
			}
		})
	}

	valid := `{"service":"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY"}`
	_, err := controlapi.ServiceAssertionVerifierFromEnvironment(func(string) string { return valid }, nil)
	if !errors.Is(err, controlapi.ErrAssertionConfiguration) {
		t.Fatalf("nil nonce store error = %v, want configuration failure", err)
	}
}
