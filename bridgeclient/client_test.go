package bridgeclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/bridgeclient"
)

type handlerAPI struct {
	handler http.HandlerFunc
}

func (a handlerAPI) PluginHTTP(r *http.Request) *http.Response {
	rec := httptest.NewRecorder()
	a.handler(rec, r)
	return rec.Result()
}

type silentAPI struct{}

func (silentAPI) PluginHTTP(*http.Request) *http.Response {
	return nil
}

func replyJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func TestLinkPostsItsRequestToTheBridgeLinkRoute(t *testing.T) {
	var gotMethod, gotPath, gotType string
	var gotBody bridgeclient.LinkRequest

	client := bridgeclient.NewClient(handlerAPI{func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		replyJSON(w, http.StatusOK, bridgeclient.LinkResponse{
			Markdown: "[PHIK](/plugins/x/decorate/airport?v=PHIK)",
			URL:      "/plugins/x/decorate/airport?v=PHIK",
			Type:     bridgeclient.TypeAirport,
			Label:    "PHIK",
		})
	}})

	req := bridgeclient.LinkRequest{Type: bridgeclient.TypeAirport, Token: "PHIK", Label: "Hickam", ReferenceTime: 42}
	resp, err := client.Link(context.Background(), req)
	if err != nil {
		t.Fatalf("Link returned %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if want := "/" + bridgeclient.PluginID + "/bridge/v1/link"; gotPath != want {
		t.Errorf("path = %s, want %s", gotPath, want)
	}
	if gotType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotType)
	}
	if gotBody != req {
		t.Errorf("request body = %+v, want %+v", gotBody, req)
	}
	if resp.Markdown != "[PHIK](/plugins/x/decorate/airport?v=PHIK)" {
		t.Errorf("Markdown = %q", resp.Markdown)
	}
}

func TestDecoratePostsToTheBridgeDecorateRoute(t *testing.T) {
	var gotPath string
	client := bridgeclient.NewClient(handlerAPI{func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		replyJSON(w, http.StatusOK, bridgeclient.DecorateResponse{Message: "decorated", Changed: true, FitsPost: true})
	}})

	resp, err := client.Decorate(context.Background(), bridgeclient.DecorateRequest{Message: "plain"})
	if err != nil {
		t.Fatalf("Decorate returned %v", err)
	}
	if want := "/" + bridgeclient.PluginID + "/bridge/v1/decorate"; gotPath != want {
		t.Errorf("path = %s, want %s", gotPath, want)
	}
	if resp != (bridgeclient.DecorateResponse{Message: "decorated", Changed: true, FitsPost: true}) {
		t.Errorf("response = %+v", resp)
	}
}

func TestInfoIsAGetWithNoBody(t *testing.T) {
	var gotMethod string
	var gotBody []byte
	client := bridgeclient.NewClient(handlerAPI{func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		replyJSON(w, http.StatusOK, bridgeclient.InfoResponse{PluginVersion: "1.2.3", APIVersion: 1})
	}})

	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("Info returned %v", err)
	}
	if gotMethod != http.MethodGet || len(gotBody) != 0 {
		t.Errorf("sent %s with a %d byte body, want GET with none", gotMethod, len(gotBody))
	}
	if info.PluginVersion != "1.2.3" || info.APIVersion != 1 {
		t.Errorf("info = %+v", info)
	}
}

func TestAirportIsAGetCarryingTheIdentInTheQuery(t *testing.T) {
	var gotMethod, gotPath, gotIdent string
	var gotBody []byte
	client := bridgeclient.NewClient(handlerAPI{func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotIdent = r.Method, r.URL.Path, r.URL.Query().Get("ident")
		gotBody, _ = io.ReadAll(r.Body)
		replyJSON(w, http.StatusOK, bridgeclient.AirportResponse{
			Found: true, Ident: "PHIK", Name: "Hickam Air Force Base", Lat: 21.3353, Lon: -157.9483,
		})
	}})

	field, err := client.Airport(context.Background(), "phik")
	if err != nil {
		t.Fatalf("Airport returned %v", err)
	}

	if gotMethod != http.MethodGet || len(gotBody) != 0 {
		t.Errorf("sent %s with a %d byte body, want GET with none", gotMethod, len(gotBody))
	}
	if want := "/" + bridgeclient.PluginID + "/bridge/v1/airport"; gotPath != want {
		t.Errorf("path = %s, want %s", gotPath, want)
	}
	if gotIdent != "phik" {
		t.Errorf("ident = %q, want it as the caller wrote it", gotIdent)
	}
	if field != (bridgeclient.AirportResponse{Found: true, Ident: "PHIK", Name: "Hickam Air Force Base", Lat: 21.3353, Lon: -157.9483}) {
		t.Errorf("airport = %+v", field)
	}
}

func TestAirportEscapesWhatTheCallerWrote(t *testing.T) {
	var gotIdent, gotOther string
	client := bridgeclient.NewClient(handlerAPI{func(w http.ResponseWriter, r *http.Request) {
		gotIdent, gotOther = r.URL.Query().Get("ident"), r.URL.Query().Get("other")
		replyJSON(w, http.StatusOK, bridgeclient.AirportResponse{})
	}})

	if _, err := client.Airport(context.Background(), "PH K&other=1#x"); err != nil {
		t.Fatalf("Airport returned %v", err)
	}
	if gotIdent != "PH K&other=1#x" || gotOther != "" {
		t.Errorf("ident = %q, other = %q", gotIdent, gotOther)
	}
}

