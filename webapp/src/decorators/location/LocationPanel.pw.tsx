import React from 'react';

import LocationPanelHarness from './LocationPanelHarness';
import {MAP_ID, ROWS} from './rows';

import {expect, test} from '../../../playwright/ct-coverage';

/*
 * Built from the catalog rather than written out. The literal it replaced had
 * gone stale twice over: it was missing the Region row and the map, so a test
 * named "every row being hidden" was hiding eleven of thirteen things and still
 * passing.
 */
const EVERYTHING_HIDEABLE = [MAP_ID, ...ROWS.map((row) => row.id)];

/*
 * The panel is split by where a value can be worked out, not by format, and
 * these tests are that split made visible.
 *
 * Anything sliced out of the canonical token is on screen before the request is
 * answered and stays there whatever the request does. Anything needing the
 * ellipsoid comes from the server, because the projection is in Go and a second
 * one here would be two implementations of the same geodesy.
 */

test('a lat/lon link is readable before the conversion answers', async ({mount}) => {
    const component = await mount(<LocationPanelHarness/>);

    // Everything the token yields by dividing, immediately.
    await expect(component.getByText('34.0561° N, 118.2500° W').first()).toBeVisible();
    await expect(component.getByText('about 11 m')).toBeVisible();

    // And the grid rows saying which of the two reasons they are not here yet.
    await expect(component.getByText('converting…').first()).toBeVisible();

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    await expect(component.getByText('11S LT 8463 6908').first()).toBeVisible();
    await expect(component.getByText('11S 384640E 3769080N')).toBeVisible();
    await expect(component.getByText('converting…')).toHaveCount(0);
});

// A grid link is the mirror image: the reference and its resolution are in the
// token, the position is not.
test('a grid link shows its own reference before the conversion answers', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            format='mgrs'
            canonical='11SLT84636908'
        />);

    await expect(component.getByText('11S LT 8463 6908').first()).toBeVisible();
    await expect(component.getByText('10 m grid, at center')).toBeVisible();
    await expect(component.getByText('converting…').first()).toBeVisible();

    await component.getByRole('button', {name: 'answer the conversion'}).click();
    await expect(component.getByText('34.0561° N, 118.2500° W').first()).toBeVisible();
});

test('an area code shows its own code and resolution before the conversion answers', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            format='georef'
            canonical='GJNJ5753'
        />);

    await expect(component.getByText('GJNJ5753').first()).toBeVisible();
    await expect(component.getByText('1.9 km cell, at center')).toBeVisible();
    await expect(component.getByText('converting…').first()).toBeVisible();
});

// Nothing may fail the panel. A request that never arrives costs the rows it
// would have filled and nothing else, and it must not leave a zero behind: a
// row reading "0.0000 N, 0.0000 E" because a conversion failed is a position,
// and a wrong one.
test('a failed request degrades rather than blanking the panel', async ({mount}) => {
    const component = await mount(<LocationPanelHarness outcome='fail'/>);

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    await expect(component.getByText('unavailable').first()).toBeVisible();
    await expect(component.getByText('34.0561° N, 118.2500° W').first()).toBeVisible();
    await expect(component.getByText('about 11 m')).toBeVisible();
    await expect(component.getByText('0.0000')).toHaveCount(0);
});

// A placeholder is not a coordinate, and a copy button over one would put
// "converting…" on the clipboard.
test('offers no copy button for a value that has not arrived', async ({mount}) => {
    const component = await mount(<LocationPanelHarness/>);

    await expect(component.getByRole('button', {name: 'Copy MGRS'})).toHaveCount(0);
    await expect(component.getByRole('button', {name: 'Copy Lat / lon'})).toBeVisible();

    await component.getByRole('button', {name: 'answer the conversion'}).click();
    await expect(component.getByRole('button', {name: 'Copy MGRS'})).toBeVisible();
});

