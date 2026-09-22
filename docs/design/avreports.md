# Aviation reports

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

## What it is

`server/avreport/` decodes a METAR, SPECI, TAF or NOTAM that somebody pasted.
It is both a decorator and a stamper, which no other package is: a report on
one line is a link, and a report on several lines, or in a labeled fence, is a
card. The decoder is the work and the two surfaces are thin, so they live
together rather than as `decorators/avreport` and `hooks_avreport` in two
packages that would each own half of the vocabulary.

Nothing fetches. There is no weather source and no NOTAM feed, and there is no
setting for one, because the target is an air-gapped network. Live weather is a
possible later plan and it would be a different thing: a fetch is a claim about
now, and a decode is a claim about what the author wrote.

## A link when it fits, a card when it does not

A single-line report is a link and never a card, because a link keeps the post
searchable and a stamp costs its search matches forever. A multi-line report is
a card and never a link, because a link label spanning lines is a rendering
claim this plugin has not verified, and a rewrite that breaks rendering is
permanent. The split is by shape rather than by kind, so a one-line TAF is a
link and a three-line FAA NOTAM is a card.

A fenced single-line METAR is stamped, not linked: a fence is a protected range
the decorator can never rewrite, and the label is the author's statement of what
the block holds.

**One report per message.** `Decode` refuses a text whose second or later line
is itself a report header (`carriesASecondReport`), so a bundle of METARs one
per line is left to decoration and every line gets its own link. That is the
opposite of Cursor on Target's "several events in one source", deliberately:
each line here is independently linkable and searchable, and a card over the
bundle would take that away for no reader's benefit. Without the refusal the
METAR decoder, which joins every field, read the second line as unknown groups
of the first and stamped the pair as one report.

A multi-line TAF inside prose is neither linked nor stamped. It is not a sole
message, so the stamper does not read it, and the TAF pattern refuses to link
its first line alone: the pattern carries an optional trailing group matching a
continuation line (`FM`, `TEMPO`, `BECMG`, `PROB`, `RMK`), and `extractReport`
hands `Parse` the whole match when that group fired, which contains a newline
and is refused. Linking the first line alone would store a `v` that is a
truncated report. `TestAMultiLineTAFInsideProseIsNeitherLinkedNorStamped` holds
it.

## The grammar

Three single-line patterns, each ending at `[ \t]*=?` with the `=` matched but
not linked, the `//` trick the airfield grammar uses. Upper case only.

The bare METAR form (`PHNL 221651Z 07012G18KT`) is admitted because the wind
group makes it as specific as a keyword. Its boundary is `lineStartOK` rather
than `BoundaryOK`: the bare pattern is read only at the start of a line. Without
that, `see wx:METAR PHNL 221651Z 28012KT` refused the keyword form on the
colon, and the bare form then matched from `PHNL` and linked a report whose
keyword was left outside the link. The keyword forms keep `BoundaryOK` and are
read anywhere on a line.

The DTG decorator also matches the `221651Z` inside a report. The report span
is longer, so `resolveOverlaps` gives it the match, and when the report's
`Parse` declines the DTG keeps the time group as it did before this decorator
existed. `TestAGarbledReportLeavesTheTimeGroupToDTG` pins the second half.

## The instant

A METAR or TAF time group is a day, an hour and a minute. The month and year
come from the reference time exactly as the DTG short form resolves them, through
`dtg.ResolveDayTime`, which the DTG package exports for this so there is one copy
of the rule. The decode marks the report inferred and every surface says so,
because a report pasted a month late would otherwise read as current.

The link carries `v` (the report text exactly as matched) and `t` (the resolved
instant in milliseconds). `Validate` decodes `v` again with `t` as the reference
and requires `Raw` to equal `v` and `Instant()` to equal `t`, so a hand-edited
instant is refused (`AvReportPageInvalid`, `APIAvReportInvalid`). A NOTAM with
no effective time carries `t=0` and validates against the current time, which
the NOTAM decode does not read.

## The stamp

`hooks_avreport.go` is the third stamper, after Cursor on Target and GeoJSON,
format-major as `geojson.md` argues. Its two sources are a fence labeled
`metar`, `speci`, `taf` or `notam` (case-insensitive like `cotInfoString`) and a
bare message: CRLF normalized, trimmed, at least one newline, no code span
(`decorators.HasCodeSpan`, for the reason `SoleElementSpan` refuses code), and a
first line `LooksLikeHeader` accepts.

The only collision with the other two formats is an attachment beside a visible
report: with reports last, a `.cot` or `.geojson` file would win over a pasted
TAF. `cotFileSource` and `geoJSONFileSource` both refuse when
`messageShowsAvReport` says so, which is defined after `messageShowsGeoJSON`: a
labeled report fence suppresses whether or not it decodes, a bare multi-line
report suppresses only if it decodes, and neither suppresses when the card
switch is off. `TestTheVisibleReportBeatsAnAttachmentAcrossFormats` pins all
three.

Two rungs: the whole blob, then `PropsWithoutRows`, which drops the rows,
periods, remarks and unknown groups and sets `rows_dropped`. `src` is never
dropped, because the webapp never reads `post.message` once stamped and the
verbatim text is the card. Refusals: a labeled fence that does not decode tells
its author and logs `HooksAvReportUnreadable`; a bare failure is silent and
unlogged, the `json`-fence argument, because ordinary multi-line text reaches
that reader constantly.

The decorator declares neither `PostRenderer` nor `MultiPostRenderer`, pinned
by `TestTheReportDecoratorDeclaresNoPostType`.

### `runStamper`

