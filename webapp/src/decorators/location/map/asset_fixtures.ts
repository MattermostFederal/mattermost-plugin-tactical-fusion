import fs from 'fs';
import path from 'path';

import type {Page} from '@playwright/test';

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
 * honour the `?copy` marker, so `assetUrl` returns null and MapLibre falls back
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
): Promise<void> {
    // PMTiles reads by byte range, so this has to answer 206 with the requested
    // slice. route.fulfill({path}) always returns the whole file with a 200,
    // which leaves the reader parsing the header as if it were a tile.
    await page.route('**/public/map/world.pmtiles*', (route) => {
        const range = (/bytes=(\d+)-(\d+)/).exec(route.request().headers().range ?? '');
        if (!range) {
            return route.fulfill({
                status: 200,
                contentType: 'application/octet-stream',
                body: ARCHIVE,
            });
        }

        const start = Number(range[1]);
        const end = Math.min(Number(range[2]), ARCHIVE.length - 1);

        return route.fulfill({
            status: 206,
            contentType: 'application/octet-stream',
            headers: {'content-range': `bytes ${start}-${end}/${ARCHIVE.length}`},
            body: ARCHIVE.subarray(start, end + 1),
        });
    });

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
