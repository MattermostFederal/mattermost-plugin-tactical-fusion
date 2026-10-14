# Tactical Fusion

Tactical Fusion turns the coordinates and times people already type in
Mattermost into something everyone can read.

Post `091630ZAUG26` or `18S UJ 23478 06483` and the plugin turns it into a link.
Hover it for a countdown; click it and the right-hand sidebar shows the same time
in your team's timezones, or the same position in every other coordinate
notation, with a copy button on each row.

Nobody has to change how they write. The formats are the ones already used in
mission traffic.

> **Status:** early. Date-time groups, coordinates, airfields, aviation reports
> (METAR, TAF, NOTAM), frequencies, notes, cyber indicators (CVE, CWE, ATT&CK,
> IP addresses, file hashes), Cursor on Target and GeoJSON work today.

Requires Mattermost **11.8.0** or later.

## What it does

### Times

Military date-time groups (`091630ZAUG26`, `091630Z`) and RFC 3339 timestamps
(`2026-08-09T16:30:00Z`) become links.

- **Hover** shows a countdown, which flashes when the time is close.
- **Click** opens a panel with the time in each timezone you care about, and the
  countdown running live.
- On mobile, and anywhere else the webapp does not run, the link opens a plain
  page from the server showing the same thing.

### Coordinates

A coordinate in any recognized notation becomes a link showing the same position
in all the others: decimal degrees, degrees/minutes/seconds, degrees and decimal
minutes, the fixed-width USMTF shapes used in ATO traffic, military grid
references (MGRS), UTM, GEOREF, GARS and Plus Codes.

```
34.0561, -118.2500          34°03'22"N 118°15'00"W
3510N07901W                 400948N1221400W
18S UJ 23478 06483          33U 291000 5628000
```

USMTF field labels (`LATM:`, `GEOK:`, and so on) work in front of a coordinate,
so a line pasted straight out of a message format is still read correctly.

Two things worth knowing:

- **Positions are WGS 84**, and every reading is shown at the resolution the
  original text carried and no finer. A coordinate written to whole degrees gets
  a 100 km grid square, not a ten-figure reference that would imply a precision
  nobody wrote.
- **Grid conversion is computed in the plugin**, with no external service and no
  network call, so it works on an air-gapped host.

### Airfields, aviation reports and frequencies

- **Airfields.** An ICAO code behind a field label (`ICAO:PHNL`, `DEPLOC:PHIK`,
  `ARRLOC:PGUA`) or an IATA code behind `IATA:` links to the field's name,
  position, runways and frequencies. A message of nothing but airfield codes
  gets a route map under it.
- **Aviation reports.** A METAR, SPECI, TAF or NOTAM is decoded into plain
  language. Posted on its own it becomes a table; in a fenced block labeled
  `metar`, `taf` or `notam` it becomes a card with the station on a map.
- **Frequencies.** `FREQ:121.5` names the band and any known allocation.

### Cyber context

CVE, CWE and MITRE ATT&CK identifiers, IP addresses and MD5, SHA-1 and SHA-256
hashes become links. The panel shows what the bundled and installed datasets
say: severity and known exploitation for a CVE, the hierarchy for a weakness or
technique, network and location for an address, and any watchlist entry. A
missing dataset is reported as missing, never as a clean result.

### Cursor on Target and GeoJSON

A Cursor on Target event or a GeoJSON document, in a fenced block labeled `cot`,
`xml` or `geojson`, or attached as a `.cot`, `.xml` or `.geojson` file, renders as a
card with its contents on a map.

### Notes and the Agents plugin

- `/tactical-fusion note <label> | <markdown>` posts a link whose hover card
  renders the markdown.
- With the Mattermost Agents plugin installed, its agents can call Tactical
  Fusion's tools over MCP: decorate text, convert coordinates, look up
  airfields and cyber indicators, decode reports, CoT, GeoJSON, frequencies and
  date-time groups, and build CoT and GeoJSON.

### Deliberately recognizing less

Decorating a message rewrites what was posted, so a false positive edits somebody
else's words. The plugin declines anything ambiguous rather than guessing:
`34.05, -118.25` (too few digits to be certain it is a coordinate), bare dates
and zoneless times, epoch seconds, `CST`-style zone abbreviations. Text inside
code blocks, code spans, links and URLs is never touched.

Two slash commands make this visible:

- `/tactical-fusion examples` posts a demonstration **to the channel**, one message
  per format, each row showing what you would type and the link it becomes. Two of
  the date-time rows are live, either side of now, so the countdown is actually
  moving. It ends with the card formats (Cursor on Target, GeoJSON and a TFR
  NOTAM), two notes and a set of prompts for the Agents plugin. This is the one
  for introducing the plugin to a team.
