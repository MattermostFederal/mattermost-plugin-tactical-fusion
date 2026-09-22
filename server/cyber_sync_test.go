package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
)

func cyberWebappSource(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "webapp", "src", "decorators", "cyber", name)
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return string(source)
}

func cyberWebappInterface(t *testing.T, name string) string {
	t.Helper()

	source := cyberWebappSource(t, "types.ts")

	found := regexp.MustCompile(`(?s)export interface ` + name + ` \{(.*?)\n\}`).FindStringSubmatch(source)
	if found == nil {
		t.Fatalf("webapp/src/decorators/cyber/types.ts declares no interface %s; "+
			"point this test at the new name rather than deleting it", name)
	}

	return found[1]
}

var cyberFieldRe = regexp.MustCompile(`(?m)^\s+(\w+)\??:\s*([\w\[\]]+);`)

type webappField struct {
	name string
	kind string
}

func cyberWebappFields(t *testing.T, name string) []webappField {
	t.Helper()

	var fields []webappField
	for _, match := range cyberFieldRe.FindAllStringSubmatch(cyberWebappInterface(t, name), -1) {
		fields = append(fields, webappField{name: match[1], kind: match[2]})
	}
	if len(fields) == 0 {
		t.Fatalf("%s declares no fields", name)
	}

	return fields
}

func goWireFields(t *testing.T, value any) []webappField {
	t.Helper()

	typ := reflect.TypeOf(value)

	var fields []webappField
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			t.Fatalf("%s.%s carries no json tag", typ.Name(), typ.Field(i).Name)
		}
		fields = append(fields, webappField{
			name: strings.Split(tag, ",")[0],
			kind: wireKind(typ.Field(i).Type),
		})
	}

	return fields
}

func wireKind(typ reflect.Type) string {
	switch typ.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int64:
		return "number"
	case reflect.Slice:
		return wireKind(typ.Elem()) + "[]"
	case reflect.Struct:
		return typ.Name()
	}

	return typ.Kind().String()
}

func cyberCamelToWire(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
			b.WriteRune(r + 32)
			continue
		}
		b.WriteRune(r)
	}

	return b.String()
}

func requireSameShape(t *testing.T, what string, goFields []webappField, webapp []webappField, rename func(string) string) {
	t.Helper()

	if len(goFields) != len(webapp) {
		t.Fatalf("%s: Go carries %d fields and the webapp %d\nGo:     %+v\nwebapp: %+v",
			what, len(goFields), len(webapp), goFields, webapp)
	}

	for i := range goFields {
		name := webapp[i].name
		if rename != nil {
			name = rename(name)
		}
		if name != goFields[i].name {
			t.Errorf("%s field %d: Go calls it %q and the webapp %q", what, i, goFields[i].name, name)
		}

		want := goFields[i].kind
		if wantStruct, ok := cyberStructNames[want]; ok {
			want = wantStruct
		}
		if webapp[i].kind != want {
			t.Errorf("%s field %q: Go is %q and the webapp %q", what, goFields[i].name, want, webapp[i].kind)
		}
	}
}

var cyberStructNames = map[string]string{
	"cyberRow[]":        "CyberRow[]",
	"cyberLink[]":       "CyberLink[]",
	"cyberWatchEntry[]": "CyberWatchEntry[]",
	"cyberDataset[]":    "CyberDataset[]",
	"cyberMention[]":    "CyberMention[]",
}

func TestWebappCyberResponseShapeMatches(t *testing.T) {
	requireSameShape(t, "CyberResponse",
		goWireFields(t, cyberResponse{}), cyberWebappFields(t, "CyberResponse"), nil)
}

func TestWebappCyberRowShapeMatches(t *testing.T) {
	requireSameShape(t, "CyberRow", goWireFields(t, cyberRow{}), cyberWebappFields(t, "CyberRow"), nil)
}

func TestWebappCyberLinkShapeMatches(t *testing.T) {
	requireSameShape(t, "CyberLink", goWireFields(t, cyberLink{}), cyberWebappFields(t, "CyberLink"), nil)
}

func TestWebappCyberWatchEntryShapeMatches(t *testing.T) {
	requireSameShape(t, "CyberWatchEntry",
		goWireFields(t, cyberWatchEntry{}), cyberWebappFields(t, "CyberWatchEntry"), nil)
}

func TestWebappCyberDatasetShapeMatches(t *testing.T) {
	requireSameShape(t, "CyberDataset",
		goWireFields(t, cyberDataset{}), cyberWebappFields(t, "CyberDataset"), nil)
}

func TestWebappCyberMentionShapeMatches(t *testing.T) {
	requireSameShape(t, "CyberMention",
		goWireFields(t, cyberMention{}), cyberWebappFields(t, "CyberMention"), cyberCamelToWire)
}

func TestWebappCyberMentionsResponseShapeMatches(t *testing.T) {
	requireSameShape(t, "CyberMentionsResponse",
		goWireFields(t, cyberMentionsResponse{}), cyberWebappFields(t, "CyberMentionsResponse"), nil)
}

func TestWebappCyberTypeMatches(t *testing.T) {
	want := fmt.Sprintf("type: '%s',", cyber.Type)
	if source := cyberWebappSource(t, "index.ts"); !strings.Contains(source, want) {
		t.Fatalf("webapp/src/decorators/cyber/index.ts does not declare %q", want)
	}
}

func TestWebappCyberKindsMatch(t *testing.T) {
	source := cyberWebappSource(t, "cyber.ts")

	found := regexp.MustCompile(`export const KINDS = \[(.*?)\] as const;`).FindStringSubmatch(source)
	if found == nil {
		t.Fatalf("cyber.ts declares no KINDS list")
	}

	var webapp []string
	for _, entry := range strings.Split(found[1], ",") {
		entry = strings.TrimSpace(strings.Trim(strings.TrimSpace(entry), "'"))
		if entry != "" {
			webapp = append(webapp, entry)
		}
	}

	if len(webapp) != len(cyber.Kinds) {
		t.Fatalf("Go has %d kinds and the webapp %d: %v", len(cyber.Kinds), len(webapp), webapp)
	}
	for i, kind := range cyber.Kinds {
		if webapp[i] != string(kind) {
			t.Errorf("kind %d: Go says %q and the webapp %q", i, kind, webapp[i])
		}
	}
}

func TestWebappCyberShapeExpressionsMatch(t *testing.T) {
	source := cyberWebappSource(t, "cyber.ts")

	for _, kind := range cyber.Kinds {
		pattern := regexp.MustCompile(string(kind) + `: /\^\(\?:(.*)\)\$/,`)

		found := pattern.FindStringSubmatch(source)
		if found == nil {
			t.Errorf("cyber.ts declares no shape for %q", kind)
			continue
		}
		if found[1] != cyber.ShapeExpr(kind) {
			t.Errorf("%s shape: Go has %q and the webapp %q", kind, cyber.ShapeExpr(kind), found[1])
		}
	}
}
