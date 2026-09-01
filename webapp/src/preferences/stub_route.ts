import type {Page} from '@playwright/test';

/**
 * One request the component made, as the stub saw it.
 */
interface WireZone {
    iana: string;
    name?: string;
}

export interface Recorded {
    method: string;
    body: {
        dtg?: {zones?: WireZone[]; urgent_within_minutes?: number};
        location?: {hidden_rows?: string[]};
        cot?: {hidden_sections?: string[]};
        geojson?: {hidden_sections?: string[]};
    } | null;
}

export interface StubOptions {

    /** What the server already has stored, for the DTG editor. */
    stored?: {zones: WireZone[]; urgentWithinMinutes: number};

    /** What the server already has stored, for the location editor. */
    storedHiddenRows?: string[];

    /** What the server already has stored, for the Cursor on Target editor. */
    storedHiddenSections?: string[];

    storedGeoJsonSections?: string[];

    /**
     * Holds every GET open, handing back the function that releases them.
     *
     * The only way to test what an editor does while its settings are still in
     * the air, which is where the draft-clobbering bug lives. It is a callback
     * rather than a flag because a held request has to be released from inside
     * the test, after it has typed into the form.
     *
     * This was declared as a bare `deferLoad?: boolean` and never implemented,
     * so the one test that would have caught the DTG editor discarding a draft
     * could not be written, and anyone who set the flag got a vacuous pass.
     */
    holdLoad?: (release: () => void) => void;

    /** Status for the first GET, so a failed load can be exercised. */
    loadStatus?: number;

    /** Status for a PUT, so a rejected save can be exercised. */
    saveStatus?: number;

    /** The message the server sends with a rejected save. */
    saveMessage?: string;

    /** Status for a DELETE, so a rejected "Restore defaults" can be exercised. */
    resetStatus?: number;

    /** The message the server sends with a rejected reset. */
    resetMessage?: string;
}

const NOTHING_SAVED = {
    dtg: {zones: [], urgent_within_minutes: 0},
    location: {hidden_rows: []},
    cot: {hidden_sections: []},
    geojson: {hidden_sections: []},
};

/**
 * Stands in for the plugin's preferences route in a component test.
 *
 * Stateful rather than a fixed reply, so a test can save and then see what the
 * component does with what came back, which is where the interesting bugs live.
 *
 * Test-only, and imported only from `.pw.tsx` files, so it never reaches the
 * plugin bundle.
 */
export async function stubPreferencesRoute(page: Page, options: StubOptions = {}): Promise<Recorded[]> {
    const calls: Recorded[] = [];

    let stored = {
        dtg: {
            zones: options.stored?.zones ?? [],
            urgent_within_minutes: options.stored?.urgentWithinMinutes ?? 0,
        },
        location: {hidden_rows: options.storedHiddenRows ?? []},
        cot: {hidden_sections: options.storedHiddenSections ?? []},
        geojson: {hidden_sections: options.storedGeoJsonSections ?? []},
    };

    // Resolves when the test releases the held GETs. Held requests park on it
    // rather than polling, so a test that never releases simply times out on
    // its own assertion instead of hanging silently.
    let release = () => { /* replaced below when holdLoad is used */ };
    const held = options.holdLoad ? new Promise<void>((resolve) => {
        release = resolve;
    }) : null;
    options.holdLoad?.(() => release());

    await page.route('**/api/v1/preferences', async (route) => {
        const method = route.request().method();
        const body = method === 'PUT' ? route.request().postDataJSON() : null;
        calls.push({method, body});

        if (method === 'GET' && held) {
            await held;
        }

        if (method === 'GET' && options.loadStatus && options.loadStatus !== 200) {
            await route.fulfill({
                status: options.loadStatus,
                contentType: 'application/json',
                body: JSON.stringify({message: 'Could not read your settings.'}),
            });
            return;
        }

        if (method === 'PUT') {
            if (options.saveStatus && options.saveStatus !== 200) {
                await route.fulfill({
                    status: options.saveStatus,
                    contentType: 'application/json',
                    body: JSON.stringify({message: options.saveMessage ?? 'Rejected.'}),
                });
                return;
            }
            stored = body;
        }

        if (method === 'DELETE') {
            if (options.resetStatus && options.resetStatus !== 200) {
                await route.fulfill({
                    status: options.resetStatus,
                    contentType: 'application/json',
                    body: JSON.stringify({message: options.resetMessage ?? 'Rejected.'}),
                });
                return;
            }
            stored = NOTHING_SAVED;
        }

        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(stored),
        });
    });

    return calls;
}

/** The zone identifiers the component asked to save, in order. */
export function savedZones(calls: Recorded[]): string[] | undefined {
    return calls.find((call) => call.method === 'PUT')?.body?.dtg?.zones?.
        map((zone) => zone.iana);
}

/** The whole entries the component asked to save, names included. */
export function savedEntries(calls: Recorded[]): WireZone[] | undefined {
    return calls.find((call) => call.method === 'PUT')?.body?.dtg?.zones;
}

/** The threshold the component asked to save, in minutes. */
export function savedMinutes(calls: Recorded[]): number | undefined {
    return calls.find((call) => call.method === 'PUT')?.body?.dtg?.urgent_within_minutes;
}

/** The hidden rows the component asked to save, in order. */
export function savedHiddenRows(calls: Recorded[]): string[] | undefined {
    return calls.filter((call) => call.method === 'PUT').at(-1)?.body?.location?.hidden_rows;
}

/** The hidden sections the component asked to save, in order. */
export function savedHiddenSections(calls: Recorded[]): string[] | undefined {
    return calls.filter((call) => call.method === 'PUT').at(-1)?.body?.cot?.hidden_sections;
}

/** The hidden GeoJSON sections the component asked to save, in order. */
export function savedGeoJsonSections(calls: Recorded[]): string[] | undefined {
    return calls.filter((call) => call.method === 'PUT').at(-1)?.body?.geojson?.hidden_sections;
}
