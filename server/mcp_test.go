package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/external/pluginmcp"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const agentsPluginID = "mattermost-ai"

type mcpCalls struct {
	lock   sync.Mutex
	paths  []string
	status int
}

func (a *fakeAPI) PluginHTTP(r *http.Request) *http.Response {
	a.mcp.lock.Lock()
	defer a.mcp.lock.Unlock()

	a.mcp.paths = append(a.mcp.paths, r.URL.Path)

	status := a.mcp.status
	if status == 0 {
		status = http.StatusOK
	}

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}
}

func (a *fakeAPI) pluginHTTPPaths() []string {
	a.mcp.lock.Lock()
	defer a.mcp.lock.Unlock()

	return append([]string(nil), a.mcp.paths...)
}

func mcpPlugin(t *testing.T) *Plugin {
	t.Helper()

	p := newTestPlugin(t, "https://example.com", true)
	if err := p.ensureMCPServer(); err != nil {
		t.Fatalf("ensureMCPServer returned an error: %v", err)
	}

	return p
}

func mcpRequest(p *Plugin, method, path, pluginID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	if pluginID != "" {
		req.Header.Set("Mattermost-Plugin-ID", pluginID)
	}

	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	return rec
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	for name, value := range h.headers {
		clone.Header.Set(name, value)
	}

	return h.base.RoundTrip(clone)
}

func mcpSession(t *testing.T, p *Plugin, headers map[string]string) *mcp.ClientSession {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.ServeHTTP(&plugin.Context{}, w, r)
	}))
	t.Cleanup(server.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             server.URL + mcpPath,
		HTTPClient:           &http.Client{Transport: headerTransport{http.DefaultTransport, headers}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("could not connect an MCP client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session
}

func agentsSession(t *testing.T, p *Plugin) *mcp.ClientSession {
	t.Helper()

	return mcpSession(t, p, map[string]string{"Mattermost-Plugin-ID": agentsPluginID})
}

func toolNames(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()

	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	names := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}

	return names
}

func TestMCPRouteIsMatchedBeforeTheMethodCheck(t *testing.T) {
	rec := mcpRequest(mcpPlugin(t), http.MethodPost, mcpPath, agentsPluginID)

	if rec.Code == http.StatusMethodNotAllowed {
		t.Fatal("POST /mcp was answered 405, so the match sits below the MethodGet refusal in ServeHTTP")
	}
}

func TestMCPRouteIsMatchedAboveTheSessionGate(t *testing.T) {
	rec := mcpRequest(mcpPlugin(t), http.MethodPost, mcpPath, agentsPluginID)

	if rec.Code == http.StatusFound {
		t.Fatalf("POST /mcp redirected to %q, so the match sits below the session gate",
			rec.Header().Get("Location"))
	}
}

func TestMCPRefusesACallerThatIsNotTheAgentsPlugin(t *testing.T) {
	for _, pluginID := range []string{"", "com.example.other"} {
		rec := mcpRequest(mcpPlugin(t), http.MethodPost, mcpPath, pluginID)
		if rec.Code != http.StatusForbidden {
			t.Errorf("plugin id %q: status = %d, want 403", pluginID, rec.Code)
		}
	}
}

func TestMCPRefusesWhenTheRegistryIsNotReady(t *testing.T) {
	p := mcpPlugin(t)
	p.decorators = nil

	rec := mcpRequest(p, http.MethodPost, mcpPath, agentsPluginID)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if want := fmt.Sprintf("%s-%d", errcode.Prefix, errcode.MCPNotReady); !strings.Contains(rec.Body.String(), want) {
		t.Errorf("body %q does not carry %s", rec.Body.String(), want)
	}
}

func TestMCPRefusesBeforeTheServerIsBuilt(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := mcpRequest(p, http.MethodPost, mcpPath, agentsPluginID)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestMCPLeavesEveryOtherRouteAlone(t *testing.T) {
	p := mcpPlugin(t)

	for _, path := range []string{"/mcpx", "/decorate/dtg", "/map"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		if p.serveMCPIfMatch(rec, req) {
			t.Errorf("%s was claimed by the MCP route", path)
		}
	}
}

func TestMCPToolNamesCarryTheSanitizedPluginNamespace(t *testing.T) {
	names := toolNames(t, agentsSession(t, mcpPlugin(t)))

	if len(names) == 0 {
		t.Fatal("the server registered no tools")
	}

	prefix := strings.ReplaceAll(manifest.Id, ".", "_") + "__"
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) {
			t.Errorf("tool %q does not start with %q", name, prefix)
		}
		if strings.ContainsAny(name, ".") {
			t.Errorf("tool %q carries a character the Agents side forbids", name)
		}
	}
}

