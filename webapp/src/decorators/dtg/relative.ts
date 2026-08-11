/**
 * How close to the instant counts as urgent, in either direction, for a reader
 * who has not chosen otherwise.
 *
 * Must match urgentWithin in server/decorators/dtg/page.go and the threshold in
 * that page's countdown script. Those two have no reader to ask: the standalone
 * page is served without a session, so it always uses this default even when
 * the reader has set something else in the sidebar.
 */
export const DEFAULT_URGENT_WITHIN_MS = 30 * 60 * 1000;

/** Whether an instant is close enough to call for attention. */
export function isUrgent(now: Date, target: Date, withinMs: number = DEFAULT_URGENT_WITHIN_MS): boolean {
    return Math.abs(target.getTime() - now.getTime()) <= withinMs;
}

/**
 * Renders a counting offset between two instants.
 *
 * Seconds are always shown so the value visibly ticks. Once a larger unit
 * appears every smaller one is shown with it, including zeroes, so the display
 * counts down like a clock rather than jumping between widths as units drop
 * out. `in 1h 0m 0s` is deliberate.
 *
 * Kept in a plain module, with no React, so it can be tested without a browser.
 * It must produce the same strings as `relativeTo` in
 * `server/decorators/dtg/page.go` and the countdown script on the standalone
 * page, or the sidebar and the page would disagree about the same instant.
 */
export function formatRelative(now: Date, target: Date): string {
    const diffMs = target.getTime() - now.getTime();
    const total = Math.floor(Math.abs(diffMs) / 1000);
    if (total === 0) {
        return 'now';
    }

    const days = Math.floor(total / 86400);
    const hours = Math.floor((total % 86400) / 3600);
    const minutes = Math.floor((total % 3600) / 60);
    const seconds = total % 60;

    const parts: string[] = [];
    if (days > 0) {
        parts.push(`${days}d`);
    }
    if (days > 0 || hours > 0) {
        parts.push(`${hours}h`);
    }
    if (days > 0 || hours > 0 || minutes > 0) {
        parts.push(`${minutes}m`);
    }
    parts.push(`${seconds}s`);

    const joined = parts.join(' ');
    return diffMs < 0 ? `${joined} ago` : `in ${joined}`;
}
