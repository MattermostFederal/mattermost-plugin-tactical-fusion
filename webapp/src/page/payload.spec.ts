import {expect, test} from '@playwright/test';

import {readPageData} from './payload';

/*
 * What the page trusts, and what it does with values it cannot make sense of.
 *
 * These are the server's own output rather than a reader's input, but the page
 * is the surface of last resort: there is no fall-through behind it, so every
 * shape that can arrive has to render something true.
 */

const CONVERSION = {
    mgrs: '18S UJ 23478 06483',
    utm: '18S 323478E 4306483N',
    decimal: '38.8895° N, 77.0353° W',
    dms: '38°53\'22"N 77°02\'07"W',
    ddm: '38°53.37\'N 77°02.12\'W',
    usmtf: '385322N0770207W',
    region: 'United States of America (Natural Earth 110m)',
    lat: 38.8895,
    lon: -77.0353,
};

function root(data: Record<string, string>): HTMLElement {
    const el = {dataset: data} as unknown as HTMLElement;
    return el;
}

test('reads the shell the server wrote', () => {
    const data = readPageData(root({
        mode: 'location',
        f: 'mgrs',
        v: '18SUJ2347806483',
        r: '18S UJ 23478 06483',
        conversion: JSON.stringify(CONVERSION),
    }));

    expect(data).not.toBeNull();
    expect(data!.mode).toBe('location');
    expect(data!.payload.format).toBe('mgrs');
    expect(data!.payload.canonical).toBe('18SUJ2347806483');
    expect(data!.payload.raw).toBe('18S UJ 23478 06483');
    expect(data!.conversion.status).toBe('ready');
    expect(data!.conversion.data?.utm).toBe('18S 323478E 4306483N');
});

test('an absent r is the canonical form, not a missing value', () => {
    const data = readPageData(root({mode: 'location', f: 'latm', v: '3510N07901W'}));

    expect(data!.payload.raw).toBe('3510N07901W');
});

// A shell written without one. The format is what decides whether the page
// renders at all, so an absent token reads as empty rather than refusing, and
// the row it fills is the one the conversion answers.
test('an absent v reads as an empty canonical form', () => {
    const data = readPageData(root({mode: 'location', f: 'latm'}));

    expect(data).not.toBeNull();
    expect(data!.payload.canonical).toBe('');
    expect(data!.payload.raw).toBe('');
});

test('anything but map is the readings view', () => {
    expect(readPageData(root({mode: 'map', f: 'latm', v: '3510N07901W'}))!.mode).toBe('map');
    expect(readPageData(root({f: 'latm', v: '3510N07901W'}))!.mode).toBe('location');
});

/*
 * An unknown format DEGRADES; only a missing one refuses.
 *
 * This pinned the opposite, and the opposite blanked the page: `start()` returns
 * without rendering on a null, into a shell that is one empty div, so a format id
 * this build did not know produced a wholly empty document on the route the
 * mobile app opens, with nothing logged and the server's own conversion sitting
 * unread three attributes away.
 *
 * That is the coarser half of the drift `payloadFor` already degrades on, so it
 * degrades the same way now: no local coordinate, every row from the conversion.
 * An EMPTY id still refuses, because it means the document was not written by
 * this plugin's shell at all and there is nothing to render from.
 */
test('an unknown format degrades to the server conversion', () => {
    const data = readPageData(root({f: 'nope', v: 'x'}));

    expect(data).not.toBeNull();
    expect(data!.payload.coord).toBeNull();
    expect(data!.payload.canonical).toBe('x');
});

test('a missing format is refused outright', () => {
    expect(readPageData(root({v: '3510N07901W'}))).toBeNull();
});

/*
 * The three ways a conversion can fail to arrive. All degrade rather than
 * throwing, because every row the token yields locally is still true and the
 * rest reads "unavailable" — which is what the sidebar shows a reader whose
 * request did not arrive.
 */
