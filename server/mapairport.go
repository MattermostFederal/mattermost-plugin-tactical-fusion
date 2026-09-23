package main

import (
	"net/http"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const airportParam = "airport"

func (p *Plugin) serveAirportMapPage(w http.ResponseWriter, r *http.Request, ident string) {
	if !airport.MatchesIdentShape(ident) {
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPMapAirportUnavailable, "There is no map here."))
		return
	}

	blob, ok := airport.MapBlob(ident)
	if !ok {
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPMapAirportUnavailable, "There is no map here."))
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=300")

	location.RenderOverlayPage(w, r.URL.Query(), p.packageNames(), airport.MapKind, blob)
}
