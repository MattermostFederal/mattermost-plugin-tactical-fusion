package location

import (
	"html"
	"strings"
)

// pageStyles are the rules only this page needs.
//
// A source constant, never anything derived from a request. The shared shell
// already provides the table, the muted text and the theme variables; these add
// the label/value pairs a coordinate reads as.
//
// The copy buttons are hidden here and revealed by the script, which is what
// makes them honest: a reader with no JavaScript, or on a plain-HTTP origin
// where navigator.clipboard does not exist, never sees a control that cannot
// work. Every value stays selectable either way.
const pageStyles = `
.loc-rows { width: 100%; border-collapse: collapse; margin-top: 20px; font-size: 14px; }
.loc-rows th { width: 34%; white-space: nowrap; vertical-align: top; }
.loc-rows td { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; word-break: break-all; }
.loc-note { color: var(--muted); font-size: 13px; margin-top: 20px; }
.loc-copy { width: 28px; padding-left: 8px; text-align: right; vertical-align: top; }
.loc-copy button { display: none; }
.loc-can-copy .loc-copy button {
  display: inline-flex; align-items: center; justify-content: center;
  width: 24px; height: 24px; padding: 0; border: 0; border-radius: 4px;
  background: transparent; color: inherit; opacity: 0.5; cursor: pointer;
}
.loc-can-copy .loc-copy button:hover,
.loc-can-copy .loc-copy button:focus-visible { opacity: 1; background: var(--hover); }
.loc-copy button svg { pointer-events: none; }
.loc-copy button .tick { display: none; }
.loc-copy button[data-copied] .tick { display: block; }
.loc-copy button[data-copied] .mark { display: none; }
.loc-copy button[data-copied] { opacity: 1; }
.loc-status { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
`

// copyIcon is the button's content: the copy mark, and the tick that replaces
// it once a value has been copied. CSS swaps them, so the script sets one
// attribute and touches no markup.
const copyIcon = `<svg class="mark" width="14" height="14" viewBox="0 0 24 24" fill="none" ` +
	`stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" ` +
	`aria-hidden="true"><rect x="9" y="9" width="11" height="11" rx="2"/>` +
	`<path d="M5 15V5a2 2 0 0 1 2-2h10"/></svg>` +
	`<svg class="tick" width="14" height="14" viewBox="0 0 24 24" fill="none" ` +
	`stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" ` +
	`aria-hidden="true"><path d="M20 6 9 17l-5-5"/></svg>`

// copyScript wires the copy buttons.
//
// A source constant with nothing interpolated into it, which is what lets the
// shell pin the content-security policy to its digest instead of opening the
// page to 'unsafe-inline'. That matters here more than on most pages: this one
// echoes the author's own text from a message, on a public route whose query
// string anybody can write, so the property worth keeping is that an escaping
// mistake stays inert markup. Under 'unsafe-inline' an injected <script> would
// run; against a hash it does not match and is blocked.
//
// It reads the value out of the row rather than being handed one, so no
// coordinate is ever written into the script and there is nothing here to
// escape twice.
const copyScript = `
(function () {
  var table = document.querySelector('.loc-rows');
  if (!table || !navigator.clipboard) { return; }

  table.classList.add('loc-can-copy');

  var status = document.querySelector('.loc-status');
  var timer = null;

  table.addEventListener('click', function (event) {
    var button = event.target.closest('button[data-copy]');
    if (!button) { return; }

    var cell = button.closest('tr').querySelector('td');
    if (!cell) { return; }

    navigator.clipboard.writeText(cell.textContent).then(function () {
      button.setAttribute('data-copied', '');
      if (status) { status.textContent = button.getAttribute('aria-label') + ': copied'; }

      clearTimeout(timer);
      timer = setTimeout(function () {
        button.removeAttribute('data-copied');
        if (status) { status.textContent = ''; }
      }, 1500);
    }, function () {
      /* The browser refusing is not something the reader can act on. */
    });
  });
})();
`

// renderBody builds the location page.
//
// Every value is computed from the validated payload, and everything
// interpolated is escaped. The page's one script is a constant, pinned by its
// digest in the policy, so an escaping mistake here still cannot become
// execution.
func renderBody(page pageData) string {
	var b strings.Builder

	loc := page.loc

	// No lead line, matching the panel. The table already opens with the grid
	// reference, labeled and with a copy button beside it, so a bare repeat of
	// the same string above it was saying nothing twice.
	//
	// Rendered from the shared Rows table rather than a run of calls, so the
	// page and the panel cannot come to disagree about which rows exist. An
	// empty value is a row that does not apply and is left out: a blank row
	// would read as a failure, where its absence reads as what it is.
	//
	// A reader's hidden rows are NOT honored here, and cannot be: this page is
	// served without a session, so there is no reader to ask. The panel and the
	// page can therefore show the same coordinate differently, which is
	// inherent to a public route rather than an oversight.
	b.WriteString(`<table class="loc-rows"><tbody>`)

	for _, row := range Rows {
		if value := row.Value(loc, page.raw); value != "" {
			writeRow(&b, row.Label, value, row.Copyable)
		}
	}

	b.WriteString(`</tbody></table>`)

	// The live region the script announces a copy through, so the
	// acknowledgement is not carried by the icon swap alone.
	b.WriteString(`<span class="loc-status" role="status" aria-live="polite"></span>`)

	b.WriteString(`<p class="loc-note">Positions are WGS 84. Values are shown at the resolution the ` +
		`original text carried and no finer. A grid reference names a square, and the position ` +
		`shown for one is its center.</p>`)

	return b.String()
}

func writeRow(b *strings.Builder, label, value string, withCopy bool) {
	b.WriteString(`<tr><th>` + html.EscapeString(label) + `</th><td>` +
		html.EscapeString(value) + `</td><td class="loc-copy">`)

	if withCopy {
		// data-copy is what the delegated listener looks for, and the label is
		// the button's whole accessible name, since its content is an icon.
		b.WriteString(`<button type="button" data-copy aria-label="Copy ` +
			html.EscapeString(label) + `" title="Copy ` + html.EscapeString(label) + `">` +
			copyIcon + `</button>`)
	}

	b.WriteString(`</td></tr>`)
}
