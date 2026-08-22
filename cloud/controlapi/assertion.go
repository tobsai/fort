package controlapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAssertionInvalid     = errors.New("service assertion invalid")
	ErrAssertionSignature   = errors.New("service assertion signature invalid")
	ErrAssertionAudience    = errors.New("service assertion audience mismatch")
	ErrAssertionRoute       = errors.New("service assertion route mismatch")
	ErrAssertionDigest      = errors.New("service assertion request digest mismatch")
	ErrAssertionExpired     = errors.New("service assertion expired")
	ErrAssertionNotYetValid = errors.New("service assertion not yet valid")
	ErrAssertionReplay      = errors.New("service assertion replayed")
	ErrAssertionNonceStore  = errors.New("service assertion nonce store unavailable")
)

const maximumAssertionBytes = 8 * 1024

var (
	assertionRoutePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
	assertionNoncePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32,256}$`)
)

// ServiceAssertion is the short-lived, server-to-server identity envelope used
// between fort-gateway and fort-control. AccountID is derived by the gateway
// from its authenticated owner session and is never accepted from a client.
type ServiceAssertion struct {
	KeyID         string
	AccountID     string
	RouteClass    string
	Audience      string
	RequestDigest string
	IssuedAt      time.Time
	ExpiresAt     time.Time
	Nonce         string
}

type assertionHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type assertionClaims struct {
	AccountID     string `json:"account_id"`
	RouteClass    string `json:"route_class"`
	Audience      string `json:"aud"`
	RequestDigest string `json:"request_digest"`
	IssuedAt      int64  `json:"iat"`
	ExpiresAt     int64  `json:"exp"`
	Nonce         string `json:"nonce"`
}

// NonceClaimer atomically records a nonce until the assertion expires. A
// production implementation must be durable and shared by all function
// instances; an in-memory implementation is not safe for deployment.
type NonceClaimer interface {
	Claim(context.Context, string, string, string, time.Time) (bool, error)
}

// ServiceAssertionVerifier verifies signed gateway assertions and consumes
// their nonces exactly once.
type ServiceAssertionVerifier struct {
	Audience  string
	Keys      map[string][]byte
	Clock     func() time.Time
	Nonces    NonceClaimer
	MaxTTL    time.Duration
	ClockSkew time.Duration
}

// IssueServiceAssertion creates a compact HS256 assertion. The assertion key
// must contain at least 256 bits of secret material.
func IssueServiceAssertion(key []byte, assertion ServiceAssertion) (string, error) {
	if len(key) < sha256.Size || !validAssertion(assertion) {
		return "", ErrAssertionInvalid
	}

	header, err := json.Marshal(assertionHeader{Algorithm: "HS256", KeyID: assertion.KeyID})
	if err != nil {
		return "", fmt.Errorf("%w: encode header", ErrAssertionInvalid)
	}
	claims, err := json.Marshal(assertionClaims{
		AccountID:     assertion.AccountID,
		RouteClass:    assertion.RouteClass,
		Audience:      assertion.Audience,
		RequestDigest: assertion.RequestDigest,
		IssuedAt:      assertion.IssuedAt.Unix(),
		ExpiresAt:     assertion.ExpiresAt.Unix(),
		Nonce:         assertion.Nonce,
	})
	if err != nil {
		return "", fmt.Errorf("%w: encode claims", ErrAssertionInvalid)
	}

	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// Verify authenticates an assertion against the exact request route class and
// SHA-256 body digest, then atomically consumes its nonce.
func (verifier ServiceAssertionVerifier) Verify(
	ctx context.Context,
	token string,
	routeClass string,
	requestDigest string,
) (ServiceAssertion, error) {
	if len(token) == 0 || len(token) > maximumAssertionBytes {
		return ServiceAssertion{}, ErrAssertionInvalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ServiceAssertion{}, ErrAssertionInvalid
	}

	var header assertionHeader
	if err := decodeAssertionPart(parts[0], &header); err != nil || header.Algorithm != "HS256" || header.KeyID == "" {
		return ServiceAssertion{}, ErrAssertionInvalid
	}
	key, ok := verifier.Keys[header.KeyID]
	if !ok || len(key) < sha256.Size {
		return ServiceAssertion{}, ErrAssertionSignature
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != sha256.Size || base64.RawURLEncoding.EncodeToString(signature) != parts[2] {
		return ServiceAssertion{}, ErrAssertionSignature
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return ServiceAssertion{}, ErrAssertionSignature
	}

	var claims assertionClaims
	if err := decodeAssertionPart(parts[1], &claims); err != nil {
		return ServiceAssertion{}, ErrAssertionInvalid
	}
	assertion := ServiceAssertion{
		KeyID:         header.KeyID,
		AccountID:     claims.AccountID,
		RouteClass:    claims.RouteClass,
		Audience:      claims.Audience,
		RequestDigest: claims.RequestDigest,
		IssuedAt:      time.Unix(claims.IssuedAt, 0).UTC(),
		ExpiresAt:     time.Unix(claims.ExpiresAt, 0).UTC(),
		Nonce:         claims.Nonce,
	}
	if !validAssertion(assertion) {
		return ServiceAssertion{}, ErrAssertionInvalid
	}
	if assertion.Audience != verifier.Audience {
		return ServiceAssertion{}, ErrAssertionAudience
	}
	if assertion.RouteClass != routeClass {
		return ServiceAssertion{}, ErrAssertionRoute
	}
	wantDigest, digestErr := hex.DecodeString(requestDigest)
	gotDigest, assertionDigestErr := hex.DecodeString(assertion.RequestDigest)
	if digestErr != nil || assertionDigestErr != nil || len(wantDigest) != sha256.Size ||
		subtle.ConstantTimeCompare(gotDigest, wantDigest) != 1 {
		return ServiceAssertion{}, ErrAssertionDigest
	}

	now := time.Now().UTC()
	if verifier.Clock != nil {
		now = verifier.Clock().UTC()
	}
	if verifier.MaxTTL <= 0 || assertion.ExpiresAt.Sub(assertion.IssuedAt) > verifier.MaxTTL {
		return ServiceAssertion{}, ErrAssertionInvalid
	}
	if assertion.IssuedAt.After(now.Add(verifier.ClockSkew)) {
		return ServiceAssertion{}, ErrAssertionNotYetValid
	}
	if !now.Before(assertion.ExpiresAt.Add(verifier.ClockSkew)) {
		return ServiceAssertion{}, ErrAssertionExpired
	}
	if verifier.Nonces == nil {
		return ServiceAssertion{}, ErrAssertionNonceStore
	}
	claimed, err := verifier.Nonces.Claim(
		ctx,
		assertion.AccountID,
		assertion.KeyID,
		assertion.Nonce,
		assertion.ExpiresAt,
	)
	if err != nil {
		return ServiceAssertion{}, fmt.Errorf("%w: %v", ErrAssertionNonceStore, err)
	}
	if !claimed {
		return ServiceAssertion{}, ErrAssertionReplay
	}
	return assertion, nil
}

func validAssertion(assertion ServiceAssertion) bool {
	if !assertionKeyIDPattern.MatchString(assertion.KeyID) ||
		!assertionRoutePattern.MatchString(assertion.RouteClass) ||
		!assertionRoutePattern.MatchString(assertion.Audience) ||
		!assertionNoncePattern.MatchString(assertion.Nonce) {
		return false
	}
	accountID, err := uuid.Parse(assertion.AccountID)
	if err != nil || accountID.String() != assertion.AccountID {
		return false
	}
	digest, err := hex.DecodeString(assertion.RequestDigest)
	if err != nil || len(digest) != sha256.Size || hex.EncodeToString(digest) != assertion.RequestDigest {
		return false
	}
	return !assertion.IssuedAt.IsZero() && assertion.ExpiresAt.After(assertion.IssuedAt)
}

func decodeAssertionPart(part string, destination any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrAssertionInvalid
	}
	return nil
}
