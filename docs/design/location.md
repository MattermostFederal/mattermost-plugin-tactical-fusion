# Location

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

## Location

The location decorator normalizes every coordinate grammar it recognizes to one
WGS84 latitude and longitude. **The identity of a location is the pair `(f, v)`**,
the format id and the canonical token, and everything else is derived from those
two. Nothing derived is ever carried in the URL, which is what makes a link
impossible to disagree with itself.

A third parameter, `r`, carries the author's own text. It is **display only and
derives nothing**, and it exists because the standalone page is what mobile
clients open: that page shows only the link, not the message it came from, so
without `r` the author's spelling is unreachable there. `r` is omitted when it
would only repeat `v`, which is the common case for the USMTF shapes.

`r` is the one parameter that is text from a message echoed onto a public page,
so it is never treated as free text. Four gates, in `validateRaw`, and a failure
rejects the **whole link** rather than dropping the row:

1. at most 64 bytes;
2. an explicit rune whitelist, never a blacklist;
3. it must **anchored-match the token sub-expression for `f`**, which is what
   makes content spoofing structurally impossible rather than merely escaped;
4. `canonical(parse(r))` must equal `v`, so it can never name a different place.

Escaping on output is the fifth layer, not the first.

### Why Location holds text, not floats

`Axis` holds `Deg`, `Min`, `Sec`, a `Frac` **string** and a `FracUnit`, and
`Decimal()` is derived and **must never be reached from `canonical()`**. Holding
a float and rebuilding the canonical form from it cannot round-trip a
sexagesimal token: `340322N` parses to 34.05611111..., and recovering whole
seconds from that float can land on 21.99999999, so the canonical form comes back
as `340321N`. Since a link is rejected when it does not reproduce its own token,
that would turn a token accepted at decoration into a permanent 400 on the page
behind it, editable only by hand.

`Frac` is a string because it is what `canonical()` writes back out. `Digits()`
is `len(Frac)`, derived, so the two cannot disagree. Fractional digits are capped
at 8, past which a float64 stops reproducing the decimal string it came from.

`Conf` carries the USMTF verified confidence digits. They say how well a position
is known rather than where it is, so they never reach `Decimal()` and are
rendered beside the resolution rather than folded into it. The reference
implementation in `mattermost-plugin-aocanywhere` matches them and drops them.

### Grammars

`dd` (signed decimal degrees), `ddh` (hemisphere letters), `dms` (which doubles
as USMTF LATS, and with a fractional second as LATDS and DMPID), `ddm` (USMTF
GEOK), the fixed-width `latd`, `latm` and `vlatm`, the two projected
grammars `mgrs` and `utm`, and the three area-reference systems `georef`, `gars`
and `pluscode`.

**There is no `usng`.** The United States National Grid is MGRS on WGS 84,
character for character, so a separate id would be a second grammar accepting
the first one's tokens, and the format named on the page would become a guess
about which of two identical readings the author meant. `USNG:` is a moniker in
front of the grid grammar instead, which is what it actually is, and
`TestUSNGLabelsAGridReference` pins that the label reaches `f=mgrs`.

Every scanning pattern and every anchored validator is built from the same
sub-expressions in `grammar.go`, so a change to what a token looks like cannot
reach one and miss the other. That shared origin is also what makes gate 3 above
well defined.

**Signed decimal degrees is the weakest grammar** and has its own switch for that
reason. It requires a comma and at least four fractional digits on both values.
`34.05, -118.25` is declined, deliberately, and that is documented rather than
hidden.

**`latd` has no bare pattern.** `35N079W` is seven characters resolving to a
111 km square, and is reachable behind a USMTF label only. Turning everything off
except `EnableLocationUSMTF` and `EnableLocationMoniker` reproduces the posture of
`mattermost-plugin-aocanywhere`, whose `enhanced_text/patterns.ts` decorates a
coordinate only when the author labeled it. That is a supported configuration
and a test pins it.

