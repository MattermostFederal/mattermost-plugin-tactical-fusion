import type {Locator} from '@playwright/test';
import React from 'react';

import {serveMapAssets} from './asset_fixtures';
import LocationMapHarness from './LocationMapHarness';
import type {ViewName} from './LocationMapHarness';
import {MARKER_SIZE} from './maplibre';
import type {MapGeometry} from './overlay';
import type {MapShape} from './paint';
import {DATA_MAX_ZOOM, MAX_ZOOM} from './span';

import {expect, test} from '../../../../playwright/ct-coverage';

/*
 * The map, with a real basemap behind it.
 *
 * Every other suite that reaches this component leaves the basemap unanswered,
 * so MapLibre was never constructed under test: the creation path, the camera
 * and both overlay sources were dead to the whole suite. The unlock is one
 * route. The committed archive is served rather than a fixture, so the header
 * probe in basemap.ts runs against the bytes this build actually ships.
 *
 * Once the archive, the worker and the glyphs are routed, the map fetches
 * nothing else, which is what makes this deterministic rather than a network
 * test. `serveMapAssets` is shared with the hover suite; `starveFonts` 404s the
 * glyph ranges instead, which is how the two label tests below tell apart what
 * the fonts are actually responsible for.
 *
 * Assertions go through the harness's outputs rather than the DOM wherever the
 * subject is MapLibre's own state. An earlier version of this file watched the
 * placeholder text and the Reset button instead, and deleting the overlay
 * writes, the camera move, `remove()` on unmount, or restoring the historic
 * `live`-flag load guard left all of it green.
 */

const LOADING = 'Loading map…';
const NO_WEBGL = 'This browser cannot draw the map.';
const NO_BASEMAP = 'The map could not be loaded.';
const NO_POSITION = 'The position for this coordinate is unavailable.';
const RESET = 'Reset view';

/*
 * The visible note. The same text is also in the label.
 *
 * Addressed by test id rather than by role, because the frame carries other
 * paragraphs now: the overzoom notice and the zoom readout. This used to be
 * `getByRole('paragraph')`, and `expectDrawn` asserts the note is ABSENT, so
 * every one of those assertions started failing the moment a second paragraph
 * was drawn over a perfectly working map.
 */
function noteOf(component: Locator): Locator {
    return component.getByTestId('map-note');
}

/**
 * Drawn once the note clears and the one control appears.
 *
 * An `expect` rather than a bare `waitFor`, so a failure prints the note the
 * component is actually showing. Every way this file can go wrong (no WebGL2, a
 * renamed MapLibre chunk, a stale basemap digest) arrives here, and the note is
 * the only thing that says which.
 */
async function expectDrawn(component: Locator): Promise<void> {
    await expect(component.getByRole('button', {name: RESET})).toBeVisible({timeout: 15_000});
    await expect(noteOf(component)).toHaveCount(0);
}

/**
 * Drawn, for a preview card, which has none of the furniture to wait on.
 *
 * `expectDrawn` keys on the Reset button, and preview mode deliberately draws
 * no controls, so the preview tests had nothing to wait on and polled a reading
 * straight away. Every reading is -1 until MapLibre exists, so a map that failed
 * to come up at all reported `expected "1", received "-1"`, which names neither
 * the failure nor its cause and is the shape the intermittent one took.
 *
 * The note is rendered in preview mode too, so this gets the same diagnosis
 * `expectDrawn` does.
 */
async function expectPreviewDrawn(component: Locator): Promise<void> {
    // Reports the note rather than the readiness flag, so the failure names the
    // cause: "The map could not be loaded." rather than "expected yes, got no".
    await expect.poll(async () => {
        if (await component.getByTestId('live-map').textContent() === 'yes') {
            return 'drawn';
        }

        return await noteOf(component).textContent().catch(() => null) ?? 'no map, and no note';
    }, {message: 'the preview map never came up', timeout: 15_000}).toBe('drawn');

    await expect(noteOf(component)).toHaveCount(0);
}

/** Reads MapLibre's own state into the harness's outputs. */
async function readMap(component: Locator): Promise<void> {
    await component.getByRole('button', {name: 'read the map'}).click();
}

/*
 * These tests need a real WebGL2 context, which headless Chromium supplies
 * through SwiftShader. Asserted once, by name, because without it eleven tests
 * fail as timeouts that say nothing about the cause.
 */
test.beforeAll(async ({browser}) => {
    const page = await browser.newPage();
    const ok = await page.evaluate(() => Boolean(document.createElement('canvas').getContext('webgl2')));
    await page.close();

    expect(ok, 'these tests need WebGL2 in the component-test browser').toBe(true);
});

test.describe('with a basemap it can verify', () => {
    test('draws a map and offers Reset view', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);

        await expectDrawn(component);
        await expect(component.getByTestId('maps-created')).toHaveText('1');
    });

    // The pin and the cell are what the map is for, and neither reaches the
    // DOM. Without this the whole overlay path could be deleted with the suite
    // still green.
    test('puts a pin and a cell on the map', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);
        await expectDrawn(component);
        await readMap(component);

        await expect(component.getByTestId('pin-features')).toHaveText('1');
        await expect(component.getByTestId('cell-features')).toHaveText('1');
    });

    // The camera is held on the coordinate, not left at the style's default.
    test('opens with the camera on the coordinate', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);
        await expectDrawn(component);
        await readMap(component);

        await expect(component.getByTestId('camera')).toHaveText('34.056,-118.250');
    });

    // A token with no resolution to draw gets its dot and no rectangle. There
    // is no minimum cell size and no threshold below which one is dropped:
    // these surfaces zoom, so a meter-wide cell is invisible until the reader
    // zooms in, which is more honest than a number guessing on their behalf.
    test('a coordinate with no cell draws the pin alone', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness start='no cell'/>);
        await expectDrawn(component);
        await readMap(component);

        await expect(component.getByTestId('pin-features')).toHaveText('1');
        await expect(component.getByTestId('cell-features')).toHaveText('0');
    });

    // The country reaches a reader through this label and nowhere else, now
    // that the Region row is retired: no map, no country, and with the map
    // drawn, no country for anyone reading it with their eyes.
    test('names the region in its accessible label', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness region='United States of America (Natural Earth 110m)'/>,
        );

        await expectDrawn(component);
        await expect(
            component.getByText(
                'World map. The marked position is in United States of America (Natural Earth 110m).',
            ),
        ).toBeAttached();
    });

    // The label is the one value on this map that comes from data rather than
    // from a source literal, so its escaping is what makes "nothing from a
    // request reaches it" a defense rather than an accident. React escapes a
    // text node, and this asserts that it does: the guard CLAUDE.md described
    // was pinned by nothing in either language once the label moved out of
    // aria-label and into a visually-hidden span.
    test('renders a hostile region as text rather than as markup', async ({mount, page}) => {
        await serveMapAssets(page);

        const hostile = '<img src=x onerror="alert(1)">';
        const component = await mount(<LocationMapHarness region={hostile}/>);

        await expectDrawn(component);
        await expect(
            component.getByText(`World map. The marked position is in ${hostile}.`),
        ).toBeAttached();
        await expect(component.locator('img')).toHaveCount(0);
    });

    // A position in no polygon yields nothing, and the label never guesses at a
    // nearest country.
    test('says only that a position is marked when there is no region', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);

        await expectDrawn(component);
        await expect(component.getByText('World map with the position marked.')).toBeAttached();
    });

    test('offers the larger view in its caption', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness pageHref='/plugins/x/map?f=dd'/>);

        await expectDrawn(component);
        await expect(component.getByRole('link', {name: 'Open larger'})).toBeVisible();
    });

    // The page that IS the larger view fills its parent, and must not offer a
    // link to itself.
    test('filling its parent omits the caption', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                fill={true}
                pageHref='/plugins/x/map?f=dd'
            />,
        );

        await expectDrawn(component);
        await expect(component.getByText('Open larger')).toHaveCount(0);
    });

    // Once a reader can zoom and pan there is otherwise no way back to the pin,
    // which is why the control resets the zoom as well as the center.
    test('Reset view brings the camera back to the coordinate', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);
        await expectDrawn(component);

        await page.mouse.move(160, 120);
        await page.mouse.down();
        await page.mouse.move(40, 120, {steps: 8});
        await page.mouse.up();

        await readMap(component);
        await expect(component.getByTestId('camera')).not.toHaveText('34.056,-118.250');

        await component.getByRole('button', {name: RESET}).click();
        await readMap(component);

        await expect(component.getByTestId('camera')).toHaveText('34.056,-118.250');
    });
});

