package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	agentsPluginID    = "mattermost-ai"
	fusionPluginID    = "com.mattermost.plugin-tactical-fusion"
	autoRunEverywhere = "auto_run_everywhere"
	serviceID         = "tactical-fusion-openai"
	accessAll         = 0
)

var (
	siteURL  = flag.String("url", envOr("MM_SITEURL", "http://localhost:8065"), "the Mattermost server to configure")
	username = flag.String("username", envOr("MM_ADMIN_USERNAME", "admin"), "a system admin to sign in as")
	password = flag.String("password", envOr("MM_ADMIN_PASSWORD", "password"), "that admin's password")
	botName  = flag.String("bot", "fusion", "the agent's username, mentioned as @name")
	model    = flag.String("model", envOr("AGENT_MODEL", "gpt-5.5"), "the OpenAI model the agent uses")
)

const instructions = "You are Fusion, the Tactical Fusion assistant in this Mattermost workspace. " +
	"Use the Tactical Fusion tools to answer questions about coordinates, date-time groups, airfields, aviation reports and NOTAMs, " +
	"Cursor on Target, GeoJSON, radio frequencies and security indicators (CVEs, CWEs, MITRE ATT&CK ids, IP addresses and file hashes). " +
	"Cite what a tool returns, link indicators with the links it gives you, and say plainly when a dataset is not installed rather than guessing."

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func main() {
	flag.Parse()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("  OPENAI_API_KEY is not set, so no @" + *botName + " agent was configured")
		return
	}

	if err := run(apiKey); err != nil {
		fmt.Fprintln(os.Stderr, "  could not configure the @"+*botName+" agent: "+err.Error())
		os.Exit(1)
	}
}

type client struct {
	http  *http.Client
	base  string
	token string
}

