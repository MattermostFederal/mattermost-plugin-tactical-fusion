import {expect, test} from '@playwright/test';

import {
    axisResolutionDegrees,
    confidenceText,
    ddmText,
    decimalText,
    dmsText,
    gridResolutionMeters,
    remoteResolutionText,
    gridText,
    isCanonical,
    parseCanonical,
    resolutionDegrees,
    resolutionText,
    usmtfText,
} from './format';
import type {Coordinate, LocationFormat} from './format';

function parse(format: LocationFormat, value: string): Coordinate {
    const c = parseCanonical(format, value);
    if (!c) {
        throw new Error(`parseCanonical(${format}, ${value}) returned null`);
    }
    return c;
}

test.describe('parseCanonical', () => {
    test('reads every canonical shape the server emits', () => {
        const cases: Array<[LocationFormat, string, number, number]> = [
            ['dd', '34.0561,-118.2500', 34.0561, -118.25],
            ['ddh', '34.0561N,118.2500W', 34.0561, -118.25],
            ['dms', '340322N1181500W', 34.0561111, -118.25],
            ['dms', '331000.0N1183000.0W', 33.1666667, -118.5],
            ['ddm', '3403.366N11815.000W', 34.0561, -118.25],
            ['latd', '35N079W', 35, -79],
            ['latm', '3510N07901W', 35.1666667, -79.0166667],
            ['vlatm', '3510N9-07901W7', 35.1666667, -79.0166667],
        ];

        for (const [format, value, lat, lon] of cases) {
            const c = parse(format, value);
            expect(c.lat.decimal, value).toBeCloseTo(lat, 6);
            expect(c.lon.decimal, value).toBeCloseTo(lon, 6);
        }
    });

    // A zero half beside a real one is an ordinary coordinate on the equator or
    // the prime meridian. A truthiness check would drop both, which is a real
    // bug in the sibling plugin's click handler.
    test('keeps a zero component', () => {
        const c = parse('dd', '0.0000,32.5000');
        expect(c.lat.decimal).toBe(0);
        expect(c.lon.decimal).toBeCloseTo(32.5, 6);
    });

    test('returns null for anything that is not the canonical form', () => {
        expect(parseCanonical('dms', '34 03 22 N 118 15 00 W')).toBeNull();
        expect(parseCanonical('dms', 'not a coordinate')).toBeNull();
        expect(parseCanonical('latm', '3510N07901W extra')).toBeNull();
        expect(parseCanonical('dd', '')).toBeNull();
    });

    test('isCanonical agrees with parseCanonical', () => {
        expect(isCanonical('latm', '3510N07901W')).toBe(true);
        expect(isCanonical('latm', '35N079W')).toBe(false);
        expect(isCanonical('dd', '34.0561,-118.2500')).toBe(true);
    });

    // Shaped like a token and still not one. These match their own pattern
    // character for character and are refused on the values inside it, which
    // the pattern cannot express: 60 minutes is the next degree and latitude 99
    // is nowhere.
    //
    // Worth pinning because the refusal is SILENT by construction. fromParams
    // returns null, the click handler reads that as "not one of ours" and
    // stands aside, and the browser follows the link to the standalone page.
    // Nothing is logged on either side and the page renders correctly, so the
    // only symptom is that clicking a coordinate opens a page instead of the
    // sidebar. That is exactly how a band-class drift between this file and
    // grammar.go got shipped once already.
    test('returns null for a field outside its range', () => {
        // Minutes are 0-59. Sixty of them is the degree above.
        expect(parseCanonical('ddm', '3460.000N11815.000W')).toBeNull();
        expect(parseCanonical('vlatm', '3560N9-07901W7')).toBeNull();

        // And the pair still has to land on the planet.
        expect(parseCanonical('vlatm', '9910N9-07901W7')).toBeNull();
    });
});