test.describe('changing selection', () => {
    /*
     * The panel stays mounted across a change of selection, so the map is moved
     * rather than rebuilt. Rebuilding meant a fresh WebGL context and a
     * re-tessellation of the whole basemap per click, against the browser's cap
     * of roughly sixteen live contexts, and those clicks arrive in a run.
     *
     * The count is the assertion. A canvas count cannot tell the two apart,
     * because a rebuild removes the old canvas on its way out.
     */
    test('moves the one map rather than building a second', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);
        await expectDrawn(component);

        await component.getByRole('button', {name: 'select Washington'}).click();
        await expect(component.getByTestId('selected')).toHaveText('Washington');
        await readMap(component);

        await expect(component.getByTestId('maps-created')).toHaveText('1');
        await expect(component.getByTestId('camera')).toHaveText('38.889,-77.035');
    });

    /*
     * A stale pin is worse than no pin. Clicking a grid coordinate while an
     * earlier one is drawn would otherwise leave the previous position on
     * screen, beside the new one's readings, until the conversion lands, and
     * permanently if it never does.
     */
    test('a position that is not known yet takes the pin down', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);
        await expectDrawn(component);

        await component.getByRole('button', {name: 'select unknown'}).click();
        await expect(noteOf(component)).toHaveText(NO_POSITION);
        await readMap(component);

        await expect(component.getByTestId('pin-features')).toHaveText('0');
        await expect(component.getByTestId('cell-features')).toHaveText('0');
        await expect(component.getByRole('button', {name: RESET})).toHaveCount(0);
    });

    test('and the pin comes back when the position does', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);
        await expectDrawn(component);

        await component.getByRole('button', {name: 'select unknown'}).click();
        await expect(noteOf(component)).toHaveText(NO_POSITION);

        await component.getByRole('button', {name: 'select Washington'}).click();
        await expectDrawn(component);
        await readMap(component);

        await expect(component.getByTestId('pin-features')).toHaveText('1');
        await expect(component.getByTestId('camera')).toHaveText('38.889,-77.035');
    });

    /*
     * Web Mercator caps at about 85 while the grammars validate latitude to 90,
     * so a decoratable token can land past it. Clamping would put the pin up to
     * 550 km from what the author wrote, so the map is omitted, the overlays go
     * down, and the note says which way.
     */
    test('a position past the projection is named, not clamped', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);
        await expectDrawn(component);

        await component.getByRole('button', {name: 'select too far north'}).click();
        await expect(noteOf(component)).toHaveText('This position is too far north for the map.');
        await readMap(component);
        await expect(component.getByTestId('pin-features')).toHaveText('0');

        await component.getByRole('button', {name: 'select too far south'}).click();
        await expect(noteOf(component)).toHaveText('This position is too far south for the map.');
    });
});

/*
 * Labels need the bundled fonts. This was measured, not assumed, and it came out
 * the opposite way to the expectation it was written to confirm.
 *
 * Whether a label is on screen is decided by the opening camera, which always
 * frames a fixed ground span: none of the other views has a label anchor in
 * frame, so they answer zero for reasons that have nothing to do with fonts.
 * That is why these use a view sitting on an anchor.
 *
 * What is NOT asserted here is what a missing glyph range looks like to a
 * reader. MapLibre still places the symbol without it, so the query cannot tell
 * a painted label from an unpainted one, and this file deliberately does not
 * assert pixels on a WebGL canvas. The contract pinned below is the one that can
 * be: a font outage costs neither the map nor the panel.
 */
test.describe('labels', () => {
    test('are drawn from the archive', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness start='on a label'/>);
        await expectDrawn(component);

        await expect.poll(async () => {
            await component.getByRole('button', {name: 'read the map'}).click();

            return Number(await component.getByTestId('labels-drawn').textContent());
        }, {message: 'labels drawn'}).toBeGreaterThan(0);
    });

    test('do not take the map with them when the fonts cannot be served', async ({mount, page}) => {
        await serveMapAssets(page, undefined, true);

        const component = await mount(<LocationMapHarness start='on a label'/>);

        // The map still draws, and says nothing about a failure: geometry does
        // not depend on the fonts, and a 404 on a glyph range is not a broken
        // deploy. Before the error handler learned to ignore source-scoped
        // errors, a failure like this printed "The map could not be loaded."
        // over a map that was drawing perfectly.
        await expectDrawn(component);
        await expect(noteOf(component)).toHaveCount(0);
    });
});

/*
 * The basemap says the truth about land and water.
 *
 * Migrated from the Go suite, which asked the same five positions of the GeoJSON
 * basemap's polygons directly. That check could not follow the data into vector
 * tiles without a protobuf decoder in the shipping module, and asking a renderer
 * is the better question regardless: a basemap whose polygons are right but
 * whose tiling is shifted or clipped would pass the old test and still draw the
 * sea over Kansas.
 *
 * The failure this guards is the one worth catching loudly. A position drawn
 * with the wrong fill under it is a map that says the opposite of the truth, and
 * nothing else in the suite would notice.
 */
test.describe('land and water', () => {
    const positions: Array<[ViewName, boolean]> = [
        ['Washington', true],
        ['Kansas', true],
        ['central Australia', true],
        ['mid Pacific', false],
        ['mid Atlantic', false],
    ];

    for (const [where, onLand] of positions) {
        test(`${where} is ${onLand ? 'on land' : 'at sea'}`, async ({mount, page}) => {
            await serveMapAssets(page);

            const component = await mount(<LocationMapHarness start={where}/>);
            await expectDrawn(component);

            // The sea cases are why this waits for tiles rather than polling
            // the count alone: 0 is also what an unloaded map, a map with no
            // archive, and a map whose style died all report, so without a
            // positive control "at sea" passes against a basemap that draws
            // nothing at all.
            await expect.poll(async () => {
                await component.getByRole('button', {name: 'read the map'}).click();

                return component.getByTestId('tiles').textContent();
            }, {message: `tiles loaded at ${where}`}).toBe('loaded');

            const drawn = Number(await component.getByTestId('land-at-center').textContent());
            if (onLand) {
                // Greater-than rather than exactly one: a coastline split
                // across tiles or tiers can legitimately return several
                // polygons under one pixel.
                expect(drawn).toBeGreaterThan(0);
            } else {
                expect(drawn).toBe(0);
            }
        });
    }
});

