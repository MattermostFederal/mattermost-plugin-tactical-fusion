import React from 'react';

import LocationPanelHarness from './LocationPanelHarness';

import {expect, test} from '../../../playwright/ct-coverage';

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
// labelled buttons underneath.
test('offers a copy control on every value row and none on the prose rows', async ({mount}) => {
    const component = await mount(<LocationPanelHarness/>);
    await component.getByRole('button', {name: 'answer the conversion'}).click();

    const labels = ['Copy MGRS', 'Copy Lat / lon', 'Copy DMS', 'Copy DDM', 'Copy USMTF',
        'Copy UTM', 'Copy Original text'];
    await Promise.all(labels.map((label) =>
        expect(component.getByRole('button', {name: label}), label).toBeVisible()));

    // Prose, not a value: copying "about 11 m" or "WGS 84" gets you a sentence.
    await expect(component.getByRole('button', {name: 'Copy Resolution'})).toHaveCount(0);
    await expect(component.getByRole('button', {name: 'Copy Datum'})).toHaveCount(0);
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

    // Nothing above the table at all.
    await expect(component.locator('table')).toHaveCount(1);
    await expect(component.locator('div > p').first()).toContainText('Positions are WGS 84');
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
                hidden={['mgrs', 'decimal', 'dms', 'ddm', 'usmtf', 'utm',
                    'resolution', 'confidence', 'datum', 'raw', 'canonical']}
            />);

        await expect(component.getByRole('button', {name: 'Copy Lat / lon'})).toHaveCount(0);
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
        await expect(component.getByText('Lat / lon', {exact: true})).toHaveCount(0);

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
