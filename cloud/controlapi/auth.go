package controlapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
)

const (
	// MaximumFunctionBodyBytes is Fort's normative encoded request/response
	// limit, kept below Vercel's platform limit.
	MaximumFunctionBodyBytes = 4 << 20
	ServiceAssertionHeader   = "X-Fort-Service-Assertion"
)

type accountIDContextKey struct{}

// AccountIDFromContext returns the account authenticated by the signed
// fort-gateway assertion. Handlers must not read account identity from input.
func AccountIDFromContext(ctx context.Context) (string, bool) {
	accountID, ok := ctx.Value(accountIDContextKey{}).(string)
	return accountID, ok && accountID != ""
}

// RequireServiceAssertion bounds and digests the exact request body, verifies
// the gateway assertion, and injects only its signed account into context.
func RequireServiceAssertion(
	verifier ServiceAssertionVerifier,
	routeClass string,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		limited := http.MaxBytesReader(response, request.Body, MaximumFunctionBodyBytes)
		body, err := io.ReadAll(limited)
		if err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				writeJSON(response, http.StatusRequestEntityTooLarge, map[string]string{"code": "payload_limit"})
				return
			}
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "request_body_invalid"})
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		request.ContentLength = int64(len(body))

		digestBytes := sha256.Sum256(body)
		digest := hex.EncodeToString(digestBytes[:])
		token := strings.TrimSpace(request.Header.Get(ServiceAssertionHeader))
		if token == "" {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"code": "service_assertion_required"})
			return
		}
		assertion, err := verifier.Verify(request.Context(), token, routeClass, digest)
		if err != nil {
			if errors.Is(err, ErrAssertionNonceStore) {
				writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "service_assertion_unavailable"})
				return
			}
			writeJSON(response, http.StatusUnauthorized, map[string]string{"code": "service_assertion_invalid"})
			return
		}

		request.Header.Del("X-Fort-Account-ID")
		ctx := context.WithValue(request.Context(), accountIDContextKey{}, assertion.AccountID)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}