test.describe('the map outliving one coordinate', () => {
    /*
     * The historic defect, reproduced. The 'load' handler is guarded on the
     * instance, not on the effect run that made it: the map is stored on a ref
     * and removed only on unmount, so it outlives that run. Guarded on the run's
     * `live` flag, a reader who changed selection between construction and
     * `load` left readiness false forever, every later applyView no-opped, and
     * the panel sat on "Loading map…" until the page was reloaded.
     *
     * Holding the worker is what makes the window deterministic: the map is
     * constructed as soon as the basemap lands, but nothing tiles and `load`
     * cannot fire until the worker arrives. Selection has to leave `known` and
     * come back, since that is the only thing that re-runs the creation effect
     * and therefore the only thing that flips the old guard's flag.
     */
    test('a selection changed between construction and load still ends up drawn', async ({mount, page}) => {
        let release = () => { /* replaced below */ };
        const held = new Promise<void>((resolve) => {
            release = resolve;
        });
        await serveMapAssets(page, held);

        const component = await mount(<LocationMapHarness/>);

        await expect(component.getByTestId('live-map')).toHaveText('yes');
        await expect(noteOf(component)).toHaveText(LOADING);

        await component.getByRole('button', {name: 'select unknown'}).click();
        await expect(noteOf(component)).toHaveText(NO_POSITION);
        await component.getByRole('button', {name: 'select Washington'}).click();

        release();

        await expectDrawn(component);
        await readMap(component);

        await expect(component.getByTestId('maps-created')).toHaveText('1');
        await expect(component.getByTestId('camera')).toHaveText('38.889,-77.035');
    });

    // Browsers cap live WebGL contexts at about sixteen and the panel outlives
    // any one coordinate, so the map is removed when the component finally
    // goes. The canvas disappearing proves nothing: React takes the container
    // subtree with it either way.
    test('unmounting removes the map, not just its canvas', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);
        await expectDrawn(component);

        await component.getByRole('button', {name: 'unmount the map'}).click();
        await readMap(component);

        await expect(component.getByTestId('removed')).toHaveText('yes');
        await expect(component.getByTestId('live-map')).toHaveText('no');
        await expect(component.locator('canvas')).toHaveCount(0);
    });
});

/*
 * Nothing here may fail the panel: every failure hides the map and leaves every
 * reading on screen. Which failure it was still has to be stated, because
 * reporting a load failure as a missing capability sends the reader, and whoever
 * they report it to, looking at the wrong thing.
 */
test.describe('when there is no map to draw', () => {
    test('a browser without WebGL2 is told that, and not that the map failed', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness noWebGL={true}/>);

        await expect(noteOf(component)).toHaveText(NO_WEBGL);
        await expect(component.getByTestId('maps-created')).toHaveText('0');
    });

    test('a library that will not load is a load failure, not a capability one', async ({mount, page}) => {
        await serveMapAssets(page);
        await page.route('**/maplibre-gl-*.js', (route) => route.abort());

        const component = await mount(<LocationMapHarness/>);

        await expect(noteOf(component)).toHaveText(NO_BASEMAP);
        await expect(component.getByTestId('maps-created')).toHaveText('0');
    });

    // The other half: the library loaded, so this is a broken deploy rather
    // than a browser that cannot draw. Distinguished from the case above by the
    // library having got far enough to be asked for a map.
    test('a basemap that will not load says so', async ({mount, page}) => {
        await page.route('**/public/map/world.pmtiles*', (route) => route.fulfill({status: 404}));

        const component = await mount(<LocationMapHarness/>);

        await expect(noteOf(component)).toHaveText(NO_BASEMAP);
    });

    /*
     * A style or context failure never fires 'load', so without the error
     * handler the panel sits on "Loading map…" with no way out. The worker is
     * held so the map is constructed and unready, which is the only state in
     * which this handler is allowed to speak.
     */
    test('a map that fails before it is usable says so rather than hanging', async ({mount, page}) => {
        const held = new Promise<void>(() => { /* never released */ });
        await serveMapAssets(page, held);

        const component = await mount(<LocationMapHarness/>);
        await expect(component.getByTestId('live-map')).toHaveText('yes');
        await expect(noteOf(component)).toHaveText(LOADING);

        await component.getByRole('button', {name: 'make the map fail'}).click();

        await expect(noteOf(component)).toHaveText(NO_BASEMAP);
    });

    /*
     * A transient failure is retried by the next coordinate.
     *
     * This is the property the whole retry design exists for and it was pinned
     * by nothing: basemap.ts deliberately declines to remember a network throw,
     * and the panel then made that retry unreachable by keeping the position out
     * of the creation effect's deps, so every later coordinate read "The map
     * could not be loaded" until the panel unmounted.
     *
     * Aborted rather than 404'd, and the difference is the whole test: a 404 is
     * DEFINITIVE, so basemap.ts latches it on purpose and no retry is possible.
     */
    test('a transient failure is retried by the next coordinate', async ({mount, page}) => {
        await page.route('**/public/map/world.pmtiles*', (route) => route.abort());

        const component = await mount(<LocationMapHarness/>);
        await expect(noteOf(component)).toHaveText(NO_BASEMAP);

        await page.unroute('**/public/map/world.pmtiles*');
        await serveMapAssets(page);

        await component.getByRole('button', {name: 'select Washington'}).click();

        await expectDrawn(component);
    });

    /*
     * And the harder half: a map that was BUILT and then errored before 'load'.
     *
     * That is the likelier failure, because loadBasemap reads only 127 bytes and
     * every tile fetch happens after construction. The instance stayed in the
     * ref, so `map.current` was truthy forever and every later creation attempt
     * returned at the guard: a frozen map under a stale note, with applyView
     * no-opping on !ready.current and no way back short of unmounting the panel.
     * Clearing the verdict alone does not fix it; the dead instance has to go.
     */
    test('a map that failed before it was usable is released, not kept in the ref',
        async ({mount, page}) => {
            const held = new Promise<void>(() => { /* never released */ });
            await serveMapAssets(page, held);

            const component = await mount(<LocationMapHarness/>);
            await expect(component.getByTestId('live-map')).toHaveText('yes');

            await component.getByRole('button', {name: 'make the map fail'}).click();
            await expect(noteOf(component)).toHaveText(NO_BASEMAP);

            // The assertion that matters, and it has to be this one rather than
            // "a later coordinate draws": releasing the worker would let the
            // original instance finish loading and clear its own verdict, so a
            // recovery test passes whether or not the dead map was ever let go.
            // What must be true is that the ref no longer holds it, since that
            // is what every later creation attempt returns at.
            await expect(component.getByTestId('live-map')).toHaveText('no');
        });

    // And the other side of the same latch: an error once the map works is
    // ignored, because a one-way latch replaced a working map with a notice
    // saying it could not be loaded.
    test('a map that fails after it is usable keeps working', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness/>);
        await expectDrawn(component);

        await component.getByRole('button', {name: 'make the map fail'}).click();

        await expect(noteOf(component)).toHaveCount(0);
        await expect(component.getByRole('button', {name: RESET})).toBeVisible();
    });

    // A conversion still in the air is not a failure, and must not read as one.
    test('a pending position reads as loading rather than as missing', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                start='unknown'
                pending={true}
            />,
        );

        await expect(noteOf(component)).toHaveText(LOADING);
    });

    test('a position that never arrives says so rather than waiting forever', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness start='unknown'/>);

        await expect(noteOf(component)).toHaveText(NO_POSITION);
    });
});