test.describe('rendering at the token resolution', () => {
    test('never shows more decimals than the token carried', () => {
        expect(decimalText(parse('dd', '34.0561,-118.2500'))).toBe('34.0561° N, 118.2500° W');
        expect(decimalText(parse('latd', '35N079W'))).toBe('35° N, 79° W');
    });

    // Padding a field the token never carried would be a claim. A degrees-only
    // token renders degrees only, not 34°00'00".
    test('drops the fields a coarse token never had', () => {
        expect(dmsText(parse('latd', '35N079W'))).toBe('35°N 79°W');
        expect(dmsText(parse('latm', '3510N07901W'))).toBe('35°10\'N 79°01\'W');
        expect(ddmText(parse('latd', '35N079W'))).toBe('35°N 79°W');
    });

    test('renders full DMS for a token written that finely', () => {
        expect(dmsText(parse('dms', '340322N1181500W'))).toBe('34°03\'22"N 118°15\'00"W');
    });

    // The shared fixtures. These six rows are asserted identically in
    // server/decorators/location/format_test.go. The page is Go and this panel
    // is TypeScript, so they cannot share a render function; this table is what
    // stops them describing the same coordinate two different ways.
    test('matches the shared fixtures', () => {
        const cases: Array<[LocationFormat, string, string, string, string, string, string]> = [
            ['dd', '34.0561,-118.2500', '34.0561° N, 118.2500° W',
                '34°03\'22.0"N 118°15\'00.0"W', '34°03.366\'N 118°15.000\'W',
                '340322.0N1181500.0W', 'about 11 m'],
            ['latd', '35N079W', '35° N, 79° W',
                '35°N 79°W', '35°N 79°W',
                '35N079W', 'about 111.3 km'],
            ['latm', '3510N07901W', '35.17° N, 79.02° W',
                '35°10\'N 79°01\'W', '35°10\'N 79°01\'W',
                '3510N07901W', 'about 1.9 km'],
            ['dms', '340322N1181500W', '34.0561° N, 118.2500° W',
                '34°03\'22"N 118°15\'00"W', '34°03.37\'N 118°15.00\'W',
                '340322N1181500W', 'about 31 m'],

            // The halves need not have been written to the same precision, and
            // each renders at its OWN. Rendering the fine half at the coarse
            // one's count moved it 4.9 km. The resolution stays the coarser
            // half's, because that is how well the pair is known.
            //
            // The USMTF column is the exception, and deliberately so: it is one
            // fixed-width shape covering both halves, so it has no spelling in
            // which latitude carries a finer field than longitude. It is sized
            // from the pair, like the grid rows, and the fine half loses digits
            // in that one column only.
            ['ddh', '34.0561N,118.2W', '34.0561° N, 118.2° W',
                '34°03\'22.0"N 118°12\'W', '34°03.366\'N 118°12\'W',
                '3403N11812W', 'about 11.1 km'],
            ['dms', '340322.5N1181500W', '34.05625° N, 118.2500° W',
                '34°03\'22.5"N 118°15\'00"W', '34°03.375\'N 118°15.00\'W',
                '340322N1181500W', 'about 31 m'],
        ];

        for (const [format, canonical, decimal, dms, ddm, usmtf, resolution] of cases) {
            const c = parse(format, canonical);
            expect(decimalText(c), canonical).toBe(decimal);
            expect(dmsText(c), canonical).toBe(dms);
            expect(ddmText(c), canonical).toBe(ddm);
            expect(usmtfText(c), canonical).toBe(usmtf);
            expect(resolutionText(c), canonical).toBe(resolution);
        }
    });

    // Rounding, not truncation. Truncating minutes biases every result up to a
    // whole minute south and west.
    test('rounds rather than truncating', () => {
        // 34.0561 degrees is 34°03'21.96". Four decimal degrees is finer than a
        // second, so this renders one decimal second, and 21.96 rounds to 22.0.
        // Truncating, which is what the sibling plugin does when it converts
        // back, would show 21.9 and bias every result south and west.
        expect(dmsText(parse('dd', '34.0561,-118.2500'))).toContain('34°03\'22.0"N');
    });

    // "0 m" would read as infinite precision. An eight-decimal token reaches
    // this.
    test('does not claim zero meters for a very fine token', () => {
        expect(resolutionText(parse('dd', '34.12345678,-118.12345678'))).toBe('finer than 0.01 m');
        expect(resolutionText(parse('dms', '340322.1234N1181500.1234W'))).toBe('finer than 0.01 m');
    });

    test('describes resolution in words, never as accuracy', () => {
        expect(resolutionText(parse('dd', '34.0561,-118.2500'))).toBe('about 11 m');
        expect(resolutionText(parse('latd', '35N079W'))).toBe('about 111.3 km');
        expect(resolutionText(parse('latm', '3510N07901W'))).toBe('about 1.9 km');
        expect(resolutionText(parse('ddm', '3403.366N11815.000W'))).toBe('about 2 m');
    });

    // The rungs between "about 11 m" and "finer than 0.01 m", which the cases
    // above step straight over.
    //
    // humanMeters has three arms and the sub-meter one was never taken here:
    // the fixture table stops at four decimals and the test above starts at
    // eight, which returns before humanMeters is called at all. Six decimals is
    // not an edge case, it is what an ordinary phone emits, and it lands at
    // 0.11 m squarely in the gap.
    //
    // This is now the only implementation. Go rendered resolution too while the
    // page was server-rendered, and the twin of this table lived in
    // TestResolutionTextBelowAMeter; every surface renders from here now, so
    // these three rows are the whole guard rather than half of a pair.
    test('names the rungs below a meter', () => {
        const cases: Array<[string, string]> = [
            ['34.05611N,118.25000W', 'about 1 m'],
            ['34.056111N,118.250000W', 'about 0.11 m'],
            ['34.0561111N,118.2500000W', 'about 0.01 m'],
        ];

        for (const [canonical, want] of cases) {
            expect(resolutionText(parse('ddh', canonical)), canonical).toBe(want);
        }
    });
});

