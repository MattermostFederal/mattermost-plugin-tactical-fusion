package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/bridgeclient"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const (
	bridgePath         = bridgeclient.BridgePath
	bridgeDecoratePath = bridgePath + "/decorate"
	bridgeLinkPath     = bridgePath + "/link"
	bridgeInfoPath     = bridgePath + "/info"

	decoratePathAPI = apiPath + "/decorate"
	linkPathAPI     = apiPath + "/link"

	maxBridgeBody = 64 * 1024

	dtgOffsetParam = "o"
)

type bridgeRefusal struct {
	status  int
	code    int
	reason  string
	message string
}

var (
	refusedNotAuthorized = bridgeRefusal{http.StatusUnauthorized, errcode.BridgeNotAuthorized, "", "Not authorized."}
	refusedNotFound      = bridgeRefusal{http.StatusNotFound, errcode.BridgeNotFound, "", "Not found."}
	refusedMethod        = bridgeRefusal{http.StatusMethodNotAllowed, errcode.BridgeMethodNotAllowed, "", "Method not allowed."}
	refusedBody          = bridgeRefusal{http.StatusBadRequest, errcode.BridgeInvalidBody, "", "That is not a valid request body."}
	refusedLabel         = bridgeRefusal{http.StatusBadRequest, errcode.BridgeInvalidBody, "", "A label may not contain a line break."}
	refusedNotReady      = bridgeRefusal{http.StatusServiceUnavailable, errcode.BridgeNotReady, "", "Not ready."}
	refusedPanic         = bridgeRefusal{http.StatusInternalServerError, errcode.BridgePanic, "", "Internal error."}

	refusedUnknownType = bridgeRefusal{
		http.StatusUnprocessableEntity, errcode.BridgeUnknownType,
		bridgeclient.ReasonUnknownType, "Unknown decorator type.",
	}
	refusedNotRecognized = bridgeRefusal{
		http.StatusUnprocessableEntity, errcode.BridgeTokenNotRecognized,
		bridgeclient.ReasonNotRecognized, "The token is not one this decorator type reads.",
	}
	refusedDisabled = bridgeRefusal{
		http.StatusUnprocessableEntity, errcode.BridgeFormatDisabled,
		bridgeclient.ReasonDisabled, "The token's format is switched off by an administrator.",
	}
)

func callingPluginID(r *http.Request) string {
	return r.Header.Get("Mattermost-Plugin-ID")
}

func (p *Plugin) serveBridge(w http.ResponseWriter, r *http.Request) {
	if callingPluginID(r) == "" {
		writeBridgeRefusal(w, refusedNotAuthorized)
		return
	}

	switch r.URL.Path {
	case bridgeDecoratePath:
		p.serveBridgeOperation(w, r, p.serveDecorateText)
	case bridgeLinkPath:
		p.serveBridgeOperation(w, r, p.serveLink)
	case bridgeInfoPath:
		p.serveBridgeOperation(w, r, p.serveBridgeInfo)
	default:
		writeBridgeRefusal(w, refusedNotFound)
	}
}

func (p *Plugin) serveBridgeOperation(w http.ResponseWriter, r *http.Request, operation http.HandlerFunc) {
	api := p.API
	defer func() {
		if recovered := recover(); recovered != nil {
			api.LogError("Bridge request panicked",
				"error_code", errcode.BridgePanic,
				"path", r.URL.Path,
				"plugin_id", callingPluginID(r),
				"panic", fmt.Sprint(recovered))
			writeBridgeRefusal(w, refusedPanic)
		}
	}()

	w.Header().Set("Cache-Control", "no-store")

	if p.decorators == nil || !p.configurationLoaded() {
		writeBridgeRefusal(w, refusedNotReady)
		return
	}

	operation(w, r)
}

func (p *Plugin) serveDecorateText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeBridgeRefusal(w, refusedMethod)
		return
	}

	var req bridgeclient.DecorateRequest
	if !decodeBridgeRequest(w, r, &req) {
		return
	}

	writeAPIJSON(w, http.StatusOK, p.decorateText(req))
}

func (p *Plugin) serveLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeBridgeRefusal(w, refusedMethod)
		return
	}

	var req bridgeclient.LinkRequest
	if !decodeBridgeRequest(w, r, &req) {
		return
	}

	resp, refusal := p.buildLink(req)
	if refusal != nil {
		writeBridgeRefusal(w, *refusal)
		return
	}

	writeAPIJSON(w, http.StatusOK, resp)
}

func (p *Plugin) serveBridgeInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeBridgeRefusal(w, refusedMethod)
		return
	}

	writeAPIJSON(w, http.StatusOK, p.bridgeInfo())
}