const ZOOM_READOUT = /^z\d+\.\d$/;

/*
 * The camera's zoom, on the map.
 *
 * It is one decimal because the wheel and a trackpad pinch are continuous, so a
 * whole number would sit still through most of a gesture and read as broken.
 * Derived from the same state the overzoom notice is, so the two can never
 * disagree about which side of the data ceiling the reader is on.
 */
test.describe('the zoom readout', () => {
    test('is there before the reader touches anything, and follows them', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness start='Los Angeles'/>);
        await expectDrawn(component);

        // Seeded at construction: creating a map at a zoom fires no zoom event,
        // so without the seed this stays blank until the first gesture.
        const readout = component.getByText(ZOOM_READOUT);
        await expect(readout).toBeVisible();
        const opening = await readout.textContent();

        await component.getByRole('button', {name: 'zoom past the data'}).click();
        await expect(readout).toHaveText(`z${(DATA_MAX_ZOOM + 3).toFixed(1)}`);
        expect(await readout.textContent()).not.toBe(opening);

        await component.getByRole('button', {name: 'Reset view'}).click();
        await expect(readout).toHaveText(opening!);
    });

    test('is absent while there is no map to have a zoom', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness noWebGL={true}/>);
        await expect(noteOf(component)).toHaveText(NO_WEBGL);
        await expect(component.getByText(ZOOM_READOUT)).toBeHidden();
    });

    /*
     * An absolutely positioned box shrinks to fit only while nothing gives it a
     * width, and this one is a <p> drawn inside a Mattermost post body, whose
     * own paragraph styling does exactly that. The readout stretched the width
     * of the map on the inline surface and nowhere else, which is why the
     * hostile rule is injected here rather than waited for.
     */
    test('stays its own width under a host that stretches paragraphs', async ({mount, page}) => {
        await serveMapAssets(page);
        await page.addStyleTag({content: 'p { width: 100%; }'});

        const component = await mount(<LocationMapHarness start='Los Angeles'/>);
        await expectDrawn(component);

        const readout = await component.getByText(ZOOM_READOUT).boundingBox();
        const canvas = await component.locator('canvas').first().boundingBox();

        expect(readout!.width).toBeLessThan(canvas!.width / 2);
    });
});

/*
 * Preview mode, which is what the hover card renders.
 *
 * A card is dismissed by moving the pointer, so everything that makes the
 * panel's map operable is wrong inside one: controls too small to hit before the
 * card vanishes, and a wheel handler that would swallow a scroll over a channel.
 * What is left is the picture, which is the whole of what a glance is asking.
 */
test.describe('preview mode', () => {
    test('draws the map and none of the furniture', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                start='Los Angeles'
                preview={true}
            />);

        await expectPreviewDrawn(component);

        // The map itself is there: the pin lands, which is the only proof that
        // survives the controls being gone.
        await expect.poll(async () => {
            await readMap(component);

            return component.getByTestId('pin-features').textContent();
        }, {message: 'pin drawn'}).toBe('1');

        await expect(component.getByRole('button', {name: RESET})).toBeHidden();
        await expect(component.getByText(ZOOM_READOUT)).toBeHidden();
        await expect(component.locator('.maplibregl-ctrl-zoom-in')).toBeHidden();
        await expect(component.locator('.maplibregl-ctrl-scale')).toBeHidden();
    });

    /*
     * The card's map has a size of its own.
     *
     * The frame carries only a height in the panel, where a block element fills
     * the sidebar. Inside a tooltip that sizes itself to its content there is
     * nothing to fill, so the map came out a narrow strip, and every test above
     * passed while it did: a pin lands, labels draw and the wheel is ignored at
     * any width at all. Nothing but a measurement catches this.
     */
    test('is a map rather than a strip', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                start='Los Angeles'
                preview={true}
            />);
        await expect(component.locator('canvas').first()).toBeVisible();

        const box = await component.locator('canvas').first().boundingBox();
        expect(box, 'the map has no box at all').not.toBeNull();
        expect(box!.width).toBeGreaterThan(280);
        expect(box!.width / box!.height).toBeGreaterThan(1.4);
        expect(box!.width / box!.height).toBeLessThan(2.2);
    });

    // The wheel belongs to the channel behind the card, not to the card. A map
    // that consumed it would trap a scroll on a hover the reader did not ask for.
    test('does not take the wheel', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                start='Los Angeles'
                preview={true}
            />);

        await expectPreviewDrawn(component);

        await expect.poll(async () => {
            await readMap(component);

            return component.getByTestId('pin-features').textContent();
        }, {message: 'pin drawn'}).toBe('1');

        await readMap(component);
        const before = await component.getByTestId('zoom').textContent();

        // The handler's own state, not a duration. A fixed sleep proved this by
        // hoping 300ms outlasted any zoom a re-enabled wheel would start, which
        // an easing longer than that, or a loaded machine, defeats silently.
        expect(await component.getByTestId('wheel-zoom').textContent()).toBe('off');

        await component.locator('canvas').first().hover();
        await page.mouse.wheel(0, -600);

        // A round trip through the map, which is a real barrier: the click that
        // reads it is queued behind the wheel event the browser has to deliver.
        await readMap(component);
        await readMap(component);
        expect(await component.getByTestId('zoom').textContent()).toBe(before);
    });
});

/*
 * The OpenStreetMap credit.
 *
 * Unlike the Natural Earth credit this plugin deliberately dropped, this one is
 * a license condition: OpenStreetMap is ODbL and the OpenMapTiles schema is
 * CC-BY. It is a line beside the map rather than a MapLibre control because
 * every corner is already taken, and because a compact AttributionControl is a
 * button whose text only appears on click, which at a 300px panel width is the
 * only form that fits.
 *
 * The three cases below are the three deployments: the tier is drawn, it is not
 * shipped, and it cannot be reached at the zoom this surface uses.
 */
test.describe('the OpenStreetMap credit', () => {
    const OSM = '© OpenStreetMap contributors';
    const OMT = '© OpenMapTiles';

    test('is drawn whenever the detail tier is in the style', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness start='Los Angeles'/>);

        await expect(component.getByText(OSM)).toBeVisible();
        await expect(component.getByText(OMT)).toBeVisible();
        await expect(component.getByRole('link', {name: OSM})).
            toHaveAttribute('href', 'https://www.openstreetmap.org/copyright');
    });

    /*
     * A global-only build is a supported shipping profile, so its map is
     * today's map: no credit, because there is nothing to credit, and no note,
     * because nothing failed.
     */
    test('is absent, silently, when no detail archive is shipped', async ({mount, page}) => {
        const warnings: string[] = [];
        page.on('console', (message) => {
            if (message.type() === 'warning') {
                warnings.push(message.text());
            }
        });

        await serveMapAssets(page, undefined, false, false);

        const component = await mount(<LocationMapHarness start='Los Angeles'/>);

        await expect.poll(async () => {
            await readMap(component);

            return component.getByTestId('pin-features').textContent();
        }, {message: 'pin drawn'}).toBe('1');

        await expect(component.getByText(OSM)).toBeHidden();
        await expect(component.getByRole('button', {name: RESET})).toBeVisible();
        expect(warnings.filter((line) => line.includes('detail'))).toEqual([]);
    });

    /*
     * The hover card is clamped below the seam by zoomForSpan and cannot pan or
     * zoom, so it can never request an OpenStreetMap tile. It therefore carries
     * no detail source, and crediting one on a card with no OSM on it would be
     * the same kind of untruth as omitting it from one that has.
     */
    test('is absent from a hover card, which draws no OpenStreetMap', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                start='Los Angeles'
                preview={true}
            />);

        await expectPreviewDrawn(component);

        await expect.poll(async () => {
            await readMap(component);

            return component.getByTestId('pin-features').textContent();
        }, {message: 'pin drawn'}).toBe('1');

        await expect(component.getByText(OSM)).toBeHidden();
        await expect(component.getByText(OMT)).toBeHidden();
    });
});