// Every rendering path rounds rather than truncating, so a value a hair under a
// whole degree rounds its minutes to exactly 60, and the field below has to be
// emptied and the one above incremented. Left undone, the USMTF row reads
// "3360N", which no grammar in this package accepts, so a reader could paste it
// into an ATO and have the next tool along refuse it.
//
// Reached through the USMTF row rather than through DMS or DDM, and that is the
// whole trick: the USMTF shape is sized from the PAIR, so a coordinate whose
// halves were written to different precision renders its fine half into the
// coarse half's fields. Every other row is sized per axis, which makes the
// rounding exact and the carry unreachable. `34.0561N,118.2W` is an ordinary
// paste, so this is a real path and not a contrivance.
//
// Mirrors TestRoundingCarriesIntoTheNextField in format_test.go.
test.describe('rounding carries into the next field', () => {
    test('minutes rounding to 60 become the next degree', () => {
        // The pair is worth 0.1 degrees, so the shape is LATM and the latitude
        // is rendered to whole minutes: 59.99994' rounds to 60 and carries.
        expect(usmtfText(parse('ddh', '33.999999N,118.2W'))).toBe('3400N11812W');
    });

    test('seconds rounding to 60 carry through minutes into the degree', () => {
        // Both carries at once, which is the only way the minutes arm of the
        // seconds split ever runs: the minutes there are truncated rather than
        // rounded, so they only reach 60 by being pushed there from below.
        expect(usmtfText(parse('ddh', '33.99999999N,118.250W'))).toBe('340000N1181500W');
    });

    // The counterpart, so the tests above are pinning a carry rather than a
    // coarsening. Written to one precision, the same latitude renders its own
    // digits and never reaches 60 at all.
    test('an axis rendered at its own resolution does not carry', () => {
        expect(usmtfText(parse('ddh', '33.999999N,118.250000W'))).toBe('335959.996N1181500.000W');
    });

    /*
     * The decimal-minutes carry is a float-drift guard rather than a case a
     * token reaches: with d fractional digits the largest minute a token can
     * state is 60 - 60x10^-d, which is always further from 60 than the rounding
     * step at that resolution. What can reach it is a value that is not exactly
     * the decimal it came from, which is the same class of defect that made
     * degMinSec render a negative zero on arm64.
     *
     * So it is driven from a Coordinate directly. Without the carry this prints
     * 33 degrees 60 minutes, which is not a coordinate.
     */
    test('a value drifting under a degree does not print sixty minutes', () => {
        const justUnder34: Coordinate = {
            lat: {decimal: 34 - (Number.EPSILON * 34), digits: 1, confidence: null},
            lon: {decimal: -118.2, digits: 1, confidence: null},
            format: 'dd',
            digits: 1,
        };

        expect(ddmText(justUnder34)).toBe('34°00\'N 118°12\'W');
    });

    // DDM needs a Coordinate built by hand where USMTF does not, and the reason
    // is the split those two rows sit on opposite sides of. USMTF is sized from
    // the PAIR, so a mixed-precision token reaches its carry, which is what the
    // tests above do. DDM is sized per AXIS, so each half renders at its own
    // digit count and the minute field always carries two more decimal places
    // than the value can fill: no token reaches 60.
    //
    // That makes it a guard rather than a path, and it is fired here the way Go
    // fires its copy, by calling degDecimalMin directly in
    // TestRoundingCarriesIntoTheNextField. Until now the Go copy was fired and
    // the TypeScript one was fired by nothing, in a pair of renderers whose
    // whole design is that they produce the same string.
    test('a minute rounding to 60 carries into the degree', () => {
        const coarse: Coordinate = {
            lat: {decimal: 33.999999, confidence: null, digits: 1},
            lon: {decimal: -117.999999, confidence: null, digits: 1},
            format: 'ddh',
            digits: 1,
        };

        // Without the carry this reads 33°60'N, which is not a place.
        expect(ddmText(coarse)).toBe('34°00\'N 118°00\'W');
    });
});

