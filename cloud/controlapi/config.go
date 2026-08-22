package controlapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

var ErrAssertionConfiguration = errors.New("service assertion configuration invalid")

const (
	assertionKeyringEnvironment = "FORT_CONTROL_ASSERTION_KEYS_JSON"
	maximumKeyringBytes         = 64 << 10
)

var assertionKeyIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// ServiceAssertionVerifierFromEnvironment loads the control-side verification
// key ring. The gateway carries one active signing key, while control may
// authenticate multiple explicitly configured keys during a bounded rotation.
func ServiceAssertionVerifierFromEnvironment(
	getenv func(string) string,
	nonces NonceClaimer,
) (ServiceAssertionVerifier, error) {
	if getenv == nil || nonces == nil {
		return ServiceAssertionVerifier{}, ErrAssertionConfiguration
	}
	raw := strings.TrimSpace(getenv(assertionKeyringEnvironment))
	if raw == "" || len(raw) > maximumKeyringBytes {
		return ServiceAssertionVerifier{}, ErrAssertionConfiguration
	}
	encodedKeys, err := decodeStringMap(raw)
	if err != nil || len(encodedKeys) == 0 {
		return ServiceAssertionVerifier{}, ErrAssertionConfiguration
	}

	keys := make(map[string][]byte, len(encodedKeys))
	for keyID, encoded := range encodedKeys {
		if !assertionKeyIDPattern.MatchString(keyID) || encoded == "" || strings.Contains(encoded, "=") {
			return ServiceAssertionVerifier{}, ErrAssertionConfiguration
		}
		key, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || len(key) < 32 || base64.RawURLEncoding.EncodeToString(key) != encoded {
			return ServiceAssertionVerifier{}, ErrAssertionConfiguration
		}
		keys[keyID] = key
	}

	return ServiceAssertionVerifier{
		Audience:  "fort-control",
		Keys:      keys,
		Nonces:    nonces,
		MaxTTL:    time.Minute,
		ClockSkew: 5 * time.Second,
	}, nil
}

func decodeStringMap(raw string) (map[string]string, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("%w: key ring must be an object", ErrAssertionConfiguration)
	}
	values := make(map[string]string)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, ErrAssertionConfiguration
		}
		if _, duplicate := values[key]; duplicate {
			return nil, ErrAssertionConfiguration
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		values[key] = value
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, ErrAssertionConfiguration
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, ErrAssertionConfiguration
	}
	return values, nil
}
