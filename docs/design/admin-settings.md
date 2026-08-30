# Admin settings

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

## Admin settings

The count is **twenty-five switches across six sections**. Cursor on Target added
the fifth section and two of the switches, `EnableCot` and `EnableCotFile`, and
GeoJSON added the sixth and three more. Cursor on Target
deliberately added no third: the card's map reads `EnableLocationMapInline`,
whose parent ANDs with `EnableLocation` and `EnableLocationMap` already live in
Go. A second "is the map on" answer is the thing `features/types.ts` argues
against by name. See [`cot.md`](cot.md), "Switches".

`EnableLocationMapInline` is now worded as **the map under a post** rather than
"under a coordinate-only post", because two surfaces reach it.

`EnableCot` carries the same Elasticsearch and OpenSearch warning
`EnableLocationMapInline` carries, in the same words, plus the two costs only it
pays: link previews and image embeds are dropped, and Markdown written around
an event renders as plain text. The FILE attachment list is not among them; see
[`mapping.md`](mapping.md), "What setting `Post.Type` costs".

The counts written in prose are the one thing about the settings that no test
would otherwise read, and they are stated in four places: this file, two help
pages and `CLAUDE.md`. Adding the GeoJSON section left the design note and the
help pages disagreeing, none of them true of the same manifest.
`TestTheStatedSettingCountsMatchTheManifest` is what holds all four to
`plugin.json`: the two help pages, this file, and `CLAUDE.md`. It derives the
totals from the manifest rather than restating them, so adding a switch or a
section fails the test until the prose is caught up.

The **first four** of those sections hold twenty of the switches, plus the two
map-package settings, which are a path and a control rather than switches. The
Cursor on Target and GeoJSON sections below add the remaining five:

- **Date and time**: `EnableDTG` with `EnableDTGMilitary`, `EnableDTGTimestamp`
  and `EnableDTGMoniker`.
- **Coordinates**: `EnableLocation` with `EnableLocationDDSigned`,
  `EnableLocationLatLon`, `EnableLocationUSMTF`, `EnableLocationMGRS`,
  `EnableLocationUTM`, `EnableLocationGEOREF`, `EnableLocationGARS`,
  `EnableLocationPlusCode` and `EnableLocationMoniker`.