// A verdict, not an outage.
//
// Two of the four checks on the author's own text need the token grammar, which
// lives in Go, so before the conversion carried "r" a hand-written link could
// show prose in the "Original text" row beside a position derived from a different
// token, with a copy button next to it, while the server-rendered page refused
// the identical link. The panel now asks the same question the page asks.
test('refuses a link the server says it did not issue', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            raw='34.0561,-118.2500 ALFA'
            outcome='reject'
        />);

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    await expect(component.getByText('Not a coordinate')).toBeVisible();
    await expect(component.getByText('34.0561,-118.2500 ALFA')).toHaveCount(0);
    await expect(component.getByRole('button', {name: 'Copy Lat / lon'})).toHaveCount(0);
});

// The panel stays mounted across a change of selection, so a second coordinate
// arrives as a prop change rather than a remount.
//
// Before the state was reset during render, this committed one frame in which
// the heading and the token were the new coordinate's while every server-derived
// row was still the old one's: a grid square in Maryland over a latitude in
// California. It painted, so a reader saw it, and the copy button was armed on
// the wrong value while it did.
test('never shows one coordinate\'s position under another\'s token', async ({mount}) => {
    const component = await mount(<LocationPanelHarness/>);

    await component.getByRole('button', {name: 'answer the conversion'}).click();
    await expect(component.getByText('11S LT 8463 6908').first()).toBeVisible();

    await component.getByRole('button', {name: 'select the second coordinate'}).click();

    await expect(component.getByText('18S UJ 23478 06483').first()).toBeVisible();

    // The assertion that matters, and it has to be over every COMMITTED state
    // rather than the settled one. The bug lasted a single frame, and
    // Playwright's assertions retry until they succeed, so a settled-state
    // check passed whether or not the bug was there. The harness records each
    // commit through a MutationObserver; not one of them may name both
    // coordinates.
    await expect(component.getByTestId('mixed-frames')).toHaveText('0');

    // And the new coordinate still resolves normally afterwards.
    await component.getByRole('button', {name: 'answer the conversion'}).click();
    await expect(component.getByText('38.8895° N, 77.0353° W').first()).toBeVisible();
    await expect(component.getByTestId('mixed-frames')).toHaveText('0');
});

// `r` is the author's own text, and two of the four checks on it need the token
// grammar, which lives in Go. The alphabet this side can check had to widen to
// the whole Latin alphabet when the grid grammars arrived, so a hand-written
// link can put a short run of words in `r` that this side cannot tell from a
// coordinate. It must not appear in a row labeled as the author's words until
// the server has confirmed it.
test('does not show the author text until the server vouches for it', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            format='mgrs'
            canonical='18SUJ2347806483'
            raw='DISREGARD USE 18SUJ11111111'
        />);

    // While the request is in flight, the row falls back to the token.
    await expect(component.getByText(/DISREGARD/)).toHaveCount(0);
    await expect(component.getByText(/18SUJ2347806483/).first()).toBeVisible();

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    // The stub answers 200, so this particular string is now vouched for and
    // does appear. What matters is that it waited for the verdict: a server
    // that refused would have taken the panel to "Not a coordinate" instead.
    await expect(component.getByText(/DISREGARD/).first()).toBeVisible();
});

// A failed request is not a verdict, so the row stays on the token rather than
// showing text the server never confirmed.
test('keeps the author text hidden when the request never lands', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            format='mgrs'
            canonical='18SUJ2347806483'
            raw='DISREGARD USE 18SUJ11111111'
            outcome='fail'
        />);

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    await expect(component.getByText('unavailable').first()).toBeVisible();
    await expect(component.getByText(/DISREGARD/)).toHaveCount(0);
});

// A UTM link already is its own UTM row, so that row is rendered locally and
// never waits on the server or degrades with it. Everything the token cannot
// answer for itself still degrades, which is what the "unavailable" beside it
// is checking: the local row must not be coming from a request that failed.
test('a UTM link keeps its own reference when the request fails', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            format='utm'
            canonical='33U2910005628000'
            outcome='fail'
        />);

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    await expect(component.getByText('33U 291000E 5628000N').first()).toBeVisible();
    await expect(component.getByText('unavailable').first()).toBeVisible();
});

