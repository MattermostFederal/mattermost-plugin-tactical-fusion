# Cyber datasets

Builds the files the cyber decorator reads: the two catalogs embedded in the
plugin, the bundled known-exploited list, and the datasets an operator drops
into the directory named by `CyberDatasetsDir`.

Nothing here is a prerequisite of `make test`, and nothing here runs in CI. The
pipeline needs the network. The plugin never does.

## Running it

The full set, from pinned upstream sources:

```
make cyber-sources
make cyber-data
```

A small current example, the CVEs NVD published in the last seven days:

```
make cyber-recent DAYS=7
```

## Publishing

```
make cyber-package
make cyber-release TAG=v0.8.0
```

`cyber-package` gzips each dataset in `out/` and writes `DATASETS.sha256` over
the archives, which are what an operator downloads and checks. `cyber-release`
checks `TAG` first, packages, and uploads only the archives and the checksum
file. Packing uses `gzip -n`, so the same dataset always packs to the same bytes.

An operator drops `cve.tsv.gz` into `CyberDatasetsDir` as it is. The plugin
unpacks it into `cve.tsv` in the same directory, because it searches a dataset
in place and cannot do that inside an archive:

- It compares the stamp line inside the archive with the one at the top of the
  `.tsv`, and unpacks only when they differ. Modification times are not used,
  because `cp -p` and `rsync -a` keep an old one.
- It checks the archive's stamp names the dataset its file name does before
  writing anything, so a mislabeled archive cannot overwrite a good file.
- It writes to a hidden staging file in the same directory and renames it into
  place, so a reader, or another node sharing the directory, never sees half a
  file, and gzip's own checksum rejects a truncated archive before the rename.
- It refuses an archive that unpacks to more than 2 GiB.

Measured with the full vulnerability archive: the first open unpacked 40 MB into
186 MB in about half a second, and the next open took a millisecond. A failure
is logged once as `TF-21006`. A server killed mid-unpack can leave a hidden
`.cve.tsv.*.unpacking` file behind, which is safe to delete.

## How large the full build is

Measured on 2026-09-23, from all 25 yearly NVD feeds:

| | |
|---|---|
| Download, gzipped | 232 MB |
| Unpacked JSON | 2.76 GB |
| `cve.tsv` | 185.7 MB, 396,474 rows |
| `cve.tsv` gzipped | 40.4 MB |
| `cvedetail.tsv` | 432.6 MB, 396,474 rows |
| `cvedetail.tsv` gzipped | 37.2 MB |
| Build time and peak memory, both files | 30 seconds, 3.9 GB |

`cve.tsv` keeps eight fields of each record: what the hover and the top of the
panel need. `cvedetail.tsv` keeps the rest a responder asks about, described
below. Between them they drop the pretty-printing, every translation, every
scoring but one, and NVD's internal match identifiers.

### What the detail file holds

One row per CVE, each field a compact JSON array, empty when the CVE has none:

- **Weaknesses by source**: which organization named each CWE. `cve.tsv`
  already carries the CWE identifiers themselves.
- **Configurations**: NVD's CPE matches, grouped the way NVD groups them, each
  vulnerable match with its version bounds and the platforms it applies on.
- **Affected**: the reporting organization's own vendor, product and version
  list. It is the only product information a CVE NVD has not analyzed yet has.
  An entry naming no vendor and no product, which 37% of older records carry as
  `n/a`, is dropped.
- **References**: every URL with its tags. A URL NVD lists once per source is
  kept once, with the union of its tags, which is what took the file from
  514 MB to 433 MB.

It is a separate file rather than more columns in `cve.tsv` for two reasons,
both measured. Its largest row is 0.5 MB, and the reader reads a whole row at
each step of its binary search, so every hover lookup landing near one would
pay for it. And an operator short of disk can leave it out: the panel then says
it is not installed, and everything `cve.tsv` answers still works. Lookups in it
ran at 44 microseconds at the median and 5.4 ms at worst across all 396,474
rows, including decoding the JSON.

The panel and the page render it, once, in Go: one line per product, version
bounds in words, platforms named up to five and counted after that, and each
section collapsed with its count. Only an `http` or `https` reference becomes a
link, checked in Go and again in the webapp against the same table of cases.
Some reporting organizations write product names with CPE escapes in them, such
as `team\+`; those are shown as the organization wrote them.