**MGRS and UTM are separate switches** (`Formats.MGRS` and `Formats.UTM`), and
**UTM is the one grammar in this package that ships off**. They were one switch
until the band letter was read as a band rather than refused: at that point UTM
became the only grammar that can decorate a *real coordinate* and point at the
wrong place, which is a different cost from every other grammar here, where the
worst case is a false positive on text that was never a coordinate. An install
that wants grid references without that has to be able to have exactly that, so
the switch is split and the default is the safe reading. See the Admin settings
section for the whole argument and `TestMGRSAndUTMSwitchIndependently` for the
pin. Turning UTM off costs no UTM **row**: that is derived from the position, not
matched in the text, and `TestUTMSwitchedOffStillRendersTheUTMRow` says so.

**The grid grammars have a bare pattern narrower than their labeled one**, and
that distinction is per-*expression* rather than per-format, which is why it
lives in `bareExprs` in `grammar.go` rather than in `bareFormats`.

A run-together `18SUJ2347806483` **is** detected unlabeled, under two
restrictions that were arrived at by measuring rather than by arguing. The
argument they replaced was wrong in an instructive way: the stated worry was
part numbers, and the real collision is **hexadecimal**. Over 200,000 generated
short git SHAs, in sentences, through the real tagger, an any-case run-together
pattern decorated 51 of them, about one in 3,900. `58cbe40` is zone 58, band C,
square BE, a 10 km square off the coast of Antarctica.

So `mgrsBareCompactExpr` is **upper case only** and requires **three digits per
axis**. Both numbers were arrived at by being wrong first, which is worth
recording: uppercase-only was claimed to leave nothing behind, and at two digits
per axis it did not, at about one uppercase run in 75,000 (`26HMA1997`,
`3UTA7623`, `6CZA9867` are all valid grid references and all look like part
numbers). Three digits finds none across 1.2 million.
`TestBareGridReferencesDoNotMatchOrdinaryRuns` pins it with a *generated* corpus
sized against the rate each restriction removes, because the version before it
used 30,000 samples against a 1-in-75,000 rate and passed only because its seed
was lucky. Neither restriction puts anything out of reach, since spacing or a
label reaches both.

**UTM run together stays label-only**, and that is not an oversight. After the
zone and band it is thirteen bare digits with no letters to check, so the only
validation available is that the northing lands in the band, which a great many
thirteen-digit runs do. There is nothing there to make narrower.

The letter classes are written once in upper case (`bandBody`, `colBody`,
`rowBody`) and lowered mechanically by `anyCase`, because the bare grammar
depends on the upper-case class being exactly the upper half of the any-case
one.

**The letter after a UTM zone is a latitude band, never a hemisphere.** `11S` is
band S (34 north), not "zone 11, southern hemisphere" (56 south). Nothing in the
token disambiguates the two, so this is a **convention rather than a
deduction**, chosen because this is a plugin for a military audience: MGRS is
the military implementation of UTM (USGS) and the third character of an MGRS
grid zone designator is a latitude band (NGA). UTM therefore takes the whole
band alphabet and there is no separate `utmBandLetter` class.

An earlier version refused `N` and `S` outright as ambiguous. That declined the
ordinary military spelling of a position in order to protect a civilian reading
this audience does not write, and the coordinate that exposed it,
`11S 384640E 3769080N`, is as plain a paste as this feature gets.

**The cost is not symmetric between the two letters**, which is what to know
before touching this again:

- **`N` is nearly free.** Hemisphere-north and band N both use the northing as
  written, so the two readings put the point in the *same place*; the band
  reading merely also requires it to be in the first 8 degrees north. Reading N
  as a band can decline a civilian token but can never misplace one, and
  `TestBandNCannotMisplaceAPosition` says so.
- **`S` is where the risk is.** Hemisphere-south measures from the false origin
  and band S does not, so the readings differ by up to 90 degrees of latitude.