test.describe('confidence', () => {
    // How well a position is known and how finely it was written are different
    // facts, so the verified digits are surfaced rather than folded into the
    // resolution or silently dropped.
    test('surfaces the USMTF verified digits', () => {
        expect(confidenceText(parse('vlatm', '3510N9-07901W7'))).toBe(
            'stated confidence 9 (latitude), 7 (longitude)');
    });

    test('says nothing when the token carried none', () => {
        expect(confidenceText(parse('latm', '3510N07901W'))).toBe('');
    });

    test('does not let confidence reach the position', () => {
        const verified = parse('vlatm', '3510N9-07901W7');
        const plain = parse('latm', '3510N07901W');
        expect(verified.lat.decimal).toBe(plain.lat.decimal);
        expect(verified.lon.decimal).toBe(plain.lon.decimal);
    });
});

test.describe('hardening', () => {
    // An unbounded fraction reaches toFixed(), which throws RangeError above
    // 100 digits. There is no error boundary in this bundle, so one crafted
    // link posted in a channel crashed the panel for every reader who clicked
    // it. The pattern is bounded to mirror maxFrac in Go.
    test('rejects more fractional digits than the server can emit', () => {
        const many = `34.${'1'.repeat(120)},-118.${'1'.repeat(120)}`;
        expect(isCanonical('dd', many)).toBe(false);
        expect(parseCanonical('dd', many)).toBeNull();

        const nine = `34.${'1'.repeat(9)},-118.${'1'.repeat(9)}`;
        expect(parseCanonical('dd', nine)).toBeNull();
        expect(parseCanonical('dd', '34.12345678,-118.12345678')).not.toBeNull();
    });

    // The server rejects these outright. Without the same check the panel
    // rendered latitude 91 as fact while the page 400d on the same link.
    test('rejects an out-of-range coordinate', () => {
        expect(parseCanonical('dd', '91.0000,10.0000')).toBeNull();
        expect(parseCanonical('dd', '10.0000,181.0000')).toBeNull();
        expect(parseCanonical('dms', '349999N1189999W')).toBeNull();
        expect(parseCanonical('latm', '9999N99999W')).toBeNull();
    });

    // Negative zero is a real coordinate on the equator. `v < 0` reads it as
    // positive, so 0.0000S rendered as N in the panel while the Go page, which
    // uses math.Signbit, said S.
    test('keeps the southern and western letters at zero', () => {
        expect(decimalText(parse('ddh', '0.0000S,32.5000E'))).toBe('0.0000° S, 32.5000° E');
        expect(decimalText(parse('ddh', '12.0000N,0.0000W'))).toBe('12.0000° N, 0.0000° W');
        expect(dmsText(parse('ddh', '0.0000S,32.5000E'))).toContain('S');
    });

    /*
     * A format this file has no case for. The grammar is Go-only and this side
     * keeps a copy of the canonical shapes, so the pair can drift; the answer
     * has to be a refusal rather than a throw, because a link the panel cannot
     * read falls through to the standalone page and a throw would take the
     * whole panel with it.
     */
    test('a format this build has no grammar for is refused, not thrown on', () => {
        // Every id, not one hand-picked miss. This asserted a universal claim
        // against a single own-property miss, which is the same shape as the
        // fixtures-chosen-to-satisfy-the-claim defect CLAUDE.md records against
        // TestUSMTFRowIsATokenThisPackageAccepts. The prototype keys are the
        // ones that mattered: `CANONICAL['toString']` resolves up the chain to a
        // function, which is truthy and has no `.exec`, so `?.` sails through
        // and the call throws.
        for (const name of ['someday', 'toString', 'constructor', 'valueOf', '__proto__']) {
            const unknown = name as LocationFormat;

            expect(isCanonical(unknown, '34.0000,-118.2500'), name).toBe(false);
            expect(parseCanonical(unknown, '34.0000,-118.2500'), name).toBeNull();
            expect(gridText(unknown, '18SUJ2347806483'), name).toBe('');
        }
    });

    /*
     * A grid resolution belongs to a grid grammar, and to no other.
     *
     * gridResolutionMeters read the MGRS pattern for EVERY non-UTM id, which was
     * unreachable while the page refused an unknown format and went live the
     * moment it began degrading instead. A token whose format this build does
     * not know, whose canonical happens to match the MGRS shape, was rendered as
     * "1 m grid, at center" with a 1 m cell drawn around it: a resolution
     * claimed from a grammar the page had just said it does not have.
     */
    test('only a grid format has a grid resolution', () => {
        expect(remoteResolutionText('mgrs', '18SUJ2347806483')).toBe('1 m grid, at center');
        expect(gridResolutionMeters('mgrs', '18SUJ2347806483')).toBe(1);

        for (const name of ['dd', 'ddh', 'latm', 'someday']) {
            const other = name as LocationFormat;

            expect(remoteResolutionText(other, '18SUJ2347806483'), name).toBe('');
            expect(gridResolutionMeters(other, '18SUJ2347806483'), name).toBeNull();
        }
    });

    // The same fall-through on the rendering side. A degree is the coarsest
    // thing any grammar here states, so it is the honest default.
    test('a format with no stated resolution is read as whole degrees', () => {
        const unknown: Coordinate = {
            lat: {decimal: 34.0561, digits: 4, confidence: null},
            lon: {decimal: -118.25, digits: 4, confidence: null},
            format: 'someday' as LocationFormat,
            digits: 4,
        };

        expect(resolutionDegrees(unknown)).toBe(1);
        expect(axisResolutionDegrees(unknown, unknown.lat)).toBe(1);
    });
});

