# Cyber context

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

## Cyber context

The cyber decorator recognizes five kinds of security indicator and renders
what the installed datasets say about each: **CVE** identifiers, **CWE**
identifiers, **MITRE ATT&CK** techniques, sub-techniques and tactics, **IPv4
and IPv6 addresses**, and **MD5, SHA-1 and SHA-256 file hashes**.

It is the first decorator whose answer depends on **install state** rather than
only on the build. The airfield database is compiled into the binary, so an
ident either resolves or does not on every install alike. Here the ATT&CK and
CWE catalogs are compiled in, and everything else is a file an operator syncs
into a directory. That split is the whole design and everything below follows
from it.

### Decoration is a function of the build; enrichment is a function of the install

`Parse` runs in `MessageWillBePosted` and **permanently rewrites the stored
message**. So it may read only what every node has: the embedded catalogs and
`net/netip`. It never opens a dataset file. Two consequences the code depends
on:

- The same message decorates the same way on every node in a cluster and on
  every day, whatever an operator has dropped in or taken away.
- Nothing on the post path pays for a disk read. A 70 MB CVE file would
  otherwise put a binary search behind every message posted.

Enrichment is read at **render**, in `RenderPage` and in `/api/v1/cyber`, which
is the same rule `Decorator.Maps` follows and for the same reason: a picture
drawn afresh each time somebody looks may change when the admin changes it,
where text already written into a message may not. `Decorator.Intel` is a
`func() *intel.Set` beside `Enabled func() Formats`, and nil means no datasets,
which is what every test uses.

### ATT&CK and CWE validate by lookup; CVE validates by shape

`T9999` and `CWE-99999` are well formed and name nothing. Linking them would
write a permanent link to a page with nothing on it, so both are validated
against the catalog this build carries, exactly as an airfield ident is.

**A CVE cannot be.** Identifiers are issued daily and no list of them ships in
the plugin, so a lookup would make decoration depend on which node answered and
on how recently its dataset was refreshed, which is precisely what the previous
section forbids. A CVE is therefore recognized by shape, with a floor of 1999
because the scheme starts there, and an identifier no dataset holds still links
and says so on its own page.

Hashes are shape only. A 40 digit hash is also the shape of a Git object id;
the panel says exactly that rather than choosing between them, because they are
the same hash and a reader looking for either is served by the link.

### The consumed tail, and why it is not a weaker guard

`Pattern.Boundary` is handed the runes outside the **whole match**, while
`ReplaceGroup` narrows the claim and the rewrite. Every cyber pattern uses that
split the way the airfield grammar uses it for `//`: the trailing rune is
matched, so the guard looks past it, and it stays in the message because it is
outside group 1.

What is consumed, and what each one buys:

| Consumed | Example | Without it |
|---|---|---|
| `[.,;:!?]` | `Patched CVE-2021-44228.` | a sentence-ending indicator never decorates |
| `:<port>` | `203.0.113.7:443` | the commonest way an address is written in this traffic |
| `%<zone>` | `fe80::a%eth0` | a zoned address never decorates |
| `md5:`, `sha1:`, `sha256:` | `sha256:e3b0...` | a Docker digest never decorates |

`1.2.3.4: connection refused` is the log line this audience pastes most, and it
decorates because the colon is consumed. The DTG decorator had to write its own
guard (`outsideDelimitedField`) for the same collision; this one does not,
because consuming the character is the sanctioned move and **`BoundaryOK` is
used unmodified**. Do not add a third hand-written guard.

The hash label is **kept, not consumed**, which is the one place this differs
from the DTG moniker. Consuming it would edit what the author wrote out of the
stored message, and `sha256:` in front of a digest is information.

Each label is its own pattern paired with its own length, so `md5:` in front of
64 digits does not match the labeled pattern, and the bare pattern's guard sees
the `:` and refuses. The check falls out of the grammar rather than needing
`Parse` to see a label it is never handed.

### What is declined, and why each one

- `1.2.3.4.5` matches `1.2.3.4.` with a digit after it, so the guard refuses and
  the scanner has already consumed the tail: the lone `5` cannot match either.
- `1.2.3.4/24` stops at the address and the guard sees `/`. A link over half a
  CIDR misrepresents the range, and a CIDR kind is a later phase.
- `01.2.3.4` and `999.1.1.1` fail `netip.ParseAddr`, which refuses leading zeros
  and octets above 255.
- `0.0.0.0`, `::`, `127.0.0.1` and `::1` are refused in `Parse`. Nothing can
  enrich them, so a link over one is a permanent rewrite for nothing. Both
  families are treated alike so the two cannot disagree.
- `12:30:45`, a MAC address and `1:2:3` are IPv6 candidates the regex admits and
  `netip.ParseAddr` refuses. `std::vector` and a timestamp's `T12:30:45` are
  refused earlier still, by the guard's letter test.
- A 33 digit hexadecimal run matches its first 32 and the guard sees a hex digit.

