import React, {useState} from 'react';

import type {Conversion} from './convert';
import {parseCanonical} from './format';
import type {LocationFormat} from './format';
import LocationPanel from './LocationPanel';

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
const ANSWERS: Record<string, Conversion> = {
    '34.0561,-118.2500': FIRST,
    '18SUJ2347806483': SECOND,
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
        // The preferences request answers straight away. Only the CONVERSION is
        // deferred, because what these tests are about is what the panel shows
        // while that one is in the air, and a settings read left hanging would
        // just leave every row on its default forever.
        if (String(input).includes('/preferences')) {
            resolve({
                ok: true,
                status: 200,
                json: () => Promise.resolve({location: {hidden_rows: hiddenRows}}),
            } as Response);
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
}

/**
 * Watches every committed state of the panel, not just the settled one.
 *
 * This exists because the bug it guards lasts a single frame, and Playwright's
 * assertions retry until they succeed, so by the time one looks the frame is
 * gone. A test written the obvious way passes whether or not the bug is there,
 * which is worse than no test. A `MutationObserver` sees each commit as it
 * happens, so the question becomes "did any committed state ever mix the two
 * coordinates" rather than "does the settled state look right".
 */
function useMixedFrameCounter(): [React.RefObject<HTMLDivElement | null>, number] {
    const ref = React.useRef<HTMLDivElement>(null);
    const [mixed, setMixed] = useState(0);

    React.useEffect(() => {
        const node = ref.current;
        if (!node) {
            return undefined;
        }

        const check = () => {
            const text = node.textContent ?? '';
            const hasFirst = text.includes(FIRST.mgrs) || text.includes(FIRST.decimal);
            const hasSecond = text.includes('18SUJ2347806483') || text.includes(SECOND.decimal);
            if (hasFirst && hasSecond) {
                setMixed((n) => n + 1);
            }
        };

        const observer = new MutationObserver(check);
        observer.observe(node, {subtree: true, childList: true, characterData: true});
        return () => observer.disconnect();
    }, []);

    return [ref, mixed];
}

const LocationPanelHarness: React.FC<Props> = ({
    format = 'dd',
    canonical = '34.0561,-118.2500',
    raw,
    outcome: requested = 'ok',
    hidden = [],
}) => {
    outcome = requested;
    hiddenRows = hidden;

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
