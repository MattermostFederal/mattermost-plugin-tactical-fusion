package airport

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

func routeTokens(codes ...string) []decorators.Token {
	tokens := make([]decorators.Token, 0, len(codes))
	for _, code := range codes {
		param := ParamValue
		if MatchesIATAShape(code) {
			param = ParamIATA
		}
		tokens = append(tokens, decorators.Token{Type: Type, Params: url.Values{param: {code}}})
	}
	return tokens
}

func TestRoutePostTypeFitsTheColumn(t *testing.T) {
	if !strings.HasPrefix(PostType, decorators.PostTypePrefix) || len(PostType) > decorators.PostTypeMaxLen {
		t.Fatalf("PostType %q is not a custom type Mattermost can store", PostType)
	}
	postType, propsKey := decorators.MultiPostType(&Decorator{})
	if postType != PostType || propsKey != PropsKey {
		t.Fatalf("MultiPostType = %q, %q", postType, propsKey)
	}
}

func TestRoutePropsListAirfieldsInMessageOrder(t *testing.T) {
	blob, ok := (&Decorator{}).MultiPostProps(routeTokens("PHIK", "HNL", "PGUA"))
	if !ok {
		t.Fatal("the route was refused")
	}
	if blob["version"] != PropsVersion {
		t.Errorf("version = %v", blob["version"])
	}

	entries, _ := blob["airfields"].([]any)
	if len(entries) != 3 {
		t.Fatalf("%d entries, want 3", len(entries))
	}
	for i, want := range []struct{ ident, code string }{{"PHIK", "PHIK"}, {"PHNL", "HNL"}, {"PGUA", "PGUA"}} {
		entry := entries[i].(map[string]any)
		if entry["ident"] != want.ident || entry["code"] != want.code {
			t.Errorf("entry %d = %v, want %s written as %s", i, entry, want.ident, want.code)
		}
		if entry["name"] == "" || entry["format"] != "dd" || entry["value"] == "" {
			t.Errorf("entry %d carries no name or position: %v", i, entry)
		}
	}

	if _, err := json.Marshal(blob); err != nil {
		t.Fatalf("the blob does not marshal: %v", err)
	}
}

func TestRoutePropsRefuseWhatIsNotARoute(t *testing.T) {
	d := &Decorator{}

	if _, ok := d.MultiPostProps(nil); ok {
		t.Error("an empty route was accepted")
	}
	if _, ok := d.MultiPostProps(routeTokens("PHIK", "QZQZ")); ok {
		t.Error("an unknown ident was accepted")
	}
	if _, ok := d.MultiPostProps([]decorators.Token{{Type: "dtg", Params: url.Values{"v": {"PHIK"}}}}); ok {
		t.Error("a token of another type was accepted")
	}
	if _, ok := d.MultiPostProps([]decorators.Token{{Type: Type, Params: url.Values{}}}); ok {
		t.Error("a token with no code was accepted")
	}

	many := make([]string, MaxRouteAirfields+1)
	for i := range many {
		many[i] = "PHIK"
	}
	if _, ok := d.MultiPostProps(routeTokens(many...)); ok {
		t.Error("a route past the cap was accepted")
	}
	if _, ok := d.MultiPostProps(routeTokens(many[:MaxRouteAirfields]...)); !ok {
		t.Error("a route at the cap was refused")
	}
}

func TestRoutePropsAnswerNothingWhenTheSwitchIsOff(t *testing.T) {
	off := &Decorator{Enabled: func() Formats { return Formats{Airfield: true, Table: true} }}
	if postType, propsKey := decorators.MultiPostType(off); postType != "" || propsKey != "" {
		t.Fatalf("MultiPostType = %q, %q with the route off", postType, propsKey)
	}
}

func TestRoutePropsCarryThePairForEveryShippedAirfield(t *testing.T) {
	d := &Decorator{}
	for ident := range airfields {
		blob, ok := d.MultiPostProps(routeTokens(ident))
		if !ok {
			t.Fatalf("%s was refused", ident)
		}
		entry := blob["airfields"].([]any)[0].(map[string]any)
		value, _ := entry["value"].(string)
		if value == "" {
			t.Fatalf("%s carries no position", ident)
		}
		parsed, ok := location.Parse(location.FormatDD, value)
		if !ok || parsed.Canonical() != value {
			t.Fatalf("%s: %q is not a canonical coordinate", ident, value)
		}
	}
}