/*
 * A block of events is framed clear of the map's own chrome.
 *
 * The defect: fitBounds was called with one uniform 32px padding, and every
 * corner of this map has something in it. The zoom buttons are 58px tall and
 * the Reset button, the zoom readout and the scale bar all sit within 30px of
 * their edges, so markers near the bounds of a spread opened underneath them.
 *
 * Asserted by PROJECTING each marker onto the canvas and checking it against
 * the rectangles the chrome occupies, rather than by asserting a zoom number.
 * A zoom assertion would pass for a padding that zoomed out and still put a
 * marker under the scale bar, which is the actual complaint.
 */
test.describe('framing a block of events', () => {
    /*
     * Mounted at the width the RHS panel actually gives the map.
     *
     * Left to fill the test page this canvas is about 1264px wide, where the
     * four corners are so far apart that nothing can reach the chrome and this
     * whole suite passes against any padding at all. The bug being covered is a
     * bug about a small canvas, so the canvas has to be small.
     */
    const PANEL_WIDTH_PX = 360;

    // Spread far enough apart that the fit, not the opening span, decides the
    // camera: Hickam, Hilo and Wheeler are the three the examples use.
    const BLOCK = [
        {lat: 21.3353, lon: -157.9483, color: '#c0392b'},
        {lat: 19.7297, lon: -155.0900, color: '#2e86c1'},
        {lat: 21.4836, lon: -158.0386, color: '#2e86c1'},
    ];

    /*
     * What each control covers, measured from the edge it is anchored to.
     *
     * MapLibre's own controls carry a 10px margin: the zoom buttons are 29
     * wide and 58 tall, the scale bar is up to 90 wide and about 18 tall. The
     * Reset button and the zoom readout are this component's, at 8px in.
     */
    const CHROME = [
        {name: 'Reset view', left: 0, top: 0, width: 78, height: 32},
        {name: 'zoom buttons', right: 0, top: 0, width: 39, height: 68},
        {name: 'zoom readout', left: 0, bottom: 0, width: 63, height: 30},
        {name: 'scale bar', right: 0, bottom: 0, width: 100, height: 28},
    ];

    /*
     * The MARKER, not the point it is centered on.
     *
     * A crosshair is MARKER_SIZE across and drawn centered, so a marker whose
     * center clears the scale bar by 4px still has its bottom third under it.
     * Checking the center alone passed against the uniform padding this test
     * exists to catch, which is the whole reason it is written this way.
     */
    function covers(box: typeof CHROME[number], x: number, y: number, w: number, h: number): boolean {
        const half = MARKER_SIZE / 2;

        const x0 = box.left === undefined ? w - box.width : 0;
        const x1 = box.left === undefined ? w : box.width;
        const y0 = box.top === undefined ? h - box.height : 0;
        const y1 = box.top === undefined ? h : box.height;

        return x + half >= x0 && x - half <= x1 && y + half >= y0 && y - half <= y1;
    }

    test('no marker opens underneath a control', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <div style={{width: PANEL_WIDTH_PX}}><LocationMapHarness markers={BLOCK}/></div>,
        );
        await expectDrawn(component);
        await readMap(component);

        const raw = await component.getByTestId('pins').textContent();
        expect(raw, 'the harness reported no projection').toBeTruthy();

        const {w, h, at} = JSON.parse(raw!) as {w: number; h: number; at: number[][]};
        expect(at).toHaveLength(BLOCK.length);

        for (const [x, y] of at) {
            // On the canvas at all, which a fit that overshot would fail.
            expect(x, `marker at ${x},${y} is off a ${w}x${h} canvas`).toBeGreaterThanOrEqual(0);
            expect(x).toBeLessThanOrEqual(w);
            expect(y).toBeGreaterThanOrEqual(0);
            expect(y).toBeLessThanOrEqual(h);

            for (const box of CHROME) {
                expect(
                    covers(box, x, y, w, h),
                    `marker at ${x},${y} is under the ${box.name} on a ${w}x${h} canvas`,
                ).toBe(false);
            }
        }
    });

    /*
     * The whole set, not just the first. A fit that framed one event and left
     * the others off screen would satisfy the clearance check above by having
     * nothing left to check.
     */
    test('every event is inside the frame, not just the first', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <div style={{width: PANEL_WIDTH_PX}}><LocationMapHarness markers={BLOCK}/></div>,
        );
        await expectDrawn(component);
        await readMap(component);

        const raw = await component.getByTestId('pins').textContent();
        expect(raw, 'the harness reported no projection').toBeTruthy();

        const {w, h, at} = JSON.parse(raw!) as {w: number; h: number; at: number[][]};
        expect(at).toHaveLength(BLOCK.length);

        // ON the canvas, first. Without this the test passed for a camera that
        // never fitted at all: markers left off screen have LARGER spread than
        // framed ones, so the spread check below was satisfied by the very
        // failure its own name promises to catch.
        for (const [x, y] of at) {
            expect(x, `marker at ${x},${y} is off a ${w}x${h} canvas`).toBeGreaterThanOrEqual(0);
            expect(x).toBeLessThanOrEqual(w);
            expect(y).toBeGreaterThanOrEqual(0);
            expect(y).toBeLessThanOrEqual(h);
        }

        const xs = at.map(([x]) => x);
        const ys = at.map(([, y]) => y);

        // And spread across it rather than stacked, which is what says the
        // camera fitted the block instead of zooming out to the world.
        expect(Math.max(...xs) - Math.min(...xs)).toBeGreaterThan(20);
        expect(Math.max(...ys) - Math.min(...ys)).toBeGreaterThan(20);
    });
});

/*
 * The accessible label agrees with how many markers there are.
 *
 * A block used to be announced as "World map with the position marked. The
 * marker is 3 events." Three things wrong at once: singular grammar for N
 * markers, a sentence that is not a description of a marker, and no affiliation
 * anywhere. That last one is the defect that matters, because color is the
 * whole of what tells one marker from another on this map, so a reader who gets
 * no color got a count and nothing else.
 */
test.describe('the accessible label for a block', () => {
    const TWO = [
        {lat: 21.3353, lon: -157.9483, color: '#c0392b'},
        {lat: 19.7297, lon: -155.0900, color: '#3d85c6'},
    ];

    async function labelOf(component: Locator): Promise<string> {
        return await component.locator('span').filter({hasText: 'World map'}).first().innerText();
    }

    test('is plural, and carries what the markers are', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                markers={TWO}
                markerLabel='1 hostile and 1 friendly'
            />,
        );
        await expectDrawn(component);

        const label = await labelOf(component);

        expect(label).toContain('2 positions marked');
        expect(label).toContain('The markers are 1 hostile and 1 friendly');

        // The singular forms are the bug, so they are asserted absent rather
        // than left to the phrasing above.
        expect(label).not.toContain('the position marked');
        expect(label).not.toContain('The marker is');
    });

    test('stays singular for one position', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness markerLabel='Hostile Armor Unit'/>);
        await expectDrawn(component);

        const label = await labelOf(component);

        expect(label).toContain('the position marked');
        expect(label).toContain('The marker is Hostile Armor Unit');
        expect(label).not.toContain('positions marked');
    });
});

