import type {Locator} from '@playwright/test';
import React from 'react';

import {serveMapAssets} from './asset_fixtures';
import LocationMapHarness from './LocationMapHarness';
import type {ViewName} from './LocationMapHarness';
import {DATA_MAX_ZOOM} from './span';

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
    // these surfaces zoom, so a metre-wide cell is invisible until the reader
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
    // request reaches it" a defence rather than an accident. React escapes a
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
    // which is why the control resets the zoom as well as the centre.
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

            const drawn = Number(await component.getByTestId('land-at-centre').textContent());
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

        await expect.poll(async () => {
            await readMap(component);

            return component.getByTestId('pin-features').textContent();
        }).toBe('1');

        await readMap(component);
        const before = await component.getByTestId('zoom').textContent();

        await component.locator('canvas').first().hover();
        await page.mouse.wheel(0, -600);
        await page.waitForTimeout(300);

        await readMap(component);
        expect(await component.getByTestId('zoom').textContent()).toBe(before);
    });
});
