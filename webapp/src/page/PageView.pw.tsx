import React from 'react';

import MapPageView from './MapPageView';
// eslint-disable-next-line import/no-duplicates
import {OverlayPageView} from './OverlayPageView';
// eslint-disable-next-line import/no-duplicates
import {OVERLAY_UNREADABLE} from './OverlayPageView';
import type {LocationPageData, OverlayPageData, PageData} from './payload';

import {expect, test} from '../../playwright/ct-coverage';
import {parseCanonical} from '../decorators/location/format';
import LocationReadings from '../decorators/location/LocationReadings';
import {ROWS} from '../decorators/location/rows';
import {ALL_FEATURES} from '../features/types';

/*
 * The standalone pages, which are what a client with no Mattermost webapp
 * opens. They render the same components the sidebar renders, so what is worth
 * testing here is the environment the pages supply rather than the table: the
 * conversion arrives already answered, there is no Customize link, and the map
 * page gives the window to one picture.
 */

const GRID: LocationPageData = {
    payload: {coord: null, format: 'mgrs', canonical: '18SUJ2347806483', raw: '18S UJ 23478 06483'},
    conversion: {
        status: 'ready',
        data: {
            mgrs: '18S UJ 23478 06483',
            utm: '18S 323478E 4306483N',
            decimal: '38.8895° N, 77.0353° W',
            dms: '38°53\'22"N 77°02\'07"W',
            ddm: '38°53.37\'N 77°02.12\'W',
            usmtf: '385322N0770207W',
            georef: 'GJNJ57885337',
            gars: '206LT26',
            pluscode: '87C4VXQ7+RV44',
            region: 'United States of America (Natural Earth 110m)',
            lat: 38.8895,
            lon: -77.0353,
        },
    },
    mode: 'location',
    maps: ALL_FEATURES,
    packages: [],
};

test('a page renders every row without waiting on anything', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
            maps={ALL_FEATURES}
        />,
    );

    // The rows a grid token cannot work out for itself are already answered,
    // because the server put them in the shell rather than leaving the page to
    // ask a route it has no session for.
    await expect(component.getByText('18S 323478E 4306483N')).toBeVisible();
    await expect(component.getByText('38.8895° N, 77.0353° W')).toBeVisible();
    await expect(component.getByText('converting…')).toHaveCount(0);
    await expect(component.getByText('unavailable')).toHaveCount(0);
});

test('a page offers no Customize link, which would need a session', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
            maps={ALL_FEATURES}
        />,
    );

    await expect(component.getByText('Customize your view')).toHaveCount(0);
});

/*
 * A reader's hidden rows live behind the authenticated API, so a public page
 * has nobody to ask and shows the lot. That is what the server-rendered page
 * showed, and it is the honest answer rather than a guess at somebody's
 * settings.
 */
test('a page shows every row, because it has no reader to ask', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
            maps={ALL_FEATURES}
        />,
    );

    // Counted off the catalog rather than written out. The literal it replaced
    // said 10, went stale the moment the area references were added, and still
    // passed as an assertion about "every row" while naming three fewer than
    // there were.
    //
    // Confidence is the one row this fixture cannot fill, and correctly: only a
    // verified USMTF token states any. Normalized does render, since this
    // token's author text and its canonical form differ.
    const expected = ROWS.filter((row) => row.id !== 'confidence').length;

    await expect(component.getByRole('row')).toHaveCount(expected);
});

test('a failed conversion degrades rather than blanking the page', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={{status: 'failed', data: null}}
            hidden={[]}
            maps={ALL_FEATURES}
        />,
    );

    // The token still names its own grid square, so that row is never in doubt.
    await expect(component.getByText('18S UJ 23478 06483').first()).toBeVisible();
    await expect(component.getByText('unavailable').first()).toBeVisible();
});

/*
 * "Open larger" is outside /decorate, so the framework's click handler stands
 * aside and nothing else would tell the map page which way to paint itself. It
 * opened on the operating system preference instead, which meant a light
 * Mattermost on a dark laptop gave a dark page and a dark map palette with it.
 *
 * The same three cases the click handler is held to, on the one link it does
 * not cover.
 */