Two residual IPv6 shapes survive and are accepted rather than chased: an
eight-group EUI-64 written with colons, and prose such as `cafe::babe`. Both
parse as addresses and both are rare; the cost is a link to a panel saying
nothing is known. Bracketed `[2001:db8::1]:443` is declined, and the bracketed
span is a protected range anyway.

**A four-part version number decorates as an address.** `10.0.190.41` is
indistinguishable from one. That is the ordinary on-by-default cost every other
switch carries, it is noisy rather than confidently wrong, and
`EnableCyberIP` turns it off. It is not the `EnableLocationUTM` class, which
puts a real coordinate in the wrong place.

### Mentions, channel links and hashtags are protected spans now

`findProtectedRanges` protected bare URLs and `www.` hosts because Mattermost
autolinks them and rewriting inside one destroys the reader's link. `@user`,
`~channel` and `#hashtag` are the same class and were not protected.
`#CVE-2021-44228` would have been rewritten inside the hashtag.

Three expressions were added, each with a consumed leading context rune so
`~~strike~~` and a `##` heading stay out:

```
(?:^|[^\w@])@[\w.\-]+
(?:^|[^\w~])~[\w.\-]+
(?:^|[^\w#])#[A-Za-z][\w.\-]*
```

**This is a framework change and it reaches every decorator.** A DTG written as
`#091630Z` stops decorating, which is the safe direction. A sweep of every
decorate-path fixture in the repository found nothing that flips.

The consumed context rune joins the protected range, which produces two edges
that are tested so they are deliberate rather than discovered later:

- `203.0.113.7,@bob` does **not** decorate. The address pattern consumes the
  comma, so its match overlaps the range the mention opens on that same comma.
  `203.0.113.7 @bob` decorates, because a space is not consumed by either.
- `root@203.0.113.7` **does** decorate. The mention expression needs a non-word
  rune before the sigil, and `t` is a word character, so an email-shaped run is
  not a mention.

### The datasets are files, searched in place

One file per dataset, `<name>.tsv`, discovered from the bundle's `assets/cyber`
and then from `CyberDatasetsDir`, later directories overriding earlier ones by
file name. That is `packages.go`'s shape, including its `p.API == nil` branch
for tests and its `warnOnce` so a broken file does not spam the log.

The first line is a stamp:

```
#tactical-fusion-cyber/1<TAB><name><TAB><generated RFC 3339><TAB><source>
```

A file with no stamp, or one built for a different reader, is skipped with
`TF-20002`, which is distinct from `TF-20001` for a file that is simply
unreadable: the remedy is a newer dataset rather than a repaired one. The stamp
also carries **when the dataset was generated**, which is what lets every status
sentence name a date rather than saying only that something is missing.

**Rows are sorted bytewise on the first field and searched in place**, by
binary search over byte offsets with `ReadAt`. No index is built, so a 70 MB
CVE file costs no heap and no warm-up; the page cache carries the working set.
About twenty 4 KB reads answer a lookup.

The IP dataset keys every range on the **16 byte form of the address, hex
encoded**, so one file and one comparator serve IPv4 and IPv6 and the reader
needs to know nothing about either. `Addr.As16()` maps IPv4 into the same
ordering, and a test pins that a mapped address keys identically to its own
family.

The watchlist is read whole into a map rather than searched, because it is
small and because **one indicator may carry several rows**, which a keyed search
would hide. Its verdict vocabulary is free text, with `malicious`, `suspicious`
and `benign` recognized for styling only, so a third-party export loads without
being edited first.

Handles live across lookups and are reopened when a file's size or modification
time moves, or when the directory list changes, checked on a 5 second TTL. That
is what makes a dataset dropped in visible within a few seconds with no
restart, and it is why `Set.Changed` counts the files it finds as well as
stating each one: a newly added dataset changes the count without changing any
open file.

Vendor `.mmdb` databases are read through `maxminddb-golang/v2`, which is ISC
licensed. A vendor database wins over the range file **field by field**, since
it is the more specific source an operator went out of their way to install.
**None ships**: the GeoLite2 license does not permit redistribution.

### `assets/` rather than `public/`

Mattermost serves a bundle's `public/` directory to anybody who can reach the
server, with no session. It does not serve `assets/`, which the `bundle` target
copies in the same way. A dataset is read by the plugin and never handed out,
and the watchlist in particular is an operator's own notes, so `assets/cyber`
is the right home and `public/` would be a disclosure.

That claim about what Mattermost serves is **its behavior, not something this
repository can test**; see [`unverified.md`](unverified.md).

### Rendering happens once, in Go

`Describe` returns `Details`, and everything in it is already a string: the
rows, the status sentence, the headline the hover shows, the related links and
the dataset list. The page prints them and the API hands the same values to the
panel. There is no `format.ts` and no paired fixture table, which is the
`Conversion` precedent: the rendering rules are the interesting part, they live
in Go, and a second implementation in TypeScript would be a second thing to get
wrong.

The three status sentences are built in one place for the same reason:

