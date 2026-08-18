package dtg

import (
	"fmt"
	"html"
	"strings"
	"time"
)

// renderBody builds the DTG page body.
//
// Everything is computed server-side so the page is complete without
// JavaScript. The only script is the live Zulu clock, which is pure
// enhancement: with scripting off the table still renders in full.
func renderBody(page pageData) string {
	var b strings.Builder

	instant := page.instant

	// Loudest first: how long away it is, then the plain-language reading, then
	// the token as the author wrote it. The sidebar panel shows the same three
	// in the same order, so the two renderings cannot tell different stories.
	now := time.Now().UTC()

	class := "countdown"
	if isUrgent(now, instant) {
		class += " urgent"
	}

	fmt.Fprintf(&b, `<p class="%s" id="rel" data-instant="%d">%s</p>`,
		class, instant.UnixMilli(), html.EscapeString(relativeTo(now, instant)))
	b.WriteString(`<p class="described">` +
		html.EscapeString(describeInstant(instant, page.offsetMinutes, page.zoneLabel)) + `</p>`)
	b.WriteString(`<p class="dtg">` + html.EscapeString(page.canonical) + `</p>`)

	if note := assumedNote(page.assumed); note != "" {
		b.WriteString(`<p class="note">` + html.EscapeString(note) + `</p>`)
	}

	b.WriteString(`<table><thead><tr><th>Location</th><th style="text-align:right">Time</th><th>Date</th></tr></thead><tbody>`)
	for _, z := range OrderedZones(DisplayZones, instant) {
		b.WriteString(renderRow(instant, z, zoneReferenceDate(instant, page.offsetMinutes)))
	}
	b.WriteString(`</tbody></table>`)

	return b.String()
}

// renderRow renders one timezone row, with a day-offset badge relative to the
// DTG's own zone date rather than to UTC. A +1 measured against UTC reads as
// wrong for a DTG that was never expressed in UTC to begin with.
func renderRow(instant time.Time, z DisplayZone, reference time.Time) string {
	loc, err := time.LoadLocation(z.IANA)
	if err != nil {
		// Only reachable if the embedded tzdata is missing, which the blank
		// import in server/main.go exists to prevent. Degrade to a visible gap
		// rather than dropping the row silently.
		return fmt.Sprintf(
			`<tr><td>%s<span class="abbr">%s</span></td><td class="time">n/a</td><td>n/a</td></tr>`,
			html.EscapeString(z.Name), html.EscapeString(z.Abbr),
		)
	}

	local := instant.In(loc)
	badge := ""
	if delta := dayDelta(reference, local); delta != 0 {
		badge = fmt.Sprintf(`<span class="badge">%+d</span>`, delta)
	}

	return fmt.Sprintf(
		`<tr><td>%s<span class="abbr">%s</span></td><td class="time">%s</td><td>%s%s</td></tr>`,
		html.EscapeString(z.Name),
		html.EscapeString(z.Abbr),
		local.Format(clockLayout(instant)),
		html.EscapeString(local.Format("Mon 2 Jan")),
		badge,
	)
}

// clockLayout is how a wall clock is written for an instant: to the minute, and
// to the second when the token carried one.
//
// A date-time group has no seconds field, so this is always "15:04" for one. An
// RFC 3339 timestamp does, and "2026-08-09T16:30:45Z" rendered as "16:30" would
// drop 45 seconds the author wrote, which is the same defect the location
// package forbids under "render at the resolution the token carried and no
// finer". A timestamp written without seconds, or with ":00", carries none to
// lose and stays narrow.
//
// Every zone offset is a whole number of minutes, so the seconds field is the
// same in every row and this can be decided once from the instant rather than
// per zone.
//
// Must match clockText in webapp/src/decorators/dtg/describe.ts and the seconds
// decision in DtgPanel, or the sidebar and this page would render the same
// instant to different precisions.
func clockLayout(instant time.Time) string {
	if instant.Second() != 0 {
		return "15:04:05"
	}
	return "15:04"
}

// zoneReferenceDate is the instant expressed in the token's own zone, which is
// the baseline every row's day offset is measured against.
func zoneReferenceDate(instant time.Time, offsetMinutes int) time.Time {
	return instant.Add(time.Duration(offsetMinutes) * time.Minute)
}

