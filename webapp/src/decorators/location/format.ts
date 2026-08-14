/**
 * Rendering a coordinate from its canonical form.
 *
 * There is no grammar here and no projection. The server produced a canonical
 * token of a known fixed shape, and this file slices fixed fields out of it and
 * divides by sixty, which is the same thing `dtg/index.ts` does when it reads a
 * zone letter out of `canonical[6]`.
 *
 * Every value is rendered at the resolution the token carried and never finer.
 * Rounding, never truncation: truncating minutes biases every result up to a
 * whole minute south and west, which is a real bug in the sibling plugin.
 *
 * "Never finer" is a ceiling and not a floor, which is why a VALUE renders per
 * axis while the RESOLUTION row renders for the pair. The two halves need not
 * be written to the same precision: "34.0561N,118.2W" is an ordinary thing to
 * paste, and rendering its latitude at the longitude's one decimal moved it
 * 4.9 km north. Must match `format.go`, which splits the same way.
 */

/** The grammars the server may name in `f`. */
export type LocationFormat =
    'dd' | 'ddh' | 'dms' | 'ddm' | 'latd' | 'latm' | 'vlatm' | 'mgrs' | 'utm';

export const LOCATION_FORMATS: readonly LocationFormat[] = [
    'dd', 'ddh', 'dms', 'ddm', 'latd', 'latm', 'vlatm', 'mgrs', 'utm',
];

/**
 * The two grammars whose position cannot be worked out from the token here.
 *
 * Every other format is arithmetic on digits the author wrote: slice the
 * canonical form into fields and divide by sixty. A grid reference is a
 * projection onto an ellipsoid, and that lives in Go and only in Go, so the
 * panel asks the server for the position of one and renders what it can in the
 * meantime.
 */
export const GRID_FORMATS: readonly LocationFormat[] = ['mgrs', 'utm'];

export function isGridFormat(format: LocationFormat): boolean {
    return GRID_FORMATS.includes(format);
}

/** One half of a coordinate, as it came out of the canonical token. */
export interface Axis {
    decimal: number;

    /** The USMTF verified confidence digit, or null when the token had none. */
    confidence: number | null;

    /**
     * Fractional digits THIS half carried.
     *
     * What the value rows render at, so a finely written half keeps every digit
     * its author wrote even when the other half is coarse.
     */
    digits: number;
}

export interface Coordinate {
    lat: Axis;
    lon: Axis;
    format: LocationFormat;

    /**
     * Fractional digits the token carried, on its coarser half.
     *
     * How finely the PAIR is known, which is what the resolution row quotes.
     * Never what a value row renders at.
     */
    digits: number;
}

/** Length of a degree of latitude, near enough for a human-readable figure. */
const DEGREE_METERS = 111320;

/**
 * The most fractional digits a token may carry. Mirrors `maxFrac` in Go.
 *
 * Load-bearing, not cosmetic. An unbounded `\d+` here reaches `toFixed()`,
 * which throws `RangeError` above 100 digits, and there is no error boundary in
 * this bundle, so one crafted link posted in a channel would crash the panel
 * for every reader who clicked it.
 */
const MAX_FRAC = 8;

/**
 * The latitude band letter, shared by both grid formats.
 *
 * Written once because the server writes it once: `bandBody` in
 * `server/decorators/location/grammar.go` serves MGRS and UTM alike, and the
 * two classes here having drifted apart is exactly how a valid UTM link stopped
 * opening in the sidebar. UTM had kept an older, narrower class that excluded
 * N and S from a time when those letters were refused as ambiguous; the server
 * had since started reading them as bands and issuing links carrying them, so
 * `fromParams` returned null, the click handler stood aside, and the browser
 * followed the link to the standalone page instead.
 *
 * I and O are absent because they read as 1 and 0.
 */
const BAND = '[C-HJ-NP-X]';

/**
 * The UTM zone number, 1 to 60, with or without a leading zero.
 *
 * Written once and shared, for the same reason BAND is. Both grid patterns used
 * a bare `\\d{1,2}`, which accepts `00` and `61` through `99`. Go's scanning
 * grammar is equally loose there, but `gridPoint` refuses those zones on the
 * way to a position, so the server never issues such a link and its page
 * refuses one. Accepting it here was therefore the silent split this file
 * exists to prevent, running the other way: the panel would render a link the
 * page will not.
 */
const ZONE = '(?:0?[1-9]|[1-5][0-9]|60)';

/**
 * The canonical patterns the server emits, one per format.
 *
 * These are anchored and fixed-width. They are not the grammar: the server
 * matched the author's text against something far broader and handed back this.
 */