test.describe('the grid formats', () => {
    // These are the only two grammars this file cannot read a position out of.
    // A grid reference is a projection onto an ellipsoid, that projection lives
    // in Go, and a second one written here would be two implementations of the
    // same geodesy with nothing keeping them equal.
    //
    // What is still local is everything the token itself says: which square it
    // names and how big that square is. Those go on screen the moment the panel
    // opens and stay there even when the conversion request never lands, which
    // is what keeps a grid link useful rather than blank.
    test('has no position without the server', () => {
        expect(parseCanonical('mgrs', '18SUJ2347806483')).toBeNull();
        expect(parseCanonical('utm', '33U2910005628000')).toBeNull();
    });

    // But the token is still vouched for, and the panel relies on that: a null
    // position for a grid format is expected, and a null position for anything
    // else is a rejected link.
    test('still validates the canonical shape', () => {
        expect(isCanonical('mgrs', '18SUJ2347806483')).toBe(true);
        expect(isCanonical('utm', '33U2910005628000')).toBe(true);
    });

    // N and S are BANDS, and the two grid formats read them the same way.
    //
    // This side once kept a narrower class for UTM alone, left over from when
    // those letters were refused as ambiguous. The server had since started
    // issuing links carrying them, so fromParams returned null on a link the
    // server had just written, the click handler stood aside, and clicking a
    // perfectly good UTM coordinate opened the standalone page instead of the
    // sidebar. Nothing failed loudly; the sidebar simply never opened.
    test('accepts the band letters N and S in both grid formats', () => {
        const cases: Array<[LocationFormat, string]> = [
            ['utm', '18S3234784306483'],
            ['utm', '11S3850003769000'],
            ['utm', '11N3850000500000'],
            ['mgrs', '18SUJ2347806483'],
            ['mgrs', '18NUJ2347806483'],
        ];

        for (const [format, value] of cases) {
            expect(isCanonical(format, value), value).toBe(true);
        }
    });

    test('rejects a grid token the server would not have issued', () => {
        const cases: Array<[string, LocationFormat, string]> = [
            ['a band letter that is not one', 'mgrs', '18OUJ2347806483'],
            ['a row letter that is not one', 'mgrs', '18SUI2347806483'],
            ['an odd number of digits', 'mgrs', '18SUJ234780648'],
            ['more digits than the notation has', 'mgrs', '18SUJ234780648312'],
            ['no digits at all', 'mgrs', '18SUJ'],
            ['a three-digit zone', 'mgrs', '118SUJ2347806483'],
            ['spaces, which the canonical form never has', 'mgrs', '18S UJ 23478 06483'],

            ['an easting of the wrong width', 'utm', '33U91000562800'],
            ['a band letter that is not one', 'utm', '11O3850003769000'],
        ];

        for (const [name, format, value] of cases) {
            expect(isCanonical(format, value), name).toBe(false);
        }
    });

    // The shared fixtures, asserted identically in gridFixtures in
    // server/decorators/location/format_test.go. Only the two rows this side
    // computes appear here; the position rows are Go's alone because in the
    // running plugin they are produced in Go alone.
    test('matches the shared fixtures', () => {
        const cases: Array<[LocationFormat, string, string, string]> = [
            ['mgrs', '18SUJ2347806483', '18S UJ 23478 06483',
                '1 m grid, at center'],
            ['mgrs', '32UMV1234', '32U MV 12 34',
                '1 km grid, at center'],
            ['utm', '33U2910005628000', '33U 291000E 5628000N',
                '1 m'],
            ['utm', '18S3234784306483', '18S 323478E 4306483N',
                '1 m'],
        ];

        for (const [format, canonical, grid, resolution] of cases) {
            expect(gridText(format, canonical), canonical).toBe(grid);
            expect(remoteResolutionText(format, canonical), canonical).toBe(resolution);
        }
    });

    test('says how big every size of square is', () => {
        const sizes: Array<[string, string]> = [
            ['18SUJ23', '10 km grid, at center'],
            ['18SUJ2306', '1 km grid, at center'],
            ['18SUJ234064', '100 m grid, at center'],
            ['18SUJ23470648', '10 m grid, at center'],
            ['18SUJ2347806483', '1 m grid, at center'],
        ];

        for (const [canonical, resolution] of sizes) {
            expect(remoteResolutionText('mgrs', canonical), canonical).toBe(resolution);
        }
    });
});

