---
name: add-agents-mcp-server
description: Add an MCP (Model Context Protocol) server to Tactical Fusion so the Mattermost Agents plugin can call its tools. Use when exposing decoration, coordinate conversion or airfield lookup to Agents' LLM agents, or when wiring up the `pluginmcp` helper from mattermost-plugin-agents.
user-invocable: true
allowed-tools: Read, Write, Edit, Bash, Glob, Grep, WebFetch
---

# Add Agents MCP Server

Add a cross-plugin MCP (Model Context Protocol) server to this plugin so the
[Mattermost Agents plugin](https://github.com/mattermost/mattermost-plugin-agents)
can discover and call its tools. Uses the
`github.com/mattermost/mattermost-plugin-agents/external/pluginmcp` helper, which
handles tool-name namespacing, inter-plugin auth, user-ID propagation, and async
registration retries.

MCP is a **third transport onto operations this plugin already has**, alongside
`/bridge/v1` (another plugin's server) and `/api/v1` (a session-bearing browser).
Treat it the way `docs/design/bridge.md` treats those two: the transport decides
who may ask, never what the answer is built from.

## Read first

- [`docs/design/bridge.md`](../../docs/design/bridge.md): the existing
  cross-plugin surface, why `PluginHTTP`, why `Mattermost-Plugin-ID` is trusted,
  and the rule that neither transport owns a decoration rule of its own.
- [`docs/design/help-and-errors.md`](../../docs/design/help-and-errors.md): the
  `TF-NNNN` catalog and the four edits that add a code.
- [`docs/design/admin-settings.md`](../../docs/design/admin-settings.md): the
  switch pattern, if this ships behind one.
- Root [`CLAUDE.md`](../../CLAUDE.md): the invariants, the no-comment rule, and
  the worktree workflow.

## Prerequisites

Already satisfied in this repo, confirm they have not moved:

- **`min_server_version`** is `11.8.0` in `plugin.json`, above the `11.3` the
  helper's `Plugin.PluginHTTPStream` needs. No bump required.
- **Go** is `1.26.7` in the single root `go.mod`. There is no `server/go.mod`;
  the module is `github.com/MattermostFederal/mattermost-plugin-tactical-fusion`
  and everything under `server/` is `package main`.
- **The plugin id** is `com.mattermost.plugin-tactical-fusion`, reverse-DNS and
  free of `__`. Double-underscore is the namespace separator on the Agents side.
- **`ServeHTTP`** lives in `server/http.go` and dispatches by path prefix.
- **There is no `pluginapi` client.** Tool handlers call `p.API` directly, the
  way every other handler here does.

No `plugin.json` changes are required for MCP itself. A switch (Phase 8) is a
separate decision.

## Blocking gate: dependency licensing

The bundle ships to customers, so **verify the license of
`mattermost-plugin-agents` and `modelcontextprotocol/go-sdk` before adding
either**, including anything they pull in transitively. Root `CLAUDE.md` forbids
GPL/AGPL/SSPL outright and puts LGPL/MPL/EPL off limits by default.

Mattermost's first-party plugins are not uniformly licensed, and some carry a
source-available license rather than Apache 2.0. Do not assume. Check the
upstream `LICENSE` file, then run `make license-check`, which reads
`.licenses.json` against the SBOMs and fails the build on a violation.

If the license is copyleft, source-available, unclear or unstated: **stop and
ask.** Present alternatives and tradeoffs rather than adding it or writing an
`.licenses.json` exception unilaterally. An exception is keyed to one component
and one license, with a reason, and is the maintainer's call.

`make bundle` also writes `THIRD-PARTY-NOTICES.txt` from each dependency's own
license text and fails on one that ships none, so a dependency that publishes no
license file needs a `noticeFallbacks` entry.

## Instructions

### Phase 0: Prepare a worktree

This repo uses the bare-repo plus worktree layout. **Never work in `main/`.**

1. From the project root, confirm the tree is clean and refresh the trunk.
2. `git --git-dir=.bare worktree add agents-mcp main -b feat/agents-mcp`
3. Add the new worktree to the `folders` array in `project.code-workspace`.
4. Work from `agents-mcp/` for everything below.

### Phase 1: Add dependencies

Edit the root `go.mod`:

```
require (
    github.com/mattermost/mattermost-plugin-agents v0.0.0-<commit-from-main>
    github.com/modelcontextprotocol/go-sdk v1.4.1
)
```

Then `go mod tidy` from the repo root, and `make license-check`.

If no tagged release exports `external/pluginmcp/`, ask the maintainer before
pinning a pseudo-version or adding a `replace` against a local checkout. A
`replace` must not reach a release build.

### Phase 2: Add MCP server fields to the Plugin struct

In `server/plugin.go`, beside `configurationLock` and `packageLock`:

```go
mcpServerLock sync.RWMutex
mcpServer     *pluginmcp.Server
```

The lock guards lazy initialization in `OnActivate` against concurrent reads
from `ServeHTTP`. The struct's existing comment about `decorators` not
establishing a happens-before edge applies here too, which is why this field
gets a real lock rather than a bare pointer.

### Phase 3: Create `server/mcp.go`

This file owns the MCP server lifecycle. Indirect `pluginmcp.NewServer`,
`Register` and `Unregister` through package-level `var`s so tests can substitute
them without a live Agents plugin.

Follow this repo's error handling: `errors.Wrap` from `github.com/pkg/errors`,
every message through `errcode.WithCode`, and every `p.API.Log*` call carrying
its code as an `error_code` field. **Write no prose comments** (root
`CLAUDE.md`); rationale goes in `docs/design/mcp.md` per Phase 9.

```go
package main

import (
    "net/http"
    "strings"

    "github.com/mattermost/mattermost-plugin-agents/external/pluginmcp"
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
            "err", err.Error())
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
            "err", err.Error())
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
```

The readiness check mirrors `serveBridgeOperation`, which refuses with
`refusedNotReady` when the registry or the configuration has not landed. A tool
answering off a nil registry is the same defect on a different transport.

`Register()` returns immediately and retries asynchronously (1s, 2s, 4s, 8s, up
to 15 attempts) until the Agents plugin acknowledges, so activation is never
blocked on a plugin that is not up yet.

### Phase 4: Create `server/mcp_tools.go`

**Reuse the operations, do not reimplement them.** `decorateText` and
`buildLink` in `server/bridge.go` are already shared by both existing
transports, and `TestBridgeLinkIsTheLinkTheTaggerWrites` holds them to what the
post hook writes. Call those. For conversion and airfield lookup, the logic
currently sits inside `serveConvert` and `serveAirport` in `server/api.go`:
extract the pure part into a function both the HTTP handler and the tool call,
rather than growing a third copy that can drift.

Tool names are prefixed automatically. Here `com.mattermost.plugin-tactical-fusion`
sanitizes to `com_mattermost_plugin-tactical-fusion__`, so the LLM sees
`com_mattermost_plugin-tactical-fusion__decorate_text` while you write
`decorate_text`.

Carry `FitsPost` through rather than dropping it. An agent that pastes a
decorated message into a post needs to know it will fit; `bridgeclient` exposes
it for exactly that reason.

```go
package main

import (
    "context"

    "github.com/mattermost/mattermost-plugin-agents/external/pluginmcp"
    "github.com/modelcontextprotocol/go-sdk/mcp"

    "github.com/MattermostFederal/mattermost-plugin-tactical-fusion/bridgeclient"
)

type DecorateTextArgs struct {
    Message string `json:"message" jsonschema:"The message text to run the tagger over,minLength=1"`
}

type DecorateTextOutput struct {
    Message  string `json:"message" jsonschema:"The message with recognized tokens rewritten as links"`
    Changed  bool   `json:"changed" jsonschema:"Whether any token was linked"`
    FitsPost bool   `json:"fits_post" jsonschema:"Whether the result fits the smallest post size limit any Mattermost server enforces"`
}

func (p *Plugin) registerMCPTools(server *pluginmcp.Server) {
    pluginmcp.AddTool(server, &mcp.Tool{
        Name:        "decorate_text",
        Description: "Rewrite recognized coordinates, date-time groups and ICAO airfield codes in a message as Tactical Fusion links.",
    }, p.decorateTextTool)
}

func (p *Plugin) decorateTextTool(_ context.Context, _ *mcp.CallToolRequest, in DecorateTextArgs) (*mcp.CallToolResult, DecorateTextOutput, error) {
    res := p.decorateText(bridgeclient.DecorateRequest{Message: in.Message})

    return nil, DecorateTextOutput{
        Message:  res.Message,
        Changed:  res.Changed,
        FitsPost: res.FitsPost,
    }, nil
}
```

Handler signature is the go-sdk's `mcp.ToolHandlerFor[In, Out]`:

```
func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)
```

Return `(nil, out, nil)` and the helper packs `out` into a `CallToolResult`.
Return a non-nil `*mcp.CallToolResult` to control the response fully
(multi-content replies, `IsError`).

Candidate tools, in rough order of value: `decorate_text`, `link_token`,
`convert_coordinate`, `lookup_airfield`. Aim for **about 10 tools maximum** with
union-typed args rather than many narrow ones: every tool costs schema tokens in
every LLM request.

### Phase 5: Wire into `OnActivate` and `OnDeactivate`

In `server/plugin.go`, after the registry, the preferences store and
`RegisterCommand`, since a tool handler may depend on all three:

```go
if err := p.ensureMCPServer(); err != nil {
    return errors.Wrap(err, errcode.WithCode(errcode.MCPInitFailed,
        "failed to initialize the MCP server"))
}
p.registerMCPServerBestEffort()
```

There is no `OnDeactivate` in this repo today. Adding one for
`unregisterMCPServerBestEffort` is fine; keep it to that, and keep it
unconditional so a failure elsewhere does not leave a stale registration on the
Agents side.

`Register()` errors are logged and swallowed. A down Agents plugin must never
fail this plugin's activation.

### Phase 6: Route MCP requests in `ServeHTTP`

In `server/http.go`, add the match **after the `/bridge/v1` prefix and before
the `r.Method != http.MethodGet` refusal**. MCP is a POST transport, so a match
placed below that check would be answered with 405:

```go
if strings.HasPrefix(r.URL.Path, bridgePath+"/") {
    p.serveBridge(w, r)
    return
}

if p.serveMCPIfMatch(w, r) {
    return
}

if r.Method != http.MethodGet {
```

It also has to sit **above the session gate**. A plugin request carries no
session, and the gate redirects to a login page, which an MCP client would read
as a malformed response rather than an auth failure.

Do **not** add an auth gate of your own around `serveMCPIfMatch`.
`pluginmcp.Server.ServeHTTP` already rejects requests without
`Mattermost-Plugin-ID: mattermost-ai`, a header Mattermost strips from external
requests, so only inter-plugin RPC can set it. This is the same trust argument
`callingPluginID` in `server/bridge.go` rests on.

### Phase 7: Allocate error codes

Every user-facing failure and every `p.API.Log*` call carries a `TF-NNNN`. The
ranges in `server/errcode/codes.go` run out at `19999`, so `server/mcp.go` takes
a new one. That is **five** edits, not the usual four:

1. Add `20000-20999   server/mcp.go   the Agents MCP server` to the allocation
   table in the package doc comment.
2. Declare the constants (`MCPInitFailed`, `MCPManifestIncomplete`,
   `MCPRegistrationFailed`, `MCPUnregisterFailed`, `MCPNotReady`, and one per
   tool-level refusal).
3. Add every one to `AllCodes`, in the same order as the const block.
4. Widen the bound in `TestCodesAreInAKnownRange`
   (`server/errcode/codes_test.go`), which currently rejects anything `>= 20000`.
5. Add a section and a row per code to `public/help/error-codes.html`, matching
   the `<h2 id="bridge">` block's shape. `TestEveryCodeIsDocumented` fails
   without it.

Anchor ids in `public/help/` are a contract, and those pages must stay
light-only, self-contained and functional with scripting off.

### Phase 8: Decide on an admin switch

Ask the maintainer whether MCP ships behind `EnableMCP`. It is not automatic:
the bridge has no switch, on the argument that it answers only what the format
switches already allow.

If it gets one, follow the established pattern exactly: a field on
`configuration` in `server/configuration.go` that is **false at zero**, a new
section in `plugin.json`'s `settings_schema.sections`, and an accessor on
`Plugin` that reads fresh rather than capturing at activation. Then update the
counts in root `CLAUDE.md` ("twenty-five switches", six sections),
`docs/design/admin-settings.md`, and `public/help/`.

Note that a switch read at registration time behaves unlike the format switches:
turning it off will not retract a registration the Agents plugin already holds
until this plugin re-activates. Say so wherever the switch is documented.

Do not hand-edit `plugin.json`'s `version`; release-please owns it. And
`plugin.json` may not contain a backtick.

### Phase 9: Write the design note

Create `docs/design/mcp.md` and add a row to the table in root `CLAUDE.md`.
Nothing in this feature may be explained by a code comment. At minimum record:

- Why MCP is a third transport rather than a route under `/bridge/v1`.
- **The user-scoping rule.** The bridge invariant is that `/bridge/v1` may never
  answer with per-user or per-channel data, because a plugin request proves no
  reader. MCP is the deliberate exception: Agents propagates a user id, and
  `pluginmcp.GetUserID(ctx)` is the only trustworthy way to read it. A tool that
  wants a reader must go through that function and must refuse when it returns
  `""`. Never read `X-Mattermost-UserID` from the headers directly. Consider
  adding this to the invariants list in root `CLAUDE.md`, next to the existing
  bridge invariant it qualifies.
- Why tools call `decorateText`/`buildLink` rather than their own logic.
- Why registration failure is a warn rather than a failed activation.
- The tool budget, and what "about ten" is protecting.

### Phase 10: Tests

Add `server/mcp_test.go` and `server/mcp_tools_test.go`. Put the invariant in
the test name, the way this repo does (`TestRoundToNormalizesNegativeZero`), not
in a comment above it. Cover at least:

- `TestMCPRouteIsMatchedBeforeTheMethodCheck`: a POST to `/mcp` is not answered
  with 405.
- `TestMCPRefusesWhenTheRegistryIsNotReady`.
- `TestMCPDecorateMatchesTheTaggersOutput`: the tool's answer equals
  `decorateText` for the same input, the analogue of
  `TestBridgeLinkIsTheLinkTheTaggerWrites`.
- `TestMCPToolsRefuseAnAbsentUserID` for any user-scoped tool.
- A round-trip through `httptest.Server` with
  `Mattermost-Plugin-ID: mattermost-ai` set, using the go-sdk client for
  `ListTools` and `CallTool`.

Override `mcpNewServer` / `mcpRegister` / `mcpUnregister` rather than reaching
for a live Agents plugin. A test that wants "every tool" must read the registry
rather than list the names.

### Phase 11: Help docs

If any of this is user-visible or admin-visible, update `public/help/`. Check
`integration.html`, which documents the bridge today and is the natural home for
a second integration surface, plus `error-codes.html` from Phase 7.

### Phase 12: Verify

1. `make check-style`
2. `make test` (depends on `map-data-check`; that ordering is load-bearing)
3. `make sbom-audit`, which runs `license-check` alongside the CVE scan
4. `make dist`
5. End to end: `make docker-setup` then `make deploy` (Mattermost on `:8065`,
   `admin`/`password`). Install the Agents plugin from a build with cross-plugin
   MCP support, open its system console **Tools** tab, and confirm the tools
   appear as `com_mattermost_plugin-tactical-fusion__<name>` with per-tool policy
   controls. Tail `make docker-logs` for `Connected to plugin MCP server
   com.mattermost.plugin-tactical-fusion`; its absence means `Register()` never
   succeeded.

Commit with a conventional subject, `feat:` for the feature. No Claude
attribution, no em dashes, US spelling. A test enforces the last two.

## Configuration reference

```go
type Config struct {
    PluginID       string // required; must equal plugin.json "id"
    Name           string // human-readable; shown in admin UI
    Path           string // this plugin's MCP endpoint, "/mcp"
    ExposeExternal bool   // if true, tools may appear on Agents' external MCP aggregate
                          // (still subject to admin Enabled toggle and per-tool policy)
    Version        string // optional; defaults to "0.0.1"
}
```

## API reference

- `pluginmcp.NewServer(api, cfg) *Server`: `p.API` satisfies the `PluginAPI`
  interface.
- `pluginmcp.AddTool[In, Out](s, tool, handler)`: a free function, not a method,
  because Go disallows type parameters on methods.
- `(*Server).ServeHTTP(w, r)`: an `http.Handler`; route to it for requests under
  `cfg.Path`.
- `(*Server).Register() error`: starts async registration, returns immediately,
  retries in a goroutine.
- `(*Server).Unregister() error`: synchronously cancels pending retries and POSTs
  one unregister.
- `pluginmcp.GetUserID(ctx) string`: returns the user id stashed by `ServeHTTP`,
  or `""` if absent.

## Constraints and gotchas

- **Route above the method check and above the session gate.** MCP is POST, and
  a plugin request carries no session. See Phase 6.
- **`GetUserID` is the only user source.** It is trustworthy only inside a
  request that arrived through `pluginmcp.Server.ServeHTTP`; external callers
  cannot inject one. A tool that reads the header directly is a disclosure bug.
- **Tool-name sanitization.** `AddTool` prepends `{sanitizedPluginID}__`,
  replacing any character outside `[A-Za-z0-9_-]` with `_` to satisfy
  `^[a-zA-Z0-9_-]{1,128}$`. An already-prefixed name is not double-prefixed.
- **Do not double-gate auth.** A plugin-id check of your own around
  `serveMCPIfMatch` is redundant and will likely break the helper's.
- **Registration is one-shot per `OnActivate`.** If the Agents plugin restarts,
  admin-persisted entries come back but a never-saved registration returns only
  when this plugin re-activates. Permanent errors log `registration with Agents
  plugin failed permanently` and stop.
- **Tool budget.** Roughly 20 to 200 schema tokens per tool in every LLM
  request. Prefer union-typed args over many narrow tools.
- **`ExposeExternal` vs admin `Enabled`.** Each register POST sends
  `expose_external` from `Config`. Admins still control the server's `Enabled`
  state and per-tool policy in the Agents console, and those survive
  re-registration.
- **No webapp half.** MCP is server-only, so none of the cross-language sync
  tests in root `CLAUDE.md` apply. Do not add a `webapp/src/mcp/` without a
  guard test to match.

## Troubleshooting

- **Tool missing from the admin Tools tab.** Look for `Connected to plugin MCP
  server <pluginID>` in the Agents log. Absence means `Register()` was never
  called or kept failing; the retry loop logs `gave up after N attempts` on
  terminal failure and `failed permanently` on a non-retriable 4xx.
- **`GetUserID` returns `""`.** Either the request did not go through
  `pluginmcp.Server.ServeHTTP` (typical in unit tests, where you inject a context
  yourself), or `ServeHTTP` is not routing to it: check `mcpPath` matches
  `Config.Path`.
- **Registration keeps retrying.** Agents disabled, in a crash loop, or
  `cfg.PluginID` does not match `plugin.json`'s `id` (Agents returns a
  non-retriable 403).
- **405 on a local POST to `/mcp`.** The match is below the `MethodGet` refusal
  in `ServeHTTP`. See Phase 6.
- **A login redirect instead of an MCP response.** The match is below the
  session gate in `ServeHTTP`.
- **403 during a local curl.** Expected. Mattermost strips
  `Mattermost-Plugin-ID` from external requests. Test through the Agents plugin
  or a unit test that sets the header.

## References

- Helper package: `mattermost-plugin-agents/external/pluginmcp/` (`README.md`,
  `pluginmcp.go`, `server.go`, `tools.go`, `context.go`, `registration.go`).
- Reference implementation: `mattermost-plugin-demo` branch
  `IDEA-006-cross-plugin-mcp`, specifically `server/mcp.go`,
  `server/mcp_tools.go`, `server/activate_hooks.go`, `server/http_hooks.go` and
  `server/mcp_tools_test.go`. Its layout differs from this repo's: it has a
  separate `server/go.mod`, a `pluginapi` client, and split hook files. Take the
  shape, not the paths.
- MCP Go SDK: <https://github.com/modelcontextprotocol/go-sdk>.
