# Cyber catalogs

The two files here are compiled into the plugin binary with `go:embed` and are
what the decorator validates an ATT&CK or CWE identifier against. They are the
only cyber data that ships inside the Go binary: everything else is a file an
operator syncs into the directory named by the `CyberDatasetsDir` setting.

They are embedded rather than synced because **decoration has to be a function
of the build**. `Parse` runs in `MessageWillBePosted` and permanently rewrites
the stored message, so the same message must decorate the same way on every
node and on every day. A catalog an operator could change would make that
untrue.

## Provenance

| | |
|---|---|
| `attack.tsv` upstream | `https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master/enterprise-attack/enterprise-attack.json` |
| `attack.tsv` origin | MITRE ATT&CK, enterprise and mobile domains, STIX 2.1; mobile from `mobile-attack/mobile-attack.json` in the same repository |
| `attack.tsv` license | MITRE ATT&CK terms of use. Redistribution is permitted with attribution. |
| `cwe.tsv` upstream | `https://cwe.mitre.org/data/csv/1000.csv.zip` |
| `cwe.tsv` origin | MITRE CWE, research view 1000 |
| `cwe.tsv` license | MITRE CWE terms of use. Redistribution is permitted with attribution. |

> This product uses information from MITRE ATT&CK and MITRE CWE, both of which
> are the work of The MITRE Corporation. Neither MITRE nor ATT&CK is affiliated
> with or endorses this plugin.

## Both files are generated

Each file opens with the same stamp line a dataset carries,
`#tactical-fusion-cyber/1<TAB>name<TAB>generated<TAB>source`, above its header row.
`generated` is when `make cyber-data` compiled the file, and it is what the panel's
Data sources table shows for ATT&CK and CWE.

`cwe.tsv` is the generator's output from MITRE's research view 1000, fetched
2026-09-23: 944 weaknesses, each with its name, abstraction, status, the first
sentence of its description, and its parents in that view.

`attack.tsv` is the generator's output from Enterprise and Mobile ATT&CK 19.2,
enterprise fetched 2026-09-23 and mobile 2026-09-24: 27 tactics, 299 techniques and 522 sub-techniques that are
active, plus 188 revoked and 27 deprecated entries. The two domains are read
separately, since their tactics share names (Mobile's Initial Access is `TA0027`,
Enterprise's `TA0001`), and merged by id; an id both write differently fails the
build. ATT&CK for ICS is left out on purpose: its ids, `T0800` to `T0891`, read
like clock times in operations chat, and a wrong decoration is permanent. Earlier builds shipped a hand-written
seed of 123 entries because the build host could not reach
`raw.githubusercontent.com`; every id the seed held is still here.

### Retired ATT&CK entries are kept

A link to a technique is written into the stored message, so dropping a
technique MITRE retires would break every link to it already posted. ATT&CK 19
revoked Impair Defenses, `T1562`, in favor of `T1685`, and the seed had shipped
it. A retired entry keeps its row with `revoked` or `deprecated` in `status`,
and a revoked one names its successor in `replaced_by`, from MITRE's
`revoked-by` relationship. The panel says the entry is retired and links the
replacement; an active technique lists only its active sub-techniques.

### What the generator changes in ATT&CK's text

- `(Citation: ...)` markers are removed before the first sentence is taken,
  since one sits between most first sentences and the next.
- Markdown links become their text, and backticks and `<code>` tags are
  dropped.
- Typographic quotes and the en dash become their ASCII forms.
- Each entry's tactics are sorted by id, since ATT&CK lists a sub-technique's
  tactics in a different order from its parent's.

Either file changes what decorates: the catalog is what `Parse` validates an
identifier against, so an id the new catalog holds and the seed did not now
becomes a link in messages posted after the change. Messages already posted
keep what they were given.

The reader refuses autolink triggers (`www.`, `://`) and any character outside
letters, digits and `_-,.'"()[]/&+:;*=<>\~$`. MITRE's text uses `*`, `=`, `<` and
`>` in four CWE entries, as in `'filedir*'` and `such as <, >`, and ATT&CK uses
`\` in registry paths, `~` in `~/.bash_history` and `$` once; every surface that
prints catalog text escapes it, and a test holds the standalone page to that.

## How they are produced

```
make cyber-sources    # fetches into build/cyberdata/source, which is gitignored
make cyber-data       # writes these two files, assets/cyber/kev.tsv, and build/cyberdata/out
```

`build/cyberdata/main.go` is standard library only and:

- keeps every tactic, technique and sub-technique that is neither revoked nor
  deprecated, and every CWE row with an id;
- derives each sub-technique's parent from its own id, and its tactics from the
  kill chain phases the upstream object names;
- cuts every description at its first sentence and at 300 runes, because the
  panel and the page both print it against a label;
- collapses tabs, newlines and runs of spaces, and **refuses the whole run** if
  a field still carries a separator, since the reader splits on one;
- refuses a duplicate id.

The ATT&CK reference URL is **not** a column. It is derived from the id in
`format.go`, so a value that reaches a reader can never be a link somebody else
chose.

## Regenerating

Not a prerequisite of `make test`, deliberately, and for the reason
`airport-data` is not either: this transform is filter, cut and sort, and its
drift shows up as an identifier that declines, which is visible and harmless.
`map-data-check` earns that slot because its encoding is opaque and its drift
fails invisibly on a plain-HTTP origin.

What keeps these files honest is a test over the embedded data rather than over
the generator: `TestEveryCatalogIdIsWellFormedAndReachable`,
`TestEveryCatalogReferenceResolves` and `TestEveryCatalogFieldPassesTheWhitelist`
in `catalog_test.go` fail the build on a file that is malformed, that names a
parent or a tactic it does not hold, or that carries a character the whitelist
refuses.