What limits that is `gridPoint`, which already validated **band containment and
zone proximity** before this change, so a token survives only when the military
reading is geometrically consistent with itself.
`TestSouthernHemisphereTokensMostlyDecline` measures the residual over 20,000
generated southern-hemisphere pairs: **10.1% survive**, so about nine in ten are
declined by the band check rather than silently relocated. The assertion is a
loose bound (20%) because the number is a property of the notation, not of this
code; it exists to fail if the band check is ever weakened.

**The axis letters are optional**: `11S 384640E 3769080N` and
`11S 384640mE 3769080mN` parse to the same canonical form as the bare pair, so
they are display only and the `r` parameter carries the author's spelling. They
must be **adjacent to their digits**. An optional letter separated by a space
would reach into the following word: in `11S 384640 3769080 East` the token
would swallow the `E`, the boundary guard would then see a letter, and a token
that decorates today would silently stop.
`TestUTMAxisLettersDoNotReachIntoTheNextWord` pins it.

MGRS is unaffected by all of this: its band letter is followed by two more
letters and could never be read as a hemisphere.

### The area-reference systems

`georef`, `gars` and `pluscode` name a **cell of the graticule** rather than a
point, the way MGRS names a square, and the position reported for one is its
center. They live in `area.go`, and what separates them from the grid grammars
is that **none of them touches the ellipsoid**: they subdivide degrees, so
decoding one is arithmetic and there is no projection, no zone exception and no
polar gap. That last one is the whole reason they earn a row beside MGRS rather
than instead of it. Past 84 north and 80 south the two grid rows are blank and
these three are what is left, which
`TestAreaRowsRenderWhereTheGridRowsCannot` pins.

Their identity is held as the **code text**, in `Area`, which is the strongest
form of the rule `Axis` and `Grid` follow: `canonical()` returns the stored
string, so it cannot fail to reproduce itself and there is no reassembly step to
get wrong. Upper-casing is the whole of the normalization, since none of the
three admits internal spacing. `parseArea` validates by **decoding**, because
the alphabets admit combinations that name nothing (just over a third of the
GARS letter pairs are past the 360 bands that exist), and asking for the position is
the same call every rendered row makes.

**GEOREF is written longitude first.** That is the standard's own order and the
opposite of every other grammar here. Getting it backwards is not a parse
failure, it is a position in a different hemisphere, so
`TestGEOREFIsLongitudeFirst` pins it against a worked example rather than
against a round trip of itself.

**GEOREF and GARS are label-only, and Plus Codes are not.** `GJNJ5753` is four
letters and four digits and `006AG39` is seven alphanumerics; neither has
anything in it a part number does not, so both sit in `labeledFormats` beside
LATD. A Plus Code's alphabet is twenty characters with no vowel among them and
its separator sits at a fixed position, which is a shape ordinary text mostly
does not have.

"Mostly" was measured, and the first measurement was a correction. The stated
argument was that no word is eight characters of that alphabet, which is true
and was not enough: over 100,000 generated `<identifier>+<build>` version
strings, an any-case pattern rewrote two, about one in 50,000, and
`44vc8qch+p86` is a well-formed code. So `olcBareExpr` is **upper case only**,
exactly as `mgrsBareCompactExpr` is and for the same kind of reason, since the
collision space is lower case. That leaves a residual of about one in 50,000 on
*upper-case* build metadata, which
`TestBarePlusCodesDoNotMatchOrdinaryRuns` asserts as a **loose bound** rather
than as zero: both survivors are genuinely valid Plus Codes with nothing in them
to tell from a real one, so it is a property of the notation, not of this code.
The test keeps the lower-case corpora it was failing against rather than
swapping them for ones that pass.

Short Plus Codes (`CWC8+R9`) stay declined: resolving one needs a reference
location this plugin does not have. **Padded** codes (`849V0000+`) are accepted,
and that is not generosity: a padded code is the only way the notation writes
anything coarser than a 275 m cell, and a whole-degree coordinate's derived row
needs exactly that.