- **Maps**: `EnableLocationMap` with `EnableLocationMapPanel`,
  `EnableLocationMapInline` and `EnableLocationMapPage`, then
  `LocationMapPackagesDir` and `LocationMapPackages`. See
  [`mapping.md`](mapping.md#the-openstreetmap-detail-tier-above-the-seam) for
  why the storage is a real directory.
- **Airfields**: `EnableAirport` with `EnableAirportTable`.

`model.PluginSettingsSchema` carries a `Sections` field
(`PluginSettingsSection{Key, Title, Subtitle, Settings, Header, Footer}`), so
this is a real grouping rather than one implied by ordering. It said the opposite
here for a long time ("Mattermost's settings schema has no nesting"), which is
worth recording because it is the kind of claim that outlives the version it was
true of.

**A section groups, it does not gate.** The parent is still enforced in code, in
`Plugin.dtgFormats`, `Plugin.locationFormats` and `Plugin.locationMaps`, because
nothing about a section title reaches the switches under it. The Maps section has
**two** parents: `EnableLocationMap` and `EnableLocation`, since a map is only
ever drawn for a coordinate this plugin decorated. GEOREF and GARS have a second
parent of their own for a different reason: both are label-only, so both also
need `EnableLocationMoniker`, which no section can express and which their help
text therefore says in words.

`settings_schema.settings` is left empty and
`TestEverySettingBelongsToANamedSection` keeps it that way, because Mattermost
renders anything there above the first section heading, where it reads as a
switch belonging to no feature. That test also insists every section carries a
key (the server refuses one without) and a title, and that no setting key appears
twice. `loadSettings` in `configuration_settings_test.go` flattens the sections,
which is what stops the move making every other test in that file iterate an
empty slice and pass while checking nothing.

**Nineteen default on. `EnableLocationUTM` defaults off, and is the only one.** The
reason is a difference in kind rather than in degree: every other switch trades
a false positive against a missed decoration, so its worst case is that
something which was not a coordinate gets linked, or something which was does
not. UTM can decorate a **real coordinate and point at the wrong place**,
because its band letter is ambiguous (see Location above) and the band
containment check declines only about nine in ten of the civilian reading. A
format whose failure mode is a confidently wrong position should not be
something a workspace gets without asking.

That default is pinned by `defaultsOff` in
`configuration_settings_test.go`, a **named list with a reason per entry**
rather than a rule, since there is no rule: each is a judgment, and the only way
to be sure one is deliberate is to have written it down. The test checks both
directions and errors on a name in the list that no longer matches a setting,
which is how a default comes back on unnoticed. A second test requires an
off-by-default switch to say so in **both its display name and its help text**,
because an off-by-default format is otherwise indistinguishable from a broken
one: an admin pastes a UTM position, nothing happens, and the setting they would
check is the one already sitting at its default.

`EnableLocationMGRS` was `EnableLocationGrid`, whose generic name was kept so an
install that had it on kept MGRS on across the MGRS/UTM split. Renamed once
backwards compatibility stopped being a constraint: it sits directly above
`EnableLocationUTM` in the Coordinates section and read as though it governed
both. An upgrading install therefore gets the default rather than what it had
set, which was the accepted cost.

`EnableLocationGEOREF` and `EnableLocationGARS` default on despite being the two
weakest grammars in the package, and that is consistent rather than an
oversight: both are label-only, so their worst case needs an author to have
typed `GEOREF:` in front of something that was not a coordinate. They also
depend on `EnableLocationMoniker`, which the manifest cannot express, so their
help text says so in words and `TestAreaFormatsSwitchIndependently` pins the
composition in both directions.

There is no separate "decorate at all" switch. With one decorator it only
duplicated `EnableDTG`; a second decorator should bring its own switch rather
than reviving a global one. **Disabling the plugin is not a substitute**: that
also stops `/decorate` serving, so every link already written into a message
would 404. `EnableDTG` stops new decoration and leaves the history working.

`decoratePost` has no configuration check of its own. A decorator with
everything switched off contributes no patterns, so nothing matches and the post
is returned unchanged, which is the same path as a message with no token in it.

**A format switch governs decoration only.** `RenderPage` never consults the
format configuration, so a link already written into a message keeps working
after its format is switched off. Turning one off stops new messages being
decorated with it, and that is all. For the same reason the decorator stays
registered even with everything off, rather than being left out of the registry:
an unregistered type would 404 every link in the history.

**The map switches are the deliberate exception, and are read at render.** A
format governs text already written permanently into a message, so turning one
off may not break the history behind it; a map is drawn afresh every time
somebody looks and is written into nothing, so turning one off has to reach
links already out there or the switch would not mean what it says. `RenderPage`
therefore consults `Decorator.Maps`, which is admin configuration rather than
per-reader state and decides only whether a picture is drawn beside readings that
are still a pure function of the query string. See Mapping, "Turning maps off".

The moniker composes: `EnableDTGMoniker` only labels formats that are themselves
on, so turning military formats off also stops `DTG: 091630ZAUG26`. A moniker
with nothing left to label is dropped entirely.

Every field is false at zero, deliberately: an unloaded configuration should
decorate nothing rather than guess. That makes the manifest's `default: true`
load-bearing, which is why a test asserts every switch declares one, and two
more assert the setting keys and the `configuration` fields match in both
directions. A key that binds to no field is a switch an admin can toggle
forever with nothing happening, and nothing else would report it.

`Decorator.Enabled` is read fresh for every message, so a change in the admin
console takes effect without a restart.