func TestEnsureMCPServerIsIdempotent(t *testing.T) {
	p := mcpPlugin(t)
	first := p.currentMCPServer()

	if err := p.ensureMCPServer(); err != nil {
		t.Fatalf("second ensureMCPServer returned an error: %v", err)
	}

	if p.currentMCPServer() != first {
		t.Fatal("ensureMCPServer replaced a server it had already built, so the tools were registered twice")
	}
}

func TestMCPRegistrationFailureIsAWarnRatherThanAFailedActivation(t *testing.T) {
	p, api := newActivationPlugin(t)

	restore := mcpRegister
	t.Cleanup(func() { mcpRegister = restore })
	mcpRegister = func(*pluginmcp.Server) error {
		return errAgentsUnavailable
	}

	if err := p.OnActivate(); err != nil {
		t.Fatalf("OnActivate failed because the Agents plugin was unreachable: %v", err)
	}

	if !hasWarnCode(api, errcode.MCPRegistrationFailed) {
		t.Errorf("no warn carried %d; warns were %v", errcode.MCPRegistrationFailed, api.warnCodes)
	}
}

func TestOnActivateRegistersTheMCPServerAndOnDeactivateUnregistersIt(t *testing.T) {
	p, api := newActivationPlugin(t)

	if err := p.OnActivate(); err != nil {
		t.Fatalf("OnActivate returned an error: %v", err)
	}
	if p.currentMCPServer() == nil {
		t.Fatal("OnActivate left the MCP server nil, so /mcp would answer 503 forever")
	}

	if err := p.OnDeactivate(); err != nil {
		t.Fatalf("OnDeactivate returned an error: %v", err)
	}

	var unregistered bool
	for _, path := range api.pluginHTTPPaths() {
		if strings.HasSuffix(path, "/unregister") {
			unregistered = true
		}
	}
	if !unregistered {
		t.Errorf("OnDeactivate posted %v, none of it an unregister, so the Agents plugin keeps a stale registration",
			api.pluginHTTPPaths())
	}
}

func TestMCPUnregisterFailureIsAWarnRatherThanAFailedShutdown(t *testing.T) {
	p := mcpPlugin(t)
	api, ok := p.API.(*fakeAPI)
	if !ok {
		t.Fatal("the test plugin is not backed by fakeAPI")
	}

	restore := mcpUnregister
	t.Cleanup(func() { mcpUnregister = restore })
	mcpUnregister = func(*pluginmcp.Server) error {
		return errAgentsUnavailable
	}

	if err := p.OnDeactivate(); err != nil {
		t.Fatalf("OnDeactivate returned an error: %v", err)
	}

	if !hasWarnCode(api, errcode.MCPUnregisterFailed) {
		t.Errorf("no warn carried %d; warns were %v", errcode.MCPUnregisterFailed, api.warnCodes)
	}
}

func TestMCPDoesNothingWithoutAServer(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	p.registerMCPServerBestEffort()
	p.unregisterMCPServerBestEffort()

	api, ok := p.API.(*fakeAPI)
	if !ok {
		t.Fatal("the test plugin is not backed by fakeAPI")
	}
	if paths := api.pluginHTTPPaths(); len(paths) != 0 {
		t.Errorf("a plugin with no MCP server still called PluginHTTP: %v", paths)
	}
}

func hasWarnCode(api *fakeAPI, code int) bool {
	return slices.Contains(api.warnCodes, code)
}

var errAgentsUnavailable = errAgents{}

type errAgents struct{}

func (errAgents) Error() string { return "the Agents plugin is not loaded" }