test.describe('the area-reference formats', () => {
    test('has no position without the server', () => {
        expect(parseCanonical('georef', 'GJNJ5753')).toBeNull();
        expect(parseCanonical('gars', '006AG39')).toBeNull();
        expect(parseCanonical('pluscode', '849VCWC8+R9')).toBeNull();
    });

    test('still validates the canonical shape', () => {
        const cases: Array<[LocationFormat, string]> = [
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

        for (const [format, value] of cases) {
            expect(isCanonical(format, value), value).toBe(true);
        }
    });

    test('rejects a code the server would not have issued', () => {
        const cases: Array<[string, LocationFormat, string]> = [
            ['an I in a GEOREF zone', 'georef', 'IJNJ5753'],
            ['a GEOREF band past M', 'georef', 'GNNJ5753'],
            ['a GEOREF degree unit past Q', 'georef', 'GJRJ5753'],
            ['an odd number of GEOREF digits', 'georef', 'GJNJ57533'],
            ['a bare GEOREF quadrangle', 'georef', 'GJNJ'],
            ['lower case, which the canonical form never is', 'georef', 'gjnj5753'],

            ['an I in a GARS letter pair', 'gars', '206IT'],
            ['too few GARS band digits', 'gars', '26LT'],
            ['too many GARS digits', 'gars', '206LT263'],

            // The numeric bounds the Go decoders enforce. The webapp used to
            // take these on shape alone and accept a code the page then 400s
            // on, which is the split TestWebappGridZoneIsBounded exists for.
            ['GEOREF minutes past 59', 'georef', 'GJNJ6053'],
            ['GEOREF tenths past 599', 'georef', 'GJNJ600533'],
            ['GARS band 000', 'gars', '000AA'],
            ['GARS band past 720', 'gars', '721LT'],
            ['a GARS quadrant of 0', 'gars', '206LT0'],
            ['a GARS quadrant past 4', 'gars', '206LT5'],
            ['a GARS keypad cell of 0', 'gars', '206LT20'],

            ['a Plus Code separator in the wrong place', 'pluscode', '849VCW+C8R9'],
            ['a letter outside the alphabet', 'pluscode', '849VCWA8+R9'],
            ['nine significant characters', 'pluscode', '849VCWC8+R'],
            ['past fifteen characters', 'pluscode', '849VCWC8+R9CVWXQ2'],
            ['an odd number of padding zeroes', 'pluscode', '849VC000+'],
            ['a short code', 'pluscode', 'CWC8+R9'],
            ['lower case', 'pluscode', '849vcwc8+r9'],
        ];

        for (const [name, format, value] of cases) {
            expect(isCanonical(format, value), name).toBe(false);
        }
    });

    test('says how big every size of cell is', () => {
        const cases: Array<[LocationFormat, string, string]> = [
            ['georef', 'GJNJ', ''],
            ['georef', 'GJNJ5753', '1.9 km cell, at center'],
            ['georef', 'GJNJ575337', '186 m cell, at center'],
            ['georef', 'GJNJ57533752', '19 m cell, at center'],

            ['gars', '206LT', '55.7 km cell, at center'],
            ['gars', '206LT2', '27.8 km cell, at center'],
            ['gars', '206LT26', '9.3 km cell, at center'],

            ['pluscode', '849V0000+', '111.3 km cell, at center'],
            ['pluscode', '849VCW00+', '5.6 km cell, at center'],
            ['pluscode', '849VCWC8+', '278 m cell, at center'],
            ['pluscode', '849VCWC8+R9', '14 m cell, at center'],
            ['pluscode', '849VCWC8+R9C', '3 m cell, at center'],
        ];

        for (const [format, canonical, resolution] of cases) {
            expect(remoteResolutionText(format, canonical), canonical).toBe(resolution);
        }
    });
});