- `No vulnerability dataset is installed.`
- `Not in the vulnerability dataset generated 2026-09-01T00:00:00Z.`
- `A private address, which no dataset describes.`

**The distinction between the first two is load-bearing.** "Not listed in KEV"
and "no KEV dataset here" mean opposite things to a responder, and a panel that
collapsed them would tell somebody a vulnerability is not being exploited on
the strength of a missing file.

### The page, and the one thing it does not show

`PageStatic`, no script, so `script-src 'none'`. A kind and value that do not
reproduce each other is `TF-17003` at 400, checked before anything is echoed;
a well-formed indicator no dataset holds renders at 200 with its status
sentence, the way an unknown airfield does.

`Cache-Control: private, max-age=60`, shorter than the airfield page's 300,
because this page is a function of install state as well as of the build and a
dataset an operator just replaced should appear within about a minute.

Catalog fields pass a whitelist at parse. **Dataset fields do not**, because an
operator's watchlist note is theirs to write, so the page's escaping is the
only thing standing between a row and the markup. A test drives `<script>`,
`<img onerror=>` and `](` through a fixture row and asserts that no element the
renderer did not write appears in the output. It asserts on the **tags** rather
than on dangerous words, because a word inside escaped text is inert and a test
that failed on one would be testing the wrong thing.

The page carries **no prior mentions**. A page is the same for everybody, and
who has mentioned an indicator is not.

### Prior mentions are the one per-reader answer

`/api/v1/cyber/mentions` takes a kind, a value and a team, and searches with
`SearchPostsInTeamForUser`. The older `SearchPostsInTeam` takes no user and so
applies no channel membership: it would put posts from private channels the
reader is not in onto their screen. That is the whole reason this route exists
separately rather than being folded into the page.

The team is **required** and validated as a Mattermost id. An empty team never
means every team, which is the kind of default that turns into a leak.

The snippet collapses markdown links to their own labels and is cut at 200
runes, because a reader scanning for what was said does not want the link
syntax this plugin itself wrote into the message.

Whether a search backend tokenizes a dotted quad or a hyphenated identifier
into a findable phrase is **unverified**; the terms are sent as a quoted phrase
and the panel says "no earlier mention was found" rather than implying there
was none.

### The webapp

The client is the airfield cache in shape, keyed on `kind:value`: one in-flight
promise per key, `CACHE_TTL_MS` rather than forever because an answer is a
function of `(indicator, install state)`, and a `failed` read is never cached
so one bad minute does not cost every indicator in the channel.

Mentions are a second client with its own key, fetched by the **panel only**.
The hover never asks: a hover is a glance and a search is not, and a test pins
that no mentions request leaves the hover.

The current team comes from `currentTeamId()` in `selection.ts`, which reads
the Redux store `initRhs` already holds, through a narrow local type. No
`mattermost-redux` dependency for one string. An empty id skips the fetch
entirely rather than asking the server to decide.

### Switches

Six, in a seventh section. `EnableCyber` is the parent and is ANDed with each
kind in Go, because a manifest section groups without gating. All six default
on: every one trades a false positive that is merely noisy against a missed
decoration, and none is the confidently-wrong class that keeps
`EnableLocationUTM` off.

`CyberDatasetsDir` is a path rather than a switch, and carries the same cluster
warning `LocationMapPackagesDir` carries: every node must see the directory, or
an indicator enriches for some readers and not others.

### The catalogs that ship are a seed

The two embedded catalogs were written by hand rather than produced by
`build/cyberdata`, because the host this was built on could not reach
`raw.githubusercontent.com` or `cwe.mitre.org`: both are refused by that
environment's network policy. Inventing technique names and weakness
descriptions would be worse than shipping fewer, so the files carry only
entries that could be stated accurately: every enterprise tactic, the widely
used techniques and sub-techniques, and the CWE Top 25 with the classes around
it.

An identifier the seed does not hold is **left as written**, which is the safe
direction and the same rule an unknown airfield ident follows. `make
cyber-sources && make cyber-data` on a connected host replaces both files
wholesale, and nothing else changes: the reader, the grammar and the tests are
written against the format rather than the contents.

`assets/cyber/kev.tsv` is **not committed** for the same reason: `www.cisa.gov`
is refused there, and a fabricated KEV catalog would tell a responder that a
vulnerability is or is not being exploited in the wild on the strength of
nothing. Until it is generated the decorator behaves as it does on any install
with no datasets, and says so in those words.

### What holds the two sides together

| Duplicate | Guard |
|---|---|
| The wire shapes: names, types and order | `TestWebappCyber*ShapeMatches` |
| The decorator type | `TestWebappCyberTypeMatches` |
| The kind vocabulary and its order | `TestWebappCyberKindsMatch` |
| Each kind's canonical shape expression | `TestWebappCyberShapeExpressionsMatch` |

The token grammar itself is Go-only, so the two sides cannot drift on what a
token is. The webapp carries only the **canonical** shapes, which is what it
needs to refuse a hand-edited link before asking the server about it.
