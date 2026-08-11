package decorators_test

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-mission-context/server/decorators"
)

// fixtureDecorator is defined entirely in the test package. It is the only real
// proof of the "adding a decorator needs no framework edit" claim: if this
// compiles and works, a new decorator does too.
type fixtureDecorator struct {
	typ      string
	patterns []decorators.Pattern

	// reject makes Parse return ok=false for this exact value.
	reject string
}

func (f *fixtureDecorator) Type() string { return f.typ }

func (f *fixtureDecorator) Patterns() []decorators.Pattern { return f.patterns }

func (f *fixtureDecorator) Parse(value string, _ time.Time) (url.Values, bool) {
	if value == f.reject {
		return nil, false
	}
	return url.Values{"v": {value}}, true
}

func (f *fixtureDecorator) RenderPage(w http.ResponseWriter, params url.Values) {
	decorators.WritePage(w, decorators.Page{Title: "fixture", BodyHTML: params.Get("v")})
}

func newFixture(typ, pattern string) *fixtureDecorator {
	return &fixtureDecorator{
		typ:      typ,
		patterns: []decorators.Pattern{{Regexp: regexp.MustCompile(pattern)}},
	}
}

func TestRegistryPreservesRegistrationOrder(t *testing.T) {
	r := decorators.NewRegistry()
	first := newFixture("first", `\bAAA\b`)
	second := newFixture("second", `\bBBB\b`)

	mustRegister(t, r, first)
	mustRegister(t, r, second)

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d decorators, want 2", len(all))
	}
	if all[0].Type() != "first" || all[1].Type() != "second" {
		t.Fatalf("All() = [%s %s], want [first second]", all[0].Type(), all[1].Type())
	}
}

func TestRegistryRejectsDuplicateType(t *testing.T) {
	r := decorators.NewRegistry()
	mustRegister(t, r, newFixture("dup", `\bAAA\b`))

	if err := r.Register(newFixture("dup", `\bBBB\b`)); err == nil {
		t.Fatal("Register() accepted a duplicate type, want an error")
	}
}

func TestRegistryRejectsEmptyType(t *testing.T) {
	r := decorators.NewRegistry()

	if err := r.Register(newFixture("", `\bAAA\b`)); err == nil {
		t.Fatal("Register() accepted an empty type, want an error")
	}
}

func TestRegistryGetMissReturnsNil(t *testing.T) {
	r := decorators.NewRegistry()
	mustRegister(t, r, newFixture("known", `\bAAA\b`))

	if got := r.Get("unknown"); got != nil {
		t.Fatalf("Get(unknown) = %v, want nil", got)
	}
	if got := r.Get("known"); got == nil {
		t.Fatal("Get(known) = nil, want the registered decorator")
	}
}

func mustRegister(t *testing.T, r *decorators.Registry, d decorators.Decorator) {
	t.Helper()
	if err := r.Register(d); err != nil {
		t.Fatalf("Register(%s) failed: %v", d.Type(), err)
	}
}
