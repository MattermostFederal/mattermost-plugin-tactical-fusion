# bridgeclient

A Go client for the Tactical Fusion plugin bridge. Import it from **another
Mattermost plugin's server** to build Tactical Fusion decorator links and embed
them in whatever your plugin writes: bot posts, ephemeral replies, slash command
responses, interactive dialogs, or any other markdown a Mattermost client renders.

A decorator link is ordinary markdown, `[label](url)`. Rendered by Mattermost it
behaves exactly like a link Tactical Fusion wrote itself:

- a **hover card** in the web app (the countdown for a time, the map for a
  coordinate),
- a **click** that opens the Tactical Fusion sidebar panel with every reading,
- a **standalone page** on clients without the web app bundle, such as mobile.

The complete contract, including the raw HTTP shape for callers not written in Go
and the web app API, is the **Plugin Integration** page of the plugin's built-in
documentation, at
`/plugins/com.mattermost.plugin-tactical-fusion/public/help/integration.html` on
any server with the plugin installed, and in this repository at
[`public/help/integration.html`](../public/help/integration.html).

## Requirements

| | |
|---|---|
| Tactical Fusion | installed and enabled on the same server, any version serving `/bridge/v1` |
| Mattermost | 11.8 or later (Tactical Fusion's own minimum); `PluginHTTP` itself is 5.18+ |
| Go | this module's `go` directive (currently 1.26.7); the package imports only the standard library |

Your plugin does **not** need Tactical Fusion at build time beyond this package,
and should keep working when it is absent: every call returns
`ErrPluginNotActive` then, and you fall back to the plain token.

## Install

```sh
go get github.com/MattermostFederal/mattermost-plugin-tactical-fusion/bridgeclient@latest
```

## Create a client

`NewClient` takes anything with a `PluginHTTP` method, which a plugin's `p.API`
has. A `Client` holds no state beyond that and is safe for concurrent use, so
create one in `OnActivate` and keep it.

```go
import "github.com/MattermostFederal/mattermost-plugin-tactical-fusion/bridgeclient"

type Plugin struct {
    plugin.MattermostPlugin
    tacticalFusion *bridgeclient.Client
}

func (p *Plugin) OnActivate() error {
    p.tacticalFusion = bridgeclient.NewClient(p.API)
    return nil
}
```

## The three calls

### `Link`: one link for a token you already know

Use it when your plugin has a structured value (a field in a record, an API
response) and knows what kind of value it is.

```go
link, err := p.tacticalFusion.Link(ctx, bridgeclient.LinkRequest{
    Type:  bridgeclient.TypeLocation,
    Token: "18S UJ 23478 06483",
    Label: "Rally point",
})
```

```go
link.Markdown // "[Rally point](/plugins/com.mattermost.plugin-tactical-fusion/decorate/location?f=mgrs&r=18S+UJ+23478+06483&v=18SUJ2347806483)"
link.URL      // "/plugins/com.mattermost.plugin-tactical-fusion/decorate/location?f=mgrs&r=18S+UJ+23478+06483&v=18SUJ2347806483"
link.Type     // "location"
link.Label    // "Rally point"
```

| Field | Required | Meaning |
|---|---|---|
| `Type` | yes | `TypeDTG`, `TypeLocation`, `TypeAirport` or `TypeAvReport` |
| `Token` | yes | The value **without** a field label: `PHIK`, not `ICAO:PHIK`. Surrounding whitespace is trimmed |
| `Label` | no | Link text; defaults to the trimmed token. Markdown characters are escaped for you. May not contain a line break |
| `ReferenceTime` | no | Unix milliseconds. Supplies the month and year of a short date-time group such as `091630Z`. Zero means now |

Tokens each type accepts:

| Type | Examples |
|---|---|
| `TypeDTG` | `141200ZSEP26`, `141200ZSEP2026`, `141200Z`, `2026-08-09T16:30:00Z`, `2026-08-09T20:30:00+04:00` |
| `TypeLocation` | `34.0561, -118.2500`, `34.0561 N, 118.2500 W`, `3510N07901W`, `18S UJ 23478 06483`, `18SUJ2347806483`, `11S 384640E 3769080N` (UTM, off by default), `GJPJ3718` (GEOREF), `006AG39` (GARS), `849VCWC8+R9` (Plus Code) |
| `TypeAirport` | `PHIK`, `KIND`, any four-letter ICAO ident in the plugin's database |
| `TypeAvReport` | `METAR PHNL 221651Z 07012KT 10SM CLR 27/19 A3010`, a METAR, SPECI, TAF or FAA-format NOTAM on one line; `ReferenceTime` supplies the month and year |

The full grammars are on the plugin's **Recognized Formats** help page.

### `Decorate`: every token in a piece of text

Use it when your plugin produces free text (a generated report, a relayed
message) and you want every time, coordinate and airfield in it linked, by the
same rules Tactical Fusion applies to a posted message.

```go
decorated, err := p.tacticalFusion.Decorate(ctx, bridgeclient.DecorateRequest{
    Message: "Convoy departs 141200ZSEP26 from ICAO:PHIK to 18S UJ 23478 06483",
})
```

```text
Convoy departs [141200ZSEP26](/plugins/com.mattermost.plugin-tactical-fusion/decorate/dtg?a=&dtg=141200ZSEP26&t=1789387200000&z=Z) from ICAO:[PHIK](/plugins/com.mattermost.plugin-tactical-fusion/decorate/airport?v=PHIK) to [18S UJ 23478 06483](/plugins/com.mattermost.plugin-tactical-fusion/decorate/location?f=mgrs&r=18S+UJ+23478+06483&v=18SUJ2347806483)
```

| Response field | Meaning |
|---|---|
| `Message` | The text with links written in. Markdown; never escape it |
| `Changed` | Whether anything was linked |
| `FitsPost` | Whether `Message` fits 4,000 runes, the smallest post limit any Mattermost server enforces. Links are long, so text that fits before decoration may not after. If it is `false` and you are posting, post your original text instead |

Unlike `Link`, `Decorate` follows the posted-message rules exactly: an airfield
still needs its `ICAO:` label, and tokens inside code, links and URLs are left
alone. Decorating text that is already decorated changes nothing.

### `Info`: what the installed plugin offers

```go
info, err := p.tacticalFusion.Info(ctx)
// info.PluginVersion "0.5.0"
// info.APIVersion    1
// info.Types         ["dtg" "location" "airport" "avreport"]
// info.EnabledTypes  the types an administrator has left on
```

## Errors

Every call returns an `*Error` when Tactical Fusion refused or did not answer.
Test for the cases you handle with `errors.Is`, and use `errors.As` for the
details.

| Sentinel | When | What to do |
|---|---|---|
| `ErrPluginNotActive` | Tactical Fusion is not installed, not enabled, or did not answer | Output the plain token |
| `ErrNotRecognized` | The token is not a value of that type (reason `not_recognized`) | Output the plain token |
| `ErrDisabled` | The token is valid but its format is switched off by an admin (reason `disabled`) | Output the plain token |
| `ErrUnknownType` | `Type` is not a decorator type (reason `unknown_type`) | A bug in the caller |

```go
text := "Recovery field PHIK"
link, err := p.tacticalFusion.Link(ctx, bridgeclient.LinkRequest{Type: bridgeclient.TypeAirport, Token: "PHIK"})
switch {
case err == nil:
    text = "Recovery field " + link.Markdown
case errors.Is(err, bridgeclient.ErrPluginNotActive),
    errors.Is(err, bridgeclient.ErrNotRecognized),
    errors.Is(err, bridgeclient.ErrDisabled):
default:
    p.API.LogWarn("Tactical Fusion link failed", "error", err.Error())
}
```

```go
var bridgeErr *bridgeclient.Error
if errors.As(err, &bridgeErr) {
    bridgeErr.StatusCode // HTTP status, 0 when there was no response
    bridgeErr.Code       // numeric TF code, e.g. 19007; 0 when the answer was not Tactical Fusion's
    bridgeErr.Reason     // "unknown_type", "not_recognized", "disabled" or ""
    bridgeErr.Message    // human readable, ends "(TF-19007)"
}
```

The codes are listed on the plugin's **Error Codes** help page, under "Plugin
bridge".

## Worked examples

### A bot post

```go
func (p *Plugin) postArrival(ctx context.Context, channelID, icao string, eta time.Time) error {
    field := icao
    if link, err := p.tacticalFusion.Link(ctx, bridgeclient.LinkRequest{
        Type: bridgeclient.TypeAirport, Token: icao,
    }); err == nil {
        field = link.Markdown
    }

    when := eta.UTC().Format("021504Z") + strings.ToUpper(eta.UTC().Format("Jan06"))
    if link, err := p.tacticalFusion.Link(ctx, bridgeclient.LinkRequest{
        Type: bridgeclient.TypeDTG, Token: when,
    }); err == nil {
        when = link.Markdown
    }

    _, appErr := p.API.CreatePost(&model.Post{
        UserId:    p.botUserID,
        ChannelId: channelID,
        Message:   "Inbound to " + field + ", wheels down " + when,
    })
    if appErr != nil {
        return appErr
    }
    return nil
}
```

### An ephemeral slash command reply

```go
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
    report := p.buildReport(args.ChannelId)

    decorated, err := p.tacticalFusion.Decorate(context.Background(), bridgeclient.DecorateRequest{Message: report})
    if err == nil {
        report = decorated.Message
    }

    return &model.CommandResponse{
        ResponseType: model.CommandResponseTypeEphemeral,
        Text:         report,
    }, nil
}
```

### Decorating text that has a time of its own

A short date-time group such as `091630Z` takes its month and year from the
reference time. Pass the time the text is about, not the time you happen to
decorate it.

```go
decorated, err := p.tacticalFusion.Decorate(ctx, bridgeclient.DecorateRequest{
    Message:       relayed.Message,
    ReferenceTime: relayed.CreateAt,
})
```

## Rules worth knowing

- **Admin switches apply.** A format an administrator turned off is not linked
  for you either. `Link` says so with `ErrDisabled`; `Decorate` leaves the token
  as text.
- **URLs are root-relative** and carry the server's subpath, if it has one. They
  follow whichever hostname the reader uses, so do not prefix a host.
- **Posting the result is safe.** Tactical Fusion's own message hook leaves
  existing decorator links alone, so nothing is linked twice.
- **Links outlive the plugin's switches but not the plugin.** A link keeps working
  after its format is turned off, and stops working if Tactical Fusion is
  uninstalled, like any link it wrote itself.
- **Calls are cheap** (no database, no network beyond `PluginHTTP`), but they are
  still an RPC round trip. Do not call `Link` in a tight loop over thousands of
  values; `Decorate` one text instead.

## Versioning

Everything here is bridge version 1, served at `/bridge/v1`. Within a version
changes are additive only: new optional request fields, new response fields, new
types and new reasons. Treat an unknown `Reason` as "not linked". A breaking
change would be a new path served alongside this one.

## Without Go

The client is a thin wrapper over three JSON endpoints. Any plugin can call them
through `PluginHTTP` directly; the request and response shapes are on the
**Plugin Integration** help page under "Calling the bridge over HTTP".