A Plus Code's grid refinement is **five rows of latitude by four columns of
longitude**, not the other way round, and getting that backwards is the defect
this phase shipped and a review caught. It survived because both reference
vectors to hand (`8FVC2222+22` and `849VCWC8+R9`) are ten characters and so
never reach the grid section at all, and because the encoder and the decoder
were transposed *consistently*, so every round-trip test passed while every
code past ten characters named the wrong sub-cell. That is exactly what
`geodesy_test.go` warns about: a round trip proves the inverse undoes the
forward and says nothing about whether either is right. It is pinned now
against the spec's own constants rather than against a vector,
`FINAL_LAT_PRECISION` = 8000 x 5^5 = 25,000,000 and `FINAL_LNG_PRECISION` =
8000 x 4^5 = 8,192,000, in
`TestPlusCodeGridRefinementIsFiveRowsByFourColumns`.

Because of that split, `areaResolutionDegrees` quotes the **larger** of the two
extents rather than latitude specifically: past ten characters longitude is the
coarser half, and below that the two are equal.

**The encoders snap before they floor**, in `cellIndex` and `cellIndex64`, and
that is the second defect this phase shipped and a review caught. A bare
`math.Floor` puts a position that sits exactly ON a cell boundary in the
previous cell, because the product lands an ulp low:
`(35.166666666666664 + 90) * 60` is `7509.999999999999`, so 35 degrees 10
minutes encoded as minute **09**. Whole minutes, whole seconds and round
decimals are nearly everything this plugin decorates, so this was the common
case rather than an edge of it: 1,855 m south on the GEOREF row, 9.3 km on the
GARS row, and about a fifth of two-decimal coordinates on the Plus Code row.
The same token's DMS and USMTF rows read `35°10'` while its GEOREF row read
`5909`, on one page.

`math.Round(x * 1e6) / 1e6` before the floor is what the Open Location Code
reference does, and the snap is a millionth of a cell, far below the 0.9 of a
cell `TestAreaEncodersTruncateRatherThanRound` asserts. The lesson is the same
one `roundTo` records for the arm64 FMA: a float that is a hair off looks right
until something reads the field back.

Two fixtures pinned the **wrong** values through this, `GJLF5909` and
`202LL46`. Re-derive an area fixture from the notation rather than re-recording
it from the implementation, or a defect becomes its own expectation.

The derived rows follow the same resolution rule everything else does, off a
fixed ladder per notation (`georefLadder`, `garsLadder`, `olcLadder`) instead of
a digit count, and compared **in degrees** rather than in meters, since these
subdivide the graticule directly and routing through `degreeMeters` would put an
approximation between a token and a row exactly as coarse as it.

**The coarsest GEOREF row is not an input token.** A whole-degree coordinate
renders `GJMF`, four letters and no digits, which `Parse` declines because four
letters is a word. That is the same exception the 100 km MGRS square is and it
holds for GEOREF alone: the coarsest GARS and Plus Code rows do read back.
`TestAreaRowsAreTokensThisPackageAccepts` never generates a coordinate coarse
enough to reach it, so `TestTheCoarsestGEOREFRowIsNotAnInputToken` pins the
exception directly rather than leaving it to a corpus that happens not to look. All three
encoders **truncate**, which is the same rule `mgrsAt` follows: a cell is chosen
by containment and a distance by nearness. Each shifts into a non-negative frame
before flooring, so truncation means containment on both sides of the equator
and the prime meridian, and `cellIndex` clamps the pole and the antimeridian
into the last cell rather than one past it, where every alphabet in the file
would be indexed off its end.

Unlike the other derived rows these three are served from the **conversion
endpoint** rather than computed in the panel, and that is a judgment rather than
a necessity. `format.ts` could render them; what it would need is an encoder and
a decoder each, six pieces of arithmetic held to the Go ones by fixtures, to
save a request the panel already makes unconditionally on every open. What the
panel still computes locally is their **resolution**, from the length of the
code alone, so that row is on screen at once and survives the request never
landing.

**Monikers are the USMTF field labels**, taken from the standard rather than
invented, and like `DTG:` they are **consumed**: `LATM:3510N07901W` becomes a
link reading `3510N07901W`, with the label gone. The label says what kind of
thing follows, and once the thing is a link that says so itself the label is
repeating it.

They were kept for a long time, on the argument that `LATM:` is part of a
structured line an author may be quoting verbatim. That cost is real and is now
accepted rather than avoided: this hook rewrites the STORED message, so a
labeled coordinate loses its field label permanently, in exports and in the raw
text an author sees when editing. What bounds it is the boundary guard: a
genuine USMTF set line ends `//`, and the trailing side declines a `/`, so the
lines most likely to be quoted verbatim as a structured record never decorated
in the first place. `TestAGenuineUSMTFSetLineIsLeftAlone` pins that half, and
it is what makes the trade a small one rather than a large one. `LOC:`, `DEPLOC:`, `ARRLOC:` and `ICAO:` are permanently
excluded, because in USMTF they introduce an **ICAO airfield code**, which is a
facility whose position must be looked up rather than computed. An earlier draft
of the plan invented `LOC:` for coordinates, which would have collided with both
the standard and the sibling plugin.