test('Open larger tells the map page which theme to paint itself with', async ({mount, page}) => {
    await page.evaluate(() => document.documentElement.style.setProperty('--center-channel-bg', '#090a0b'));

    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
            maps={ALL_FEATURES}
        />,
    );

    const larger = component.getByRole('link', {name: 'Open larger'});
    await expect(larger).toHaveAttribute('href', /_theme=dark/);

    // Still root-relative: storing a host is what the whole URL design avoids.
    const href = await larger.getAttribute('href');
    expect(href!.startsWith('/')).toBe(true);

    await page.evaluate(() => document.documentElement.style.removeProperty('--center-channel-bg'));
});

test('Open larger passes a light theme when the sidebar is light', async ({mount, page}) => {
    await page.evaluate(() => document.documentElement.style.setProperty('--center-channel-bg', '#ffffff'));

    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
            maps={ALL_FEATURES}
        />,
    );

    await expect(component.getByRole('link', {name: 'Open larger'})).toHaveAttribute('href', /_theme=light/);

    await page.evaluate(() => document.documentElement.style.removeProperty('--center-channel-bg'));
});

test('Open larger leaves the theme unstated when it cannot be read', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
            maps={ALL_FEATURES}
        />,
    );

    await expect(component.getByRole('link', {name: 'Open larger'})).not.toHaveAttribute('href', /_theme/);
});

test('the map page carries the author\'s text and the way back', async ({mount}) => {
    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByText('18S UJ 23478 06483')).toBeVisible();

    const back = component.getByRole('link', {name: 'All readings'});
    await expect(back).toBeVisible();
    await expect(back).toHaveAttribute('href', /\/decorate\/location\?f=mgrs&v=18SUJ2347806483/);
});

/*
 * The return trip. A page has no webapp around it and therefore no click
 * handler, so this link states the theme itself or the reader lands back on the
 * readings page painted the other way.
 */
test('the way back carries the theme the map page was painted with', async ({mount, page}) => {
    await page.evaluate(() => document.documentElement.style.setProperty('--center-channel-bg', '#090a0b'));

    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByRole('link', {name: 'All readings'})).toHaveAttribute('href', /_theme=dark/);

    await page.evaluate(() => document.documentElement.style.removeProperty('--center-channel-bg'));
});

/*
 * Everything else the bar used to carry is a reading, and the page one link
 * away is nothing but readings with their labels beside them. Four of them
 * crammed unlabeled under a picture is a worse table rather than a summary.
 */
test('the map page leaves the readings to the readings page', async ({mount}) => {
    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByText('18SUJ2347806483', {exact: true})).toHaveCount(0);
    await expect(component.getByText('38.8895° N, 77.0353° W')).toHaveCount(0);
    await expect(component.getByText('United States of America (Natural Earth 110m)')).toHaveCount(0);
});

/*
 * The page that IS the larger view does not offer to open a larger view.
 */
test('the map page does not link to itself', async ({mount}) => {
    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByText('Open larger')).toHaveCount(0);
});

/*
 * No surface credits the basemap in words any more. The region's own value
 * still carries its citation, which is a different thing: it says where the
 * country came from, and it reaches the map's accessible label rather than the
 * screen.
 */
test('the map page does not print the basemap credit', async ({mount}) => {
    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByText('Natural Earth 110m', {exact: true})).toHaveCount(0);
});

test('the readings do not print the basemap credit either', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
            maps={ALL_FEATURES}
        />,
    );

    await expect(component.getByText('Natural Earth 110m', {exact: true})).toHaveCount(0);

    // The map is still there, and still offers its one control.
    await expect(component.getByRole('link', {name: 'Open larger'})).toBeVisible();
});

/*
 * The author's text is what the bar names, so a conversion that did not arrive
 * must fall back to the token rather than showing an `r` nothing vouched for.
 */
test('the map page falls back to the token when nothing vouched for the text', async ({mount}) => {
    const component = await mount(
        <MapPageView data={{...GRID, conversion: {status: 'failed', data: null}, mode: 'map'}}/>,
    );

    await expect(component.getByText('18SUJ2347806483', {exact: true})).toBeVisible();
    await expect(component.getByText('18S UJ 23478 06483')).toHaveCount(0);
});

/*
 * A map page for a textual coordinate, which is the ordinary case: the position
 * comes out of the token rather than out of the conversion, and the conversion
 * is only there for the rows a projection is needed for.
 */
