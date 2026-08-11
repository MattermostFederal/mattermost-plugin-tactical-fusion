const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

function pad(value: number): string {
    return String(value).padStart(2, '0');
}

/**
 * Renders an instant in the token's own zone, e.g. `09 Aug 2026 16:30 Z` or
 * `09 Aug 2026 20:30 +04:00`.
 *
 * Built by hand rather than through Intl so the output cannot shift with the
 * reader's locale. This string is the plain-language reading of a DTG, and two
 * people looking at the same message should see the same words.
 *
 * Kept in a plain module, with no React, so it can be tested without a browser.
 * It must match describeInstant in server/decorators/dtg/page.go, or the sidebar
 * and the standalone page would describe the same instant differently.
 */
export function describeInstant(instant: Date, offsetMinutes: number, zoneLabel: string): string {
    const local = new Date(instant.getTime() + (offsetMinutes * 60000));

    const day = pad(local.getUTCDate());
    const month = MONTHS[local.getUTCMonth()];
    const year = local.getUTCFullYear();
    const time = `${pad(local.getUTCHours())}:${pad(local.getUTCMinutes())}`;

    return `${day} ${month} ${year} ${time} ${zoneLabel}`;
}

/**
 * Renders an offset the way RFC 3339 writes it: `Z`, `+05:30`, `-08:00`.
 *
 * Distinct from `formatZoneOffset` in zones.ts, which writes `UTC+05:30` for
 * the timezone picker. This one is the label on a token, so it has to read the
 * way the token was written.
 *
 * Must match FormatOffset in server/decorators/dtg/parse.go.
 */
export function formatOffsetLabel(minutes: number): string {
    if (minutes === 0) {
        return 'Z';
    }

    const sign = minutes < 0 ? '-' : '+';
    const absolute = Math.abs(minutes);

    return `${sign}${pad(Math.floor(absolute / 60))}:${pad(absolute % 60)}`;
}
