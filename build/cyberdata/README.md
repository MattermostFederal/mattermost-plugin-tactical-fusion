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
