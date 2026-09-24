# Bundled cyber datasets

Everything in this directory is copied into the plugin bundle by the `bundle`
target in the `Makefile` and read at render time by the cyber decorator.

**This directory is under `assets/` rather than `public/` on purpose.**
Mattermost serves a plugin bundle's `public/` directory to anybody who can
reach the server, with no session; it does not serve `assets/`. A dataset here
is read by the plugin and never handed out, which is what makes this the right
place for one and the wrong place for anything an operator would not publish.
The watchlist in particular must never be moved under `public/`.

## What belongs here

| File | What it is | Ships |
|---|---|---|
| `kev.tsv` | CISA Known Exploited Vulnerabilities | yes |
| `advisory.tsv` | The IP addresses and file hashes named in the CISA advisories listed in `build/cyberdata/advisories.txt` | yes |
| `attackdetail.tsv` | MITRE Enterprise and Mobile ATT&CK: each technique's and tactic's whole description, mitigations, detection strategies and analytics, procedure examples and references | yes |
| `cwedetail.tsv` | MITRE CWE research view 1000: each weakness's whole description, background, consequences, mitigations, detection methods and observed examples | yes |
| `cve.tsv`, `cvedetail.tsv` | The NVD records of every CVE in `kev.tsv`, and only those | yes |
| `ip.tsv.gz` | IPtoASN's address ranges with their autonomous system and country, gzipped; the plugin unpacks it beside itself on first read | yes, as the archive only |

Everything else the decorator reads is too large or changes too fast to bundle
and is attached to a release instead, for operators to drop into the directory
named by the `CyberDatasetsDir` setting: the full `cve.tsv` and `cvedetail.tsv`,
`epss.tsv`, `threat.tsv`, `malware.tsv`, any vendor `.mmdb` database and the
operator's own `watchlist.tsv`. A file there replaces the bundled one of the same
name.

## Refreshing `kev.tsv`

CISA adds to the catalog most weekdays, so the committed file is a snapshot. Its
stamp names the catalog version it was built from. To refresh it, on a host with
network access:

```
curl -fL -o build/cyberdata/source/known_exploited_vulnerabilities.json \
  https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json
go run ./build/cyberdata -only kev -label "CISA KEV catalog <catalogVersion>"
```

`make cyber-sources` followed by `make cyber-data` rebuilds it along with
everything else. An operator who wants a newer catalog than the bundle carries
drops a `kev.tsv` or `kev.tsv.gz` into `CyberDatasetsDir`; a file there replaces
the bundled one.

## Provenance

### `kev.tsv`

| | |
|---|---|
| Upstream | `https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json` |
| Origin | Cybersecurity and Infrastructure Security Agency, United States |
| License | A work of the United States government, so not subject to domestic copyright |
| Format | The stamped, tab separated, bytewise-sorted shape `server/decorators/cyber/intel` reads |

The first line of every file here is a schema stamp. A file carrying no stamp,
or one built for a different reader, is skipped with `TF-21002` rather than
misread.

### `cwedetail.tsv`

| | |
|---|---|
| Upstream | `https://cwe.mitre.org/data/csv/1000.csv.zip` |
| Origin | MITRE CWE, research view 1000, fetched 2026-09-23 |
| License | MITRE CWE terms of use. Redistribution is permitted with attribution. |
| Format | Seven fields: the id, the whole description, the extended description, then consequences, mitigations, detection methods and observed examples as compact JSON arrays |

> This product uses information from MITRE CWE, the work of The MITRE
> Corporation. MITRE is not affiliated with and does not endorse this plugin.

The same export produces the `cwe.tsv` catalog compiled into the plugin, so the
two always name the same weaknesses. Rebuild both together:

```
go run ./build/cyberdata -only cwe,cwedetail -label "MITRE CWE research view 1000, fetched <date>"
```

### `attackdetail.tsv`

| | |
|---|---|
| Upstream | `https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master/enterprise-attack/enterprise-attack.json` |
| Origin | MITRE ATT&CK, enterprise and mobile domains, version 19.2, enterprise fetched 2026-09-23 and mobile 2026-09-24 |
| License | MITRE ATT&CK terms of use. Redistribution is permitted with attribution. |
| Format | Six fields: the id, the whole description, then references, mitigations, detection strategies and procedure examples as compact JSON arrays |

> This product uses information from MITRE ATT&CK, the work of The MITRE
> Corporation. Neither MITRE nor ATT&CK is affiliated with or endorses this
> plugin.

Kept: every technique, sub-technique and tactic the `attack.tsv` catalog holds,
retired ones included; the mitigations, detection strategies and procedure
examples MITRE relates to each, from active objects only, each with its
attack.mitre.org address; every analytic of a detection strategy with its
platforms, log sources and tunable fields; and every citation with an address
or a description. Dropped: contributors, version stamps, STIX object ids, and
the citation markers inside the text, whose sources are listed as references
instead. It is 7.5 MB, 1.8 MB compressed, and its largest row, `T1105`, is
110 KB. Rebuild it with the catalog, from the same file:

```
go run ./build/cyberdata -only attack,attackdetail -label "MITRE Enterprise and Mobile ATT&CK <version>"
```

### `advisory.tsv`

| | |
|---|---|
| Upstream | Each advisory's STIX JSON, linked from its page under `https://www.cisa.gov/news-events/cybersecurity-advisories/` |
| Origin | Cybersecurity and Infrastructure Security Agency, United States; the advisories are marked TLP:CLEAR |
| License | A work of the United States government, so not subject to domestic copyright |
| Format | Two fields: the indicator (an address in its canonical form, or a lowercase digest) and a compact JSON array of reports, one per advisory naming it |

An advisory is a snapshot: the addresses in it were attacker infrastructure when
it was written and may be reassigned since. Each report carries the date the
indicator became valid and the date the advisory was published, and links to
the advisory. Add an advisory by adding its id to `build/cyberdata/advisories.txt`
and running `make cyber-advisories`.


### `cve.tsv` and `cvedetail.tsv`

| | |
|---|---|
| Upstream | The full files built from NVD's yearly JSON feeds, `https://nvd.nist.gov/feeds/json/cve/2.0/` |
| Origin | National Institute of Standards and Technology, National Vulnerability Database |
| License | A work of the United States government, so not subject to domestic copyright |
| Format | The same rows as the full files, filtered to the CVE ids in `kev.tsv` |

The stamp keeps the full file's compile time and prefixes its source with
`KEV entries only, from`, which is how the panel knows a miss means "not in the
slice" rather than "no such CVE". Rebuild them after `kev.tsv` or the full files
change:

```
go run ./build/cyberdata -only cvekev,cvedetailkev
```

### `ip.tsv.gz`

| | |
|---|---|
| Upstream | `https://iptoasn.com/data/ip2asn-combined.tsv.gz` |
| Origin | IPtoASN |
| License | Public domain, Open Data Commons PDDL 1.0 |
| Format | Address ranges with their autonomous system number, name and country |

The plugin unpacks the archive into `ip.tsv` beside it on first read, about
59 MB, which `.gitignore` keeps out of the tree and the `bundle` target keeps out
of the bundle. Rebuild it with `make cyber-sources`, then:

```
go run ./build/cyberdata -only ip -label "IPtoASN ip2asn-combined, fetched <date>"
gzip -9 -c build/cyberdata/out/ip.tsv > assets/cyber/ip.tsv.gz
```
