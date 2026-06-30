package core

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestCoreDoesNotImportUIOrExec enforces the AO-001 module seam: nothing under
// core/ may import the ui module or any exec concrete package. core depends on
// execution only through the runtime.Runtime interface (which lives in core).
func TestCoreDoesNotImportUIOrExec(t *testing.T) {
	const (
		uiPkg   = "github.com/tobsai/fort/ui"
		execPkg = "github.com/tobsai/fort/exec"
	)

	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		// The seam governs production code. Test files legitimately inject a
		// concrete runtime (exec/fake) to exercise the runtime.Runtime contract.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if p == uiPkg || strings.HasPrefix(p, uiPkg+"/") {
				t.Errorf("%s imports ui package %q (core must not depend on ui)", path, p)
			}
			if p == execPkg || strings.HasPrefix(p, execPkg+"/") {
				t.Errorf("%s imports exec package %q (core->exec only via runtime.Runtime interface)", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk core tree: %v", err)
	}
}
