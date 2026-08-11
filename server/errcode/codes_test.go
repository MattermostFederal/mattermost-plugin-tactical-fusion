package errcode

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A duplicated code means two unrelated failures answer to the same identifier,
// which makes every support ticket quoting it ambiguous.
func TestCodesUnique(t *testing.T) {
	seen := map[int]bool{}
	for _, code := range AllCodes {
		if seen[code] {
			t.Errorf("code %d appears more than once in AllCodes", code)
		}
		seen[code] = true
	}
}

// Every code must be in range for the file it belongs to. A code allocated out
// of range still works, but it makes the allocation table in the package
// documentation a lie, and that table is the only thing telling the next person
// which number to take next.
func TestCodesAreInAKnownRange(t *testing.T) {
	for _, code := range AllCodes {
		if code < 10000 || code >= 18000 {
			t.Errorf("code %d is outside every range the package documents", code)
		}
	}
}

// AllCodes is what the documentation test and the uniqueness test both iterate,
// so a constant missing from it is invisible to every other check here. Parsing
// the source is the only way to notice: nothing at runtime can see a constant
// that is never mentioned.
func TestAllCodesComplete(t *testing.T) {
	declared, listed := parseCodesFile(t)

	if len(declared) == 0 {
		t.Fatal("parsed no constants out of codes.go; the parser is looking in the wrong place")
	}
	if len(listed) == 0 {
		t.Fatal("found no AllCodes slice literal in codes.go; the parser is looking in the wrong place")
	}

	for name := range declared {
		if !listed[name] {
			t.Errorf("constant %s is declared but missing from AllCodes", name)
		}
	}
	for name := range listed {
		if !declared[name] {
			t.Errorf("AllCodes names %s, which is not declared in codes.go", name)
		}
	}
}

// parseCodesFile returns the constants declared in codes.go and the identifiers
// named in the AllCodes literal.
func parseCodesFile(t *testing.T) (declared, listed map[string]bool) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed; cannot locate codes.go")
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(filepath.Dir(thisFile), "codes.go"), nil, 0)
	if err != nil {
		t.Fatalf("failed to parse codes.go: %v", err)
	}

	declared = map[string]bool{}
	listed = map[string]bool{}

	for _, decl := range file.Decls {
		gen, isGen := decl.(*ast.GenDecl)
		if !isGen {
			continue
		}

		for _, spec := range gen.Specs {
			value, isValue := spec.(*ast.ValueSpec)
			if !isValue {
				continue
			}

			switch gen.Tok {
			case token.CONST:
				for _, name := range value.Names {
					declared[name.Name] = true
				}
			case token.VAR:
				for i, name := range value.Names {
					if name.Name != "AllCodes" || i >= len(value.Values) {
						continue
					}
					composite, isComposite := value.Values[i].(*ast.CompositeLit)
					if !isComposite {
						continue
					}
					for _, elt := range composite.Elts {
						if ident, isIdent := elt.(*ast.Ident); isIdent {
							listed[ident.Name] = true
						}
					}
				}
			}
		}
	}

	return declared, listed
}

// The suffix format is what support tooling greps for, so it is pinned rather
// than left to whatever fmt happens to produce.
func TestWithCode(t *testing.T) {
	got := WithCode(APINotAuthorized, "Not authorized.")
	want := "Not authorized. (TF-13000)"

	if got != want {
		t.Fatalf("WithCode = %q, want %q", got, want)
	}
}

func TestErrorf(t *testing.T) {
	err := Errorf(PreferencesZoneIDUnknown, "unknown timezone %q", "Mars/Olympus")

	want := `unknown timezone "Mars/Olympus" (TF-14004)`
	if err.Error() != want {
		t.Fatalf("Errorf = %q, want %q", err.Error(), want)
	}
}

// A code rendered without the prefix would not be findable by the one search an
// operator knows to run.
func TestEveryRenderedCodeCarriesThePrefix(t *testing.T) {
	for _, code := range AllCodes {
		if rendered := WithCode(code, "x"); !strings.Contains(rendered, "("+Prefix+"-") {
			t.Fatalf("WithCode(%d, ...) = %q, which does not carry the %s- prefix", code, rendered, Prefix)
		}
	}
}