const CANONICAL: Record<LocationFormat, RegExp> = ((f: string) => ({
    dd: new RegExp(`^(-?)(\\d{1,2})(?:\\.(${f}))?,(-?)(\\d{1,3})(?:\\.(${f}))?$`),
    ddh: new RegExp(`^(\\d{1,2})(?:\\.(${f}))?([NS]),(\\d{1,3})(?:\\.(${f}))?([EW])$`),
    dms: new RegExp(
        `^(\\d{2})(\\d{2})(\\d{2})(?:\\.(${f}))?([NS])(\\d{3})(\\d{2})(\\d{2})(?:\\.(${f}))?([EW])$`),
    ddm: new RegExp(`^(\\d{2})(\\d{2})\\.(${f})([NS])(\\d{3})(\\d{2})\\.(${f})([EW])$`),
    latd: /^(\d{2})([NS])(\d{3})([EW])$/,
    latm: /^(\d{2})(\d{2})([NS])(\d{3})(\d{2})([EW])$/,
    vlatm: /^(\d{2})(\d{2})([NS])(\d)-(\d{3})(\d{2})([EW])(\d)$/,

    // Zone, band, the two 100 km square letters, then an even number of digits
    // split equally between easting and northing. I and O are absent from every
    // class because they read as 1 and 0, and the row letters stop at V.
    mgrs: new RegExp(`^(${ZONE})(${BAND})([A-HJ-NP-Z])([A-HJ-NP-V])((?:\\d{2}){1,5})$`),

    // Zone, band, a six-digit easting and a seven-digit northing.
    utm: new RegExp(`^(${ZONE})(${BAND})(\\d{6})(\\d{7})$`),
}))(`\\d{1,${MAX_FRAC}}`);

/** Latitude and longitude bounds, checked the way the server checks them. */
const MAX_LAT = 90;
const MAX_LON = 180;

/**
 * Whether a parsed pair is a place.
 *
 * The server rejects an out-of-range coordinate outright. Without the same
 * check here the panel renders "91.0000° N, 181.0000° E" as fact while the
 * standalone page 400s on the identical link.
 */
function inRange(lat: number, lon: number): boolean {
    return Number.isFinite(lat) && Number.isFinite(lon) &&
        Math.abs(lat) <= MAX_LAT && Math.abs(lon) <= MAX_LON;
}

/**
 * Whether every sexagesimal field is a legal one.
 *
 * Bounding the resulting degrees is not enough: 34°99'99" is 35.6775, which is
 * in range as a decimal while being nonsense as a token. The server rejects it,
 * so accepting it here would put a coordinate in the panel that the standalone
 * page refuses.
 */
function fieldsOK(...fields: number[]): boolean {
    return fields.every((f) => f >= 0 && f < 60);
}

/** Whether a string is exactly the canonical form for a format. */
export function isCanonical(format: LocationFormat, value: string): boolean {
    const pattern = CANONICAL[format];
    return Boolean(pattern) && pattern.test(value);
}

function sign(hemisphere: string): number {
    return hemisphere === 'S' || hemisphere === 'W' ? -1 : 1;
}

/**
 * Whether a value should render with the southern or western letter.
 *
 * `v < 0` is wrong for negative zero, which is a real coordinate: 0.0000S is
 * on the equator and must not read as N. Go uses `math.Signbit`, and this is
 * the JavaScript equivalent.
 */
function isNegative(v: number): boolean {
    return v < 0 || Object.is(v, -0);
}

function frac(digits: string | undefined): number {
    return digits ? Number(`0.${digits}`) : 0;
}

/**
 * Reads a canonical token into a position.
 *
 * Returns null when the string is not the canonical form for that format, and
 * also for the two grid formats, whose position is a projection rather than a
 * division. Those are not a failure and the caller must not treat them as one:
 * `isCanonical` still vouches for the token, and the position arrives from the
 * conversion endpoint instead.
 */
