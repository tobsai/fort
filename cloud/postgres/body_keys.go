package postgres

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/tobsai/fort/cloud/securebody"
)

const (
	BodyKeysEnvironment      = "FORT_BODY_KEYS_JSON"
	BodyActiveKeyEnvironment = "FORT_BODY_ACTIVE_KID"
)

var bodyKeyIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// BodyKeyRingFromEnvironment parses the server-only AEAD key ring without
// accepting duplicate key IDs, padded/noncanonical base64url, or an active ID
// that is absent from the retained decryption set.
func BodyKeyRingFromEnvironment(getenv func(string) string) (securebody.KeyRing, error) {
	if getenv == nil {
		return securebody.KeyRing{}, fmt.Errorf("body key ring environment is unavailable")
	}
	activeKeyID := getenv(BodyActiveKeyEnvironment)
	if !bodyKeyIDPattern.MatchString(activeKeyID) {
		return securebody.KeyRing{}, fmt.Errorf("body key ring active key id is missing or noncanonical")
	}
	raw := strings.TrimSpace(getenv(BodyKeysEnvironment))
	if raw == "" {
		return securebody.KeyRing{}, fmt.Errorf("body key ring JSON is missing")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return securebody.KeyRing{}, fmt.Errorf("body key ring must be a JSON object")
	}
	keys := make(map[string][]byte)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return securebody.KeyRing{}, fmt.Errorf("body key ring key id: %w", err)
		}
		keyID, ok := token.(string)
		if !ok || !bodyKeyIDPattern.MatchString(keyID) {
			return securebody.KeyRing{}, fmt.Errorf("body key ring contains a noncanonical key id")
		}
		if _, duplicate := keys[keyID]; duplicate {
			return securebody.KeyRing{}, fmt.Errorf("body key ring contains duplicate key id %q", keyID)
		}
		var encoded string
		if err := decoder.Decode(&encoded); err != nil {
			return securebody.KeyRing{}, fmt.Errorf("body key ring key %q must be a string", keyID)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != encoded || len(decoded) != 32 {
			return securebody.KeyRing{}, fmt.Errorf("body key ring key %q must be canonical base64url for 32 bytes", keyID)
		}
		keys[keyID] = append([]byte(nil), decoded...)
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return securebody.KeyRing{}, fmt.Errorf("body key ring JSON object is incomplete")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return securebody.KeyRing{}, fmt.Errorf("body key ring JSON must contain exactly one object")
	}
	if _, ok := keys[activeKeyID]; !ok {
		return securebody.KeyRing{}, fmt.Errorf("body key ring active key %q is unknown", activeKeyID)
	}
	return securebody.KeyRing{ActiveKeyID: activeKeyID, Keys: keys}, nil
}

func cloneBodyKeyRing(ring securebody.KeyRing) (securebody.KeyRing, error) {
	if !bodyKeyIDPattern.MatchString(ring.ActiveKeyID) || len(ring.Keys) == 0 {
		return securebody.KeyRing{}, fmt.Errorf("body key ring is invalid")
	}
	keys := make(map[string][]byte, len(ring.Keys))
	for keyID, key := range ring.Keys {
		if !bodyKeyIDPattern.MatchString(keyID) || len(key) != 32 {
			return securebody.KeyRing{}, fmt.Errorf("body key ring key %q is invalid", keyID)
		}
		keys[keyID] = append([]byte(nil), key...)
	}
	if _, ok := keys[ring.ActiveKeyID]; !ok {
		return securebody.KeyRing{}, fmt.Errorf("body key ring active key %q is unknown", ring.ActiveKeyID)
	}
	return securebody.KeyRing{ActiveKeyID: ring.ActiveKeyID, Keys: keys, Random: ring.Random}, nil
}
