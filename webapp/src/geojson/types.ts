export const GEOJSON_POST_TYPE = 'custom_tf_geojson';

export const GEOJSON_PROPS_KEY = 'tactical_fusion_geojson';

/**
 * What the sidebar keys this panel on.
 *
 * Deliberately not a decorator type, for the reason `COT_PANEL_TYPE` is not:
 * there is no `/decorate/geojson` route and no link carrying this. Reserved
 * here so the value is chosen once.
 */
export const GEOJSON_PANEL_TYPE = 'geojson';

export const GEOJSON_PROPS_VERSION = 1;

/**
 * A version this bundle knows how to read.
 *
 * Declared here rather than borrowed from the Cursor on Target module, whose
 * own list is private to it: the two formats version independently, and a
 * shared list would tie a GeoJSON bump to a CoT one. A version NEITHER entry
 * covers is refused, and the card falls back to the post's own text rather than
 * guessing at a shape it has never seen.
 */
const READABLE_VERSIONS = [1];

export const SOURCE_FENCE = 'fence';
export const SOURCE_FILE = 'file';

/**
 * Every geometry class the server may name.
 *
 * A closed vocabulary, held to the Go side by TestWebappGeoJSONKindsMatch. Both
 * the card and the map dispatch on this string, so a kind named on one side and
 * absent on the other is a feature drawn into no channel at all.
 *
 * `none` is a feature the document gave a null geometry, which RFC 7946 permits
 * and which is listed rather than drawn.
 */
export const GEOJSON_KINDS = [
    'Point',
    'MultiPoint',
    'LineString',
    'MultiLineString',
    'Polygon',
    'MultiPolygon',
    'GeometryCollection',
    'none',
] as const;

export type GeoJsonKind = typeof GEOJSON_KINDS[number];

const KNOWN_KINDS = new Set<string>(GEOJSON_KINDS);

export interface GeoJsonPosition {
    lon: string;
    lat: string;
    alt: string;
}

/**
 * One member of a geometry.
 *
 * `ringCounts` is set only on a MultiPolygon part, where it names how many
 * rings each member polygon contributed, in order. Rings alone cannot carry
 * that, and a MultiPolygon nested in a GeometryCollection stays one part so its
 * grouping survives.
 */
export interface GeoJsonPart {
    kind: GeoJsonKind;
    rings: GeoJsonPosition[][];
    ringCounts: number[];
}

export interface GeoJsonProperty {
    key: string;
    value: string;
}

export interface GeoJsonFeature {
    name: string;
    kind: GeoJsonKind;

    /** Why this feature is not drawn, or '' when it is. Authored by the server. */
    note: string;

    /**
     * The identity the location tools take, for a lone point whose position the
     * location grammar will stand behind.
     *
     * Both empty for everything else: a polygon has no one position, a
     * MultiPoint has several, and a coarsely written coordinate is one the
     * grammar refuses. Both are also empty on a post stamped before this pair
     * existed, which is why the panel checks rather than assuming.
     */
    format: string;
    value: string;

    /**
     * What the geometry works out to, already rendered by the server.
     *
     * Both empty for a geometry that has no such measure, and for a feature the
     * server noted. Rendered rather than computed here so the card and the
     * panel cannot round the same figure into two different answers.
     */
    length: string;
    area: string;

    /**
     * The color this feature asked to be drawn in, already validated by the
     * server to a six-digit hex triple, or '' for the theme's own.
     *
     * Validated AGAIN before it reaches a paint property, because a props blob
     * is not a trusted input either: `styleOf` in LocationMap is the gate the
     * map's own tests hold.
     */
    color: string;

    /**
     * The rest of the simplestyle the feature asked for, as the document's own
     * lexemes, or '' for the theme's.
     *
     * Strings rather than numbers on the wire for the reason `length` and
     * `area` are: the document's text is what both surfaces show, and two
     * renderers rounding one figure separately is how they come to disagree.
     * Every one is validated AGAIN in `styleOf` before it reaches a paint
     * property, because a props blob is not a trusted input either.
     */
    width: string;
    lineOpacity: string;
    fillOpacity: string;
    markerSize: string;

    parts: GeoJsonPart[];
    properties: GeoJsonProperty[];
}