export function parseCanonical(format: LocationFormat, value: string): Coordinate | null {
    if (isGridFormat(format)) {
        return null;
    }

    const m = CANONICAL[format]?.exec(value);
    if (!m) {
        return null;
    }

    const plain = (decimal: number, digits: number): Axis => ({decimal, confidence: null, digits});

    // Each half keeps its own digit count and the pair takes the coarser, which
    // is the same split `format.go` makes. Collapsing to the minimum here would
    // put the loss back where no renderer could see it had happened.
    const pair = (
        lat: number, lon: number, latDigits: number, lonDigits: number,
    ): Coordinate | null => (inRange(lat, lon) ? {
        lat: plain(lat, latDigits),
        lon: plain(lon, lonDigits),
        format,
        digits: Math.min(latDigits, lonDigits),
    } : null);

    switch (format) {
    case 'dd': {
        const lat = (Number(m[2]) + frac(m[3])) * (m[1] === '-' ? -1 : 1);
        const lon = (Number(m[5]) + frac(m[6])) * (m[4] === '-' ? -1 : 1);
        return pair(lat, lon, len(m[3]), len(m[6]));
    }
    case 'ddh': {
        const lat = (Number(m[1]) + frac(m[2])) * sign(m[3]);
        const lon = (Number(m[4]) + frac(m[5])) * sign(m[6]);
        return pair(lat, lon, len(m[2]), len(m[5]));
    }
    case 'dms': {
        if (!fieldsOK(Number(m[2]), Number(m[3]), Number(m[7]), Number(m[8]))) {
            return null;
        }
        const lat = (Number(m[1]) + (Number(m[2]) / 60) + ((Number(m[3]) + frac(m[4])) / 3600)) * sign(m[5]);
        const lon = (Number(m[6]) + (Number(m[7]) / 60) + ((Number(m[8]) + frac(m[9])) / 3600)) * sign(m[10]);
        return pair(lat, lon, len(m[4]), len(m[9]));
    }
    case 'ddm': {
        if (!fieldsOK(Number(m[2]), Number(m[6]))) {
            return null;
        }
        const lat = (Number(m[1]) + ((Number(m[2]) + frac(m[3])) / 60)) * sign(m[4]);
        const lon = (Number(m[5]) + ((Number(m[6]) + frac(m[7])) / 60)) * sign(m[8]);
        return pair(lat, lon, len(m[3]), len(m[7]));
    }
    case 'latd': {
        return pair(Number(m[1]) * sign(m[2]), Number(m[3]) * sign(m[4]), 0, 0);
    }
    case 'latm': {
        if (!fieldsOK(Number(m[2]), Number(m[5]))) {
            return null;
        }
        return pair(
            (Number(m[1]) + (Number(m[2]) / 60)) * sign(m[3]),
            (Number(m[4]) + (Number(m[5]) / 60)) * sign(m[6]),
            0,
            0,
        );
    }
    case 'vlatm': {
        if (!fieldsOK(Number(m[2]), Number(m[6]))) {
            return null;
        }
        const lat = (Number(m[1]) + (Number(m[2]) / 60)) * sign(m[3]);
        const lon = (Number(m[5]) + (Number(m[6]) / 60)) * sign(m[7]);
        if (!inRange(lat, lon)) {
            return null;
        }
        return {
            lat: {decimal: lat, confidence: Number(m[4]), digits: 0},
            lon: {decimal: lon, confidence: Number(m[8]), digits: 0},
            format,
            digits: 0,
        };
    }
    default:
        return null;
    }
}

function len(s: string | undefined): number {
    return s ? s.length : 0;
}

/**
 * The angular size of the smallest field the PAIR carried, in degrees.
 *
 * That is the coarser half, which is what the resolution row quotes. Not what a
 * value row renders at: see `axisResolutionDegrees`.
 */
export function resolutionDegrees(c: Coordinate): number {
    return resolutionAt(c.format, c.digits);
}

/**
 * The angular size of the smallest field ONE HALF carried.
 *
 * The grammars with no fraction at all fall through to the pair's figure, which
 * for them is the same number. Mirrors `axisResolutionDegrees` in `format.go`.
 */
function axisResolutionDegrees(c: Coordinate, a: Axis): number {
    switch (c.format) {
    case 'dd':
    case 'ddh':
    case 'ddm':
    case 'dms':
        return resolutionAt(c.format, a.digits);
    default:
        return resolutionDegrees(c);
    }
}

/** The angular size a format's smallest field has at a given digit count. */
function resolutionAt(format: LocationFormat, digits: number): number {
    switch (format) {
    case 'dd':
    case 'ddh':
        return Math.pow(10, -digits);
    case 'latd':
        return 1;
    case 'latm':
    case 'vlatm':
        return 1 / 60;
    case 'ddm':
        return Math.pow(10, -digits) / 60;
    case 'dms':
        return Math.pow(10, -digits) / 3600;
    default:
        return 1;
    }
}

/**
 * How finely the token was written, in words.
 *
 * "Written to about", never "accurate to". A phone emitting six decimals is not
 * claiming a tenth of a meter of accuracy, and a row headed "precision" invites
 * exactly that misreading.
 */
