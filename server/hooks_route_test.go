package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
)

func routeProps(t *testing.T, post *model.Post) map[string]any {
	t.Helper()

	raw, ok := post.GetProps()[airport.PropsKey]
	if !ok {
		return nil
	}
	blob, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("props[%s] is a %T", airport.PropsKey, raw)
	}
	return blob
}

func routeIdents(t *testing.T, post *model.Post) []string {
	t.Helper()

	blob := routeProps(t, post)
	if blob == nil {
		return nil
	}
	entries, _ := blob["airfields"].([]any)
	idents := make([]string, 0, len(entries))
	for _, entry := range entries {
		idents = append(idents, entry.(map[string]any)["ident"].(string))
	}
	return idents
}

func TestDecoratePostStampsARouteOfAirfields(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, message := range []string{
		"DEPLOC:PHIK ARRLOC:PGUA//",
		"DEPLOC:PHIK\nARRLOC:PGUA//",
		"  ICAO:PHIK \t IATA:HNL  \n LOC:PGUA \n",
	} {
		t.Run(message, func(t *testing.T) {
			got := p.decoratePost(&model.Post{Message: message}, hookRef)
			if got == nil {
				t.Fatal("decoratePost left the post alone")
			}
			if got.Type != airport.PostType {
				t.Fatalf("Type = %q, want %q", got.Type, airport.PostType)
			}
			idents := routeIdents(t, got)
			if len(idents) < 2 || idents[0] != "PHIK" {
				t.Fatalf("route = %v", idents)
			}
			if strings.Count(got.Message, "/decorate/airport?") != len(idents) {
				t.Errorf("the links did not survive the stamp:\n%s", got.Message)
			}
		})
	}
}

func TestARouteKeepsItsLinksAndItsTrail(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	got := p.decoratePost(&model.Post{Message: "DEPLOC:PHIK ARRLOC:PGUA//"}, hookRef)
	if got == nil || got.Type != airport.PostType {
		t.Fatal("the route was not stamped")
	}
	if !strings.HasPrefix(got.Message, "[PHIK](") || !strings.HasSuffix(got.Message, ")//") {
		t.Errorf("message = %q", got.Message)
	}
	if strings.Contains(got.Message, "| Airfield |") {
		t.Error("a route was expanded into a table")
	}
}

func TestASoleAirfieldIsStampedWhenTheTableIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := p.getConfiguration().Clone()
	config.EnableAirportTable = false
	p.setConfiguration(config)

	got := p.decoratePost(&model.Post{Message: "ICAO:PHIK"}, hookRef)
	if got == nil || got.Type != airport.PostType {
		t.Fatalf("a sole airfield with the table off was not stamped: %+v", got)
	}
	if idents := routeIdents(t, got); len(idents) != 1 || idents[0] != "PHIK" {
		t.Errorf("route = %v", idents)
	}
}

func TestASoleAirfieldIsExpandedNotStampedWhenTheTableIsOn(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	got := p.decoratePost(&model.Post{Message: "ICAO:PHIK"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost left the post alone")
	}
	if got.Type != "" {
		t.Errorf("Type = %q, want an ordinary post", got.Type)
	}
	if !strings.Contains(got.Message, "| Airfield |") {
		t.Error("the table was not written")
	}
	if routeProps(t, got) != nil {
		t.Error("route props were stamped beside the table")
	}
}

func TestAMixedTokenMessageIsNotStamped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, message := range []string{
		"DEPLOC:PHIK 091630ZAUG26",
		"DEPLOC:PHIK 21.3353, -157.9483",
	} {
		got := p.decoratePost(&model.Post{Message: message}, hookRef)
		if got == nil {
			t.Fatalf("%q: decoratePost left the post alone", message)
		}
		if got.Type != "" || routeProps(t, got) != nil {
			t.Errorf("%q: stamped as %q", message, got.Type)
		}
	}
}

func TestARouteWithProseBetweenAirfieldsIsNotStamped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, message := range []string{
		"DEPLOC:PHIK, ARRLOC:PGUA",
		"DEPLOC:PHIK then ARRLOC:PGUA",
		"from DEPLOC:PHIK ARRLOC:PGUA",
	} {
		got := p.decoratePost(&model.Post{Message: message}, hookRef)
		if got == nil {
			t.Fatalf("%q: decoratePost left the post alone", message)
		}
		if got.Type != "" {
			t.Errorf("%q: stamped as %q", message, got.Type)
		}
	}
}

