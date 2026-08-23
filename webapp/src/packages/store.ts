import {pluginBaseUrl} from '../plugin_url';

/**
 * Which detail map areas this install has.
 *
 * Module state, so a channel full of coordinates makes one request rather than
 * one per hover, the same shape `features/` and `preferences/` take.
 *
 * The list is NOT immutable the way an archive is: an operator drops a file
 * into a directory and the area appears, with no restart and nothing to
 * invalidate a cache. So this has a lifetime, and it is short: a minute rather
 * than the half hour the preferences and features caches use, because the thing
 * it is waiting for is somebody watching to see whether the file they just
 * copied worked.
 */
const CACHE_TTL_MS = 60 * 1000;

/** The same bound every other fetch in this plugin puts on itself. */
const FETCH_TIMEOUT_MS = 10000;

const NO_PACKAGES: readonly string[] = [];

let cached: readonly string[] = NO_PACKAGES;
let loadedAt: number | null = null;
let inflight: Promise<readonly string[]> | null = null;

/*
 * A page has no session and `/api/v1/packages` requires one, so the pages are
 * handed the list in their own shell and seed it here instead of fetching.
 *
 * Seeding stamps the clock, which is what stops a page making a request it
 * would only ever get a 401 from.
 */
export function seedPackages(names: readonly string[]): void {
    cached = names;
    loadedAt = Date.now();
}

export async function loadPackageNames(): Promise<readonly string[]> {
    if (loadedAt !== null && Date.now() - loadedAt < CACHE_TTL_MS) {
        return cached;
    }
    if (inflight) {
        return inflight;
    }

    const attempt = fetchPackages().then((names) => {
        if (names === null) {
            // A failed read keeps whatever was last known good and does NOT
            // stamp the clock, so the next mount retries. Falling back to the
            // empty list would take every area off the map on one bad minute,
            // which is the same failure `features/` documents and is worse
            // here: an empty list is indistinguishable from a global-only
            // install.
            return cached;
        }

        cached = names;
        loadedAt = Date.now();

        return cached;
    });

    inflight = attempt;
    attempt.finally(() => {
        if (inflight === attempt) {
            inflight = null;
        }
    }).catch(() => {
        // fetchPackages never rejects.
    });

    return attempt;
}

async function fetchPackages(): Promise<readonly string[] | null> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);

    try {
        const response = await fetch(`${pluginBaseUrl()}/api/v1/packages`, {
            credentials: 'same-origin',
            headers: {'X-Requested-With': 'XMLHttpRequest'},
            signal: controller.signal,
        });
        if (!response.ok) {
            return null;
        }

        const body: unknown = await response.json();
        const names = (body as {packages?: unknown}).packages;
        if (!Array.isArray(names)) {
            return null;
        }

        return names.filter((name): name is string => typeof name === 'string');
    } catch {
        return null;
    } finally {
        clearTimeout(timer);
    }
}

/** Test hook. Module state outlives a component, so a test must clear it. */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    cached = NO_PACKAGES;
    loadedAt = null;
    inflight = null;
}
