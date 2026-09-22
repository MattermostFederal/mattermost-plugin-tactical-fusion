# Frequencies

> Design rationale for Tactical Fusion. See [CLAUDE.md](../../CLAUDE.md) for the rules that govern day-to-day work; this file records the measurements, the defects that produced the current shape, and the contracts a later change would silently break.

## The grammar is label-only, permanently

`121.5` is any decimal. The only thing that makes a frequency recognizable is
the author saying so, which is what `FREQ:` is. The label is consumed the way
the airfield labels are, so the link's text is the author's token, unit
included: `118.300 MHZ` links as `118.300 MHZ`, because the unit is what they
wrote.

A decimal without a unit is megahertz and a whole number is kilohertz. That is
how the traffic writes them: `121.5` and `243.0` are megahertz, `8992` and
`2182` are kilohertz. A unit overrides the default, and a token whose unit puts
it outside 2 MHz to 1,300 MHz is refused whole (`121.5 KHZ`, `8992 MHZ`). The
range covers every band the panel names and nothing that is only a number.

**The unit group consumes any `?HZ`, not only `MHZ` and `KHZ`.** With the unit
group written as `(?:MHZ|KHZ)?`, `FREQ:121.5 GHZ` matched `121.5`, refused
nothing, and linked it with ` GHZ` left outside the link to contradict it. The
group is `[A-Z]HZ` and `ParseToken` refuses a unit it does not read, so the
whole token declines. The cost is nothing: a word after a frequency that is not
a unit (`FREQ:121.5 NOW`) does not end in `HZ` and still links.

**The decimal form is four digits wide.** The plan wrote `\d{1,3}\.\d{1,3}`,
which capped the megahertz form at 999.999 and refused `1090.0` and `1300.0`,
both inside the range the same plan states. The width is `\d{1,4}` and the range
check is what bounds it. A four-digit fraction is refused whole rather than
truncated, because `121.5678` is finer than any channel plan and a link to
`121.567` would be a claim the author did not make.

## Derived locally, no API route

The page and the panel derive everything from `v`, the DTG pattern: there is no
dataset, no map and nothing a server has that the browser does not. The band
table and the allocation table therefore exist twice, in `bands.go` and
`bands.ts`, and `TestWebappFrequencyBandsMatch` parses the TypeScript literal
and holds it to the Go slice, order included. `TestWebappFrequencyTokenShapeMatches`
holds the token shape and the range constants the same way.

The bands are the ITU Radio Regulations' aeronautical allocations as the FAA
Aeronautical Information Manual summarizes them: HF aeronautical 2 to 30 MHz,
VHF navigation 108 to 117.975, VHF air band 118 to 136.975, VHF marine 156 to
162.025, UHF military air band 225 to 400, and the 406 MHz distress beacon
band. The channel note inside the VHF air band says 25 kHz for a multiple of
25 and 8.33 kHz otherwise, which is ICAO Annex 10's channel plan read the simple
way; a frequency like 118.305 is an 8.33 kHz channel designator rather than a
carrier, and the note says "channel" for that reason. The allocations are the
ones every pilot knows: 121.5 and 243.0 guard, 123.1 SAR on-scene, 122.75 and
123.45 air-to-air, marine channel 16, and 406.0 for beacons.

A frequency inside no band still links and is named as outside the bands this
plugin names, so the reader still gets the megahertz and kilohertz readings and
a copy button. Refusing it would make the decorator disagree with the author
about whether they wrote a frequency.

The channel note is the same duplicate: `Channel25`, `Channel833` and the VHF
air-band edges exist as exported constants on both sides and
`TestWebappFrequencyBandsMatch` holds them, because a note reworded on one
side alone made the page and the panel disagree about the same frequency with
every test green.

## Switch

`EnableFrequency`, alone in a **Frequencies** section, default on. Like the
airfield switch there is no separate label switch, because the grammar is
label-only by construction and a second switch would turn the same thing off
under another name.