test.describe('a conversion that cannot be used degrades', () => {
    const cases: Array<[string, Record<string, string>]> = [
        ['absent', {f: 'latm', v: '3510N07901W'}],
        ['not JSON', {f: 'latm', v: '3510N07901W', conversion: '{oh dear'}],
        ['missing a field', {f: 'latm', v: '3510N07901W', conversion: '{"mgrs":"x"}'}],
        ['a non-finite position', {
            f: 'latm',
            v: '3510N07901W',
            conversion: JSON.stringify({...CONVERSION, lat: null}),
        }],
    ];

    for (const [name, attrs] of cases) {
        test(name, () => {
            const data = readPageData(root(attrs));

            expect(data).not.toBeNull();
            expect(data!.conversion.status).toBe('failed');
            expect(data!.conversion.data).toBeNull();
        });
    }
});

/*
 * The drift case this fallback exists for: a token the server issued that this
 * build's copy of the canonical shapes does not recognise. It must still render
 * from the conversion rather than refusing, because on a page there is nothing
 * behind it.
 */
test('a token this build cannot parse still yields a payload', () => {
    const data = readPageData(root({
        f: 'utm',
        v: '18Q323478E4306483N',
        conversion: JSON.stringify(CONVERSION),
    }));

    expect(data).not.toBeNull();
    expect(data!.payload.coord).toBeNull();
    expect(data!.payload.canonical).toBe('18Q323478E4306483N');
    expect(data!.conversion.status).toBe('ready');
});

/*
 * Which map surfaces the shell says are on.
 *
 * A page cannot ask /api/v1/features, so this attribute is the only thing it has
 * to go on, and the absent case is the one that matters: a missing attribute is
 * a shell this bundle cannot interpret, not an admin decision, and reading the
 * two the same way would leave a maps-off install drawing and a maps-on install
 * blank. The server writes it unconditionally, empty included, so "off" is
 * always said out loud.
 */
test('reads the map surfaces out of the shell', () => {
    const data = readPageData(root({
        mode: 'location',
        f: 'mgrs',
        v: '18SUJ2347806483',
        maps: 'panel,page',
    }));

    expect(data!.maps.mapPanel).toBe(true);
    expect(data!.maps.mapPage).toBe(true);
});

test('reads an empty list as every surface off', () => {
    const data = readPageData(root({
        mode: 'location',
        f: 'mgrs',
        v: '18SUJ2347806483',
        maps: '',
    }));

    expect(data!.maps.mapPanel).toBe(false);
    expect(data!.maps.mapPage).toBe(false);
});

test('reads one surface without the other', () => {
    const data = readPageData(root({
        mode: 'location',
        f: 'mgrs',
        v: '18SUJ2347806483',
        maps: 'panel',
    }));

    expect(data!.maps.mapPanel).toBe(true);
    expect(data!.maps.mapPage).toBe(false);
});

// An absent attribute means a shell older than this bundle, and defaults ON.
// Drawing a map nobody asked for costs a fetch on a page the reader is already
// looking at; failing to draw one somebody is paying for is silent.
test('reads an absent attribute as every surface on', () => {
    const data = readPageData(root({
        mode: 'location',
        f: 'mgrs',
        v: '18SUJ2347806483',
    }));

    expect(data!.maps.mapPanel).toBe(true);
    expect(data!.maps.mapPage).toBe(true);
});

// A surface a later server knows about and this bundle does not is ignored
// rather than drawn, and `inline` is never claimed: no page has posts in it.
test('ignores a surface this bundle does not know', () => {
    const data = readPageData(root({
        mode: 'location',
        f: 'mgrs',
        v: '18SUJ2347806483',
        maps: 'panel,hologram,inline',
    }));

    expect(data!.maps.mapPanel).toBe(true);
    expect(data!.maps.mapPage).toBe(false);
    expect(data!.maps.mapInline).toBe(false);
});
