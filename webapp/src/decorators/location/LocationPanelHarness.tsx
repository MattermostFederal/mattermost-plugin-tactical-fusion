import React, {useState} from 'react';

import type {Conversion} from './convert';
import {parseCanonical} from './format';
import type {LocationFormat} from './format';
import LocationPanel from './LocationPanel';

import {featuresReply, isFeaturesRequest, setStubbedFeatures} from '../../features/stub_fetch';
import type {Features} from '../../features/types';

import type {LocationPayload} from './index';

/**
 * Harness for the location panel.
 *
 * `fetch` is replaced at module scope, before any component renders, because
 * the panel fires its request from an effect on mount and a stub installed
 * later would miss it. Same reason the clipboard is stubbed this way in
 * `CopyButtonHarness`.
 *
 * The reply is deliberately *deferred*: the test decides when and whether it
 * lands. What is being tested is what the panel shows in the meantime and what
 * it shows when the answer never comes, neither of which is visible if the
 * request resolves before the first assertion.
 */
type Outcome = 'ok' | 'fail' | 'reject';

const FIRST: Conversion = {
    mgrs: '11S LT 8463 6908',
    utm: '11S 384640E 3769080N',
    decimal: '34.0561° N, 118.2500° W',
    dms: '34°03\'22.0"N 118°15\'00.0"W',
    ddm: '34°03.366\'N 118°15.000\'W',
    usmtf: '340322.0N1181500.0W',
    region: 'United States of America (Natural Earth 110m)',
    lat: 34.0561,
    lon: -118.25,
};

/**
 * A second coordinate, 3,800 km from the first, for the change-of-selection
 * test. Nothing in it shares a substring with FIRST, so a stale row is
 * unmistakable.
 */
const SECOND: Conversion = {
    mgrs: '18S UJ 23478 06483',
    utm: '18S 323478E 4306483N',
    decimal: '38.8895° N, 77.0353° W',
    dms: '38°53\'22.2"N 77°02\'07.1"W',
    ddm: '38°53.370\'N 77°02.118\'W',
    usmtf: '385322.2N0770207.1W',
    region: 'United States of America (Natural Earth 110m)',
    lat: 38.8895,
    lon: -77.0353,
};

let settle: (() => void) | null = null;
let outcome: Outcome = 'ok';

/*
 * The last request the panel made, rendered into the DOM so a test can assert
 * what was actually sent.
 *
 * Through the DOM rather than an exported binding, because a Playwright
 * component test file runs in Node while the component runs in the browser:
 * importing a value from here would evaluate this module in Node, where
 * `window` does not exist, and would in any case read a different copy of the
 * variable from the one the browser updates.
 *
 * It exists because the stub used to ignore its arguments entirely, so nothing
 * checked that `r` is sent, that it is omitted when it would only repeat the
 * token, or that the CSRF header is there. Dropping any of those silently
 * breaks every conversion in production.
 */
let lastRequest = '';

/*
 * Keyed by the canonical token in the request, so a change of selection gets
 * the answer belonging to the coordinate that asked for it. Without this the
 * stale-frame test could pass for the wrong reason: one fixture for two
 * coordinates cannot tell a stale row from a fresh one.
 */
const OCEAN: Conversion = {
    mgrs: '25P CN 00000 00000',
    utm: '25P 500000E 3319000N',
    decimal: '30.0000° N, 40.0000° W',
    dms: '30\u00b000\'00.0"N 40\u00b000\'00.0"W',
    ddm: '30\u00b000.000\'N 40\u00b000.000\'W',
    usmtf: '300000.0N0400000.0W',
    region: '',
    lat: 30,
    lon: -40,
};

const ANSWERS: Record<string, Conversion> = {
    '34.0561,-118.2500': FIRST,
    '18SUJ2347806483': SECOND,
    '30.0000,-40.0000': OCEAN,
};

/*
 * The rows the stubbed preferences endpoint reports as hidden.
 *
 * Module state set from a prop, because the panel reads its settings through
 * the shared store and the store fetches them once for the whole bundle. A test
 * seeds this before mounting.
 */
let hiddenRows: string[] = [];

window.fetch = ((input: RequestInfo | URL, init?: RequestInit) =>
    new Promise((resolve, rejectRequest) => {
        // The preferences and features requests answer straight away. Only the
        // CONVERSION is deferred, because what these tests are about is what the
        // panel shows while that one is in the air, and a settings read left
        // hanging would just leave every row on its default forever.
        if (isFeaturesRequest(String(input))) {
            resolve(featuresReply());
            return;
        }

        if (String(input).includes('/preferences')) {
            resolve({
                ok: true,
                status: 200,
                json: () => Promise.resolve({location: {hidden_rows: hiddenRows}}),
            } as Response);
            return;
        }

        // Anything that is not a conversion is refused rather than deferred.
        //
        // The map fetches the basemap archive through the same `fetch`, and it
        // reaches here AFTER the conversion does, because the map is not mounted
        // until the features answer lands. Letting it fall into the branch below
        // overwrote `settle` with the archive's resolver, so "answer the
        // conversion" settled the wrong request and every row stayed on
        // `converting…`. The archive is not served in a component test either
        // way, so refusing it changes nothing except which promise the button
        // holds.
        if (!String(input).includes('/api/v1/convert')) {
            resolve({ok: false, status: 404, json: () => Promise.resolve({})} as Response);
            return;
        }

        lastRequest = JSON.stringify({
            url: String(input),
            headers: init?.headers ?? {},
            credentials: init?.credentials ?? '',
        });
        const value = new URL(String(input), 'http://localhost').searchParams.get('v') ?? '';

        settle = () => {
            if (outcome === 'fail') {
                rejectRequest(new Error('offline'));
                return;
            }

            resolve({
                ok: outcome === 'ok',
                status: outcome === 'ok' ? 200 : 400,
                json: () => Promise.resolve(ANSWERS[value] ?? FIRST),
            } as Response);
        };
    })) as typeof fetch;

