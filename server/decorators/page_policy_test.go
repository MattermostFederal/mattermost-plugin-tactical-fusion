package decorators_test

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

func writtenPage(t *testing.T, p decorators.Page) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	decorators.WritePage(w, p)
	return w
}

// A page that carries no script gets a policy that cannot run one, so an
// escaping mistake there is inert markup rather than execution.
func TestPageWithoutScriptForbidsScript(t *testing.T) {
	w := writtenPage(t, decorators.Page{Title: "t", BodyHTML: "<p>b</p>"})

	if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'none'") {
		t.Fatalf("Content-Security-Policy = %q, want it to contain script-src 'none'", got)
	}
}

// A page with script is pinned to that script's digest, and never to
// 'unsafe-inline'.
//
// The difference is the whole point of the field. These pages echo author text
// from a message on a public route, so what has to survive an escaping mistake
// is that injected markup cannot execute. Under 'unsafe-inline' any <script>
// that got onto the page would run; under a hash only the one program whose
// digest is named can, and nothing an attacker adds will match it.
func TestPageWithScriptPinsItToItsDigest(t *testing.T) {
	const js = `console.log('x');`

	w := writtenPage(t, decorators.Page{Title: "t", BodyHTML: "<p>b</p>", ScriptJS: js})

	sum := sha256.Sum256([]byte(js))
	want := "script-src 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"

	got := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(got, want) {
		t.Fatalf("Content-Security-Policy = %q, want it to contain %q", got, want)
	}
	if strings.Contains(got, "script-src 'unsafe-inline'") {
		t.Fatalf("Content-Security-Policy = %q, want no unsafe-inline for script", got)
	}

	// And the script actually reaches the page, or the digest would be pinning
	// something nobody runs.
	if !strings.Contains(w.Body.String(), "<script>"+js+"</script>") {
		t.Fatal("the page does not carry the script its policy names")
	}
}

// The digest has to describe what is served. If the two ever drift, every
// browser blocks the script and the page silently loses its behavior.
func TestScriptDigestMatchesWhatIsServed(t *testing.T) {
	w := writtenPage(t, decorators.Page{Title: "t", BodyHTML: "<p>b</p>", ScriptJS: "var a = 1;"})

	body := w.Body.String()
	start := strings.Index(body, "<script>")
	end := strings.Index(body, "</script>")
	if start < 0 || end < start {
		t.Fatal("no script element in the page")
	}

	served := body[start+len("<script>") : end]
	sum := sha256.Sum256([]byte(served))
	want := "sha256-" + base64.StdEncoding.EncodeToString(sum[:])

	if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, want) {
		t.Fatalf("policy %q does not carry the digest %q of the script it serves", got, want)
	}
}

// A script that could close its own element would let whatever follows be
// parsed as markup, which is the one mistake that turns a script block into an
// injection point. Dropped whole, exactly as StyleCSS drops a "<".
func TestScriptThatClosesItsOwnElementIsDropped(t *testing.T) {
	w := writtenPage(t, decorators.Page{
		Title:    "t",
		BodyHTML: "<p>b</p>",
		ScriptJS: `var a = "</script><img src=x onerror=alert(1)>";`,
	})

	if body := w.Body.String(); strings.Contains(body, "onerror") {
		t.Fatalf("a script that closes its own element reached the page:\n%s", body)
	}
	if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'none'") {
		t.Fatalf("Content-Security-Policy = %q, want script-src 'none' once the script is dropped", got)
	}
}

// The error page never carries script, whatever the page that failed asked for.
func TestErrorPageForbidsScript(t *testing.T) {
	w := httptest.NewRecorder()
	decorators.WriteError(w, http.StatusBadRequest, "no")

	if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'none'") {
		t.Fatalf("Content-Security-Policy = %q, want it to contain script-src 'none'", got)
	}
}

func TestStyleCSSReachesTheStylesheet(t *testing.T) {
	w := writtenPage(t, decorators.Page{Title: "t", BodyHTML: "<p>b</p>", StyleCSS: ".loc { color: red; }"})

	body := w.Body.String()
	if !strings.Contains(body, ".loc { color: red; }") {
		t.Fatal("WritePage() dropped StyleCSS, want it in the stylesheet")
	}
	if i, j := strings.Index(body, ".loc {"), strings.Index(body, "</style>"); i > j {
		t.Fatal("WritePage() put StyleCSS after </style>, want it inside")
	}
}