The generator and the reader share no code, so
`server/decorators/cyber/intel/testdata/cvedetail.tsv` holds them to one shape:
the generator's test must reproduce it exactly, and the reader's test must parse
it. Regenerate it after an intended change with
`go test ./build/cyberdata -run Golden -update`.

The plugin searches the file on disk rather than loading it, so its size costs
disk rather than memory. Every row of the full file was found at about 36
microseconds a lookup, measured with the file already in the page cache.

### Descriptions are kept whole

A CVE's English description goes into the file in full. Cutting it to the first
sentence dropped 52% of the description text across the full set, and usually
the impact: Log4Shell kept the sentence naming the flaw and lost the one saying
an attacker can execute arbitrary code. The longest row in the full set is
4,134 bytes, longer than the 4 KB block the reader searches in, which is why a
test holds the reader to rows of any length.

The ATT&CK and CWE catalogs still keep only a first sentence, because they are
compiled into the plugin.

## Exploit prediction and IP address data

Both are drop-ins beside `cve.tsv`, not bundled: EPSS is rescored every day and
the IP ranges change hourly, so a copy in the plugin would be stale the day it
shipped. `make cyber-package` gzips them with the others and `make deploy`
copies them into the Docker server.

Measured on 2026-09-23:

| | Source | Rows | `.tsv` | `.tsv.gz` |
|---|---|---|---|---|
| `epss.tsv` | `epss_scores-current.csv`, 11.5 MB | 378,156 | 15.6 MB | 2.8 MB |
| `ip.tsv` | `ip2asn-combined.tsv`, 45.6 MB | 579,615 | 59.0 MB | 8.3 MB |

To build just these two after `make cyber-sources`:

```
go run ./build/cyberdata -only epss -label "FIRST EPSS $(head -1 build/cyberdata/source/epss_scores-current.csv | tr -d '#')"
go run ./build/cyberdata -only ip -label "IPtoASN ip2asn-combined, fetched $(date -u +%Y-%m-%d)"
make cyber-package
```

### EPSS

Each score carries the date FIRST scored it, read from the `score_date` in the
export's first line; an export without one is refused rather than stamped with
the day it happened to be built. FIRST writes a few very small percentiles in
exponent form (`4e-05`); the generator writes them as plain decimals so the
panel can show every value as a percentage, moving the decimal point in the text
rather than through a float, so no digit FIRST published is rounded away.

| | |
|---|---|
| Upstream | `https://epss.empiricalsecurity.com/epss_scores-current.csv.gz` |
| Origin | The Exploit Prediction Scoring System, maintained by the EPSS Special Interest Group at FIRST, with scores generated by Empirical Security |
| Terms | Free to use; FIRST's FAQ says "Attribution is requested when EPSS data is used in publications or products." |