test.describe('the color a shape is drawn in', () => {
    const SQUARE: MapGeometry = {
        kind: 'outline',
        points: [
            {lat: 34.05, lon: -118.25},
            {lat: 34.06, lon: -118.25},
            {lat: 34.06, lon: -118.24},
            {lat: 34.05, lon: -118.24},
        ],
        closed: true,
    };

    test('is the stated one, at the fill alpha the theme uses', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                geometry={SQUARE}
                geometryColor='#ff0000'
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        // Read off the FEATURE, not the layer: the paint is an expression now,
        // so the layer reads the same whatever the author supplied.
        await expect(component.getByTestId('shape-style')).
            toHaveText('#ff0000|rgba(255, 0, 0, 0.16)');
    });

    test('falls back to the theme when the source states nothing', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness geometry={SQUARE}/>);
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        // No color on the feature at all, so the expression's fallback paints
        // it in the theme's own color.
        await expect(component.getByTestId('shape-style')).toHaveText('|');
    });

    /*
     * An ELLIPSE carries its stated color too.
     *
     * When the paint became a data-driven expression, ellipseFeature still
     * emitted empty properties, so a Cursor on Target drawn circle silently
     * lost the color it states. The outline tests above could not see it.
     */
    test('an ellipse carries its stated color, not just an outline', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                geometry={{kind: 'ellipse', major: 4000, minor: 2000, angle: 0}}
                geometryColor='#ff0000'
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        await expect(component.getByTestId('shape-style')).
            toHaveText('#ff0000|rgba(255, 0, 0, 0.16)');
    });

    test('falls back to the theme for a color that is not a hex triple', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                geometry={SQUARE}
                geometryColor='url(https://attacker.example/px)'
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        // The gate: a value that is not a hex triple reaches the feature as no
        // color at all, so the expression falls back to the theme and nothing
        // an author wrote is ever handed to MapLibre.
        await expect(component.getByTestId('shape-style')).toHaveText('|');
        await expect(component.getByTestId('shape-style')).
            not.toContainText('attacker.example');
    });
});

/*
 * Several shapes at once, which is what a GeoJSON document draws.
 *
 * The Cursor on Target card draws at most one outline on purpose, so every
 * assertion here is about behavior nothing else exercises.
 */
test.describe('several shapes at once', () => {
    const SQUARE_RINGS = [[
        {lat: 34.00, lon: -118.30},
        {lat: 34.00, lon: -118.10},
        {lat: 34.20, lon: -118.10},
        {lat: 34.20, lon: -118.30},
    ]];

    const HOLE = [
        {lat: 34.05, lon: -118.25},
        {lat: 34.05, lon: -118.20},
        {lat: 34.10, lon: -118.20},
    ];

    /*
     * The whole reason the plural prop carries rings.
     *
     * Two single-ring polygons and one two-ring polygon are indistinguishable
     * in the DOM, and the difference is whether the hole is cut out of the fill
     * or painted over it as a solid island.
     */
    test('a holed polygon is one polygon of two rings, not two polygons', async ({mount, page}) => {
            await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness geometries={[{rings: [SQUARE_RINGS[0], HOLE], closed: true}]}/>,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        await expect(component.getByTestId('shapes')).toHaveText('Polygon:2');
    });

    test('each shape stays its own feature', async ({mount, page}) => {
            await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                geometries={[
                    {rings: SQUARE_RINGS, closed: true},
                    {rings: [HOLE], closed: false},
                ]}
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        await expect(component.getByTestId('shapes')).toHaveText('Polygon:1|LineString:1');
    });

    test('an open shape is a line rather than a closed polygon', async ({mount, page}) => {
            await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness geometries={[{rings: [HOLE], closed: false}]}/>,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        await expect(component.getByTestId('shapes')).toHaveText('LineString:1');
    });
});

/*
 * A surface with no position of its own.
 *
 * `lat`/`lon` of null already means "no mappable position" and renders a note
 * over the frame, so extent-only is a separate prop rather than an overload of
 * it. Every assertion here is about that separation holding.
 */
test.describe('extent-only', () => {
    const AREA = [{
        rings: [[
            {lat: 34.00, lon: -118.30},
            {lat: 34.00, lon: -118.10},
            {lat: 34.20, lon: -118.10},
            {lat: 34.20, lon: -118.30},
        ]],
        closed: true,
    }];

    test('draws the overlay with no position at all', async ({mount, page}) => {
            await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                start='unknown'
                extentOnly={true}
                extentLabel='3 drawn shapes'
                geometries={AREA}
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        await expect(component.getByTestId('shapes')).toHaveText('Polygon:1');
    });

    // The invariant this whole prop exists for: no pin at a position nobody
    // stated. Without it drawableMarkers falls back to a pin at lat/lon, which
    // for a positionless document is 0,0 in the Gulf of Guinea.
    test('draws no pin, because there is no position to pin', async ({mount, page}) => {
            await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                start='unknown'
                extentOnly={true}
                extentLabel='3 drawn shapes'
                geometries={AREA}
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        await expect(component.getByTestId('pin-features')).toHaveText('0');
    });

    test('says what the overlay is rather than claiming a marked position', async ({mount, page}) => {
            await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                start='unknown'
                extentOnly={true}
                extentLabel='3 drawn shapes'
                geometries={AREA}
            />,
        );
        await expectDrawn(component);

        const label = component.getByText(/World map/);
        await expect(label).toContainText('3 drawn shapes');
        await expect(label).not.toContainText('position marked');
    });

    // Null lat/lon without the prop still means what it always meant. This is
    // the regression the separate prop exists to avoid.
    test('an unknown position without the prop still reads as unavailable', async ({mount, page}) => {
            await serveMapAssets(page);

        const component = await mount(<LocationMapHarness start='unknown'/>);

        await expect(component.getByText('The position for this coordinate is unavailable.').first()).
            toBeVisible();
    });
});

/*
 * A box with no width or no height. fitBounds takes those to maxZoom, which is
 * a street-level view of something that may be a country wide.
 */
test.describe('degenerate extents', () => {
    const cases = [
        {name: 'a single point', markers: [{lat: 34.0561, lon: -118.25, color: '#3f7fbf'}], geometries: undefined},
        {
            name: 'a due-north line',
            markers: undefined,
            geometries: [{rings: [[{lat: 34.0, lon: -118.25}, {lat: 34.5, lon: -118.25}]], closed: false}],
        },
        {
            name: 'a zero-area polygon',
            markers: undefined,
            geometries: [{rings: [[
                {lat: 34.0, lon: -118.25},
                {lat: 34.0, lon: -118.25},
                {lat: 34.0, lon: -118.25},
            ]],
closed: true}],
        },
    ];

    for (const one of cases) {
        test(`${one.name} opens at a readable zoom rather than at the maximum`, async ({mount, page}) => {
            await serveMapAssets(page);

            const component = await mount(
                <LocationMapHarness
                    start='unknown'
                    extentOnly={true}
                    extentLabel='an overlay'
                    markers={one.markers}
                    geometries={one.geometries}
                />,
            );
            await expectDrawn(component);
            await component.getByRole('button', {name: 'read the map'}).click();

            const zoom = Number(await component.getByTestId('zoom').textContent());
            expect(zoom).toBeLessThan(MAX_ZOOM);
        });
    }
});