// What the panel actually sends. The stub used to ignore its arguments, so
// nothing checked that `r` travels at all, that it is omitted when it would only
// repeat the token, or that the CSRF header is there. Dropping any of those
// silently breaks every conversion in production.
test('sends the format, the token and the author text, with the CSRF header', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            format='mgrs'
            canonical='18SUJ2347806483'
            raw='18S UJ 23478 06483'
        />);

    await expect(component.getByText('18S UJ 23478 06483').first()).toBeVisible();
    await component.getByRole('button', {name: 'show the request'}).click();

    const sent = JSON.parse(await component.getByTestId('last-request').textContent() ?? '{}');

    const url = new URL(sent.url, 'http://localhost');
    expect(url.pathname).toContain('/api/v1/convert');
    expect(url.searchParams.get('f')).toBe('mgrs');
    expect(url.searchParams.get('v')).toBe('18SUJ2347806483');
    expect(url.searchParams.get('r')).toBe('18S UJ 23478 06483');

    expect(sent.headers['X-Requested-With']).toBe('XMLHttpRequest');
    expect(sent.credentials).toBe('same-origin');
});

// `r` is omitted when it would only repeat the token, exactly as the server
// omits it from a link it writes.
test('omits the author text when it only repeats the token', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            format='mgrs'
            canonical='18SUJ2347806483'
        />);

    await expect(component.getByText('18S UJ 23478 06483').first()).toBeVisible();
    await component.getByRole('button', {name: 'show the request'}).click();

    const sent = JSON.parse(await component.getByTestId('last-request').textContent() ?? '{}');
    expect(new URL(sent.url, 'http://localhost').searchParams.has('r')).toBe(false);
});

// A 500 is an outage, not a verdict, so it must degrade rather than accuse the
// link of not being a coordinate.
test('a server error degrades rather than refusing the link', async ({mount}) => {
    const component = await mount(<LocationPanelHarness outcome='fail'/>);

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    await expect(component.getByText('unavailable').first()).toBeVisible();
    await expect(component.getByText('Not a coordinate')).toHaveCount(0);
});

// One copy control per value row, beside the value, rather than a row of
// labeled buttons underneath.
test('offers a copy control on every value row and none on the prose rows', async ({mount}) => {
    const component = await mount(<LocationPanelHarness/>);
    await component.getByRole('button', {name: 'answer the conversion'}).click();

    // Read off the catalog rather than listed by hand, for the reason the
    // hide-everything test below is: the hand-written list went stale the moment
    // rows were added, still named seven of them, still passed, and had quietly
    // stopped meaning "every value row".
    //
    // Normalized is the one value row absent here, and correctly: this fixture's
    // author text already is the canonical form, so that row does not apply.
    const copyable = ROWS.filter((row) => row.copyable && row.id !== 'canonical');

    await Promise.all(copyable.map((row) =>
        expect(component.getByRole('button', {name: `Copy ${row.label}`}), row.id).toBeVisible()));

    // Prose, not a value: copying "about 11 m" or "WGS 84" gets you a sentence.
    await Promise.all(ROWS.filter((row) => !row.copyable).map((row) =>
        expect(component.getByRole('button', {name: `Copy ${row.label}`}), row.id).toHaveCount(0)));
});

// The control sits inside the row it copies, which is the whole point of moving
// it off the bottom of the panel.
test('puts each copy control in its own row', async ({mount}) => {
    const component = await mount(<LocationPanelHarness/>);
    await component.getByRole('button', {name: 'answer the conversion'}).click();

    const mgrsRow = component.locator('tr', {hasText: '11S LT 8463 6908'});
    await expect(mgrsRow.getByRole('button', {name: 'Copy MGRS'})).toBeVisible();
    await expect(mgrsRow.getByRole('button', {name: 'Copy DMS'})).toHaveCount(0);
});

