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
// browser blocks the script and the page silently loses its behaviour.
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