export function resolutionText(c: Coordinate): string {
    const meters = resolutionDegrees(c) * DEGREE_METERS;

    // Below a centimeter the figure rounds to "0 m", which reads as a claim of
    // infinite precision. Must match ResolutionText in format.go.
    if (meters < 0.01) {
        return 'finer than 0.01 m';
    }

    return `about ${humanMeters(meters)}`;
}

function humanMeters(m: number): string {
    if (m >= 1000) {
        return `${trimZeroes((m / 1000).toFixed(1))} km`;
    }
    if (m >= 1) {
        return `${m.toFixed(0)} m`;
    }

    // resolutionText refuses anything below a centimeter before it reaches
    // here, so two decimals is always enough to say something.
    return `${trimZeroes(m.toFixed(2))} m`;
}

function trimZeroes(s: string): string {
    return s.includes('.') ? s.replace(/0+$/, '').replace(/\.$/, '') : s;
}

/**
 * A grid token written for a reader: spaced into its parts, which is how one is
 * read aloud and checked against a map.
 *
 * Slicing a canonical form, not a grammar and not a projection. The server
 * writes the same characters without the spaces, so this and the canonical
 * token differ only in whitespace. Mirrors `gridText` in `format.go`.
 */
export function gridText(format: LocationFormat, canonical: string): string {
    const m = CANONICAL[format]?.exec(canonical);
    if (!m) {
        return '';
    }

    if (format === 'utm') {
        return `${m[1]}${m[2]} ${m[3]}E ${m[4]}N`;
    }
    if (format !== 'mgrs') {
        return '';
    }

    const digits = m[5];
    const half = digits.length / 2;
    return `${m[1]}${m[2]} ${m[3]}${m[4]} ${digits.slice(0, half)} ${digits.slice(half)}`;
}

/**
 * How finely a grid token was written.
 *
 * Computed here rather than taken from the conversion, so it is on screen
 * immediately and stays there even if the request never lands. It needs only
 * the digit count, which is in the token.
 *
 * Must match `ResolutionText` in `format.go`.
 */
export function gridResolutionText(format: LocationFormat, canonical: string): string {
    if (format === 'utm') {
        // Exact rather than hedged: a UTM easting is written to the meter.
        return '1 m';
    }

    const m = CANONICAL.mgrs.exec(canonical);
    if (!m) {
        return '';
    }

    const meters = 100000 / Math.pow(10, m[5].length / 2);
    return `${humanMeters(meters)} grid, at center`;
}

function decimalPlaces(c: Coordinate, a: Axis): number {
    return clampPlaces(Math.ceil(-Math.log10(axisResolutionDegrees(c, a))));
}

/** `toFixed` throws above 100, so no computed width may reach it unbounded. */
function clampPlaces(places: number): number {
    if (!Number.isFinite(places) || places < 0) {
        return 0;
    }
    return Math.min(places, 20);
}

/** The pair as decimal degrees with hemisphere letters. */
export function decimalText(c: Coordinate): string {
    return `${axisDecimal(c.lat.decimal, decimalPlaces(c, c.lat), 'N', 'S')}, ` +
        `${axisDecimal(c.lon.decimal, decimalPlaces(c, c.lon), 'E', 'W')}`;
}

function axisDecimal(v: number, places: number, pos: string, neg: string): string {
    return `${Math.abs(v).toFixed(places)}° ${isNegative(v) ? neg : pos}`;
}

/** The pair as degrees, minutes and seconds, at the token's own resolution. */
export function dmsText(c: Coordinate): string {
    return `${axisDMS(c.lat.decimal, axisResolutionDegrees(c, c.lat), 'N', 'S')} ` +
        `${axisDMS(c.lon.decimal, axisResolutionDegrees(c, c.lon), 'E', 'W')}`;
}

function axisDMS(v: number, resolution: number, pos: string, neg: string): string {
    const hemi = isNegative(v) ? neg : pos;
    const abs = Math.abs(v);

    if (resolution >= 1) {
        return `${Math.round(abs)}°${hemi}`;
    }

    if (resolution >= 1 / 60) {
        const {deg, min} = splitMinutes(abs);
        return `${deg}°${pad2(min)}'${hemi}`;
    }

    const places = resolution < 1 / 3600 ? clampPlaces(Math.ceil(-Math.log10(resolution * 3600))) : 0;
    const {deg, min, sec} = splitSeconds(abs, places);
    return `${deg}°${pad2(min)}'${padField(sec, places)}"${hemi}`;
}

/** The pair as degrees and decimal minutes. */
export function ddmText(c: Coordinate): string {
    return `${axisDDM(c.lat.decimal, axisResolutionDegrees(c, c.lat), 'N', 'S')} ` +
        `${axisDDM(c.lon.decimal, axisResolutionDegrees(c, c.lon), 'E', 'W')}`;
}