The recover, the strip and the `enabled && post.Type == ""` gate were copied in
`cotStamp` and `geoJSONStamp`, and a third copy is where drift starts.
`runStamper` in `hooks_stamp.go` wraps the **whole** stamper body: it declares
the recover first, owns `stripped` and returns it from the recover, applies the
gate, and calls a `recognize` function holding source-finding, any filestore
call, the parse and `commitStamped`. That meets both objections `geojson.md`
recorded against a shared recover: it spans the filestore call, and it has the
stripped clone. `panicOnFileInfo` still lands inside the span and now tests the
one recover all three share; `TestRunStamperRecoversAndHandsBackTheStrippedPost`
tests it directly.

## The blob

`custom_tf_avreport` is 18 bytes. `tactical_fusion_avreport` is its own props
key and is in the strip table, so `/map?post=` finds it through
`stampedPropsKey` and a forged sibling blob is cleared. Keys: `version`,
`source`, `lead`, `trail`, `src`, `kind`, `station`, `station_name`, `format`,
`value`, `region`, `issued`, `issued_at`, `inferred`, `summary`, `flags`,
`rows`, `periods`, `remarks`, `unknown`, `radius_nm`, `rows_dropped`.
`/api/v1/avreport` answers `Blob`, the same shape minus the five stamp-only
keys, so there is one wire shape and one TypeScript reader, held to Go by
`TestWebappAvReportShapeMatches`, which walks the blob rather than scraping a
struct.

The station pair comes from `airport.Lookup` and the cheap `location.Parse`
path, never `location.Convert` on the post path; `Blob` runs `Convert` for the
region because the API and the page are off the post path. A station outside
the database carries no pair and the card draws no map; the overlay page
answers 404 for such a post, because the card offers no "Open larger" link to
it.

Go refuses past its caps; the webapp reader slices a list past `MAX_REPORT_ROWS`
or `MAX_REPORT_PERIODS` rather than refusing the blob. That is acceptable for
rows where it was not for GeoJSON rings: a shortened list of readings is still a
true list of readings, and a shortened ring is a polygon nobody posted.

## What the decoders refused to panic on

`expandContractions` indexed byte 0 of a word after trimming its punctuation,
and a word that was nothing but punctuation (a lone `.`, an ellipsis) trimmed
to nothing. The decoder runs unrecovered on `/api/v1/avreport` and
`/decorate/avreport`, so any logged-in reader could crash the plugin with one
FAA NOTAM ending in a full stop. An empty ICAO `A)` line reached
`strings.Fields(a)[0]` the same way. Both are held by tests now, and the ICAO
station is admitted only when it is four letters, the shape every other caller
of `airport.Lookup` enforces. `faaStation` answers nothing rather than
inventing a `K`-prefixed five-letter ident when no lookup resolves the
affected location. The summary is cut on runes through `sanitizeText`, because
a byte cut landed inside a multibyte rune and put invalid UTF-8 into the props.
`Flags` is capped at `MaxFlags`, as every other list in the blob is, and the
webapp caps it to the same number.

`messageShowsAvReport` consults the kind's own switch after the decode, so a
TAF whose kind is off no longer suppresses a `.cot` attachment for a card that
`recognizeAvReport` would then decline. The bridge classifies a report against
the request's reference time, never the wall clock, which is what makes its
answer a pure function of the request.

## The webapp

`webapp/src/avreport/` holds the reader, the client for the link, the card,
the post body, the panel for both the link and the stamped post, the hover and
the map. The hover is the summary line and returns null in every other state.
The panel for the link uses the five-state client copied from `airport.ts`:
TTL cache, ten-second bound, `failed` never cached, `rejected` on 400, and the
answer checked against the question (`src` equals `v`, `issued_at` equals
`t`). The map draws the station's marker and, for an ICAO NOTAM with a `Q)`
center and radius, an ellipse of that radius in meters, through the same
`MapEllipse` the CoT CE circle uses.

The panel has no Customize section. The plan listed one, mirroring GeoJSON's
hideable sections; it was left out because the panel is short and every row is
the report's own, and adding a fourth preferences section for it would be an
abstraction the code does not need yet. Adding it later is the GeoJSON shape:
a section catalog, a preferences key, and a sync test.

## Vocabulary and provenance

Everything decoded is from a citable public source: the WMO code tables as the
FAA Aviation Weather Handbook reproduces them for the weather groups, FAA Order
JO 7340.2 for the NOTAM contractions, and the ICAO Q-code subject and condition
tables as the FAA NOTAM manual carries them. The two tables are CSVs under
`server/avreport/data/` with a README. A group the vocabulary lacks is kept
verbatim under "Not decoded" and never guessed; a report is refused only when
its header does not parse.

The NOTAM `Q)` line's center is decoded as degrees and minutes with a hemisphere
letter, which is how the ICAO format states it; `TestDecodesAnICAONotam`
holds it. The FAA form's station is resolved by IATA code first (`!HNL` names
Honolulu, `PHNL`, and a `K` prefix would name a field that does not exist), then
by the four-letter prefixes.

## Switches

An **Aviation reports** section: `EnableAvReport` (parent), `EnableAvReportMETAR`
(METAR and SPECI), `EnableAvReportTAF`, `EnableAvReportNOTAM`, and
`EnableAvReportCard` (the stamp, with the search warning `EnableCot` carries).
All default on. `avreportFormats()` ANDs the parent in Go; `avreportCardEnabled`
is what the stamper reads, and the kind's own switch is checked again after the
decode so a card is never written for a kind whose links are off. The map in the
card reads `mapInline` and the map in the panel reads `mapPanel`, the GeoJSON
split.
