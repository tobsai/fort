package ui

// md() is the security-critical XSS sink for markdown bodies (spec 029), so it
// is executed for real: the /* md:start */../* md:end */ region of boardHTML is
// extracted and run under goja (pure-Go JS engine, test-only dependency).

import (
	"strings"
	"testing"

	"github.com/dop251/goja"
)

// mdFn extracts and evaluates the marker region, returning a callable md().
func mdFn(t *testing.T) func(string) string {
	t.Helper()
	i := strings.Index(boardHTML, "/* md:start")
	j := strings.Index(boardHTML, "/* md:end */")
	if i < 0 || j < i {
		t.Fatal("md:start/md:end markers missing from boardHTML")
	}
	vm := goja.New()
	if _, err := vm.RunString(boardHTML[i:j]); err != nil {
		t.Fatalf("eval md region: %v", err)
	}
	fn, ok := goja.AssertFunction(vm.Get("md"))
	if !ok {
		t.Fatal("md is not a function in the marker region")
	}
	return func(src string) string {
		v, err := fn(goja.Undefined(), vm.ToValue(src))
		if err != nil {
			t.Fatalf("md(%q): %v", src, err)
		}
		return v.String()
	}
}

func TestMdRendersSubset(t *testing.T) {
	md := mdFn(t)
	cases := []struct{ name, in, want string }{
		{"heading", "# Title", "<h3>Title</h3>"},
		{"heading-cap", "###### deep", "<h6>deep</h6>"},
		{"bold", "**b**", "<strong>b</strong>"},
		{"italic", "*i*", "<em>i</em>"},
		{"inline-code", "`x<y`", "<code>x&lt;y</code>"},
		{"link", "[t](https://x.dev)", `<a href="https://x.dev" rel="noopener nofollow" target="_blank">t</a>`},
		{"ul", "- a\n- b", "<ul><li>a</li><li>b</li></ul>"},
		{"ol", "1. a\n2. b", "<ol><li>a</li><li>b</li></ol>"},
		{"para-br", "l1\nl2", "<p>l1<br>l2</p>"},
	}
	for _, c := range cases {
		if got := md(c.in); !strings.Contains(got, c.want) {
			t.Errorf("%s: md(%q) = %q, want contains %q", c.name, c.in, got, c.want)
		}
	}
	// fenced code: content held verbatim (escaped), not formatted
	got := md("```go\n**not bold** <tag>\n```")
	if !strings.Contains(got, "<pre><code>") || strings.Contains(got, "<strong>") || strings.Contains(got, "<tag>") {
		t.Errorf("fence: %q", got)
	}
	// plain text passes through escaped, unformatted
	if got := md("just plain text"); !strings.Contains(got, "just plain text") || strings.Contains(got, "<h") {
		t.Errorf("plain: %q", got)
	}
	// empty in, empty out
	if got := md("  \n "); got != "" {
		t.Errorf("blank body should render empty, got %q", got)
	}
}