func TestAnAirportTheDatabaseDoesNotHoldIsNotAnError(t *testing.T) {
	client := bridgeclient.NewClient(handlerAPI{func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"found":false,"ident":"QQQQ","name":"","lat":0,"lon":0}`))
	}})

	field, err := client.Airport(context.Background(), "QQQQ")
	if err != nil {
		t.Fatalf("Airport returned %v", err)
	}
	if field != (bridgeclient.AirportResponse{Ident: "QQQQ"}) {
		t.Errorf("airport = %+v", field)
	}
}

func TestAMalformedAirportIdentIsAnErrorCarryingItsCode(t *testing.T) {
	client := bridgeclient.NewClient(handlerAPI{func(w http.ResponseWriter, _ *http.Request) {
		replyJSON(w, http.StatusBadRequest, bridgeclient.ErrorResponse{
			Message: "An airfield ident is four letters. (TF-19009)", Code: 19009,
		})
	}})

	_, err := client.Airport(context.Background(), "PHI")

	var bridgeErr *bridgeclient.Error
	if !errors.As(err, &bridgeErr) {
		t.Fatalf("error = %#v, want an *Error", err)
	}
	if bridgeErr.StatusCode != http.StatusBadRequest || bridgeErr.Code != 19009 || bridgeErr.Reason != "" {
		t.Errorf("error = %+v", bridgeErr)
	}
	if errors.Is(err, bridgeclient.ErrPluginNotActive) || errors.Is(err, bridgeclient.ErrNotRecognized) {
		t.Errorf("error = %v matches a sentinel it should not", err)
	}
}

func TestAirportWithNoResponseMeansThePluginIsNotActive(t *testing.T) {
	_, err := bridgeclient.NewClient(silentAPI{}).Airport(context.Background(), "PHIK")
	if !errors.Is(err, bridgeclient.ErrPluginNotActive) {
		t.Fatalf("error = %v, want ErrPluginNotActive", err)
	}
}

func TestNoResponseMeansThePluginIsNotActive(t *testing.T) {
	_, err := bridgeclient.NewClient(silentAPI{}).Link(context.Background(), bridgeclient.LinkRequest{})
	if !errors.Is(err, bridgeclient.ErrPluginNotActive) {
		t.Fatalf("error = %v, want ErrPluginNotActive", err)
	}
}

func TestAResponseWithoutATacticalFusionCodeMeansThePluginIsNotActive(t *testing.T) {
	client := bridgeclient.NewClient(handlerAPI{func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "plugin not found", http.StatusNotFound)
	}})

	_, err := client.Link(context.Background(), bridgeclient.LinkRequest{})
	if !errors.Is(err, bridgeclient.ErrPluginNotActive) {
		t.Fatalf("error = %v, want ErrPluginNotActive", err)
	}

	var bridgeErr *bridgeclient.Error
	if !errors.As(err, &bridgeErr) || bridgeErr.StatusCode != http.StatusNotFound {
		t.Fatalf("error = %#v, want an *Error carrying 404", err)
	}
}

func TestDeclineReasonsMatchTheirSentinels(t *testing.T) {
	sentinels := map[string]error{
		bridgeclient.ReasonUnknownType:   bridgeclient.ErrUnknownType,
		bridgeclient.ReasonNotRecognized: bridgeclient.ErrNotRecognized,
		bridgeclient.ReasonDisabled:      bridgeclient.ErrDisabled,
	}
	all := []error{
		bridgeclient.ErrPluginNotActive,
		bridgeclient.ErrUnknownType,
		bridgeclient.ErrNotRecognized,
		bridgeclient.ErrDisabled,
	}

	for reason, want := range sentinels {
		t.Run(reason, func(t *testing.T) {
			client := bridgeclient.NewClient(handlerAPI{func(w http.ResponseWriter, _ *http.Request) {
				replyJSON(w, http.StatusUnprocessableEntity, bridgeclient.ErrorResponse{
					Message: "Declined. (TF-19006)", Code: 19006, Reason: reason,
				})
			}})

			_, err := client.Link(context.Background(), bridgeclient.LinkRequest{})
			for _, sentinel := range all {
				if got := errors.Is(err, sentinel); got != (sentinel == want) {
					t.Errorf("errors.Is(%v, %v) = %t", err, sentinel, got)
				}
			}

			var bridgeErr *bridgeclient.Error
			if !errors.As(err, &bridgeErr) {
				t.Fatalf("error = %#v, want an *Error", err)
			}
			if bridgeErr.Code != 19006 || bridgeErr.Message != "Declined. (TF-19006)" {
				t.Errorf("error = %+v", bridgeErr)
			}
		})
	}
}

func TestAnUndecodableSuccessIsAnError(t *testing.T) {
	client := bridgeclient.NewClient(handlerAPI{func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}})

	if _, err := client.Decorate(context.Background(), bridgeclient.DecorateRequest{}); err == nil {
		t.Fatal("Decorate accepted a body that is not JSON")
	}
}
