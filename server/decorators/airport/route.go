package airport

import (
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

const (
	PostType          = decorators.PostTypePrefix + "tf_airfields"
	PropsKey          = "tactical_fusion_airfields"
	PropsVersion      = 1
	MaxRouteAirfields = 64
)

var _ decorators.MultiPostRenderer = (*Decorator)(nil)

func (d *Decorator) MultiPost() (string, string) {
	if !d.formats().Route {
		return "", ""
	}
	return PostType, PropsKey
}

func (d *Decorator) MultiPostProps(tokens []decorators.Token) (map[string]any, bool) {
	if len(tokens) == 0 || len(tokens) > MaxRouteAirfields {
		return nil, false
	}

	entries := make([]any, 0, len(tokens))
	for _, token := range tokens {
		entry, ok := routeEntry(token)
		if !ok {
			return nil, false
		}
		entries = append(entries, entry)
	}

	return map[string]any{
		"version":   PropsVersion,
		"airfields": entries,
	}, true
}

func routeEntry(token decorators.Token) (map[string]any, bool) {
	if token.Type != Type {
		return nil, false
	}

	ref, err := ReferenceFromParams(token.Params)
	if err != nil {
		return nil, false
	}
	ident, ok := ref.Resolve()
	if !ok {
		return nil, false
	}
	a, _ := Lookup(ident)

	entry := map[string]any{
		"ident": ident,
		"code":  ref.Code(),
		"name":  a.Name,
	}
	if position, ok := endCoordinate(a.Lat, a.Lon); ok {
		entry["format"] = position.Format
		entry["value"] = position.Token
	}

	return entry, true
}