export interface GeoJsonCounts {
    features: number;
    points: number;
    lines: number;
    polygons: number;
    collections: number;
    unlocated: number;
    undrawable: number;
}

export interface GeoJsonPayload {
    source: string;
    lead: string;
    trail: string;
    src: string;
    fileId: string;

    /**
     * The post this was read from, or empty when nothing knows.
     *
     * Not read from props: props do not carry it and could not be trusted for
     * it if they did. The post body sets it, and it rides the payload into the
     * sidebar, which is what lets both surfaces address the map page. Empty is
     * the honest value for a harness or a card built from a payload directly,
     * and it costs exactly the "Open larger" link.
     */
    postId?: string;
    fileName: string;

    /** What the document says about itself, or '' when it says nothing. */
    note: string;

    /**
     * Whether NOTHING in the document can be put on a map.
     *
     * Read as the server's own flag rather than inferred from the note: a
     * malformed bbox carries a note that explicitly says the features ARE still
     * drawn, so branching on "is there a note" blanked the map against its own
     * explanation.
     */
    unplaceable: boolean;

    /**
     * Whether the server dropped the property bags to fit the props budget.
     *
     * A presence key on the wire, because an absent properties array is
     * otherwise indistinguishable from a feature that genuinely carries none.
     */
    propertiesDropped: boolean;

    counts: GeoJsonCounts;
    features: GeoJsonFeature[];
}

function text(blob: Record<string, unknown>, key: string): string {
    const value = Object.hasOwn(blob, key) ? blob[key] : undefined;
    return typeof value === 'string' ? value : '';
}

function count(blob: Record<string, unknown>, key: string): number {
    const value = Object.hasOwn(blob, key) ? blob[key] : undefined;
    return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function record(value: unknown): Record<string, unknown> | null {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
        return null;
    }
    return value as Record<string, unknown>;
}

export function fromProps(props: unknown): GeoJsonPayload | null {
    const outer = record(props);
    if (outer === null) {
        return null;
    }

    const rawBlob = record(outer[GEOJSON_PROPS_KEY]);

    // typeof, not Number(). The loose form accepted true, '1', ' 1 ' and [1]
    // as version 1, which is a forged blob passing the one check that decides
    // whether this build understands the shape at all. count() two dozen lines
    // down already spells it strictly.
    const version = rawBlob === null ? null : rawBlob.version;
    if (rawBlob === null || typeof version !== 'number' || !READABLE_VERSIONS.includes(version)) {
        return null;
    }

    const source = text(rawBlob, 'source');
    if (source !== SOURCE_FENCE && source !== SOURCE_FILE) {
        return null;
    }

    return {
        source,
        lead: text(rawBlob, 'lead'),
        trail: text(rawBlob, 'trail'),
        src: text(rawBlob, 'src'),
        fileId: text(rawBlob, 'file_id'),
        fileName: text(rawBlob, 'file_name'),
        note: text(rawBlob, 'note'),
        unplaceable: text(rawBlob, 'unplaceable') !== '',
        propertiesDropped: text(rawBlob, 'properties_dropped') !== '',
        counts: readCounts(rawBlob),
        features: readFeatures(rawBlob),
    };
}

function readCounts(rawBlob: Record<string, unknown>): GeoJsonCounts {
    const counts = record(rawBlob.counts) ?? {};

    return {
        features: count(counts, 'features'),
        points: count(counts, 'points'),
        lines: count(counts, 'lines'),
        polygons: count(counts, 'polygons'),
        collections: count(counts, 'collections'),
        unlocated: count(counts, 'unlocated'),
        undrawable: count(counts, 'undrawable'),
    };
}

/**
 * The features, exactly as the server wrote them.
 *
 * Nothing here caps or slices. The server already refused every document past
 * its own limits rather than truncating one, so a cap repeated on this side
 * could only disagree with it: a ring cut short here would close onto the wrong
 * vertex and draw a polygon nobody posted.
 * TestWebappGeoJSONTruncatesNothing is the guard.
 */
