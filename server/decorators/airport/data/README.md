# Airfield data

`airports.csv`, `runways.csv` and `frequencies.csv` are the airfield database
this plugin embeds. They are generated, and committed, so that a clean checkout
builds and an air-gapped `go test` runs without anyone having fetched anything
first.

## Provenance

All three come from [OurAirports](https://ourairports.com/data/), whose data is
public domain. They were retrieved on 2026-09-22 from the raw files the
project publishes on GitHub (`davidmegginson/ourairports-data`, branch `main`).

| Upstream file | Size | Rows | SHA-256 |
|---|---|---|---|
| `airports.csv` | 12,727,797 bytes | 86,112 | `12a2bcceaa11c4ac9b7e2fa5dac8f91e4750bdf997d9036fd05ebe5ee8faa870` |
| `runways.csv` | 3,964,978 bytes | 48,250 | `d67808edd63f53e89ae300eee99c121c186e4e06111e55f500f1e6b527db3b56` |
| `airport-frequencies.csv` | 1,299,761 bytes | 30,348 | `95f9c3573899eca872a3779bf5163e1cabc98bd255736ee3e9920b889e3ef923` |

Public domain, so no notice has to travel with it. The provenance is recorded
because this repository ships an SBOM and targets air-gapped installs, where
"where did this file come from" is a question somebody will have to answer.

No airfield surface prints a credit for this data. That is the same judgment
the Natural Earth basemap credit got: public domain means a notice is a
courtesy rather than a requirement, and the provenance belongs where somebody
would look for it, which is this file and `public/help/airfields.html`. The
region a position falls in is a different thing and keeps its own citation,
because that one stops a border lookup reading as a determination.

An earlier build read the DataHub `airport-codes` repackaging of the first
file. It carries no runways and no frequencies, which is why the generator now
reads the three direct files.

## How it is produced

```sh
go run ./build/airportdata [path/to/source/dir]
```

The upstream files are **not** committed. Put them in
`build/airportdata/source/`, which is gitignored, or pass the directory.

The transform, for `airports.csv`:

- keep rows whose `ident` matches `^[A-Z]{4}$`, which is 19,281 of 86,112 and is
  exactly the set the grammar can name;
- drop the reserved ident `ZZZZ`, leaving **19,280**. See below;
- keep `type`, `name`, `municipality`, `iso_country`, `iso_region`,
  `iata_code` and `elevation_ft`; round `latitude_deg` and `longitude_deg` to
  four decimals as `lat` and `lon`; round `elevation_ft` to a whole number,
  keeping an absent value absent;
- add `military`, the designator matched in the name (below), or empty;
- refuse a duplicate ident, an IATA code that is not three upper-case letters,
  an IATA code carried by two rows, a coordinate pair of exact zeroes, an axis
  outside its range, a non-finite number, and any field carrying a line break;
- sort by ident, so a regeneration produces a reviewable diff.

For `runways.csv`, keep rows whose `airport_ident` is in the kept set (18,255
of 48,250), with `le_ident`, `he_ident`, `length_ft`, `width_ft`, `surface`,
`lighted`, `closed`, both ends' coordinates rounded to four decimals, and
`le_heading_degT` as a whole number. A runway with only one end stated carries
neither. A runway whose designations fail the text whitelist is dropped (22).
A length, width or heading outside its range is kept as unstated rather than
refusing the row.

For `airport-frequencies.csv`, keep rows whose `airport_ident` is in the kept
set (25,915 of 30,348), with `type`, `description` and `frequency_mhz` written
to three decimals. A row whose type fails the whitelist, exceeds twelve runes,
or whose value is not between 0.1 and 1,300 MHz is dropped (79). A description
that fails the whitelist or exceeds 64 runes is blanked (37); the row stays.

The generator refuses more than 32 runways or 48 frequencies on one airfield.
The measured maxima are 11 (KORD) and 31 (KCVG).

## The military designator

OurAirports has no operator field. `military` is the first of these designators
found as a whole word in the airfield's name, in this order:

```
Air Force Base, Air Base, Airbase, Naval Air Station, Marine Corps Air Station,
Army Airfield, Army Air Field, Joint Base, Air Station,
AFB, AB, NAS, MCAS, AAF, AFS, ANGB, RAF, RAAF, RNZAF, CFB, NAF, MCAF
```

601 of 19,280 names carry one. It is rendered as the designator itself
("Military (Air Force Base)"), never as a bare claim about who operates the
field: the designator is the name's own text, and the name is all the data
says.

## Runway surfaces

The upstream `surface` is free text: 355 distinct spellings across the kept
rows, `ASP` and `Asphalt` and `asphalt` among them. The generator maps the
common codes and spellings through a table to one name each (Asphalt,
Concrete, Grass, Turf, Gravel, Dirt, Earth, Clay, Sand, Water, Permeable,
Metal, Matting, Coral, Composite, Ice, Snow, Laterite, Macadam, Tarmac), maps
the codes for "unknown" (`UNK`, `X`, `N`, `S`, `L`, `U`) to empty, keeps
anything else as written when it passes the whitelist and is at most 24 runes,
and drops the rest. At the retrieval above: 17,326 normalized, 629 kept as
written, 10 dropped.

## Why four decimals

Not cosmetic. The upstream carries anything from zero to eighteen fractional
digits, `FormatDD` requires at least four and `Axis.Frac` caps at eight, so the
raw values sit outside the coordinate grammar at both ends and would convert
for nobody.

Four is also the coarsest the grammar admits, which is the right direction for
a crowd-sourced reference point whose meaning (tower, ARP, terminal) the source
never states. Negative zero is normalized away, or a `-0.0000` axis would fail
to reproduce its own canonical form, which is the defect `roundTo` records.
A non-finite axis is refused before the range check, because every comparison
against NaN is false and `ParseFloat` accepts the spelling.

## Why ZZZZ is not here

`ZZZZ` is a real upstream row, Satsuma Iojima in Japan. It is also the ICAO code
for an aerodrome that is **not listed**, with the real field named in remarks.
This decorator reads the USMTF `DEPLOC` and `ARRLOC` fields, which are flight
plan fields, so shipping the row would make `DEPLOC:ZZZZ` resolve to a specific
island airfield instead of to "see remarks".

That is the one failure this plugin refuses everywhere else: not a false
positive on text that was never a code, but a real code rendered as a
confidently wrong place. It is the same argument the UTM band letter gets, and
the same answer. `AFIL`, the other reserved code, is not in the upstream data at
all.

## Regenerating

This is deliberately **not** wired into `make test`. `map-data-check` earns that
slot because its encoding is opaque and its drift fails invisibly on a
plain-HTTP origin; this transform is filter, round and drop, and its drift means
an ident declines, which is visible and harmless.

`TestEveryAirfieldIsUsable`, `TestEveryIATACodeNamesOneAirfield`,
`TestEveryRunwayEndIsAcceptedByLocation` and
`TestEveryRunwayAndFrequencyFieldPassesTheWhitelist` in the package above are
what hold the files honest: every ident is unique and well-shaped, every IATA
code names one airfield, every coordinate round-trips through the location
grammar, and every text field passes the same whitelist the parser enforces at
init.

## words4.txt

Every four-letter English word, lower case, one per line, 4,360 of them.

It exists so the measurement the label-only grammar rests on can run in CI.
`TestManyIdentsAreOrdinaryWords` reports how many shipped idents are ordinary
words, and that number is the whole argument for this decorator having no bare
pattern. It used to read `/usr/share/dict/words`, which exists on macOS and not
on the GitHub Actions runner image, and `pr.yml` installs no apt packages: the
test skipped silently on every pull request, so the one measurement that could
have failed never ran where failing would have mattered.

**Source**: the BSD `web2` list, `/usr/share/dict/words` on macOS and on the
BSDs, which is Webster's Second International (1934). Its own README records
that the 1934 copyright has lapsed, so it is public domain and nothing has to
travel with it.

**The filter**: four characters exactly; all lower case or all upper case, so
mixed-case proper nouns are excluded; lowercased, de-duplicated and sorted.
Reproduce it with

```sh
awk 'length($0)==4' /usr/share/dict/words |
  awk '{ if ($0 == toupper($0) || $0 == tolower($0)) print tolower($0) }' |
  sort -u > server/decorators/airport/data/words4.txt
```

It is committed rather than generated at test time for the same reason
`airports.csv` is: a clean checkout must run `go test` with no network and no
prior generator run.
