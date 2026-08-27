/**
 * How long to wait before looking at a stale time again, and whether reaching
 * that wait settles it.
 *
 * setTimeout keeps its delay in a signed 32-bit integer, so a delay past
 * MAX_TIMEOUT_MS wraps and the timer fires at once. A geofence or a standing
 * marker therefore opened the panel already declaring itself stale, which is
 * the card claiming the opposite of what the event says. Waiting the cap and
 * asking again costs one timer per 24.8 days and cannot make that claim.
 */
export const MAX_TIMEOUT_MS = 2_147_483_647;

export interface StaleWait {

    /** What to pass to setTimeout. */
    ms: number;

    /** Whether the event is stale once that wait elapses. */
    settles: boolean;
}

export function staleWait(remaining: number): StaleWait {
    if (remaining > MAX_TIMEOUT_MS) {
        return {ms: MAX_TIMEOUT_MS, settles: false};
    }

    return {ms: remaining, settles: true};
}
