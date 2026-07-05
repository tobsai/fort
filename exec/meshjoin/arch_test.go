package meshjoin

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestMeshjoinTakesNoModelCalls guards the spec-024 determinism invariant:
// enrollment (invite/join/remove) is plain CRUD and must never reach a provider
// runtime. Production meshjoin code touches execution only through the transport
// seams (exec/cluster, exec/remote) — it may not import a concrete agent runtime
// (exec/native) or the fake (exec/fake, test-only), so no code path here can make
// a model call. Enforced statically instead of relying on a manual `go list -deps`.
func TestMeshjoinTakesNoModelCalls(t *testing.T) {
	banned := []string{
		"github.com/tobsai/fort/exec/native",
		"github.com/tobsai/fort/exec/fake",
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		// The seam governs production code; test files legitimately use exec/fake.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, b := range banned {
				if p == b {
					t.Errorf("%s imports %q — meshjoin enrollment must take zero model calls (spec 024)", path, p)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk meshjoin tree: %v", err)
	}
}
