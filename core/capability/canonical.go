package capability

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"unicode/utf16"
)

// canonicalJSON implements the RFC 8785 rules needed by Fort's closed control
// objects. Capability contracts contain integers but no floating-point values;
// floats are rejected rather than risking a non-ECMAScript representation.
func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var normalized any
	if err := dec.Decode(&normalized); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, normalized); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeCanonical(out *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		if value {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case string:
		encoded, _ := json.Marshal(value)
		encoded = bytes.ReplaceAll(encoded, []byte(`\u003c`), []byte("<"))
		encoded = bytes.ReplaceAll(encoded, []byte(`\u003e`), []byte(">"))
		encoded = bytes.ReplaceAll(encoded, []byte(`\u0026`), []byte("&"))
		out.Write(encoded)
	case json.Number:
		if _, err := strconv.ParseInt(value.String(), 10, 64); err != nil {
			if _, err := strconv.ParseUint(value.String(), 10, 64); err != nil {
				return fmt.Errorf("capability: non-integer number %q is not allowed", value)
			}
		}
		out.WriteString(value.String())
	case []any:
		out.WriteByte('[')
		for i, item := range value {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return utf16Less(keys[i], keys[j])
		})
		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, key); err != nil {
				return err
			}
			out.WriteByte(':')
			if err := writeCanonical(out, value[key]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return fmt.Errorf("capability: unsupported canonical JSON type %s", reflect.TypeOf(value))
	}
	return nil
}

func utf16Less(a, b string) bool {
	aa, bb := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(aa) && i < len(bb); i++ {
		if aa[i] != bb[i] {
			return aa[i] < bb[i]
		}
	}
	return len(aa) < len(bb)
}

func controlHash(domain string, value any) (string, error) {
	sum, err := rawControlHash(domain, value)
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func rawControlHash(domain string, value any) ([32]byte, error) {
	canonical, err := canonicalJSON(value)
	if err != nil {
		return [32]byte{}, err
	}
	hash := sha256.New()
	hash.Write([]byte(domain))
	hash.Write([]byte{0})
	hash.Write(canonical)
	var sum [32]byte
	copy(sum[:], hash.Sum(nil))
	return sum, nil
}

func shortContentID(domain, prefix string, value any) (string, error) {
	sum, err := rawControlHash(domain, value)
	if err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(sum[:16]), nil
}

// OpaqueRevision derives one node-local proof from stable, non-secret
// invocation semantics. Callers must exclude paths, token bytes/expiry, raw
// probe output, and timestamps before calling.
func OpaqueRevision(key []byte, domain string, value any) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("capability: revision key must contain 32 bytes")
	}
	canonical, err := canonicalJSON(value)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(domain))
	mac.Write([]byte{0})
	mac.Write(canonical)
	return "opaque:" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