- `/tactical-fusion check <text>` tells you what would be decorated in some text,
  and what would not, without posting anything.

The first writes to the channel; `check` is visible to you alone. Every boundary
case and every near miss that is deliberately declined is in the built-in help,
on the page for that format.

### Customizing your view

Below each panel, "Customize your view" lets each person set it up their own
way: for times, which timezone rows to show (from a catalog of military bases,
or any timezone) and how close a date-time group has to be before the countdown
flashes; for coordinates, which rows to show, so you can drop the notations you
never read. It is stored per user, so it changes nothing for anyone else.

## Before you install

Decoration happens on the server as a message is posted, which is what makes the
links work on mobile. It comes with trade-offs an admin should know about:

- **The stored message is rewritten.** Editing a post shows the author the raw
  markdown link. Exports contain the links. Uninstalling the plugin leaves links
  behind that no longer resolve.
- **Only new posts are decorated.** Message history from before installation is
  untouched.
- **Edits are kept exactly as typed.** Deleting the link syntax while editing is
  the supported way to opt one post out.
- **Turning a format off** stops new decoration and leaves every link already
  posted working.

## Installing

Download the bundle from the
[releases page](../../releases) and upload it in **System Console → Plugins →
Plugin Management**, then enable it.

## Admin settings

Forty switches in **System Console → Plugins → Tactical Fusion**, in nine
sections, plus directories for detail map packages and cyber datasets. All on
by default except `EnableLocationUTM` and `EnableGeoJSONUnlabeled`. Each section
opens with its own parent switch, which gates everything below it. The built-in
help's admin page describes every one.

**Date and time**

| Setting | Controls | Default |
|---|---|---|
| `EnableDTG` | The date-time group feature, and the three below it | On |
| `EnableDTGMilitary` | `091630ZAUG26` and friends | On |
| `EnableDTGTimestamp` | `2026-08-09T16:30:00Z` | On |
| `EnableDTGMoniker` | A `DTG:` label in front of a time | On |

**Coordinates**

| Setting | Controls | Default |
|---|---|---|
| `EnableLocation` | The coordinate feature, and the nine below it | On |
| `EnableLocationDDSigned` | `34.0561, -118.2500` | On |
| `EnableLocationLatLon` | Hemisphere and degrees/minutes/seconds forms | On |
| `EnableLocationUSMTF` | The fixed-width USMTF shapes | On |
| `EnableLocationMGRS` | MGRS, `18S UJ 23478 06483` | On |
| `EnableLocationUTM` | UTM, `33U 291000 5628000` | **Off** |
| `EnableLocationGEOREF` | GEOREF behind its label, `GEOREF:GJPJ3718` | On |
| `EnableLocationGARS` | GARS behind its label, `GARS:006AG39` | On |
| `EnableLocationPlusCode` | Plus Codes, `849VCWC8+R9` | On |
| `EnableLocationMoniker` | USMTF field labels in front of a coordinate | On |

**Maps**

Gated on `EnableLocation` as well as on their own parent, since a map is only
ever drawn for a coordinate the plugin decorated. Unlike every other switch here
these are read at render, so turning one off reaches coordinates posted long ago.

| Setting | Controls | Default |
|---|---|---|
| `EnableLocationMap` | Maps everywhere, and the three below it | On |
| `EnableLocationMapPanel` | The map in the sidebar and the hover card | On |
| `EnableLocationMapInline` | The map under a coordinate-only post | On |
| `EnableLocationMapPage` | The full-window map page behind "Open larger" | On |

With `EnableLocationMap` off, no client requests the basemap archive, its fonts
or the map library, which together are the largest thing the plugin transfers.
The file still ships in the bundle either way: Mattermost serves `public/` before
the plugin sees the request, so the switch stops clients asking rather than
making it unreachable.

`EnableLocationMapInline`, `EnableAirportRoute`, `EnableAvReportCard`,
`EnableCot` and `EnableGeoJSON` are the switches that change a stored post.
Drawing a map or a card under a message means marking that post with a custom
type, and Elasticsearch and OpenSearch index a marked post but never match it, so
it is missing from search and from Recent Mentions; link previews, embeds and
auto-translation are dropped for it too. Postgres search is unaffected. Turning
one off leaves those messages ordinary. Posts already marked keep their mark.

**The other sections**