> Exploit prediction scores are from EPSS, the Exploit Prediction Scoring
> System, maintained by FIRST (https://www.first.org/epss) and generated by
> Empirical Security.

### IP addresses

`ip2asn-combined.tsv` gives each routed range its autonomous system number,
owner name and country. Ranges marked not routed, or with AS 0, are dropped. It
carries no region or city, so the panel shows those only when a vendor `.mmdb`
database sits in the same directory.

| | |
|---|---|
| Upstream | `https://iptoasn.com/data/ip2asn-combined.tsv.gz` |
| Origin | IPtoASN, maintained by Frank Denis |
| License | Open Data Commons Public Domain Dedication and License (PDDL) 1.0, per the dataset's own metadata on iptoasn.com |

### Region and city: DB-IP City Lite

`ip.tsv` has no region or city. DB-IP's free IP to City Lite database does, and
it is published in MaxMind's format, so the plugin reads it as it is:

```
make cyber-geo
make deploy
```

`fetch-geo.sh` downloads this month's file, or last month's early in a month
before DB-IP publishes, checks that it is a MaxMind-format database, and moves
it into `out/dbip-city-lite.mmdb` in one step. `make deploy` copies `.mmdb`
files beside the gzipped datasets. It is not gzipped for that trip because the
plugin unpacks only `.tsv.gz`. Measured on 2026-09-23: 60 MB to download,
127 MB unpacked, updated monthly.

| | |
|---|---|
| Upstream | `https://download.db-ip.com/free/dbip-city-lite-<YYYY-MM>.mmdb.gz` |
| Origin | DB-IP.com |
| License | Creative Commons Attribution 4.0 International. "In the case of a web application, you must include a link back to DB-IP.com on pages that display or use results from the database." |

The plugin meets that condition itself. It recognizes a DB-IP database by the
type its metadata declares, and every answer it contributed to carries the
credit "IP Geolocation by DB-IP": linked to `https://db-ip.com` in the panel
and on the page, and as text in the hover card, which cannot be clicked. MaxMind
GeoLite2 is credited the same way with MaxMind's required sentence. An answer
the database added nothing to carries no credit.

Two things the numbers do not show. The sources can disagree: iptoasn gives the
country an address's network is registered in, DB-IP where the address
geolocates, so `1.1.1.1` reads US from one and AU from the other, and the panel
shows DB-IP's and lists both sources. And "Lite" is DB-IP's word for reduced
accuracy.

## Threat reports

`advisory.tsv` holds each indicator (an address or a file hash) and a JSON array of
reports, one per CISA advisory in `advisories.txt` that names it. It ships in the
bundle, and `make cyber-advisories` rebuilds it.

- An address is written in its canonical form, so `::ffff:203.0.113.10` and
  `203.0.113.10` are one indicator; a digest is lowercased.
- A report's category is `malicious` or `context`; only `malicious` makes the
  panel say "reported malicious".

### Why abuse.ch is not used

Earlier builds read abuse.ch ThreatFox, Feodo Tracker and MalwareBazaar into
drop-in `threat.tsv` and `malware.tsv` files. They were removed on 2026-09-24:
abuse.ch's terms of use require an authenticated account, say commercial use
"may require a paid subscription, which will be managed by Spamhaus", and forbid
making "derivative works based on the Platforms" without consent, so the data
could never ship and each operator would have needed their own agreement. A
leftover `threat.tsv` or `malware.tsv` in `CyberDatasetsDir` is now skipped as a
name this build does not read.

Spamhaus DROP was considered in their place and left out: its terms grant no
license and forbid using the Spamhaus name, which conflicts with the credit its
product page asks for.

## The recent build

`fetch-recent.sh` asks the NVD API for every CVE published in the window and
writes the pages to `recent/nvd/`. The generator then turns them into
`recent/cve.tsv` with the same transform the full build uses. Point
`CyberDatasetsDir` at `build/cyberdata/recent` to try it; the plugin reads the
`.tsv` there and ignores the pages beside it.

`DAYS` defaults to 7 and may be anything from 1 to 120, the widest window NVD
answers in one query. Without a key NVD allows five requests in thirty seconds,
so the script waits six seconds between pages. Set `NVD_API_KEY` to raise the
limit.

Measured on 2026-09-23: a seven-day window held 3,065 CVEs, came back in two
pages, and built in about 25 seconds. The pages were 11 MB and `cve.tsv` was
2.2 MB.

### It holds only its window

A CVE published before the window is not in the file, so its panel says
**"Not in the vulnerability dataset generated ..."**. That is true of the file
and not of NVD: `CVE-2021-44228` reads that way against a seven-day build. Use
the full build wherever that difference matters. The stamp's source field
records the window, for example:

```
NVD API, published 2026-09-16T19:24:16Z to 2026-09-23T19:24:16Z, 3065 CVEs
```

### The window cannot be pinned

The full build verifies its sources against `sources.lock`. A window ending now
is different on every run, so it has nothing to pin to. The script checks what
it can instead:

- The window's end is fixed before the first request, so a CVE published while
  the pages are fetched cannot shift the paging and produce a duplicate or a gap.
- It counts the CVEs it received against the `totalResults` NVD reported, and
  fails on any difference.

Rejected CVEs are left out with NVD's `noRejected` filter.

## What a recent CVE looks like

Most CVEs in a recent window have not been analyzed by NVD yet. In the build
above, 458 of 3,065 carried no score at all; those are kept, with empty score
fields, so the panel still shows the summary and dates.

278 were scored only under CVSS 4.0. The generator reads scores in the order
3.1, 3.0, 4.0, 2.0, so a record carrying both 3.1 and 4.0 shows the 3.1 score
it showed before 4.0 was read, and the vector's `CVSS:4.0/` prefix says which
version a score came from.