const TEXTUAL: PageData = {
    payload: {
        coord: parseCanonical('dd', '34.0561,-118.2500'),
        format: 'dd',
        canonical: '34.0561,-118.2500',
        raw: '34.0561, -118.2500',
    },
    conversion: {
        status: 'ready',
        data: {
            mgrs: '11S LT 8463 6908',
            utm: '11S 384640E 3769080N',
            decimal: '34.0561\u00b0 N, 118.2500\u00b0 W',
            dms: '34\u00b003\'22.0"N 118\u00b015\'00.0"W',
            ddm: '34\u00b003.366\'N 118\u00b015.000\'W',
            usmtf: '340322.0N1181500.0W',
            georef: 'EJBE45000336',
            gars: '124LJ47',
            pluscode: '85633Q42+C2R',
            region: 'United States of America (Natural Earth 110m)',
            lat: 34.0561,
            lon: -118.25,
        },
    },
    mode: 'map',
    maps: ALL_FEATURES,
    packages: [],
};

test('the map page draws a position the token itself carries', async ({mount}) => {
    const component = await mount(<MapPageView data={TEXTUAL}/>);

    // The author's own spelling, which is what the bar carries and all it
    // carries now that the readings live one link away.
    await expect(component.getByText('34.0561, -118.2500')).toBeVisible();
    await expect(component.getByRole('link', {name: 'All readings'})).toBeVisible();

    // The map is handed a position worked out here rather than one waited on,
    // so it is never told there is none.
    await expect(component.getByText('The position for this coordinate is unavailable.')).toHaveCount(0);
});

// The way back carries the author's text, so the readings page leads with the
// same line this one does.
test('the way back carries the author\'s text when it differs from the token', async ({mount}) => {
    const component = await mount(<MapPageView data={TEXTUAL}/>);

    const href = await component.getByRole('link', {name: 'All readings'}).getAttribute('href');
    const params = new URLSearchParams(href!.split('?')[1]);

    expect(params.get('f')).toBe('dd');
    expect(params.get('v')).toBe('34.0561,-118.2500');
    expect(params.get('r')).toBe('34.0561, -118.2500');
});

/*
 * The readings page with maps switched off.
 *
 * It reads the answer out of the shell rather than fetching it, because a page
 * is handed everything it needs in one document and /api/v1/features needs a
 * session this route does not otherwise require. Everything the table carries
 * stays: the switch is about the picture and the bytes behind it.
 */
test('a page with maps off renders the table and no map', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
            maps={{mapPanel: false, mapInline: false, mapPage: false}}
        />,
    );

    await expect(component.getByText(/^World map/)).toHaveCount(0);
    await expect(component.getByTestId('map-note')).toHaveCount(0);
    await expect(component.getByText('Open larger')).toHaveCount(0);

    // Every reading still there, which is the whole point of only /map being
    // refused outright.
    await expect(component.getByText('18SUJ2347806483', {exact: true})).toBeVisible();
    await expect(component.getByText('38.8895° N, 77.0353° W')).toBeVisible();
});

// The page map and the "Open larger" link are separate switches, so a page can
// draw its own map while the full-window one is not served.
test('a page keeps its map when only the map page is off', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
            maps={{mapPanel: true, mapInline: false, mapPage: false}}
        />,
    );

    await expect(component.getByText('Open larger')).toHaveCount(0);
    await expect(component.getByTestId('map-note')).toBeAttached();
});

/*
 * The overlay page, which is what "Open larger" opens for a stamped post.
 *
 * The shell carries the post's props blob rather than a list of markers and
 * shapes worked out in Go, so this page runs the card's own reader and the
 * card's own canvas. What is worth testing here is that it picks the right
 * reader for the kind, and that a blob it cannot read says so instead of
 * drawing an empty world.
 */