func (c *client) do(method, path string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusMultipleChoices {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return resp.StatusCode, fmt.Errorf("%s %s answered %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	if out != nil {
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, nil
}

func (c *client) login() error {
	req, err := http.NewRequest(http.MethodPost, c.base+"/api/v4/users/login",
		strings.NewReader(fmt.Sprintf(`{"login_id":%q,"password":%q}`, *username, *password)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("signing in as %s answered %d", *username, resp.StatusCode)
	}
	c.token = resp.Header.Get("Token")
	if c.token == "" {
		return errors.New("signing in returned no token")
	}
	return nil
}

func listOf(config map[string]any, key string) []any {
	list, _ := config[key].([]any)
	return list
}

func matches(current map[string]any, want map[string]any) bool {
	for key, value := range want {
		if fmt.Sprint(current[key]) != fmt.Sprint(value) {
			return false
		}
	}
	return true
}

func upsertService(config map[string]any, want map[string]any) bool {
	services := listOf(config, "services")
	for _, entry := range services {
		service, ok := entry.(map[string]any)
		if !ok || service["id"] != want["id"] {
			continue
		}
		if matches(service, want) {
			return false
		}
		maps.Copy(service, want)
		return true
	}
	config["services"] = append(services, want)
	return true
}

func switchOn(settings map[string]any, key string) bool {
	if on, _ := settings[key].(bool); on {
		return false
	}
	settings[key] = true
	return true
}

func (c *client) ensureConfig(apiKey string) (bool, error) {
	configPath := "/plugins/" + agentsPluginID + "/admin/config"

	var config map[string]any
	if _, err := c.do(http.MethodGet, configPath, nil, &config); err != nil {
		return false, err
	}

	changed := upsertService(config, map[string]any{
		"id":              serviceID,
		"name":            "OpenAI (Tactical Fusion)",
		"type":            "openai",
		"apiKey":          apiKey,
		"defaultModel":    *model,
		"useResponsesAPI": true,
	})

	if switchOn(config, "enableChannelMentionToolCalling") {
		changed = true
	}

	mcpSettings, ok := config["mcp"].(map[string]any)
	if !ok {
		mcpSettings = map[string]any{}
		config["mcp"] = mcpSettings
	}
	if switchOn(mcpSettings, "enablePluginServer") {
		changed = true
	}

	if !changed {
		return false, nil
	}
	_, err := c.do(http.MethodPut, configPath, config, nil)
	return true, err
}

func (c *client) ensureAgent() (string, error) {
	want := map[string]any{
		"username":                *botName,
		"displayName":             "Fusion",
		"serviceID":               serviceID,
		"model":                   *model,
		"customInstructions":      instructions,
		"structuredOutputEnabled": true,
		"enableVision":            true,
		"disableTools":            false,
		"channelAccessLevel":      accessAll,
		"userAccessLevel":         accessAll,
		"reasoningEnabled":        true,
		"reasoningEffort":         "medium",
		"autoEnableNewMCPTools":   true,
	}

	agentsPath := "/plugins/" + agentsPluginID + "/agents"
	var agents []map[string]any
	if _, err := c.do(http.MethodGet, agentsPath, nil, &agents); err != nil {
		return "", err
	}

	for _, agent := range agents {
		if agent["name"] != *botName {
			continue
		}
		current := maps.Clone(agent)
		current["username"] = agent["name"]
		if matches(current, want) {
			return "already current", nil
		}
		id, _ := agent["id"].(string)
		body := maps.Clone(agent)
		maps.Copy(body, want)
		if _, err := c.do(http.MethodPut, agentsPath+"/"+id, body, nil); err != nil {
			return "", err
		}
		return "updated", nil
	}

	if _, err := c.do(http.MethodPost, agentsPath, want, nil); err != nil {
		return "", err
	}
	return "created", nil
}

type toolConfig struct {
	Name    string `json:"name"`
	Policy  string `json:"policy"`
	Enabled bool   `json:"enabled"`
}

type mcpServer struct {
	URL         string       `json:"url"`
	Enabled     bool         `json:"enabled"`
	ToolConfigs []toolConfig `json:"toolConfigs"`
	Tools       []struct {
		Name string `json:"name"`
	} `json:"tools"`
}

func (c *client) ensureToolsAutoRun() (string, error) {
	var listing struct {
		Servers []mcpServer `json:"servers"`
	}
	if _, err := c.do(http.MethodGet, "/plugins/"+agentsPluginID+"/admin/mcp/tools", nil, &listing); err != nil {
		return "", err
	}

	for _, server := range listing.Servers {
		if server.URL != "plugin://"+fusionPluginID+"/mcp" {
			continue
		}

		existing := map[string]toolConfig{}
		for _, config := range server.ToolConfigs {
			existing[config.Name] = config
		}

		changed := !server.Enabled
		configs := make([]toolConfig, 0, len(server.Tools))
		for _, tool := range server.Tools {
			config, known := existing[tool.Name]
			if !known {
				config = toolConfig{Name: tool.Name, Enabled: true}
			}
			if config.Policy != autoRunEverywhere {
				config.Policy = autoRunEverywhere
				changed = true
			}
			configs = append(configs, config)
		}

		if !changed {
			return "all " + fmt.Sprint(len(configs)) + " tools already auto run everywhere", nil
		}
		body := map[string]any{"enabled": true, "tool_configs": configs}
		if _, err := c.do(http.MethodPut, "/plugins/"+agentsPluginID+"/admin/mcp/plugin-servers/"+fusionPluginID, body, nil); err != nil {
			return "", err
		}
		return "all " + fmt.Sprint(len(configs)) + " tools set to auto run everywhere", nil
	}

	return "the Tactical Fusion MCP server is not registered with Agents yet, so its tools were left alone", nil
}

func run(apiKey string) error {
	c := &client{http: &http.Client{Timeout: 30 * time.Second}, base: strings.TrimRight(*siteURL, "/")}
	if err := c.login(); err != nil {
		return err
	}

	if status, err := c.do(http.MethodGet, "/plugins/"+agentsPluginID+"/services", nil, nil); err != nil {
		if status == http.StatusNotFound {
			fmt.Println("  the Agents plugin is not running, so no @" + *botName + " agent was configured")
			return nil
		}
		return err
	}

	changed, err := c.ensureConfig(apiKey)
	if err != nil {
		return err
	}
	serviceNote := "Agents settings current"
	if changed {
		serviceNote = "Agents settings saved (OpenAI service, channel mention tool calling, Mattermost MCP server over HTTP)"
	}

	outcome, err := c.ensureAgent()
	if err != nil {
		return err
	}
	fmt.Println("  " + serviceNote + "; @" + *botName + " agent " + outcome + " (" + *model + ", structured output on)")

	tools, err := c.ensureToolsAutoRun()
	if err != nil {
		return err
	}
	fmt.Println("  Tactical Fusion MCP: " + tools)
	return nil
}
