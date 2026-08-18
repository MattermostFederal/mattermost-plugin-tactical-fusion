import {expect, test} from '@playwright/test';

import decorator, {fromParams} from './index';

function params(entries: Record<string, string>): URLSearchParams {
    return new URLSearchParams(entries);
}

test.describe('fromParams', () => {
    test('accepts what the server produces', () => {
        const payload = fromParams(params({f: 'latm', v: '3510N07901W'}));

        expect(payload).not.toBeNull();
        expect(payload?.canonical).toBe('3510N07901W');
        expect(payload?.format).toBe('latm');
    });

    // An absent r reads as "the author wrote the canonical form", not as a
    // missing value, so nothing downstream has to special-case it.
    test('falls back to the canonical form when there is no r', () => {
        const payload = fromParams(params({f: 'latm', v: '3510N07901W'}));
        expect(payload?.raw).toBe('3510N07901W');
    });

    test('keeps the author spelling when r is present', () => {
        const payload = fromParams(params({
            f: 'dms',
            v: '340322N1181500W',
            r: '34°03\'22"N 118°15\'00"W',
        }));

        expect(payload?.raw).toBe('34°03\'22"N 118°15\'00"W');
    });

    test('rejects each mutation of a valid link', () => {
        const cases: Array<[string, Record<string, string>]> = [
            ['unknown format', {f: 'mgrs', v: '3510N07901W'}],
            ['missing format', {v: '3510N07901W'}],
            ['missing value', {f: 'latm'}],
            ['format disagrees with the token', {f: 'dd', v: '3510N07901W'}],
            ['token is not canonical', {f: 'dms', v: '34 03 22 N 118 15 00 W'}],
            ['prose in the token', {f: 'latm', v: 'TARGET 3510N07901W'}],
            ['empty', {}],
        ];

        for (const [name, entries] of cases) {
            expect(fromParams(params(entries)), name).toBeNull();
        }
    });

    // The two gates the webapp can enforce: a length cap and an alphabet.
    //
    // The other two, an anchored match against the token grammar and a round
    // trip back to the canonical form, need that grammar, and it lives in Go
    // only. So this is a shape check and nothing more, and the panel gets the
    // verdict on the rest from the conversion endpoint, which runs all four.
    test('rejects an r that is too long or carries the wrong characters', () => {
        const base = {f: 'latm', v: '3510N07901W'};

        const cases: Array<[string, string]> = [
            ['too long', '3'.repeat(65)],

            // The cap is BYTES, matching maxRawBytes in Go, and the alphabet
            // deliberately admits typographic symbols that cost two or three
            // bytes each. Counted as UTF-16 code units this is 33 characters
            // and passes; counted the way the server counts it, it is 65 bytes
            // and does not. The webapp used to wave it through, so the panel
            // committed to a link the page would refuse.
            ['64 characters but over 64 bytes', "34\u00b003'22\u2033N " + '\u2033'.repeat(20)],
            ['markup', '3510N07901W<script>'],
            ['ampersand', '3510N07901W&x'],
            ['a newline, which no grammar admits inside a token', '3510N\n07901W'],
            ['an underscore', '3510N07901W_x'],
            ['non-ASCII', '3510N07901WЀ'],
        ];

        for (const [name, raw] of cases) {
            expect(fromParams(params({...base, r: raw})), name).toBeNull();
        }
    });

    // The alphabet admits every letter, because a grid reference is made of
    // them: "18S UJ 23478 06483" carries a band letter and two 100 km square
    // letters. That is a real loosening of this gate, and it is why prose made
    // of coordinate characters gets through here.
    //
    // Pinned rather than left implicit, so that anybody tightening it knows
    // they are also changing what the panel can display, and anybody who reads
    // this gate as the defense learns here that it is not.
    test('lets prose built from coordinate characters through, for the server to refuse', () => {
        const payload = fromParams(params({
            f: 'latm',
            v: '3510N07901W',
            r: '3510N07901W ALFA',
        }));

        expect(payload).not.toBeNull();
        expect(payload?.raw).toBe('3510N07901W ALFA');
    });

    // An empty r reads as absent, matching the server. Treating '' as present
    // left a blank "Original text" row beside a spurious "Normalized" one.
    test('treats an empty r as absent', () => {
        const payload = fromParams(params({f: 'latm', v: '3510N07901W', r: ''}));
        expect(payload?.raw).toBe('3510N07901W');
    });

    test('rejects an out-of-range coordinate the server would refuse', () => {
        expect(fromParams(params({f: 'dd', v: '91.0000,181.0000'}))).toBeNull();
    });

    test('accepts the typographic variants the server accepts', () => {
        const payload = fromParams(params({
            f: 'dms',
            v: '340322N1181500W',
            r: '34°03′22″N 118°15′00″W',
        }));

        expect(payload).not.toBeNull();
    });
});