// StyleCSS is documented as a source constant. A "<" in one could close the
// style element and start markup, so the value is dropped rather than escaped.
func TestStyleCSSCarryingMarkupIsDropped(t *testing.T) {
	w := writtenPage(t, decorators.Page{
		Title:    "t",
		BodyHTML: "<p>b</p>",
		StyleCSS: "</style><script>alert(1)</script>",
	})

	if body := w.Body.String(); strings.Contains(body, "alert(1)") {
		t.Fatal("WritePage() emitted markup from StyleCSS, want it dropped")
	}
}

// The two policies, side by side, so the difference between them is something a
// reader can see rather than reconstruct from a builder.
//
// PageStatic is what a page that only echoes text gets, and it is the default
// precisely because it gives nothing back. PageMapping is the one exception,
// and every directive in it is there for MapLibre: a real script file, a worker
// beside it, a fetch for the basemap and a canvas that draws through data: and
// blob: images. Widening either one should be a visible diff here.
func TestPageCapabilityDecidesTheWholePolicy(t *testing.T) {
	cases := []struct {
		name string
		page decorators.Page
		want string
	}{
		{
			name: "static, no script",
			page: decorators.Page{},
			want: "default-src 'none'; style-src 'unsafe-inline'; script-src 'none'; " +
				"base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
		},
		{
			name: "mapping",
			page: decorators.Page{Capability: decorators.PageMapping},
			want: "default-src 'none'; style-src 'unsafe-inline'; script-src 'self'; " +
				"worker-src 'self'; img-src data:; connect-src 'self'; " +
				"base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			decorators.WritePage(rec, c.page)

			if got := rec.Header().Get("Content-Security-Policy"); got != c.want {
				t.Errorf("policy =\n  %s\nwant\n  %s", got, c.want)
			}
		})
	}
}

// A mapping page that also carries an inline script keeps the digest beside
// 'self'. The two say different things: 'self' is permission to load MapLibre
// out of the bundle, the digest still names the only inline program that may
// run, and an escaping mistake has to survive the second one.
func TestMappingPageStillPinsItsInlineScript(t *testing.T) {
	rec := httptest.NewRecorder()
	decorators.WritePage(rec, decorators.Page{
		ScriptJS:   "console.log(1);",
		Capability: decorators.PageMapping,
	})

	sum := sha256.Sum256([]byte("console.log(1);"))
	want := "script-src 'self' 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"

	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, want) {
		t.Errorf("policy = %q, want it to contain %q", got, want)
	}
}

// A page that does not declare PageMapping cannot load a script, whatever it
// puts in the field: its own policy names no source to load one from, so
// emitting the tag would produce a page that is broken rather than obviously
// wrong. An absolute URL is refused for a different reason: this renderer
// cannot see the host it is serving, so same-origin is a claim nothing checked.
func TestAScriptNeedsThePolicyThatAllowsIt(t *testing.T) {
	cases := []struct {
		name string
		page decorators.Page
		want bool
	}{
		{"static", decorators.Page{ScriptSrc: "./page.js"}, false},
		{"mapping", decorators.Page{ScriptSrc: "./page.js", Capability: decorators.PageMapping}, true},
		{"one level up", decorators.Page{ScriptSrc: "../page.js", Capability: decorators.PageMapping}, true},
		{"absolute url", decorators.Page{ScriptSrc: "https://cdn.example/page.js", Capability: decorators.PageMapping}, false},
		{"protocol relative", decorators.Page{ScriptSrc: "//cdn.example/page.js", Capability: decorators.PageMapping}, false},
		{"root relative", decorators.Page{ScriptSrc: "/page.js", Capability: decorators.PageMapping}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			decorators.WritePage(rec, c.page)

			if got := strings.Contains(rec.Body.String(), `<script src=`); got != c.want {
				t.Errorf("script emitted = %v, want %v", got, c.want)
			}
		})
	}
}