// dayDelta counts whole calendar days between two wall-clock dates, ignoring
// their clock times and zones.
func dayDelta(reference, other time.Time) int {
	ry, rm, rd := reference.Date()
	oy, om, od := other.Date()
	refDay := time.Date(ry, rm, rd, 0, 0, 0, 0, time.UTC)
	otherDay := time.Date(oy, om, od, 0, 0, 0, 0, time.UTC)
	return int(otherDay.Sub(refDay).Hours() / 24)
}

// describeInstant renders the plain-language reading, in the zone the token was
// written in: "09 Aug 2026 16:30 Z", or "09 Aug 2026 20:30 +04:00".
//
// Seconds appear when the token carried them: "09 Aug 2026 16:30:45 Z". See
// clockLayout.
func describeInstant(instant time.Time, offsetMinutes int, zoneLabel string) string {
	local := zoneReferenceDate(instant, offsetMinutes)
	return fmt.Sprintf("%s %s", local.Format("02 Jan 2006 "+clockLayout(instant)), zoneLabel)
}

func assumedNote(assumed string) string {
	switch assumed {
	case "my":
		return "Month and year were not in the original text; both were taken from the date the message was posted."
	case "y":
		return "The year was not in the original text; it was taken from the date the message was posted."
	default:
		return ""
	}
}

// urgentWithin is how close to the instant counts as urgent, in either
// direction.
//
// Must match DEFAULT_URGENT_WITHIN_MS in webapp/src/decorators/dtg/relative.ts
// and the threshold in countdownScript below, or the sidebar and this page
// would disagree about whether the same DTG is imminent.
//
// This page does not ask its reader: the renderer is handed a query string and
// no user, so it always uses the default even when the reader has chosen a
// different threshold in the sidebar. See the reader preferences section of
// CLAUDE.md.
const urgentWithin = 30 * time.Minute

// isUrgent reports whether an instant is close enough to call for attention.
func isUrgent(now, target time.Time) bool {
	return target.Sub(now).Abs() <= urgentWithin
}

// relativeTo renders a counting offset between two instants.
//
// Seconds are always shown so the value visibly ticks. Once a larger unit
// appears every smaller one is shown with it, including zeroes, so the display
// counts down like a clock rather than jumping between widths as units drop
// out. "in 1h 0m 0s" is deliberate.
//
// This is the no-JavaScript rendering. The page's own script recomputes the
// same string every second, and DtgPanel does the same in the sidebar, so all
// three formatters must agree.
func relativeTo(now, target time.Time) string {
	diff := target.Sub(now)
	if diff.Abs() < time.Second {
		return "now"
	}

	total := int(diff.Abs().Seconds())
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if days > 0 || hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if days > 0 || hours > 0 || minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	parts = append(parts, fmt.Sprintf("%ds", seconds))

	joined := strings.Join(parts, " ")
	if diff < 0 {
		return joined + " ago"
	}
	return "in " + joined
}

// countdownScript makes the offset count down, or up, once a second.
//
// It must produce the same strings as relativeTo, which renders the first frame
// server-side so the value is correct with scripting disabled. No external
// assets.
const countdownScript = `
(function () {
  var rel = document.getElementById('rel');
  if (!rel) { return; }

  var instant = Number(rel.getAttribute('data-instant'));
  if (isNaN(instant)) { return; }

  function relative(diffMs) {
    var total = Math.floor(Math.abs(diffMs) / 1000);
    if (total === 0) { return 'now'; }

    var days = Math.floor(total / 86400);
    var hours = Math.floor((total % 86400) / 3600);
    var minutes = Math.floor((total % 3600) / 60);
    var seconds = total % 60;

    var parts = [];
    if (days > 0) { parts.push(days + 'd'); }
    if (days > 0 || hours > 0) { parts.push(hours + 'h'); }
    if (days > 0 || hours > 0 || minutes > 0) { parts.push(minutes + 'm'); }
    parts.push(seconds + 's');

    var joined = parts.join(' ');
    return diffMs < 0 ? joined + ' ago' : 'in ' + joined;
  }

  // Must match urgentWithin in page.go.
  var urgentWithin = 30 * 60 * 1000;

  function tick() {
    var diff = instant - Date.now();
    rel.textContent = relative(diff);
    rel.className = Math.abs(diff) <= urgentWithin ? 'countdown urgent' : 'countdown';
  }

  tick();
  setInterval(tick, 1000);
})();
`
