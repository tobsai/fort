package ui

import (
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

func TestUIPortsDoNotImportPersistenceTypes(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "ports.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if path == "github.com/tobsai/fort/core/store" {
			t.Fatal("ui ports expose persistence types; Spec 041 requires bounded wire types")
		}
	}
}
