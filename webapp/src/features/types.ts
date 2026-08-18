/**
 * Which optional surfaces this install has left on.
 *
 * Mirrors `featuresResponse` in `server/api.go`, and is held to it by
 * `TestWebappFeatureShapeMatches` on names, types and order. That test is not
 * ceremony: every field here is read as a boolean, so a name that drifts reads
 * `undefined`, which is falsy, and the symptom is a map that silently stops
 * being drawn on an install that never turned it off.
 *
 * Per surface rather than per setting, because the parent switches are ANDed in
 * Go beside `locationFormats`. Two implementations of "is this on" is two things
 * that can disagree, and this one would disagree about whether a reader sees
 * anything at all.
 */
export interface Features {

    /** The map in the sidebar panel and in the hover card. */
    mapPanel: boolean;

    /** The map under a message whose whole text is one coordinate. */
    mapInline: boolean;

    /** The standalone full-window map page, and the "Open larger" link to it. */
    mapPage: boolean;
}

/**
 * Every surface off, which is what is assumed until the server has answered.
 *
 * Deliberately not the other way round. Assuming a map is wanted and correcting
 * afterwards would pull the basemap archive, its fonts and the map library once
 * per tab on exactly the installs the switch exists for, which is the entire
 * thing an admin turns it off to avoid. The cost is that a map appears a moment
 * after the panel does, on the same beat as the conversion it is drawn from.
 */
export const NO_FEATURES: Features = {
    mapPanel: false,
    mapInline: false,
    mapPage: false,
};

/**
 * Every surface on, which is what a FAILED read falls back to.
 *
 * The opposite default from the one above, and the pair is the point. "Not
 * answered yet" is a moment and resolves itself; "could not be answered" could
 * last, and nothing may fail the panel into permanently hiding a feature the
 * admin is paying for. A reader whose server is unreachable has a broken map
 * either way, and a broken map says so on its own.
 */
export const ALL_FEATURES: Features = {
    mapPanel: true,
    mapInline: true,
    mapPage: true,
};

/**
 * Reads the server's answer, refusing anything that is not a boolean.
 *
 * Strict rather than coerced, and it throws rather than defaulting a field:
 * `Boolean(undefined)` is a confident `false`, so a coercing reader would turn
 * a shape disagreement into "the admin turned maps off" and give nobody
 * anywhere a reason. Throwing lands in the load path's catch, which falls back
 * to ALL_FEATURES and reports the error.
 */
export function fromWire(payload: unknown): Features {
    // Array.isArray as well as the typeof, because typeof [] is 'object'. An
    // array would be refused a line later anyway, when its numeric keys fail to
    // produce a boolean, but refusing it here is deliberate rather than a
    // consequence of how JSON happens to be shaped.
    if (payload === null || typeof payload !== 'object' || Array.isArray(payload)) {
        throw new Error('The server sent something that is not a feature list.');
    }

    const wire = payload as Record<string, unknown>;

    return {
        mapPanel: asBoolean(wire, 'map_panel'),
        mapInline: asBoolean(wire, 'map_inline'),
        mapPage: asBoolean(wire, 'map_page'),
    };
}

function asBoolean(wire: Record<string, unknown>, key: string): boolean {
    // hasOwn, so a name that happens to exist on Object.prototype is read as
    // absent rather than inherited. None of these three collide today; the
    // repository has been bitten by exactly this once already, when
    // CANONICAL['toString'] resolved up the chain to a function.
    const value = Object.hasOwn(wire, key) ? wire[key] : undefined;
    if (typeof value !== 'boolean') {
        throw new Error(`The server sent no ${key}.`);
    }

    return value;
}