/*
 * The antimeridian, which is the case the per-shape unwrap could not see.
 *
 * Each shape unwraps to itself, so two either side of the seam union to a 359
 * degree box, which slips under spansTheWorld's 360 test. The camera then frames
 * the planet the wrong way round with both features at the edges.
 */
test.describe('the antimeridian', () => {
    test('two shapes either side of the seam frame the seam, not the planet', async ({mount, page}) => {
            await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                start='unknown'
                extentOnly={true}
                extentLabel='two shapes'
                geometries={[
                    {rings: [[{lat: 0, lon: 179.0}, {lat: 1, lon: 179.5}]], closed: false},
                    {rings: [[{lat: 0, lon: -179.5}, {lat: 1, lon: -179.0}]], closed: false},
                ]}
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        // The center is what says which way round the camera went. Framing the
        // seam puts it near 180; framing the planet the other way puts it near
        // zero.
        const center = await component.getByTestId('camera').textContent();
        const lon = Math.abs(Number((center ?? '0,0').split(',')[1]));
        expect(lon).toBeGreaterThan(150);
    });

    // The single-shape case the per-shape unwrap already handled, so that
    // moving the unwrap up does not regress it.
    test('one shape crossing the seam still frames the seam', async ({mount, page}) => {
            await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                start='unknown'
                extentOnly={true}
                extentLabel='one shape'
                geometries={[
                    {rings: [[{lat: 0, lon: 179.0}, {lat: 1, lon: -179.0}]], closed: false},
                ]}
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        const center = await component.getByTestId('camera').textContent();
        const lon = Math.abs(Number((center ?? '0,0').split(',')[1]));
        expect(lon).toBeGreaterThan(150);
    });
});

/*
 * A map that is constructed and then never becomes ready.
 *
 * MapLibre tiles in a worker, so a worker that never arrives leaves every
 * source unfinished and `load` never fires. That is not an error, so the error
 * handler never runs either, and the map sat on "Loading map…" forever with a
 * reload the only way out. It is the failure a worker URL that 404s produces in
 * the field.
 */
test.describe('a map that never becomes ready', () => {
    test('says so rather than waiting forever', async ({mount, page}) => {
        // Held open for the life of the test: the worker never arrives, which
        // is the only way to hold a map between construction and load.
        await serveMapAssets(page, new Promise<void>(() => { /* never resolves */ }));

        const component = await mount(<LocationMapHarness readyDeadlineMs={2500}/>);

        await expect(component.getByText('Loading map…').first()).toBeVisible();
        await expect(component.getByText('The map could not be loaded.').first()).toBeVisible({timeout: 15_000});
    });

    // The deadline must not fire over a map that came up normally, or every
    // working map would blank itself twenty seconds in.
    test('a map that loads normally is never failed by the deadline', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationMapHarness readyDeadlineMs={2500}/>);
        await expectDrawn(component);

        await page.waitForTimeout(4000);

        await expect(component.getByText('The map could not be loaded.')).toHaveCount(0);
        await expect(component.getByRole('button', {name: RESET})).toBeVisible();
    });

    // Releasing the context matters as much as reporting: a map left in the ref
    // makes every later attempt return at the creation guard, and holds a WebGL
    // context for something that draws nothing.
    test('releases the map it gave up on', async ({mount, page}) => {
        await serveMapAssets(page, new Promise<void>(() => { /* never resolves */ }));

        const component = await mount(<LocationMapHarness readyDeadlineMs={2500}/>);
        await expect(component.getByText('The map could not be loaded.').first()).toBeVisible({timeout: 15_000});
        await component.getByRole('button', {name: 'read the map'}).click();

        // The observer is handed null when the map is released, so the harness
        // has nothing left to read a camera off. That is the release: a map
        // kept in the ref would make map.current truthy forever and every later
        // attempt would return at the creation guard.
        await expect(component.getByTestId('camera')).toHaveText('none');
    });
});

/*
 * The rest of the simplestyle a document may state.
 *
 * Colors were the whole of it once. Width, the two opacities and marker-size
 * are read now, and each has to survive the second gate rather than be trusted
 * because Go already checked: a props blob is not a trusted input either.
 *
 * Every assertion reads the FEATURE, never the layer, for the reason the color
 * tests record: these are data-driven expressions and the layer reads the same
 * whatever the document said.
 */
test.describe('the rest of a stated style', () => {
    const RING = [[
        {lat: 34.00, lon: -118.30},
        {lat: 34.00, lon: -118.10},
        {lat: 34.20, lon: -118.10},
        {lat: 34.00, lon: -118.30},
    ]];

    test('a width and a line opacity reach the shape', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                geometries={[{rings: RING, closed: true, color: '#ff0000', width: '3', lineOpacity: '0.8'}]}
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        await expect(component.getByTestId('shape-stroke')).toHaveText('3|0.8');
    });

    // A quarter, not a quarter of the 0.16 an unstyled shape composites at.
    test('a stated fill opacity replaces the theme alpha rather than multiplying it', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                geometries={[{rings: RING, closed: true, color: '#ff0000', fillOpacity: '0.25'}]}
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        await expect(component.getByTestId('shape-style')).
            toHaveText('#ff0000|rgba(255, 0, 0, 0.25)');
    });

    test('a shape that states none is drawn at the theme alpha', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness geometries={[{rings: RING, closed: true, color: '#ff0000'}]}/>,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        await expect(component.getByTestId('shape-style')).
            toHaveText('#ff0000|rgba(255, 0, 0, 0.16)');
        await expect(component.getByTestId('shape-stroke')).toHaveText('|');
    });

    /*
     * The second gate, on values Go would already have refused.
     *
     * A width of 4000 paints the whole map solid with no way to tell it from a
     * rendering fault, and Number('') is 0 while Number('1e999') is Infinity,
     * so an unguarded parse turns an empty string into a real width.
     */
    for (const [name, style] of [
        ['a width past what this build draws', {width: '4000'}],
        ['a width that is not a number', {width: 'wide'}],
        ['an infinite width', {width: '1e999'}],
        ['a negative width', {width: '-3'}],
        ['an opacity above one', {lineOpacity: '1.5'}],
        ['an opacity that is not a number', {lineOpacity: 'solid'}],
        ['an empty width, which Number() reads as 0', {width: ''}],
        ['a width of zero', {width: '0'}],
    ] as Array<[string, Partial<MapShape>]>) {
        test(`falls back to the theme for ${name}`, async ({mount, page}) => {
            await serveMapAssets(page);

            const component = await mount(
                <LocationMapHarness geometries={[{rings: RING, closed: true, color: '#ff0000', ...style}]}/>,
            );
            await expectDrawn(component);
            await component.getByRole('button', {name: 'read the map'}).click();

            await expect(component.getByTestId('shape-stroke')).toHaveText('|');
        });
    }

    test('marker-size scales the reticle, and an unknown size does not', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                markers={[
                    {lat: 34.05, lon: -118.25, color: '#ff0000', size: 'large'},
                    {lat: 34.06, lon: -118.24, color: '#ff0000', size: 'small'},
                    {lat: 34.07, lon: -118.23, color: '#ff0000'},
                    {lat: 34.08, lon: -118.22, color: '#ff0000', size: 'enormous'},
                ]}
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        await expect(component.getByTestId('marker-scales')).toHaveText('1.5,0.7,,');
    });
});

