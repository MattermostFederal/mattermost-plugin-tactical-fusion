package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// checkResponse tells an author what would happen to some text, and why.
//
// This exists because the largest usability gap in decoration is invisible by
// construction. "34.0561, -118.2500" is linked and "34.0561, -118.25" is not,
// and nothing about the non-event points the author at the rule that decided
// it. Documentation alone carried the date-time group decorator because that
// one has three memorable shapes; coordinates have several grammars and the
// most-pasted form on earth is declined by a digit-count rule.
//
// Ephemeral, and it decorates the supplied text directly rather than posting
// it, so asking the question never puts anything in the channel.
func (p *Plugin) checkResponse(text string) *model.CommandResponse {
	if p.decorators == nil {
		return ephemeralResponse(errcode.WithCode(errcode.CommandCheckNotReady,
			"Decorators are not registered yet. Try again once the plugin has finished activating."))
	}

	if text == "" {
		return ephemeralResponse("Usage: `/" + commandTrigger + " check <text>`\n\n" +
			"Paste a message and this will show what would be turned into a link, and what would be left alone.")
	}

	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}
	decorated := tagger.Decorate(text, referenceTime(nil))

	var b strings.Builder
	b.WriteString("**You typed**\n" + inlineCode(text) + "\n\n")

	if decorated == text {
		b.WriteString("**Nothing would be decorated.**\n\n")
		b.WriteString(whyNothingMatched())
		return ephemeralResponse(b.String())
	}

	b.WriteString("**Would be stored as**\n" + decorated + "\n\n")
	b.WriteString("Anything above that is still plain text was left alone. " +
		whyNothingMatched())

	return ephemeralResponse(b.String())
}

// whyNothingMatched is the short version of the declined rules.
//
// Deliberately the rules rather than a list of formats: a reader running this
// has already tried something and wants to know which rule caught it. The full
// list, with reasons, is in the documentation.
func whyNothingMatched() string {
	return "The most common reasons a coordinate is left alone:\n" +
		"- **Not enough decimal places.** `34.0561, -118.2500` is linked; `34.05, -118.25` is not. " +
		"Signed decimal degrees needs at least four decimals on both values, which is what separates " +
		"a coordinate from an ordinary pair of numbers.\n" +
		"- **No hemisphere letter and no decimal point.** `12 N, 5 W` is newtons and watts, not a position.\n" +
		"- **Next to another number.** A coordinate inside a longer numeric run, a range or a " +
		"comma-separated list is left alone.\n" +
		"- **Inside code, a link or a URL.** Those spans are never rewritten.\n" +
		"- **A grid reference in lower case, or coarser than 100 m, written without spaces.** " +
		"`18SUJ2347806483` is linked; `18suj2347806483` and `18SUJ2306` are not, because a " +
		"lower-case run of that shape is what a short git commit hash looks like, and a " +
		"shorter one is what a part number looks like. Space them out, or put `MGRS:` in " +
		"front.\n" +
		"- **UTM is off by default.** It is the only format that is, because its band letter is " +
		"ambiguous: `11S` is band S here and \"zone 11, southern hemisphere\" to a civilian, " +
		"90 degrees of latitude apart. An admin can turn it on in the System Console. Every " +
		"decorated coordinate still shows a UTM row either way.\n" +
		"- **A UTM northing that is not in the band the token names.** With UTM on, the letter " +
		"after the zone is a latitude band, so `11S 385000 3769000` works while " +
		"`11N 385000 3769000` does not: that northing is 34 north, which is band S.\n" +
		"- **A grid square that does not exist.** Only eight of the twenty-four 100 km letters are " +
		"legal in any given zone.\n" +
		"- **A format that is switched off**, or one this version does not recognize yet, such as " +
		"GARS, GEOREF and Plus Codes.\n\n" +
		"See the [documentation](" + docsPath() + ") for every recognized format and the full declined list."
}

// docsPath is the built-in documentation, which Mattermost serves out of the
// bundle's public directory.
//
// A function rather than a package-level var: var initialisers run before the
// generated init() populates the manifest, so a var would read an empty id and
// produce a link to nowhere.
func docsPath() string {
	return "/plugins/" + manifest.Id + "/public/help/help.html"
}
