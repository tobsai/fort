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

func TestMdSecurityCorpus(t *testing.T) {
	md := mdFn(t)
	// every payload must render inert: no executable markup, no non-http(s) href
	payloads := []string{
		`<script>alert(1)</script>`,
		`<img src=x onerror=alert(1)>`,
		`<svg onload=alert(1)>`,
		`[x](javascript:alert(1))`,
		`[x](data:text/html,<script>alert(1)</script>)`,
		`[x](vbscript:msgbox)`,
		`"><img src=x onerror=alert(1)>`,
		`**<iframe src=//evil>**`,
		"`</code><script>alert(1)</script>`",
		"```\n</pre><script>alert(1)</script>\n```",
		`[<script>x</script>](https://ok.dev)`,
		`# <script>h</script>`,
		`- <img src=x onerror=1>`,
		" " + `0` + " ", // placeholder forgery
		`[x](https://ok.dev" onmouseover="alert(1))`,
	}
	// NOTE (deviation from the plan's literal blocklist): the plan's inner loop
	// also checked bare "onerror=", "onload=", "onmouseover=", "javascript:",
	// "data:text", "vbscript:" with no requirement that they sit inside live
	// markup. Those bare words are expected to survive as literal escaped text
	// per the plan's own note ("javascript:/data: links ... stay literal
	// escaped text") — checking for them unconditionally makes every such
	// payload a false failure even though nothing executable renders (verified:
	// no unescaped tag, no href starting with a dangerous scheme, no attribute
	// breakout). Narrowed to the tag-prefixed markers, which do detect a real
	// escaping failure, plus an explicit live-href-scheme check below.
	for _, p := range payloads {
		out := md(p)
		lower := strings.ToLower(out)
		for _, bad := range []string{"<script", "<img", "<svg", "<iframe"} {
			if strings.Contains(lower, bad) {
				t.Errorf("payload %q rendered executable content: %q", p, out)
			}
		}
		for _, bad := range []string{`href="javascript`, `href="data`, `href="vbscript`} {
			if strings.Contains(lower, bad) {
				t.Errorf("payload %q produced a live dangerous-scheme href: %q", p, out)
			}
		}
	}
	// the attribute-injection URL must not produce a second attribute:
	// esc() turned the quote into &quot; so the href stays one attribute.
	out := md(`[x](https://ok.dev" onmouseover="alert(1))`)
	if strings.Contains(out, `" onmouseover="`) {
		t.Errorf("attribute breakout: %q", out)
	}
	// links that ARE allowed still work
	if ok := md(`[t](https://x.dev)`); !strings.Contains(ok, `href="https://x.dev"`) {
		t.Errorf("https link should render: %q", ok)
	}
}