// The panel says each reading once.
//
// There used to be a lead line above the table repeating the grid reference,
// three lines above the labeled row that already carried it with a copy button
// beside it. Counting rather than asserting presence, because most of the tests
// here reach for `.first()` and would not have noticed either way.
test('shows each reading once, with no lead line repeating the table', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            format='mgrs'
            canonical='18SUJ2347806483'
            raw='18S UJ 23478 06483'
        />);

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    // Twice, and both are labeled: the MGRS row, and the Original text row
    // holding the author's own spelling, which happens to be identical here.
    await expect(component.getByText('18S UJ 23478 06483')).toHaveCount(2);

    // One table, and the note still closes the panel. What sits above the table
    // is the map and nothing else: the lead line this guards against was a bare
    // repeat of the grid reference, which would make the count above 3.
    await expect(component.locator('table')).toHaveCount(1);
    await expect(component.getByText('Positions are WGS 84', {exact: false})).toBeVisible();
});

// The USMTF row is the only derived reading a person pastes into another
// system's fixed-width field, so it is worth having beside the punctuated DMS
// row rather than instead of it.
//
// Rendered here rather than fetched, like DMS and DDM, whenever the token
// carries axes; a grid link has none and gets it from the conversion endpoint
// with everything else.
test('renders USMTF locally for a token that carries axes', async ({mount}) => {
    const component = await mount(<LocationPanelHarness/>);

    // Before the request answers, which is what "locally" means.
    await expect(component.getByText('340322.0N1181500.0W')).toBeVisible();
    await expect(component.getByRole('button', {name: 'Copy USMTF'})).toBeVisible();
});

test('takes USMTF from the server for a grid link', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            format='mgrs'
            canonical='11SLT84636908'
        />);

    // A grid token has no axes to slice, so this row waits with the others.
    await expect(component.getByText('converting…').first()).toBeVisible();
    await expect(component.getByRole('button', {name: 'Copy USMTF'})).toHaveCount(0);

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    await expect(component.getByText('340322.0N1181500.0W')).toBeVisible();
    await expect(component.getByRole('button', {name: 'Copy USMTF'})).toBeVisible();
});

test.describe('when the admin has turned maps off', () => {
    // Every reading stays. The switch is about the picture and the bytes behind
    // it, not about what the plugin knows, so a panel with no map is the same
    // table it always was.
    test('drops the map and keeps every reading', async ({mount}) => {
        const component = await mount(<LocationPanelHarness features={{mapPanel: false}}/>);
        await component.getByRole('button', {name: 'answer the conversion'}).click();

        // Through the map's own note, which this harness always has: the
        // basemap archive is not served in a component test, so a mounted map
        // reports that it could not load. Its absence therefore means no map
        // was mounted at all rather than one that merely failed to paint.
        await expect(component.getByTestId('map-note')).toHaveCount(0);

        await expect(component.getByText('34.0561° N, 118.2500° W')).toBeVisible();
        await expect(component.getByRole('button', {name: 'Copy MGRS'})).toBeVisible();
    });

    // "Open larger" points at /map, which answers 404 when its own switch is
    // off, so the link has to go with it rather than becoming a dead end.
    test('offers no way to open a page that is not served', async ({mount}) => {
        const component = await mount(<LocationPanelHarness features={{mapPage: false}}/>);
        await component.getByRole('button', {name: 'answer the conversion'}).click();

        await expect(component.getByText('Open larger')).toHaveCount(0);
    });

    // The two switches are independent, so the panel map survives the page
    // being off. It is the caption under the frame that goes.
    test('keeps the panel map when only the page is off', async ({mount}) => {
        const component = await mount(<LocationPanelHarness features={{mapPage: false}}/>);
        await component.getByRole('button', {name: 'answer the conversion'}).click();

        await expect(component.getByTestId('map-note')).toBeAttached();
    });
});

