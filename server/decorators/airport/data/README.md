# Airfield data

`airports.csv` is the airfield database this plugin embeds. It is generated, and
committed, so that a clean checkout builds and an air-gapped `go test` runs
without anyone having fetched anything first.

## Provenance

| | |
|---|---|
| Upstream | `airport-codes.csv`, the DataHub `airport-codes` dataset |
| Origin | [OurAirports](https://ourairports.com/data/) |
| Licence | Public domain |
| Retrieved | 2026-08-18 |
| Upstream SHA-256 | `a916fd8b262c9ab87be40229d93bc6ee9756f2a99dcc2b05b8b7a014f7cdeed6` |
| Upstream size | 8,584,060 bytes, 82,808 rows |

Public domain, so no notice has to travel with it. The provenance is recorded
because this repository ships an SBOM and targets air-gapped installs, where
"where did this file come from" is a question somebody will have to answer.

No airfield surface prints a credit for this data. That is the same judgment the
Natural Earth basemap credit got: public domain means a notice is a courtesy
rather than a requirement, and the provenance belongs where somebody would look
for it, which is this file and `public/help/formats.html`. The region a position
falls in is a different thing and keeps its own citation, because that one stops
a border lookup reading as a determination.

## How it is produced

```sh
go run ./build/airportdata [path/to/upstream/airport-codes.csv]
```

The upstream file is **not** committed. Put it at
`build/airportdata/source/airport-codes.csv`, which is gitignored, or pass its
path.

The transform:

- keep rows whose `ident` matches `^[A-Z]{4}$`, which is 19,013 of 82,808 and is
  exactly the set the grammar can name;
- drop the reserved ident `ZZZZ`, leaving **19,012**. See below;
- drop `continent`, `gps_code`, `local_code` and `icao_code`, which are unused,
  or equal to `ident`, or empty across this subset;
- split `coordinates` into `lat` and `lon`, **rounded to four decimals**;
- round `elevation_ft` to a whole number, keeping an absent value absent;
- sort by ident, so a regeneration produces a reviewable diff.

The program refuses a duplicate ident, a coordinate pair of exact zeroes, an
axis outside its range, and any field carrying a line break. Those are build
failures rather than something a reader finds later.

## Why four decimals

Not cosmetic. The upstream carries anything from zero to eighteen fractional
digits: 31 axis values have none at all and over a thousand have nine or more.
`FormatDD` requires at least four and `Axis.Frac` caps at eight, so the raw
values sit outside the coordinate grammar at both ends and would convert for
nobody.

Four is also the coarsest the grammar admits, which is the right direction for a
crowd-sourced reference point whose meaning (tower, ARP, terminal) the source
never states. Negative zero is normalized away, or a `-0.0000` axis would fail
to reproduce its own canonical form, which is the defect `roundTo` records.

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

`TestEveryAirfieldIsUsable` in the package above is what holds the file honest:
every ident is unique and well-shaped, and every coordinate pair round-trips
through the location grammar and converts.

## words4.txt

Every four-letter English word, lower case, one per line, 4,360 of them.

It exists so the measurement the label-only grammar rests on can run in CI.
`TestManyIdentsAreOrdinaryWords` reports how many shipped idents are ordinary
words, and that number (343 at the time of writing) is the whole argument for
this decorator having no bare pattern. It used to read
`/usr/share/dict/words`, which exists on macOS and not on the GitHub Actions
runner image, and `pr.yml` installs no apt packages: the test skipped silently
on every pull request, so the one measurement that could have failed never ran
where failing would have mattered.

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
