import fs from 'fs';
import path from 'path';

import type {Page, Route} from '@playwright/test';

/*
 * The routes a real map needs, shared by every suite that draws one.
 *
 * Every other suite that reaches LocationMap leaves the basemap unanswered, so
 * MapLibre was never constructed under test: the creation path, the camera and
 * both overlay sources were dead. The unlock is these routes. The committed
 * archive is served rather than a fixture, so the header probe in basemap.ts
 * runs against the bytes this build actually ships.
 *
 * Once the archive, the worker and the glyphs are routed, the map fetches
 * nothing else, which is what makes a suite using this deterministic rather than
 * a network test.
 *
 * It lives here rather than inline in one suite because the hover card needs the
 * identical routes, and two copies of this are two things that can disagree
 * about how the archive is served.
 */

const BASEMAP = path.resolve(__dirname, '../../../../../public/map/world.pmtiles');
const ARCHIVE = fs.readFileSync(BASEMAP);
export const PILOT_PACKAGE = 'indopacom-hawaii';
const DETAIL = path.resolve(__dirname, `../../../../../public/map/packages/${PILOT_PACKAGE}.pmtiles`);

/*
 * Read once, like the basemap above, and not once per request.
 *
 * PMTiles reads by byte range, so a single map asks the detail archive for
 * several slices, and this used to be a `readFileSync` inside the route handler:
 * every slice re-read all seven megabytes, synchronously, on the Node event loop
 * that Playwright serves those very responses from. Across a suite of forty map
 * tests that is most of a gigabyte of blocking disk reads whose only effect was
 * to make the browser wait for a map it had already asked for.
 *
 * Lazy, because a suite that never draws a detail tier should not pay for it at
 * import.
 */
let detailArchive: Buffer | null = null;

function detailBytes(): Buffer {
    detailArchive = detailArchive ?? fs.readFileSync(DETAIL);
    return detailArchive;
}
const FONTS = path.resolve(__dirname, '../../../../../public/map/fonts/NotoSans-Regular');

/*
 * Resolved through the package rather than composed from a path, so a hoisted
 * or differently-laid-out install fails at collection with a module-not-found
 * rather than as a run of unexplained timeouts.
 */
const WORKER = require.resolve('maplibre-gl/dist/maplibre-gl-worker.mjs');
const SHARED = require.resolve('maplibre-gl/dist/maplibre-gl-shared.mjs');

/**
 * Serves the basemap, and the worker MapLibre asks for.
 *
 * The worker is a property of the test bundler rather than of this component.
 * The shipping build imports it as a webpack asset and hands the emitted URL to
 * `setWorkerUrl`; Playwright's component runner builds with Vite, which does not
 * honor the `?copy` marker, so `assetUrl` returns null and MapLibre falls back
 * to deriving `./maplibre-gl-worker.mjs` from its own `import.meta.url`. Vite
 * emits that chunk under a hashed name, so the request fails, the GeoJSON source
 * never finishes tiling, `load` never fires, and the map sits on "Loading map…"
 * forever. Pointing both names at the package's own dist files puts the runner
 * back in the position the real build is in.
 *
 * The cost, stated because tests using this should not be read as covering it:
 * `setWorkerUrl` itself (`maplibre.ts`) is never executed here, so a regression
 * in the shipped worker wiring is not catchable through this route.
 *
 * `holdWorker` keeps the worker request open, which is the only way to hold a
 * map between construction and `load`, which is the window the readiness guard
 * exists for. `starveFonts` 404s the glyph ranges instead, which is how a label test
 * tells apart what the fonts are actually responsible for.
 */
export async function serveMapAssets(
    page: Page,
    holdWorker?: Promise<void>,
    starveFonts = false,
    serveDetail = true,
): Promise<void> {
    // The detail tier is optional, so both answers are real deployments and a
    // suite has to be able to ask for either. Absent means a global-only build,
    // which must render as today's map with nothing said about it.
    // The panel learns which areas exist from the API; the pages are handed the
    // same list in their shell. Without this route the store answers "none" and
    // every detail assertion below would be testing a global-only install.
    await page.route('**/api/v1/packages', (route) => route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({packages: serveDetail ? [PILOT_PACKAGE] : []}),
    }));

    await page.route('**/packages/*.pmtiles*', (route) => {
        if (!serveDetail) {
            return route.fulfill({status: 404});
        }

        return rangeReply(route, detailBytes());
    });

    // PMTiles reads by byte range, so this has to answer 206 with the requested
    // slice. route.fulfill({path}) always returns the whole file with a 200,
    // which leaves the reader parsing the header as if it were a tile.
    await page.route('**/public/map/world.pmtiles*', (route) => rangeReply(route, ARCHIVE));

    if (starveFonts) {
        await page.route('**/public/map/fonts/**', (route) => route.fulfill({status: 404}));
    } else {
        await page.route('**/public/map/fonts/**', (route) => route.fulfill({
            path: path.resolve(FONTS, path.basename(new URL(route.request().url()).pathname)),
            contentType: 'application/x-protobuf',
        }));
    }

    await page.route('**/maplibre-gl-shared.mjs', (route) => route.fulfill({
        path: SHARED,
        contentType: 'text/javascript',
    }));

    await page.route('**/maplibre-gl-worker.mjs', async (route) => {
        if (holdWorker) {
            await holdWorker;
        }
        await route.fulfill({path: WORKER, contentType: 'text/javascript'});
    });
}

/**
 * A PMTiles archive served the way a real one is.
 *
 * PMTiles reads by byte range, so this has to answer 206 with the requested
 * slice. `route.fulfill({path})` always returns the whole file with a 200,
 * which leaves the reader parsing the header as if it were a tile.
 */
function rangeReply(route: Route, archive: Buffer): Promise<void> {
    const range = (/bytes=(\d+)-(\d+)/).exec(route.request().headers().range ?? '');
    if (!range) {
        return route.fulfill({
            status: 200,
            contentType: 'application/octet-stream',
            body: archive,
        });
    }

    const start = Number(range[1]);
    const end = Math.min(Number(range[2]), archive.length - 1);

    return route.fulfill({
        status: 206,
        contentType: 'application/octet-stream',
        headers: {'content-range': `bytes ${start}-${end}/${archive.length}`},
        body: archive.subarray(start, end + 1),
    });
}