test.describe('customizing the view', () => {
    // The rows a reader hid are gone, and the rest are untouched.
    test('leaves out the rows the reader hid', async ({mount}) => {
        const component = await mount(<LocationPanelHarness hidden={['ddm', 'datum']}/>);
        await component.getByRole('button', {name: 'answer the conversion'}).click();

        await expect(component.getByText('DDM', {exact: true})).toHaveCount(0);
        await expect(component.getByText('Datum', {exact: true})).toHaveCount(0);
        await expect(component.getByText('WGS 84', {exact: true})).toHaveCount(0);

        // And the ones they kept are still there, values and copy buttons.
        await expect(component.getByText('DMS', {exact: true})).toBeVisible();
        await expect(component.getByRole('button', {name: 'Copy DMS'})).toBeVisible();
    });

    // Hiding every row is allowed. It leaves the note and the links, which is
    // what makes it recoverable: the way back is the Customize link itself.
    test('survives every row being hidden', async ({mount}) => {
        const component = await mount(
            <LocationPanelHarness
                hidden={EVERYTHING_HIDEABLE}
            />);

        await component.getByRole('button', {name: 'answer the conversion'}).click();

        await Promise.all(ROWS.map((row) => expect(
            component.getByRole('button', {name: `Copy ${row.label}`}),
            row.id,
        ).toHaveCount(0)));

        await expect(component.getByRole('button', {name: 'Customize your view'})).toBeVisible();
    });

    // A stored id from a build with a row this one does not have must hide
    // nothing rather than taking the panel down or blanking a real row.
    test('ignores a hidden row this build does not have', async ({mount}) => {
        const component = await mount(<LocationPanelHarness hidden={['sextant']}/>);
        await component.getByRole('button', {name: 'answer the conversion'}).click();

        await expect(component.getByText('DDM', {exact: true})).toBeVisible();
        await expect(component.getByText('Datum', {exact: true})).toBeVisible();
    });

    // The editor takes the panel over rather than expanding below the table, so
    // the coordinate is not buried under a list of settings.
    test('opens the editor over the panel and comes back', async ({mount}) => {
        const component = await mount(<LocationPanelHarness/>);
        await component.getByRole('button', {name: 'answer the conversion'}).click();
        await expect(component.getByText('Lat / lon', {exact: true})).toBeVisible();

        await component.getByRole('button', {name: 'Customize your view'}).click();

        await expect(component.getByText('Rows to show')).toBeVisible();

        // The reading's copy button, not its label: the editor offers one
        // tickbox per row and names it with that same label, so a bare text
        // match finds the editor and reports the panel it replaced.
        await expect(component.getByRole('button', {name: 'Copy Lat / lon'})).toHaveCount(0);

        await component.getByRole('button', {name: '← Back'}).click();
        await expect(component.getByText('Lat / lon', {exact: true})).toBeVisible();
    });

    // Clicking a second coordinate while the editor is open must land on that
    // coordinate, not on the editor. React keeps the panel mounted across a
    // change of selection, so nothing else resets it.
    test('closes the editor when a different coordinate is selected', async ({mount}) => {
        const component = await mount(<LocationPanelHarness/>);
        await component.getByRole('button', {name: 'answer the conversion'}).click();
        await component.getByRole('button', {name: 'Customize your view'}).click();
        await expect(component.getByText('Rows to show')).toBeVisible();

        await component.getByRole('button', {name: 'select the second coordinate'}).click();

        await expect(component.getByText('Rows to show')).toHaveCount(0);
        await expect(component.getByText('18SUJ2347806483')).toBeVisible();
    });
});

/*
 * The country is still computed and still travels in the conversion, because
 * the map speaks it in its accessible label. What it no longer is, is a row.
 *
 * Asserted after the conversion lands rather than before, since before it the
 * row would be absent either way and the test would pass against a build that
 * still had one.
 */
test('names no Region row, even once the conversion lands', async ({mount}) => {
    const component = await mount(<LocationPanelHarness/>);

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    await expect(component.getByText('11S LT 8463 6908').first()).toBeVisible();
    await expect(component.getByText('Region')).toHaveCount(0);
    await expect(
        component.getByText('United States of America (Natural Earth 110m)'),
    ).toHaveCount(0);
});

/*
 * The map is hideable and is not a row, so its id travels through a separate
 * union. If the read path ever narrows back to isRowID, `asRowIDs` drops 'map'
 * silently: the reader unticks Map, saves, and it is back after a reload with
 * nothing logged on either side. This drives the id through the stubbed
 * preferences endpoint, so it is a round trip rather than a write.
 */