### Boundary guards

`\b` is the wrong guard for a token that does not start and end with a word
character, and the guard **must not be part of the regex**. A pattern that
consumed its own guard characters breaks the *next* match, because
`FindAllStringSubmatchIndex` returns non-overlapping matches: the first token eats
the space the second one needs as its leading guard, and the second goes silently
undecorated. Two grids on one line is the most ordinary input this feature has,
and there is a regression test for exactly it.

**The labeled patterns use the same guard as the bare ones.** They did not: the
moniker guard refused only a letter or a digit on the leading side, so
`logs/MGRS:18SUJ2347806483` was rewritten in place while the bare token in the
identical position was correctly declined, because `badNeighbor` rejects `/` and
the moniker guard did not. Rewriting the middle of a path is the failure this
file is arranged around. The cost, named rather than hidden: a USMTF line quoted
with slash delimiters no longer decorates, which turns out to cost almost
nothing because a genuine one ends `//` and was already declined by the trailing
side of the same guard.

So the guard lives in `Pattern.Boundary`, which the framework calls with the
runes flanking a match. `.` and `,` are rejected on the trailing side, which
costs a decoration when a coordinate ends a sentence with no space. That is a
deliberate trade: at the point the guard runs, `-118.2500.` and the middle of
`-118.2500..-118.2600` are the same thing, and a missed decoration is a feature
gap while rewriting a range is corruption.

### Rendering

**Neither surface has a lead line.** The panel and the page both open straight
onto the table. There used to be a large line above it repeating the grid
reference, three lines above the labeled row that already carried it with a copy
button beside it, so it said the same thing twice and the copy of it that a
reader could actually use was the lower one. The page's
`described` class belongs to the DTG page now. The panel's "shows each reading
once" is what holds this, and it is now the only thing that does: the layout is
rendered once, in TypeScript, so the Go half of that pair
(`TestRenderPageHasNoLeadLineAboveTheTable`) went with the Go renderer.

Every row renders at the resolution the token carried and no finer, and **rounds
rather than truncating**. A coordinate written to two decimals renders
`34.06° N`; a degrees-only one renders `35°N`, not `35°00'00"N`, because padding
a field the author never wrote is a claim. `LatLonToLATM` in the sibling repo
truncates minutes, which biases every result up to 1.8 km south and west.

