// Package bridgeclient lets another Mattermost plugin build Tactical Fusion
// decorator links from its server side.
//
// A decorator link is ordinary markdown, "[label](url)". Embedded in a post,
// an ephemeral message or any other markdown a Mattermost client renders, it
// behaves exactly like a link Tactical Fusion wrote itself: a hover card in the
// web app, a click that opens the Tactical Fusion sidebar, and a standalone page
// on clients without the web app bundle.
//
// Requests travel over the plugin API's PluginHTTP, the inter-plugin transport
// Mattermost provides. The server stamps each request with the calling plugin's
// id, which is how Tactical Fusion authenticates it.
//
//	client := bridgeclient.NewClient(p.API)
//
//	link, err := client.Link(ctx, bridgeclient.LinkRequest{
//		Type:  bridgeclient.TypeLocation,
//		Token: "18S UJ 23478 06483",
//	})
//	if err != nil {
//		return err
//	}
//	post.Message = "Rally point " + link.Markdown
//
// Every call returns an *Error for a response Tactical Fusion refused. Test it
// with errors.Is against ErrPluginNotActive, ErrUnknownType, ErrNotRecognized
// and ErrDisabled.
package bridgeclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// PluginAPI is the part of plugin.API this package uses. A plugin's p.API
// satisfies it.
type PluginAPI interface {
	PluginHTTP(request *http.Request) *http.Response
}

// Client calls the Tactical Fusion bridge. It is safe for concurrent use.
type Client struct {
	api PluginAPI
}

// NewClient returns a client that sends requests through api.
func NewClient(api PluginAPI) *Client {
	return &Client{api: api}
}

// Sentinel errors, matched with errors.Is against an error a Client returns.
var (
	// ErrPluginNotActive means Tactical Fusion is not installed, not enabled,
	// or did not answer.
	ErrPluginNotActive = errors.New("tactical fusion plugin is not active")

	// ErrUnknownType means LinkRequest.Type names no decorator.
	ErrUnknownType = errors.New("unknown decorator type")

	// ErrNotRecognized means LinkRequest.Token is not a token of that type.
	ErrNotRecognized = errors.New("token not recognized")

	// ErrDisabled means the token's format is switched off by an administrator.
	ErrDisabled = errors.New("format disabled")
)

// Error is a response Tactical Fusion refused.
type Error struct {
	// StatusCode is the HTTP status of the response, or 0 when there was none.
	StatusCode int

	// Code is the numeric TF code, or 0 when the response did not come from
	// Tactical Fusion.
	Code int

	// Reason is the machine-readable reason a Link was declined, if any.
	Reason string

	// Message is the human-readable message.
	Message string
}

// Error implements error.
func (e *Error) Error() string {
	return fmt.Sprintf("tactical fusion bridge: status %d: %s", e.StatusCode, e.Message)
}

// Is reports whether e matches one of the package's sentinel errors.
func (e *Error) Is(target error) bool {
	switch target {
	case ErrPluginNotActive:
		return e.Code == 0
	case ErrUnknownType:
		return e.Reason == ReasonUnknownType
	case ErrNotRecognized:
		return e.Reason == ReasonNotRecognized
	case ErrDisabled:
		return e.Reason == ReasonDisabled
	}
	return false
}

// Decorate rewrites every recognized token in req.Message as a decorator link,
// with the same rules Tactical Fusion applies to a posted message.
func (c *Client) Decorate(ctx context.Context, req DecorateRequest) (DecorateResponse, error) {
	var resp DecorateResponse
	err := c.do(ctx, http.MethodPost, "/decorate", req, &resp)
	return resp, err
}

// Link builds one decorator link for a token of a known type.
func (c *Client) Link(ctx context.Context, req LinkRequest) (LinkResponse, error) {
	var resp LinkResponse
	err := c.do(ctx, http.MethodPost, "/link", req, &resp)
	return resp, err
}

// Info reports the installed plugin's version, bridge version and decorator
// types.
func (c *Client) Info(ctx context.Context) (InfoResponse, error) {
	var resp InfoResponse
	err := c.do(ctx, http.MethodGet, "/info", nil, &resp)
	return resp, err
}

const maxResponseBody = 1 << 20

func (c *Client) do(ctx context.Context, method, route string, body, out any) error {
	var reader io.Reader = http.NoBody
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("tactical fusion bridge: encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, "/"+PluginID+BridgePath+route, reader)
	if err != nil {
		return fmt.Errorf("tactical fusion bridge: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp := c.api.PluginHTTP(req)
	if resp == nil {
		return &Error{Message: "no response from the plugin"}
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return fmt.Errorf("tactical fusion bridge: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errorFrom(resp.StatusCode, payload)
	}

	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("tactical fusion bridge: decode response: %w", err)
	}
	return nil
}

func errorFrom(status int, payload []byte) *Error {
	var body ErrorResponse
	if err := json.Unmarshal(payload, &body); err != nil || body.Code == 0 {
		return &Error{StatusCode: status, Message: http.StatusText(status)}
	}
	return &Error{StatusCode: status, Code: body.Code, Reason: body.Reason, Message: body.Message}
}