function readFeatures(rawBlob: Record<string, unknown>): GeoJsonFeature[] {
    const raw = Array.isArray(rawBlob.features) ? rawBlob.features : [];

    const features: GeoJsonFeature[] = [];
    for (const entry of raw) {
        const rawFeature = record(entry);
        if (rawFeature === null) {
            continue;
        }

        features.push({
            name: text(rawFeature, 'name'),
            kind: readKind(rawFeature),
            note: text(rawFeature, 'note'),
            format: text(rawFeature, 'format'),
            value: text(rawFeature, 'value'),
            length: text(rawFeature, 'length'),
            area: text(rawFeature, 'area'),
            color: text(rawFeature, 'color'),
            width: text(rawFeature, 'width'),
            lineOpacity: text(rawFeature, 'line_opacity'),
            fillOpacity: text(rawFeature, 'fill_opacity'),
            markerSize: text(rawFeature, 'marker_size'),
            parts: readParts(rawFeature),
            properties: readProperties(rawFeature),
        });
    }

    return features;
}

function readKind(rawFeature: Record<string, unknown>): GeoJsonKind {
    const kind = text(rawFeature, 'kind');
    return KNOWN_KINDS.has(kind) ? kind as GeoJsonKind : 'none';
}

function readParts(rawFeature: Record<string, unknown>): GeoJsonPart[] {
    const raw = Array.isArray(rawFeature.parts) ? rawFeature.parts : [];

    const parts: GeoJsonPart[] = [];
    for (const entry of raw) {
        const rawPart = record(entry);
        if (rawPart === null) {
            continue;
        }

        parts.push({
            kind: readKind(rawPart),
            rings: readRings(rawPart),
            ringCounts: readRingCounts(rawPart),
        });
    }

    return parts;
}

function readRings(rawPart: Record<string, unknown>): GeoJsonPosition[][] {
    const raw = Array.isArray(rawPart.rings) ? rawPart.rings : [];

    const rings: GeoJsonPosition[][] = [];
    for (const entry of raw) {
        if (!Array.isArray(entry)) {
            continue;
        }

        const ring: GeoJsonPosition[] = [];
        for (const item of entry) {
            const position = record(item);
            if (position === null) {
                continue;
            }
            ring.push({
                lon: text(position, 'lon'),
                lat: text(position, 'lat'),
                alt: text(position, 'alt'),
            });
        }
        rings.push(ring);
    }

    return rings;
}

function readRingCounts(rawPart: Record<string, unknown>): number[] {
    const raw = Array.isArray(rawPart.ring_counts) ? rawPart.ring_counts : [];

    return raw.filter((value): value is number => typeof value === 'number' && Number.isInteger(value) && value > 0);
}

function readProperties(rawFeature: Record<string, unknown>): GeoJsonProperty[] {
    const raw = Array.isArray(rawFeature.properties) ? rawFeature.properties : [];

    const properties: GeoJsonProperty[] = [];
    for (const entry of raw) {
        const property = record(entry);
        if (property === null) {
            continue;
        }
        properties.push({key: text(property, 'key'), value: text(property, 'value')});
    }

    return properties;
}

/**
 * Whether this feature can be handed to the location tools.
 *
 * The pair IS the question, rather than a finiteness test on the coordinates.
 * The server writes it only when it parsed a position the location grammar will
 * stand behind, so a coarse coordinate, a polygon and a post stamped before the
 * pair existed all answer no here and render as plain text.
 */
export function isLinkable(feature: GeoJsonFeature): boolean {
    return feature.format !== '' && feature.value !== '';
}

/** How many positions a feature carries, for the card's counts. */
export function vertexCount(feature: GeoJsonFeature): number {
    return feature.parts.reduce(
        (total, part) => total + part.rings.reduce((sum, ring) => sum + ring.length, 0), 0);
}

/** How many rings a feature carries. */
export function ringCount(feature: GeoJsonFeature): number {
    return feature.parts.reduce((total, part) => total + part.rings.length, 0);
}

/** The sole position of a single-point feature, or null. */
export function solePosition(feature: GeoJsonFeature): GeoJsonPosition | null {
    if (feature.kind !== 'Point' || feature.parts.length !== 1) {
        return null;
    }

    const rings = feature.parts[0].rings;
    if (rings.length !== 1 || rings[0].length !== 1) {
        return null;
    }

    return rings[0][0];
}