"No finer" is a **ceiling and not a floor**, which is why a **value** renders
per axis while the **resolution** renders for the pair. The two halves need not
have been written to the same precision, and `ddh` admits that from ordinary
text: `34.0561N,118.2W` is a thing people paste. Rendering its latitude at the
longitude's one decimal gave `34.1° N`, **4.9 km north of what the author
wrote**. That is the identical defect `canonicalString` is held away from, with
the identical magnitude, so it is fixed the identical way: `Location.Digits()`
(the coarser half) sizes `ResolutionText` and every derived grid row, and
`axisResolutionDegrees` sizes the decimal, DMS and DDM rows from each half's own
`Axis.Digits()`. A pair reading `34.0561° N, 118.2° W` beside "about 11.1 km" is
telling the truth twice rather than contradicting itself, and `Digits()` must
reach nothing that writes a value out.

Both implementations are still live and still have to agree, though not for the
reason they used to. Every surface renders through `format.ts` now, but Go
renders the same values into the `Conversion` payload, which is where a grid
token's readings come from on every surface. So `renderFixtures` in
`server/decorators/location/format_test.go` and the matching table in
`webapp/src/decorators/location/format.spec.ts` are the same inputs and the same
expected strings. **Change one and change the other.** Two of those rows are
mixed-precision pairs and exist only to pin the split above.

The webapp also keeps its own copy of the **canonical shapes**, in `CANONICAL`
in `format.ts`, because `fromParams` has to validate the token a link carries
and the grammar is Go-only. That is a smaller duplication than the grammar and
still a duplication, and it has cost once: the band class was widened here to
read N and S as latitude bands, the webapp kept the older narrower one, and a
UTM link the server had just issued failed the webapp's check.

**That failure is silent by construction.** The click handler reads a null
payload as "not one of ours", stands aside, and the browser follows the link to
the standalone page. The page renders correctly, so it looks like a routing
choice rather than a rejected payload, and nothing is logged on either side. The
only symptom is that clicking a coordinate opens a page instead of the sidebar.

The area-reference systems widened that surface from one alphabet to five: the
webapp needs `GEOREF_ZONE`, `GEOREF_BAND`, `GEOREF_UNIT`, `GARS_LETTER` and
`OLC_CHAR` to validate a token's shape, and none of them is checkable by
geometry the way a grid reference's letters are.
`TestWebappAreaAlphabetsMatch` holds all five to the Go copies, in a table, so
adding a sixth is one row rather than a new test.

Two things guard it now. The webapp writes the band class **once**, as `BAND`,
and builds both grid patterns from it, for the same reason `bandBody` is written
once here. And `webapp_sync_test.go` reads `format.ts` and compares the band,
column and row classes against this package's, so a change on either side that
does not reach the other fails in Go with the reason spelled out.

The row is labeled `RESOLUTION`, not "precision": a phone with 5 m of real
accuracy still emits six decimals, and "precision" invites reading that as a
claim about the fix.

The **USMTF row** is derived the same way and is the one place a family rather
than a format has to be collapsed to a single answer. The shape follows the
token's resolution on the same principle `gridDigitsFor` uses to size a square,
the coarsest field set no coarser than what the author wrote: LATD for whole
degrees, LATM for whole minutes, LATS for whole seconds, LATDS below that.
Padding LATM onto a degrees-only token would be a claim about two fields nobody
wrote.

It is sized from the **pair**, unlike the DMS and DDM rows and like the grid
rows, because a USMTF token is one fixed-width shape covering both halves: there
is no spelling of it in which latitude carries seconds and longitude only
minutes. `34.0561N,118.2W` therefore renders `3403N11812W`, losing the fine
half's digits in that column alone, and a fixture pins exactly that.

It never carries **confidence**. A verified token states how well its position is
known, which is a property of the measurement rather than of the arithmetic, so
a derived reading cannot inherit it; the token keeps its own Confidence row.

