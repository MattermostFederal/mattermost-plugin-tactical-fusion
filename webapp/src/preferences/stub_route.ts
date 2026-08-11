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
    body: {dtg?: {zones?: WireZone[]; urgent_within_minutes?: number}} | null;
}

export interface StubOptions {

    /** What the server already has stored. */
    stored?: {zones: WireZone[]; urgentWithinMinutes: number};

    /** Status for the first GET, so a failed load can be exercised. */
    loadStatus?: number;

    /** Status for a PUT, so a rejected save can be exercised. */
    saveStatus?: number;

    /** The message the server sends with a rejected save. */
    saveMessage?: string;
}

const NOTHING_SAVED = {dtg: {zones: [], urgent_within_minutes: 0}}; // eslint-disable-line @typescript-eslint/naming-convention

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
            urgent_within_minutes: options.stored?.urgentWithinMinutes ?? 0, // eslint-disable-line @typescript-eslint/naming-convention
        },
    };

    await page.route('**/api/v1/preferences', async (route) => {
        const method = route.request().method();
        const body = method === 'PUT' ? route.request().postDataJSON() : null;
        calls.push({method, body});

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