| Section | Parent switch | Below it |
|---|---|---|
| Airfields | `EnableAirport` | `EnableAirportIATA`, `EnableAirportTable`, `EnableAirportRoute` |
| Aviation reports | `EnableAvReport` | `EnableAvReportMETAR`, `EnableAvReportTAF`, `EnableAvReportNOTAM`, `EnableAvReportTable`, `EnableAvReportCard` |
| Frequencies | `EnableFrequency` | |
| Cursor on Target | `EnableCot` | `EnableCotFile` |
| GeoJSON | `EnableGeoJSON` | `EnableGeoJSONFile`, `EnableGeoJSONUnlabeled` (**off**) |
| Cyber context | `EnableCyber` | `EnableCyberCVE`, `EnableCyberCWE`, `EnableCyberAttack`, `EnableCyberIP`, `EnableCyberHash` |

`EnableGeoJSONUnlabeled` ships off because ordinary JSON is pasted into chat
constantly, and a wrongly stamped post loses its search matches for good.

**`EnableLocationUTM` ships off** for a reason that is a difference in kind.
Every other switch trades a false positive against a missed
decoration. UTM can decorate a real coordinate and point at the **wrong place**:
`11S` is band S here (34° N) and "zone 11, southern hemisphere" to a civilian
(56° S), and nothing in the token says which was meant. The northing has to
land inside the band its letter names, which declines about nine in ten
positions written the civilian way, but not all of them. Turn it on if your
workspace writes UTM the military way. Leaving it off costs no UTM **row**: every
decorated coordinate still shows one.

Two other combinations are worth calling out:

- Leaving only `EnableLocationUSMTF` and `EnableLocationMoniker` on decorates a
  coordinate **only** where an author explicitly labeled it, which removes bare
  detection entirely.
- `EnableLocationMGRS` and `EnableLocationUTM` together are the switches that
  remove every value the plugin calculates rather than reads, for a workspace
  that wants only what a message literally says.

Changes take effect immediately, with no restart.

## Help

Full documentation ships with the plugin and is served from your own server, so
it works offline:

`/plugins/com.mattermost.plugin-tactical-fusion/public/help/help.html`

It is linked from the System Console settings page and from the sidebar panel,
and covers every recognized format, the panel, each admin switch, the slash
commands, troubleshooting, and the `TF-NNNN` error codes you may see quoted in a
message.

## Using Tactical Fusion from another plugin

Another Mattermost plugin can ask Tactical Fusion for a decorator link and embed
it in its own output, where it gets the same hover card, sidebar panel and
standalone page as a link in a posted message.

- **Server side**, over the plugin API's `PluginHTTP`, with the Go client in
  [`bridgeclient/`](bridgeclient/README.md):

  ```go
  link, err := bridgeclient.NewClient(p.API).Link(ctx, bridgeclient.LinkRequest{
      Type:  bridgeclient.TypeAirport,
      Token: "PHIK",
  })
  // link.Markdown == "[PHIK](/plugins/com.mattermost.plugin-tactical-fusion/decorate/airport?v=PHIK)"
  ```

- **Web app side**, through `window.TacticalFusion`, which offers `decorate()`,
  `link()` and a `<Link>` React component.

The full contract, with every request, response, error and worked example, is the
**Plugin Integration** page of the built-in documentation:
`/plugins/com.mattermost.plugin-tactical-fusion/public/help/integration.html`.

---

## For developers

Go server plugin plus a React webapp. `make help` lists every target.

```sh
make docker-setup   # first run: start Mattermost + PostgreSQL, create admin and team
make deploy         # build the bundle, install it, enable it
```

That serves Mattermost on `http://localhost:8065` with an `admin` / `password`
system admin (override the port with `MM_PORT`). `make deploy-local` targets your
own running server instead, via the bundled `pluginctl`.

| Command | Does |
|---|---|
| `make dist` | Build the plugin bundle |
| `make check-style` | Lint Go and webapp code |
| `make test` | Run tests |
| `make coverage` | Backend and frontend coverage summary |
| `make license-check` | Enforces `.licenses.json` over the shipped dependencies (after `make sbom`) |
| `make bundle` | Also writes `THIRD-PARTY-NOTICES.txt` into the bundle |
| `make sbom-audit` | SBOM + CVE scan + license policy |
| `make govulncheck` | Fails on a Go vulnerability the server can reach |
| `make release` | The full security-gated pipeline CI runs on a tag |

CI runs the same targets: `pr.yml` (style, tests, build), `security.yml` (SBOM +
Grype + CodeQL to Code Scanning, and govulncheck), `release-please.yml` and `release.yml`.
Releases are automated with
[release-please](https://github.com/googleapis/release-please) driven by
[Conventional Commits](https://www.conventionalcommits.org/), so versions and the
changelog are never hand-edited. Dependabot opens weekly dependency PRs.
A pull request labeled `preview` gets a throwaway preview server with the
latest release's map packages and cyber datasets installed.

Further reading: [CLAUDE.md](CLAUDE.md) for the architecture and the reasoning
behind it, and the [Mattermost plugin developer docs](https://developers.mattermost.com/extend/plugins/).