**Which tokens have one at all is a whitelist**, `confidenceFormats`, and not a
check on `Axis.Conf` alone. `NoConfidence` is -1 rather than the zero value, so
a format that keeps no `Axis` has zero-valued ones that read as "the token
stated confidence 0". Every MGRS and UTM link did: the standalone page carried a
Confidence row on a notation with no such concept, while the panel showed none,
because the webapp reaches that row through a `Coordinate` a grid token has
none of. The two surfaces disagreed about the same link, and did it in the one
place the paired fixtures do not reach. The list runs the safe way round, so a
grammar added later and forgotten shows no confidence rather than claiming one,
and `TestOnlyAVerifiedTokenReportsConfidence` drives it over the whole positive
corpus rather than over the formats that had the bug.

It is also the only derived row whose output is an input grammar, so it is held
to something the others cannot be:
`TestUSMTFRowIsATokenThisPackageAccepts` re-parses every fixture's rendering and
requires it to land within its own resolution. A row emitting a shape nothing
here recognizes would be a value a reader could paste into an ATO and have
refused by the next tool along.

The resolution rule applies to the **derived** grid rows too, which is where it
is easiest to break: every conversion tool in existence hands back a ten-figure
grid reference whatever it was given. `gridDigitsFor` picks the largest square
that is no bigger than the token's resolution, so `35N079W` renders `17S PU`, a
100 km square with no digits at all, rather than a one meter one.

**`roundTo` normalizes negative zero, and that is not tidiness.** On arm64 the
compiler may contract a multiply and a subtract into a single FMA, which the Go
spec permits, so the seconds residue in `degMinSec` came out at about -4e-14
rather than zero. `math.Round` keeps the sign and `fmt` renders `-0`, filling the
field width exactly, so nothing looked wrong: the DMS row showed `0°01'-0"N` and
the USMTF row `0001-0N1000100E`, which no USMTF grammar accepts. It reached 0.13%
of two-decimal hemisphere coordinates, and the same source on amd64 was correct,
so the Go page and the TypeScript panel disagreed about the same link.

That defect is also a lesson about the test that was supposed to catch it.
`TestUSMTFRowIsATokenThisPackageAccepts` asserts a **universal** property, and
ran it over ten hand-picked fixtures, so it was checking a claim about all
inputs against inputs chosen to satisfy it. It is now driven by a generated
corpus, with the fixtures kept only for the shapes a generator reaches rarely.
`TestNoRenderedFieldCarriesASign` is its sibling and states the defect directly,
because the DMS row rendered a sign without failing to re-parse and a round-trip
test alone would have left it broken.

`mgrsAt` **truncates** where every other rendering path rounds, and that is the
same rule rather than an exception to it: a square is chosen by containment and
a distance by nearness. `utmAt` therefore rounds.

### Geodesy

`geodesy.go` is the WGS 84 transverse Mercator series (Snyder, USGS Professional
Paper 1395), hand-written rather than taken from a dependency. The target is
air-gapped installs behind an SBOM and a CVE gate, where a dependency is a
permanent cost to the operator, against maths that has not changed since 1987.

The price of hand-writing it is the obligation to prove it, so `geodesy_test.go`
checks against **figures with an authority outside this repository**: the WGS 84
meridian quadrant (10,001,965.729 m), one degree of latitude at the equator, an
easting of exactly 500000 on a central meridian, and a northing of exactly zero
on the equator. A round trip proves the inverse undoes the forward and says
nothing about whether either is right, so the round trips come after the anchors
and measure error rather than asserting correctness. Inside a standard zone that
error is under a millimeter; in the two hand-widened zones (south-west Norway,
Svalbard) it reaches 4 cm, still twenty times finer than the finest square.

Two things are load-bearing and easy to undo. The **zone exceptions** are real
and getting either wrong puts a point in the neighboring zone, hundreds of
kilometers of easting away. And `unprojectUTM` **guards its footpoint latitude**:
decoding a 100 km row letter means trying candidate northings 2,000,000 m apart,
and a candidate past the pole does not fail loudly, it returns a number. One came
back as latitude 55.4 with a longitude of 3883 degrees, plausible enough on the
latitude alone to be mistaken for the answer by a caller checking only that.

