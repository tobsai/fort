package postgres

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/tobsai/fort/cloud/securebody"
)

func TestBodyKeyRingFromEnvironmentRetainsCanonicalActiveAndOldKeys(t *testing.T) {
	t.Parallel()
	active := []byte("0123456789abcdef0123456789abcdef")
	old := []byte("abcdef0123456789abcdef0123456789")
	values := map[string]string{
		"FORT_BODY_ACTIVE_KID": "body-2026-08",
		"FORT_BODY_KEYS_JSON": `{"body-2026-08":"` + base64.RawURLEncoding.EncodeToString(active) +
			`","body-2026-07":"` + base64.RawURLEncoding.EncodeToString(old) + `"}`,
	}
	ring, err := BodyKeyRingFromEnvironment(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("BodyKeyRingFromEnvironment: %v", err)
	}
	if ring.ActiveKeyID != "body-2026-08" || len(ring.Keys) != 2 ||
		string(ring.Keys[ring.ActiveKeyID]) != string(active) || string(ring.Keys["body-2026-07"]) != string(old) {
		t.Fatalf("body key ring = %#v", ring)
	}
}

func TestBodyKeyRingFromEnvironmentRejectsAmbiguousOrNoncanonicalConfiguration(t *testing.T) {
	t.Parallel()
	key := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	short := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcde"))
	for name, values := range map[string][2]string{
		"missing active":       {"", `{"body-1":"` + key + `"}`},
		"unknown active":       {"body-2", `{"body-1":"` + key + `"}`},
		"duplicate key id":     {"body-1", `{"body-1":"` + key + `","body-1":"` + key + `"}`},
		"padded base64":        {"body-1", `{"body-1":"` + key + `="}`},
		"wrong key length":     {"body-1", `{"body-1":"` + short + `"}`},
		"noncanonical key id":  {" body-1", `{"body-1":"` + key + `"}`},
		"unknown object shape": {"body-1", `{"body-1":{"key":"` + key + `"}}`},
	} {
		active, keys := values[0], values[1]
		t.Run(name, func(t *testing.T) {
			_, err := BodyKeyRingFromEnvironment(func(environmentKey string) string {
				switch environmentKey {
				case "FORT_BODY_ACTIVE_KID":
					return active
				case "FORT_BODY_KEYS_JSON":
					return keys
				default:
					return ""
				}
			})
			if err == nil || !strings.Contains(err.Error(), "body key ring") {
				t.Fatalf("configuration error = %v", err)
			}
		})
	}
}

func TestSharedPoolWithKeyRingClonesKeysForEveryAccountStore(t *testing.T) {
	t.Parallel()
	original := securebody.KeyRing{
		ActiveKeyID: "body-1",
		Keys:        map[string][]byte{"body-1": []byte("0123456789abcdef0123456789abcdef")},
	}
	pool, err := newSharedPoolWithKeyRing(&fakeDatabase{}, original)
	if err != nil {
		t.Fatalf("newSharedPoolWithKeyRing: %v", err)
	}
	original.Keys["body-1"][0] = 'x'
	first, err := pool.ForAccount(testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	firstCipher, ok := first.bodyCipher.(secureCollaborationBodyCipher)
	if !ok || firstCipher.ring.Keys["body-1"][0] != '0' {
		t.Fatalf("first account Store did not receive a cloned key ring: %#v", first.bodyCipher)
	}
	firstCipher.ring.Keys["body-1"][0] = 'y'
	second, err := pool.ForAccount(testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	secondCipher, ok := second.bodyCipher.(secureCollaborationBodyCipher)
	if !ok || secondCipher.ring.Keys["body-1"][0] != '0' {
		t.Fatalf("account Stores share mutable key bytes: %#v", second.bodyCipher)
	}
}
