package canonicaljson_test

import (
	"encoding/json"
	"testing"

	"github.com/tobsai/fort/core/canonicaljson"
)

func TestMarshalUsesRFC8785StringAndUTF16KeyRules(t *testing.T) {
	encoded, err := canonicaljson.Marshal(map[string]any{
		"z":          "<&>",
		"a":          []any{1, true},
		"\uE000":     "bmp",
		"\U00010000": "astral",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":[1,true],"z":"<&>","𐀀":"astral","":"bmp"}`
	if string(encoded) != want {
		t.Fatalf("canonical JSON = %s, want %s", encoded, want)
	}
}

func TestMarshalKeepsRFC8785LineSeparatorLiteral(t *testing.T) {
	encoded, err := canonicaljson.Marshal(map[string]any{
		"name": "line\u2028break",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"name\":\"line\u2028break\"}"
	if string(encoded) != want {
		t.Fatalf("canonical JSON = %q, want %q", encoded, want)
	}
}

func TestMarshalKeepsRFC8785ParagraphSeparatorLiteral(t *testing.T) {
	encoded, err := canonicaljson.Marshal(map[string]any{
		"name": "paragraph\u2029break",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"name\":\"paragraph\u2029break\"}"
	if string(encoded) != want {
		t.Fatalf("canonical JSON = %q, want %q", encoded, want)
	}
}

func TestMarshalPreservesLiteralEscapeLikeTextBesideUnicodeSeparators(t *testing.T) {
	wantValue := "slash:\\u2028 less:\\u003c actual:\u2028\u2029"
	encoded, err := canonicaljson.Marshal(map[string]any{"value": wantValue})
	if err != nil {
		t.Fatal(err)
	}
	wantJSON := "{\"value\":\"slash:\\\\u2028 less:\\\\u003c actual:\u2028\u2029\"}"
	if string(encoded) != wantJSON {
		t.Fatalf("canonical JSON = %q, want %q", encoded, wantJSON)
	}
	var decoded map[string]string
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("canonical JSON is unreadable: %v", err)
	}
	if decoded["value"] != wantValue {
		t.Fatalf("round-trip value = %q, want %q", decoded["value"], wantValue)
	}
}

func TestMarshalRejectsFloatingPointControlValues(t *testing.T) {
	if _, err := canonicaljson.Marshal(map[string]any{"fraction": 1.5}); err == nil {
		t.Fatal("canonical JSON accepted a floating-point control value")
	}
}