**A grid token is validated geometrically, not textually.** `parseMGRS` builds
the `Grid` and then asks `mgrsCenter` for a position; that one call subsumes the
column letter sets, the row stagger, the band and the zone. Nothing re-checks
those separately, because a second implementation of the letter-set rule is a
second chance to disagree with the one that writes references out.

**Grid-to-grid conversion never goes through latitude and longitude.** `gridPoint`
stays in grid coordinates, because MGRS and UTM are the same easting and northing
relabeled. Routing it through the projection twice landed a meter out:
`33U 291000 5628000` came back as `33U TS 90999 28000`, because an easting
sitting exactly on a cell boundary lost a fraction of a millimeter and then had
it truncated away.

Also note the re-encode round trip is **not** exact at a zone boundary, and that
is a property of MGRS rather than a defect here: squares are defined inside a
zone and clipped where one ends, so a square against a boundary can have its
center in the next zone's grid. `TestGridSquaresDecodeIntoTheSquareTheyName`
asserts a distance everywhere; the exact equality is asserted only away from
boundaries, where it holds.

### The conversion endpoint

`GET /api/v1/convert?f=&v=&r=` returns every derived reading, already rendered.
Strings rather than numbers, because the rendering rules are the interesting part
and they live in Go; handing the webapp numbers would put two implementations of
"never claim more than the token carried" in one repository.

It calls `validateParams`, the same function the public page uses, rather than a
check written for it. A conversion accepting a link the page rejects would let
the sidebar render a coordinate the page refuses.

Passing `r` closes a real gap. Two of the four gates on the author's text need
the token grammar, which is Go-only, so the webapp can check length and alphabet
and nothing more, and the alphabet had to widen to the whole Latin alphabet when
the grid grammars arrived. Before the conversion carried `r`, a hand-edited link
could put prose in the panel's "As written" row beside a position derived from an
unrelated token, with a copy button next to it, while the page refused the
identical link. The panel now asks the same question the page asks and renders
**Not a coordinate** when the answer is no.

That verdict is kept distinct from a request that simply did not arrive.
`rejected` (HTTP 400) refuses to render; `failed` degrades, because nothing may
fail the panel: every row the token yields locally is already on screen and stays
there. Rows waiting on the server read `converting…` and then `unavailable`,
never a zero, since `0.0000° N, 0.0000° E` from a failed conversion is a
position, and a wrong one. Copy buttons are absent over a placeholder.

### Copying

Every row carrying a value has a copy icon at its right edge, on the panel and
on the page alike; the prose rows (resolution, datum, confidence) do not, since
copying "about 11 m" gets you a sentence rather than a position. That is eleven
controls, which is why they are icons: a column of "Copy MGRS", "Copy lat/lon",
"Copy DMS" would be wider than the coordinates it sat beside.

The page's script is the reason that page now has one at all, and it is written
so that **no coordinate is ever interpolated into it**: a delegated listener
reads the value out of the row's own cell. That keeps the script a constant the
policy can pin by digest, and keeps the number of escaping contexts at one.

On both surfaces the controls are **hidden until something proves the clipboard
exists**: the page's stylesheet hides them and its script reveals them, and the
panel's `clipboardAvailable()` returns null. A plain-HTTP origin has no
`navigator.clipboard`, which for on-prem and air-gapped installs is the norm, so
a control that cannot work is never drawn. The values stay selectable regardless.

### Prior art

`mattermost-plugin-aocanywhere` parses the same USMTF shapes in
`server/model/usmtf2004/sets/location.go`, and its test vectors are reused here
as positive cases so the two plugins agree about what a LATM is. Four bugs in it
are deliberately not inherited, each with a test here: no range validation at all
(`9999N99999W` parses to latitude 99.98), truncation instead of rounding, a
truthiness check that drops the equator and the prime meridian, and lon-first
axis order in one corner of the repo and lat-first everywhere else.