// The link that found this: a UTM position whose band letter is S.
//
// fromParams returning null is not a visible failure. The click handler treats
// it as "not one of ours", stands aside, and the browser follows the href, so
// clicking a perfectly good coordinate opened the server-rendered page instead
// of the sidebar. Nothing logged, nothing threw, and the page it landed on
// rendered correctly, which made it look like a routing choice rather than a
// rejected payload.
//
// The cause was this side keeping its own narrower band class for UTM, left
// over from when N and S were refused as ambiguous. The server had since
// started reading them as latitude bands and issuing links carrying them.
test.describe('grid links the server issues', () => {
    test('accepts a UTM link whose band letter is S', () => {
        const payload = fromParams(params({
            f: 'utm',
            v: '18S3234784306483',
            r: '18S 323478E 4306483N',
        }));

        expect(payload, 'a link the server issued must open in the sidebar').not.toBeNull();
        expect(payload?.canonical).toBe('18S3234784306483');
        expect(payload?.raw).toBe('18S 323478E 4306483N');

        // Null is correct for a grid format and is not a rejection: the
        // projection lives in Go, so the position arrives from the conversion
        // endpoint rather than from the token.
        expect(payload?.coord).toBeNull();
    });

    test('accepts every band letter after a zone, in both grid formats', () => {
        const bands = 'CDEFGHJKLMNPQRSTUVWX';

        for (const band of bands) {
            expect(
                fromParams(params({f: 'utm', v: `18${band}3234784306483`})),
                `utm band ${band}`,
            ).not.toBeNull();

            expect(
                fromParams(params({f: 'mgrs', v: `18${band}UJ2347806483`})),
                `mgrs band ${band}`,
            ).not.toBeNull();
        }

        // I and O are the two that are not band letters, because they read as
        // 1 and 0.
        for (const band of 'IO') {
            expect(fromParams(params({f: 'utm', v: `18${band}3234784306483`})), band).toBeNull();
        }
    });
});

/*
 * The hover exists, and it is the map.
 *
 * This decorator carried no Hover for a long time and the reason was recorded
 * rather than forgotten: a hover fires on pointer movement, so an uncached
 * conversion would have put a request behind every coordinate a cursor crossed.
 * What unblocked it is the cache in convert.ts, which `the conversion cache` in
 * convert.spec.ts holds to one request per token.
 *
 * Asserted here because the framework reads `Hover` off the decorator and
 * renders nothing at all when it is absent: dropping it would cost every hover
 * card in the product with no error anywhere.
 */
test('declares a hover', () => {
    expect(decorator.Hover).toBeTruthy();
});


test.describe('area-reference links the server issues', () => {
    test('accepts every shape at every resolution', () => {
        const cases: Array<[string, string]> = [
            ['georef', 'GJNJ5753'],
            ['georef', 'GJNJ575337'],
            ['georef', 'GJNJ57533752'],
            ['gars', '206LT'],
            ['gars', '206LT2'],
            ['gars', '006AG39'],
            ['pluscode', '849VCWC8+R9'],
            ['pluscode', '849VCWC8+R9C'],
            ['pluscode', '849VCWC8+'],
            ['pluscode', '849V0000+'],
        ];

        for (const [f, v] of cases) {
            const payload = fromParams(params({f, v}));

            expect(payload, `${f} ${v} must open in the sidebar`).not.toBeNull();
            expect(payload?.canonical).toBe(v);
            expect(payload?.coord).toBeNull();
        }
    });

    test('accepts the author text a Plus Code link carries', () => {
        const payload = fromParams(params({
            f: 'pluscode',
            v: '849VCWC8+R9',
            r: '849vcwc8+r9',
        }));

        expect(payload).not.toBeNull();
        expect(payload?.raw).toBe('849vcwc8+r9');
    });

    test('rejects a code the server would not have issued', () => {
        const cases: Array<[string, string, string]> = [
            ['an I in a GEOREF zone', 'georef', 'IJNJ5753'],
            ['a bare GEOREF quadrangle', 'georef', 'GJNJ'],
            ['an I in a GARS letter pair', 'gars', '206IT'],
            ['a short Plus Code', 'pluscode', 'CWC8+R9'],
            ['a lower-case Plus Code as the canonical form', 'pluscode', '849vcwc8+r9'],
        ];

        for (const [name, f, v] of cases) {
            expect(fromParams(params({f, v})), name).toBeNull();
        }
    });
});