func (p *Plugin) decorateText(req bridgeclient.DecorateRequest) bridgeclient.DecorateResponse {
	decorated := p.bridgeTagger().Decorate(req.Message, bridgeReferenceTime(req.ReferenceTime))

	return bridgeclient.DecorateResponse{
		Message:  decorated,
		Changed:  decorated != req.Message,
		FitsPost: utf8.RuneCountInString(decorated) <= safePostRunes,
	}
}

func (p *Plugin) buildLink(req bridgeclient.LinkRequest) (bridgeclient.LinkResponse, *bridgeRefusal) {
	decorator := p.decorators.Get(req.Type)
	if decorator == nil {
		return bridgeclient.LinkResponse{}, &refusedUnknownType
	}

	if strings.ContainsAny(req.Label, "\r\n") {
		return bridgeclient.LinkResponse{}, &refusedLabel
	}

	token := strings.TrimSpace(req.Token)
	ref := bridgeReferenceTime(req.ReferenceTime)

	params, parsed := decorator.Parse(token, ref)
	switch {
	case parsed && p.formatEnabled(decorator.Type(), params):
	case parsed || parsesWithEveryFormat(decorator.Type(), token, ref):
		return bridgeclient.LinkResponse{}, &refusedDisabled
	default:
		return bridgeclient.LinkResponse{}, &refusedNotRecognized
	}

	label := req.Label
	if label == "" {
		label = token
	}

	tagger := p.bridgeTagger()

	return bridgeclient.LinkResponse{
		Markdown: tagger.LinkFor(decorator.Type(), label, params),
		URL:      tagger.URLFor(decorator.Type(), params),
		Type:     decorator.Type(),
		Label:    label,
	}, nil
}

func (p *Plugin) formatEnabled(typ string, params url.Values) bool {
	switch typ {
	case dtg.Type:
		formats := p.dtgFormats()
		if params.Has(dtgOffsetParam) {
			return formats.Timestamp
		}
		return formats.Military
	case airport.Type:
		return p.airportFormats().Airfield
	case cyber.Type:
		return cyberKindEnabled(p.cyberFormats(), params)
	}

	return true
}

func parsesWithEveryFormat(typ, token string, ref time.Time) bool {
	var unrestricted decorators.Decorator
	switch typ {
	case dtg.Type:
		unrestricted = &dtg.Decorator{}
	case location.Type:
		unrestricted = &location.Decorator{}
	case airport.Type:
		unrestricted = &airport.Decorator{}
	case cyber.Type:
		unrestricted = &cyber.Decorator{}
	default:
		return false
	}

	_, ok := unrestricted.Parse(token, ref)
	return ok
}

func (p *Plugin) bridgeInfo() bridgeclient.InfoResponse {
	config := p.getConfiguration()
	enabled := map[string]bool{
		dtg.Type:      config.EnableDTG,
		location.Type: config.EnableLocation,
		airport.Type:  config.EnableAirport,
		cyber.Type:    config.EnableCyber,
	}

	info := bridgeclient.InfoResponse{
		PluginVersion: manifest.Version,
		APIVersion:    bridgeclient.APIVersion,
		Types:         []string{},
		EnabledTypes:  []string{},
	}
	for _, d := range p.decorators.All() {
		info.Types = append(info.Types, d.Type())
		if enabled[d.Type()] {
			info.EnabledTypes = append(info.EnabledTypes, d.Type())
		}
	}

	return info
}

func (p *Plugin) bridgeTagger() *decorators.Tagger {
	return &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}
}

func bridgeReferenceTime(unixMillis int64) time.Time {
	if unixMillis <= 0 {
		return referenceTime(nil)
	}
	return time.UnixMilli(unixMillis).UTC()
}

func decodeBridgeRequest(w http.ResponseWriter, r *http.Request, dest any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBridgeBody))
	if err := decoder.Decode(dest); err != nil {
		writeBridgeRefusal(w, refusedBody)
		return false
	}
	return true
}

func writeBridgeRefusal(w http.ResponseWriter, refusal bridgeRefusal) {
	writeAPIJSON(w, refusal.status, bridgeclient.ErrorResponse{
		Message: errcode.WithCode(refusal.code, refusal.message),
		Code:    refusal.code,
		Reason:  refusal.reason,
	})
}

// cyberKindEnabled reads the kind out of the link's own parameters, because a
// cyber link names which grammar recognized it and the switches are per kind.
func cyberKindEnabled(formats cyber.Formats, params url.Values) bool {
	kind := cyber.Kind(params.Get(cyber.ParamKind))
	if !kind.Known() {
		return false
	}

	enabled := (&cyber.Decorator{Enabled: func() cyber.Formats { return formats }})

	_, ok := enabled.Parse(params.Get(cyber.ParamValue), time.Now())

	return ok
}
