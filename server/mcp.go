package main

import (
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-agents/v2/external/pluginmcp"
	"github.com/pkg/errors"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const mcpPath = "/mcp"

var (
	mcpNewServer = pluginmcp.NewServer

	mcpRegister = func(server *pluginmcp.Server) error {
		return server.Register()
	}

	mcpUnregister = func(server *pluginmcp.Server) error {
		return server.Unregister()
	}
)

func (p *Plugin) ensureMCPServer() error {
	p.mcpServerLock.Lock()
	defer p.mcpServerLock.Unlock()

	if p.mcpServer != nil {
		return nil
	}

	if manifest.Id == "" || manifest.Version == "" || strings.TrimSpace(manifest.Name) == "" {
		return errors.New(errcode.WithCode(errcode.MCPManifestIncomplete,
			"the manifest is missing an id, a version or a name"))
	}

	server := mcpNewServer(p.API, pluginmcp.Config{
		PluginID:       manifest.Id,
		Name:           manifest.Name + " MCP",
		Path:           mcpPath,
		ExposeExternal: true,
		Version:        manifest.Version,
	})

	p.registerMCPTools(server)
	p.mcpServer = server

	return nil
}

func (p *Plugin) registerMCPServerBestEffort() {
	server := p.currentMCPServer()
	if server == nil {
		return
	}

	if err := mcpRegister(server); err != nil {
		p.API.LogWarn("MCP registration unavailable; continuing activation",
			"error_code", errcode.MCPRegistrationFailed,
			"error", err.Error())
	}
}

func (p *Plugin) unregisterMCPServerBestEffort() {
	server := p.currentMCPServer()
	if server == nil {
		return
	}

	if err := mcpUnregister(server); err != nil {
		p.API.LogWarn("MCP unregister failed; continuing shutdown",
			"error_code", errcode.MCPUnregisterFailed,
			"error", err.Error())
	}
}

func (p *Plugin) serveMCPIfMatch(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != mcpPath && !strings.HasPrefix(r.URL.Path, mcpPath+"/") {
		return false
	}

	server := p.currentMCPServer()
	if server == nil || p.decorators == nil || !p.configurationLoaded() {
		decorators.WriteError(w, http.StatusServiceUnavailable,
			errcode.WithCode(errcode.MCPNotReady, "Not ready."))
		return true
	}

	server.ServeHTTP(w, r)

	return true
}

func (p *Plugin) currentMCPServer() *pluginmcp.Server {
	p.mcpServerLock.RLock()
	defer p.mcpServerLock.RUnlock()

	return p.mcpServer
}