/*
 * An open shape is a LINE, and a line is not filled.
 *
 * A MapLibre fill layer does not ignore a LineString. It closes the ring and
 * fills it, so a route drew a translucent wash between its first and last point
 * with the map underneath it showing through. The source carried exactly the
 * right geometry throughout; the defect was that the layer drew both kinds.
 *
 * It hid for as long as every line took the theme's own faint fill, and became
 * obvious the moment a document could state a saturated color of its own.
 */
test.describe('what the fill layer draws', () => {
    const OPEN = [[
        {lat: 21.3315, lon: -157.9513},
        {lat: 21.3435, lon: -157.9337},
        {lat: 21.3670, lon: -157.9074},
    ]];

    const CLOSED = [[
        {lat: 21.330, lon: -157.960},
        {lat: 21.330, lon: -157.900},
        {lat: 21.370, lon: -157.900},
        {lat: 21.330, lon: -157.960},
    ]];

    /*
     * Drawn WITH a closed shape, which is the positive control.
     *
     * queryRenderedFeatures answers [] until the worker has parsed the source
     * and a render pass has run, and 0 is also what a renamed layer reports. So
     * asserting 0 on an open shape alone passed before anything had painted and
     * would have passed against the missing filter this test exists for. The
     * mixed document below is the one that can tell those apart, and this one
     * polls until the count settles rather than reading once.
     */
    test('an open shape is not filled', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness geometries={[{rings: OPEN, closed: false, color: '#0000ff'}]}/>,
        );
        await expectDrawn(component);

        await expect(async () => {
            await component.getByRole('button', {name: 'read the map'}).click();
            await expect(component.getByTestId('tiles')).toHaveText('loaded');
            await expect(component.getByTestId('filled-shapes')).toHaveText('0');
        }).toPass();
    });

    // The other half, so the fix cannot be "fill nothing at all".
    test('a closed shape still is', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness geometries={[{rings: CLOSED, closed: true, color: '#ff0000'}]}/>,
        );
        await expectDrawn(component);

        // toHaveText('1'), not not.toHaveText('0'): the harness reports -1 for
        // a dead map and -2 for a missing layer, and both satisfy a negation.
        await expect(async () => {
            await component.getByRole('button', {name: 'read the map'}).click();
            await expect(component.getByTestId('filled-shapes')).toHaveText('1');
        }).toPass();
    });

    test('a document of both fills only the area', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                geometries={[
                    {rings: CLOSED, closed: true, color: '#ff0000', fillOpacity: '0.25'},
                    {rings: OPEN, closed: false, color: '#0000ff', width: '3'},
                ]}
            />,
        );
        await expectDrawn(component);

        await expect(async () => {
            await component.getByRole('button', {name: 'read the map'}).click();
            await expect(component.getByTestId('filled-shapes')).toHaveText('1');
        }).toPass();
    });
});

/*
 * The paint properties themselves, which nothing asserted.
 *
 * Every other style test reads the FEATURE, which is right for proving the gate
 * wrote a validated value. But it means `paintGeometry` could stop wiring
 * line-width and line-opacity to the layer entirely and every one of them would
 * stay green while no stated width or opacity was ever drawn.
 */
test.describe('the stroke paint is wired to the layer', () => {
    test('reads the width and the opacity off the feature, with a fallback', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationMapHarness
                geometries={[{
                    rings: [[{lat: 34.00, lon: -118.30}, {lat: 34.20, lon: -118.10}]],
                    closed: false,
                    color: '#ff0000',
                }]}
            />,
        );
        await expectDrawn(component);
        await component.getByRole('button', {name: 'read the map'}).click();

        const paint = component.getByTestId('stroke-paint');

        await expect(paint).toContainText('width');
        await expect(paint).toContainText('lineOpacity');
    });
});

/*
 * A stated fill opacity is the second gate on the alpha, and it had no test.
 *
 * Swap numberWithin for a bare Number() and `fill-opacity: "wide"` composites
 * to rgba(255, 0, 0, NaN), which is what the gate exists to stop.
 */
test.describe('a fill opacity this build will not draw', () => {
    for (const [name, fillOpacity] of [
        ['above one', '1.5'],
        ['negative', '-0.2'],
        ['not a number', 'solid'],
        ['empty', ''],
    ] as Array<[string, string]>) {
        test(`falls back to the theme alpha when it is ${name}`, async ({mount, page}) => {
            await serveMapAssets(page);

            const component = await mount(
                <LocationMapHarness
                    geometries={[{
                        rings: [[
                            {lat: 34.00, lon: -118.30},
                            {lat: 34.00, lon: -118.10},
                            {lat: 34.20, lon: -118.10},
                            {lat: 34.00, lon: -118.30},
                        ]],
                        closed: true,
                        color: '#ff0000',
                        fillOpacity,
                    }]}
                />,
            );
            await expectDrawn(component);
            await component.getByRole('button', {name: 'read the map'}).click();

            await expect(component.getByTestId('shape-style')).
                toHaveText('#ff0000|rgba(255, 0, 0, 0.16)');
        });
    }
});

/*
 * marker-size on the EXTENT-ONLY map, which is the only map GeoJSON draws.
 *
 * applyView had two write paths: the positioned one called drawableMarkers and
 * the extent-only one hand-rolled its own markedPoints call that omitted the
 * scale. GeoJsonMapCanvas always passes lat={null} extentOnly, so a stated
 * marker-size was validated, carried the whole way, and dropped at the last hop
 * on the one surface it was added for.
 *
 * The existing marker-size test mounts a positioned harness, so it exercised
 * the branch that already worked and passed against this.
 */
test('marker-size survives the extent-only write path', async ({mount, page}) => {
    await serveMapAssets(page);

    const component = await mount(
        <LocationMapHarness
            start='unknown'
            extentOnly={true}
            markers={[
                {lat: 34.05, lon: -118.25, color: '#ff0000', size: 'large'},
                {lat: 34.06, lon: -118.24, color: '#ff0000', size: 'small'},
                {lat: 34.07, lon: -118.23, color: '#ff0000'},
            ]}
        />,
    );
    await expectDrawn(component);
    await component.getByRole('button', {name: 'read the map'}).click();

    await expect(component.getByTestId('marker-scales')).toHaveText('1.5,0.7,');
});

// A props blob is not trusted input, and this lookup used to walk the prototype
// chain: 'constructor' returned a function where the type promised a number.
test('a marker size from the prototype chain is not a size', async ({mount, page}) => {
    await serveMapAssets(page);

    const component = await mount(
        <LocationMapHarness
            start='unknown'
            extentOnly={true}
            markers={[
                {lat: 34.05, lon: -118.25, color: '#ff0000', size: 'constructor'},
                {lat: 34.06, lon: -118.24, color: '#ff0000', size: '__proto__'},
                {lat: 34.07, lon: -118.23, color: '#ff0000', size: 'toString'},
            ]}
        />,
    );
    await expectDrawn(component);
    await component.getByRole('button', {name: 'read the map'}).click();

    await expect(component.getByTestId('marker-scales')).toHaveText(',,');
});
