import {isRenderable} from './span';

export const LOADING = 'Loading map…';

export const NO_POSITION = 'The position for this coordinate is unavailable.';

const TOO_FAR_NORTH = 'This position is too far north for the map.';

const TOO_FAR_SOUTH = 'This position is too far south for the map.';

/**
 * What the reader is told, which is not the same question as whether a map
 * exists: one can be drawn and still have nothing to point at.
 */
export function positionNote(
    beyond: string | null, known: boolean, pending: boolean, loaded: boolean,
): string | null {
    if (beyond) {
        return beyond;
    }
    if (!known) {
        return pending ? LOADING : NO_POSITION;
    }
    return loaded ? null : LOADING;
}

export function outsideMercator(lat: number): string | null {
    if (isRenderable(lat)) {
        return null;
    }

    return lat > 0 ? TOO_FAR_NORTH : TOO_FAR_SOUTH;
}

function accuracyClause(reading: string | undefined): string {
    if (reading === undefined || reading === '') {
        return '';
    }

    return ` The stated circular error is ${reading}.`;
}

function markerClause(what: string | undefined, marked: number): string {
    if (what === undefined || what === '') {
        return '';
    }

    if (marked > 1) {
        return ` The markers are ${what}.`;
    }

    return ` The marker is ${what}.`;
}

/**
 * The accessible label, which is the only place the country reaches a reader.
 *
 * The Region row was retired, so this string is it: with the map hidden, or
 * read by eye rather than by a screen reader, there is no country anywhere.
 * Removing the region from here removes it from every surface at once.
 *
 * The basemap is not named. The region's own value carries its citation, and
 * naming the source again here printed it twice in one line.
 */
export function label(
    region: string, note: string | null, reading?: string, what?: string, marked = 1,
    extent?: string,
): string {
    if (note !== null) {
        return note;
    }

    // An extent-only map has no position to have marked, so every clause below
    // would be a claim it cannot support. What the overlay IS takes their place,
    // because words are the only channel a map has for a reader who gets no
    // picture.
    if (extent !== undefined) {
        return `World map showing ${extent}.`;
    }

    const clauses = `${markerClause(what, marked)}${accuracyClause(reading)}`;

    if (marked > 1) {
        return `World map with ${marked} positions marked.${clauses}`;
    }

    if (region === '') {
        return `World map with the position marked.${clauses}`;
    }

    return `World map. The marked position is in ${region}.${clauses}`;
}