test.describe('the overlay page', () => {
    function overlay(over: Partial<OverlayPageData>): OverlayPageData {
        return {mode: 'overlay', kind: '', props: {}, packages: [], ...over};
    }

    const GEOJSON_PROPS = {
        tactical_fusion_geojson: {
            version: 1,
            source: 'fence',
            src: '{"type":"Point","coordinates":[-118.25,34.0561]}',
            counts: {features: 1, points: 1},
            features: [{
                name: 'Depot',
                kind: 'Point',
                parts: [{kind: 'Point', rings: [[{lat: '34.056100', lon: '-118.250000', alt: ''}]], ring_counts: []}],
            }],
        },
    };

    test('draws a GeoJSON document and says what is on it', async ({mount}) => {
        const component = await mount(
            <OverlayPageView data={overlay({kind: 'custom_tf_geojson', props: GEOJSON_PROPS})}/>,
        );

        await expect(component).not.toContainText(OVERLAY_UNREADABLE);

        // The label the card's own map uses, through the same mapLabel, so the
        // page cannot come to describe the overlay differently.
        await expect(component).toContainText('1 marked position');
    });

    /*
     * The page carries no link out of itself.
     *
     * It had a "Back to the post" permalink, which cost two API calls per page
     * load, could not be built at all for a direct or group message, and was a
     * second way to reach the post the reader had just come from.
     */
    /*
     * The Cursor on Target arm of drawingFor, which nothing exercised.
     *
     * `custom_tf_cot` appeared exactly once in this file, in the refusal case
     * below that hands the CoT kind a GeoJSON blob. That covers the `return
     * null`; it never rendered a CoT overlay successfully, so the CotMapCanvas
     * mount and the event count had never run.
     *
     * The same gap existed server-side until TestTheOverlayRouteServesACotPost:
     * every mappost_test.go fixture was GeoJSON, so the CoT arm of
     * stampedPropsKey went untested on both halves of the same feature.
     */
    const COT_PROPS = {
        tactical_fusion_cot: {
            version: 2,
            source: 'fence',
            src: '<event uid="ANDROID-1"/>',
            events: [
                {
                    uid: 'ANDROID-1',
                    cot_type: 'a-f-G-U-C',
                    type_label: 'Friendly Ground',
                    lat: '34.0561',
                    lon: '-118.2500',
                },
                {
                    uid: 'ANDROID-2',
                    cot_type: 'a-f-G-U-C',
                    type_label: 'Friendly Ground',
                    lat: '35.0000',
                    lon: '-119.0000',
                },
            ],
        },
    };

    test('draws a Cursor on Target block and counts its events', async ({mount}) => {
        const component = await mount(
            <OverlayPageView data={overlay({kind: 'custom_tf_cot', props: COT_PROPS})}/>,
        );

        await expect(component).not.toContainText(OVERLAY_UNREADABLE);
        await expect(component).toContainText('2 events');
    });

    // Singular, because "1 events" is the kind of thing a reader notices.
    test('says one event without pluralizing it', async ({mount}) => {
        const one = {
            tactical_fusion_cot: {
                ...COT_PROPS.tactical_fusion_cot,
                events: [COT_PROPS.tactical_fusion_cot.events[0]],
            },
        };

        const component = await mount(
            <OverlayPageView data={overlay({kind: 'custom_tf_cot', props: one})}/>,
        );

        await expect(component).toContainText('1 event');
        await expect(component).not.toContainText('1 events');
    });

    test('carries no link out of itself', async ({mount}) => {
        const component = await mount(
            <OverlayPageView data={overlay({kind: 'custom_tf_geojson', props: GEOJSON_PROPS})}/>,
        );

        await expect(component).toContainText('1 marked position');
        await expect(component.getByRole('link')).toHaveCount(0);
    });

    /*
     * A blob the reader refuses, a kind from a later server, and a blob read
     * with the wrong format's reader all land in the same place. Saying so
     * beats a window of empty basemap, which looks like a document that drew
     * nothing rather than one that could not be read.
     */
    for (const [name, data] of [
        ['a blob this build cannot read', overlay({kind: 'custom_tf_geojson', props: {}})],
        ['a kind this build does not know', overlay({kind: 'custom_tf_later', props: GEOJSON_PROPS})],
        ['the other format\'s reader', overlay({kind: 'custom_tf_cot', props: GEOJSON_PROPS})],
    ] as Array<[string, OverlayPageData]>) {
        test(`says so for ${name}`, async ({mount}) => {
            const component = await mount(<OverlayPageView data={data}/>);

            await expect(component).toContainText(OVERLAY_UNREADABLE);
        });
    }
});