function axisDDM(v: number, resolution: number, pos: string, neg: string): string {
    const hemi = isNegative(v) ? neg : pos;
    const abs = Math.abs(v);

    if (resolution >= 1) {
        return `${Math.round(abs)}°${hemi}`;
    }

    const places = resolution < 1 / 60 ? clampPlaces(Math.ceil(-Math.log10(resolution * 60))) : 0;
    const deg = Math.floor(abs);
    let min = round((abs - deg) * 60, places);
    let carried = deg;
    if (min >= 60) {
        min -= 60;
        carried += 1;
    }

    return `${carried}°${padField(min, places)}'${hemi}`;
}

function splitMinutes(abs: number): {deg: number; min: number} {
    const deg = Math.floor(abs);
    const min = Math.round((abs - deg) * 60);
    if (min >= 60) {
        return {deg: deg + 1, min: min - 60};
    }
    return {deg, min};
}

function splitSeconds(abs: number, places: number): {deg: number; min: number; sec: number} {
    let deg = Math.floor(abs);
    const rest = (abs - deg) * 60;
    let min = Math.floor(rest);
    let sec = round((rest - min) * 60, places);

    if (sec >= 60) {
        sec -= 60;
        min += 1;
    }
    if (min >= 60) {
        min -= 60;
        deg += 1;
    }
    return {deg, min, sec};
}

function round(v: number, places: number): number {
    const scale = Math.pow(10, places);
    return Math.round(v * scale) / scale;
}

function pad2(v: number): string {
    return String(v).padStart(2, '0');
}

/** Keeps the leading zero on a two-digit field once a decimal point is added. */
function padField(v: number, places: number): string {
    const fixed = v.toFixed(places);
    const [whole, rest] = fixed.split('.');
    const padded = whole.padStart(2, '0');
    return rest === undefined ? padded : `${padded}.${rest}`;
}

/**
 * The pair as a USMTF compact coordinate.
 *
 * USMTF names a family rather than a format, so the shape follows the token's
 * resolution: the coarsest field set whose step is no coarser than what the
 * author wrote. Padding a field nobody wrote is a claim.
 *
 *   whole degrees         LATD    35N079W
 *   whole minutes         LATM    3510N07901W
 *   whole seconds         LATS    400948N1221400W
 *   finer than a second   LATDS   331000.0N1183000.0W
 *
 * Sized from the PAIR rather than per axis, unlike the DMS and DDM rows above:
 * a USMTF token is one fixed-width shape covering both halves, so there is no
 * spelling of it in which latitude carries seconds and longitude only minutes.
 *
 * Mirrors `USMTFText` in `format.go`, and the shared fixtures hold the two to
 * the same strings.
 */
export function usmtfText(c: Coordinate): string {
    const resolution = resolutionDegrees(c);

    return axisUSMTF(c.lat.decimal, resolution, 2, 'N', 'S') +
        axisUSMTF(c.lon.decimal, resolution, 3, 'E', 'W');
}

/**
 * One half at a fixed width.
 *
 * degWidth is 2 for latitude and 3 for longitude, which is the whole difference
 * between the two halves of every shape in the family.
 */
function axisUSMTF(v: number, resolution: number, degWidth: number, pos: string, neg: string): string {
    const hemi = isNegative(v) ? neg : pos;
    const abs = Math.abs(v);

    if (resolution >= 1) {
        return `${padDegrees(Math.round(abs), degWidth)}${hemi}`;
    }

    if (resolution >= 1 / 60) {
        const {deg, min} = splitMinutes(abs);
        return `${padDegrees(deg, degWidth)}${pad2(min)}${hemi}`;
    }

    // Fractional seconds only when the token was written finer than one, which
    // is what separates LATDS from LATS.
    const places = resolution < 1 / 3600 ? clampPlaces(Math.ceil(-Math.log10(resolution * 3600))) : 0;
    const {deg, min, sec} = splitSeconds(abs, places);

    return `${padDegrees(deg, degWidth)}${pad2(min)}${padField(sec, places)}${hemi}`;
}

function padDegrees(v: number, width: number): string {
    return String(v).padStart(width, '0');
}

/** The USMTF verified digits, or "" when the token carried none. */
export function confidenceText(c: Coordinate): string {
    if (c.lat.confidence === null || c.lon.confidence === null) {
        return '';
    }
    return `stated confidence ${c.lat.confidence} (latitude), ${c.lon.confidence} (longitude)`;
}