function payloadFor(format: LocationFormat, canonical: string, raw?: string): LocationPayload {
    return {
        coord: parseCanonical(format, canonical),
        format,
        canonical,
        raw: raw ?? canonical,
    };
}

interface Props {
    format?: LocationFormat;
    canonical?: string;
    raw?: string;
    outcome?: Outcome;

    /** Rows the stubbed reader has hidden. */
    hidden?: string[];

    /** Map surfaces the stubbed admin has left on. Every one, by default. */
    features?: Partial<Features>;
}

/**
 * Watches every committed state of the panel, not just the settled one.
 *
 * This exists because the bug it guards lasts a single frame, and Playwright's
 * assertions retry until they succeed, so by the time one looks the frame is
 * gone. A test written the obvious way passes whether or not the bug is there,
 * which is worse than no test. So the question this asks is "did any committed
 * state ever mix the two coordinates" rather than "does the settled state look
 * right".
 *
 * It reads the DOM from a layout effect with no dependency array, which React
 * runs synchronously after every commit and before paint. That is the part that
 * has to be exact, and the obvious implementation is wrong: a `MutationObserver`
 * delivers COALESCED records at a microtask checkpoint, not one callback per
 * commit, so two commits landing in the same microtask produce a single
 * callback that reads only the final DOM. The intermediate frame, which is the
 * whole subject, is never seen. A layout effect cannot coalesce, because React
 * runs it as part of the commit itself.
 */
/**
 * What each coordinate looks like on screen, in every row that can carry it.
 *
 * Listed per coordinate and used symmetrically, so no mix can show one
 * coordinate through a marker the other side does not also test for.
 */
const MARKERS_FIRST = [FIRST.mgrs, FIRST.decimal, FIRST.utm, '34.0561,-118.2500'];
const MARKERS_SECOND = [SECOND.mgrs, SECOND.decimal, SECOND.utm, '18SUJ2347806483'];

function useMixedFrameCounter(): [React.RefObject<HTMLDivElement | null>, number] {
    const ref = React.useRef<HTMLDivElement>(null);
    const [mixed, setMixed] = useState(0);

    // The last text this counted, so a commit caused by its own setMixed does
    // not count the same frame twice and spin.
    const seen = React.useRef<string | null>(null);

    // No dependency array on purpose: this has to run after EVERY commit.
    React.useLayoutEffect(() => {
        const node = ref.current;
        if (!node) {
            return;
        }

        const text = node.textContent ?? '';
        if (text === seen.current) {
            return;
        }
        seen.current = text;

        // Every marker each coordinate can put on screen, and the two sides
        // must be symmetric.
        //
        // They were not. `hasSecond` looked for the unspaced canonical token
        // while `hasFirst` looked for the SPACED grid reference, so a frame
        // carrying the stale MGRS row beside the new one set only `hasFirst`
        // and the counter stayed at zero. That frame is exactly the stale-row
        // mix this counter is here to catch. It still caught the
        // effect-versus-render regression, because that frame also carries the
        // new token, which is why the asymmetry survived.
        const hasFirst = MARKERS_FIRST.some((m) => text.includes(m));
        const hasSecond = MARKERS_SECOND.some((m) => text.includes(m));
        if (hasFirst && hasSecond) {
            setMixed((n) => n + 1);
        }
    });

    return [ref, mixed];
}

const LocationPanelHarness: React.FC<Props> = ({
    format = 'dd',
    canonical = '34.0561,-118.2500',
    raw,
    outcome: requested = 'ok',
    hidden = [],
    features = {},
}) => {
    outcome = requested;
    hiddenRows = hidden;
    setStubbedFeatures(features);

    // The sidebar keeps the panel mounted across a change of selection, so the
    // harness has to change the payload rather than remount. This button is
    // that change.
    const [payload, setPayload] = useState(() => payloadFor(format, canonical, raw));
    const [ref, mixed] = useMixedFrameCounter();
    const [shown, setShown] = useState('');

    return (
        <div>
            <div ref={ref}>
                <LocationPanel payload={payload}/>
            </div>
            <button
                type='button'
                onClick={() => settle?.()}
            >
                {'answer the conversion'}
            </button>
            <button
                type='button'
                onClick={() => setPayload(payloadFor('mgrs', '18SUJ2347806483'))}
            >
                {'select the second coordinate'}
            </button>
            <button
                type='button'
                onClick={() => setShown(lastRequest)}
            >
                {'show the request'}
            </button>
            <output data-testid='mixed-frames'>{String(mixed)}</output>
            <output data-testid='last-request'>{shown}</output>
        </div>
    );
};

export default LocationPanelHarness;