func TestARouteIsNotStampedWhenTheInlineMapIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := p.getConfiguration().Clone()
	config.EnableLocationMapInline = false
	p.setConfiguration(config)

	got := p.decoratePost(&model.Post{Message: "DEPLOC:PHIK ARRLOC:PGUA//"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost left the post alone")
	}
	if got.Type != "" {
		t.Errorf("Type = %q with the inline map off", got.Type)
	}
}

func TestARouteIsNotStampedWhenTheRouteSwitchIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := p.getConfiguration().Clone()
	config.EnableAirportRoute = false
	p.setConfiguration(config)

	got := p.decoratePost(&model.Post{Message: "DEPLOC:PHIK ARRLOC:PGUA//"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost left the post alone")
	}
	if got.Type != "" {
		t.Errorf("Type = %q with the route switch off", got.Type)
	}
	if strings.Count(got.Message, "/decorate/airport?") != 2 {
		t.Errorf("the links were lost with the switch:\n%s", got.Message)
	}
}

func TestARoutePastTheCapIsDecoratedNotStamped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	found := decorators.Result{Covers: true, OnlyType: airport.Type}
	for range airport.MaxRouteAirfields + 1 {
		found.Tokens = append(found.Tokens, decorators.Token{Type: airport.Type, Params: url.Values{airport.ParamValue: {"PHIK"}}})
	}

	post := &model.Post{Message: "decorated already"}
	p.stampMultiTokenPost(post, found)
	if post.Type != "" {
		t.Errorf("Type = %q past the cap", post.Type)
	}
	if routeProps(t, post) != nil {
		t.Error("props were stamped past the cap")
	}

	found.Tokens = found.Tokens[:airport.MaxRouteAirfields]
	p.stampMultiTokenPost(post, found)
	if post.Type != airport.PostType {
		t.Errorf("Type = %q at the cap, want the route", post.Type)
	}
}

func TestARouteStampIsMeasuredAgainstThePropsBudget(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "DEPLOC:PHIK ARRLOC:PGUA//"}
	post.AddProp("someone_elses", strings.Repeat("x", stampPropsBudgetRunes))

	got := p.decoratePost(post, hookRef)
	if got == nil {
		t.Fatal("decoratePost left the post alone")
	}
	if got.Type != "" {
		t.Errorf("Type = %q over the props budget", got.Type)
	}
	if strings.Count(got.Message, "/decorate/airport?") != 2 {
		t.Error("the links were lost when the stamp would not fit")
	}
}

func TestAForgedAirfieldsBlobIsStripped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "nothing to see", Type: airport.PostType, UserId: testUserID}
	post.AddProp(airport.PropsKey, map[string]any{"version": 1, "airfields": []any{map[string]any{"ident": "PHIK"}}})

	got := p.decoratePost(post, hookRef)
	if got == nil {
		t.Fatal("a forged post was left exactly as it arrived")
	}
	if got.Type != "" {
		t.Errorf("Type = %q, want the forged type stripped", got.Type)
	}
	if routeProps(t, got) != nil {
		t.Error("the forged route blob survived")
	}
}

func TestAForgedAirfieldsTypeOverARealRouteIsRestamped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "DEPLOC:PHIK ARRLOC:PGUA//", Type: airport.PostType, UserId: testUserID}
	post.AddProp(airport.PropsKey, map[string]any{"version": 1, "airfields": []any{map[string]any{"ident": "KLAX"}}})

	got := p.decoratePost(post, hookRef)
	if got == nil || got.Type != airport.PostType {
		t.Fatal("the real route was not restamped")
	}
	if idents := routeIdents(t, got); len(idents) != 2 || idents[0] != "PHIK" || idents[1] != "PGUA" {
		t.Errorf("route = %v, want the message's own", idents)
	}
}

func TestARouteKeepsAnotherIntegrationsProps(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "DEPLOC:PHIK ARRLOC:PGUA//"}
	post.AddProp("theirs", "kept")

	got := p.decoratePost(post, hookRef)
	if got == nil || got.Type != airport.PostType {
		t.Fatal("the route was not stamped")
	}
	if got.GetProps()["theirs"] != "kept" {
		t.Error("another integration's props were dropped")
	}
}