test('a reader who hid the map gets the table and no map', async ({mount}) => {
    const component = await mount(<LocationPanelHarness hidden={['map']}/>);

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    await expect(component.getByText('11S LT 8463 6908').first()).toBeVisible();

    // "Open larger" is the map's marker here, and now its only one: the caption
    // beside it named the basemap until that credit was dropped.
    await expect(component.getByText('Open larger')).toHaveCount(0);
});

test('the map is shown when the reader has hidden nothing', async ({mount}) => {
    const component = await mount(<LocationPanelHarness/>);

    await expect(component.getByText('Open larger')).toBeVisible();
});

// A grid token has no position until the conversion lands, and if it never
// lands the frame would otherwise sit blank with nothing saying why.
test('a position that never arrives says so rather than leaving an empty frame', async ({mount}) => {
    const component = await mount(
        <LocationPanelHarness
            format='mgrs'
            canonical='11SLT84636908'
            outcome='fail'
        />);

    await component.getByRole('button', {name: 'answer the conversion'}).click();

    // Twice, deliberately: the visible placeholder and the screen-reader label,
    // because role='img' on a MapLibre container would hide the canvas's own.
    await expect(
        component.getByText('The position for this coordinate is unavailable.'),
    ).toHaveCount(2);
    await expect(
        component.getByText('The position for this coordinate is unavailable.').first(),
    ).toBeVisible();
});

/*
 * The map is created once and moved thereafter, so a change of selection no
 * longer rebuilds it. That makes a stale pin possible in a way it was not when
 * the whole map was thrown away: clicking a grid coordinate while an earlier one
 * is drawn must clear the marker rather than leave the previous position on
 * screen beside the new one's readings.
 */
test('changing selection to a grid token clears the previous pin', async ({mount}) => {
    const component = await mount(<LocationPanelHarness/>);
    await component.getByRole('button', {name: 'answer the conversion'}).click();
    await expect(component.getByText('34.0561° N, 118.2500° W').first()).toBeVisible();

    await component.getByRole('button', {name: 'select the second coordinate'}).click();

    // The second coordinate's readings, not the first's.
    await expect(component.getByText('34.0561° N, 118.2500° W')).toHaveCount(0);
});

/*
 * Grammar drift, which is the failure this repository has already shipped once:
 * the band class was widened in Go and the webapp kept the older narrower one,
 * so a UTM link the server had just issued failed this side's check.
 *
 * `gridText` returns "" for a token that does not match this side's copy of the
 * canonical shapes, which is the same condition that makes `fromParams` fall
 * back. Without the fallback to the server's own answer, the one row the reader
 * opened the link for would be the row that disappeared.
 */
test.describe('a grid token this build cannot spell', () => {
    test('takes its grid row from the server rather than dropping it', async ({mount}) => {
        const component = await mount(
            <LocationPanelHarness
                format='mgrs'
                canonical='18Q323478E4306483N'
            />,
        );

        await component.getByRole('button', {name: 'answer the conversion'}).click();

        await expect(component.getByText('11S LT 8463 6908')).toBeVisible();
    });

    test('and the UTM row with it', async ({mount}) => {
        const component = await mount(
            <LocationPanelHarness
                format='utm'
                canonical='18Q323478E4306483N'
            />,
        );

        await component.getByRole('button', {name: 'answer the conversion'}).click();

        await expect(component.getByText('11S 384640E 3769080N')).toBeVisible();
    });

    /*
     * Until the answer lands there is nothing to show, and a placeholder is the
     * honest thing rather than a zero.
     *
     * Scoped to the MGRS row. Every derived row reads "converting…" for a token
     * with no local coordinate, so `.first()` would be satisfied by any of them
     * and would pass even if the grid row rendered blank, which is the defect
     * this whole block exists for.
     */
    test('says so while the answer is still in the air', async ({mount}) => {
        const component = await mount(
            <LocationPanelHarness
                format='mgrs'
                canonical='18Q323478E4306483N'
            />,
        );

        const mgrsRow = component.locator('tr', {has: component.page().getByText('MGRS', {exact: true})});

        await expect(mgrsRow.getByText('converting…')).toBeVisible();
    });
});
