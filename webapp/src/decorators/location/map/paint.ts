import type {Map as MapLibreMap} from 'maplibre-gl';

import {mapColors} from './maplibre';

/**
 * One shape, with its rings.
 *
 * Ring 0 is the exterior and the rest are holes. Closed says whether it encloses
 * an area at all: a line has one ring and is drawn as a LineString.
 */
export interface MapShape {
    rings: ReadonlyArray<ReadonlyArray<{lat: number; lon: number}>>;
    closed: boolean;

    /**
     * What this one shape is drawn in, or absent for the theme's own color.
     *
     * Validated by `styleOf` before it reaches the collection: a color that is
     * not a hex triple carries no paint at all rather than being passed to
     * MapLibre for the browser to interpret. That gate is the only one there is
     * on this path.
     */
    color?: string;

    /**
     * The rest of the stated style, as the document's own lexemes.
     *
     * Strings, not numbers, because they arrive as strings and `styleOf` is
     * where they stop being text. Parsing at the boundary and validating later
     * would leave a NaN traveling as a number, which reads as a value.
     */
    width?: string;
    lineOpacity?: string;
    fillOpacity?: string;
}

const GEOMETRY_FILL_ALPHA = 0.16;

const HEX_COLOR = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i;

export function fillOf(color: string, alpha = GEOMETRY_FILL_ALPHA): string | null {
    const parsed = HEX_COLOR.exec(color);
    if (parsed === null) {
        return null;
    }

    const [red, green, blue] = parsed.slice(1).map((part) => parseInt(part, 16));
    return `rgba(${red}, ${green}, ${blue}, ${alpha})`;
}

/** The default the theme draws an outline at, and the widest this build draws. */
const GEOMETRY_LINE_WIDTH = 2;
const MAX_LINE_WIDTH = 10;

/**
 * A stated lexeme as a number, or null for anything that is not one.
 *
 * The second gate. The server has already refused what it will not stand
 * behind, and this refuses it again on its own terms, because a props blob is
 * not a trusted input: `Number('')` is 0 and `Number('1e999')` is Infinity, and
 * either one reaching a paint property is a rendering nobody asked for.
 */
export function numberWithin(raw: string | undefined, low: number, high: number): number | null {
    if (raw === undefined || raw.trim() === '') {
        return null;
    }

    const value = Number(raw);
    if (!Number.isFinite(value) || value < low || value > high) {
        return null;
    }

    return value;
}

/*
 * The overlay's paint, read from each feature.
 *
 * Data-driven rather than scalar, because a document may color its features
 * differently and one layer has one scalar. The values it reads are written by
 * `styleOf` below, which puts BOTH of them through `fillOf`, so what reaches
 * MapLibre is a color this build composed rather than a string an author wrote.
 *
 * The fill is precomputed rather than derived by an expression. `fillOf`
 * composites at GEOMETRY_FILL_ALPHA, and an expression reading the line color
 * straight into `fill-color` would paint every shape opaque: a visible
 * regression on the Cursor on Target card, which states its own color.
 */
export function paintGeometry(instance: MapLibreMap): void {
    const colors = mapColors();

    instance.setPaintProperty('geometry-outline', 'line-color',
        ['coalesce', ['get', 'color'], colors.cell]);
    instance.setPaintProperty('geometry-fill', 'fill-color',
        ['coalesce', ['get', 'fill'], colors.cellFill]);

    // Width and line opacity are their own paint properties, where the fill's
    // opacity is composited into `fill` by `styleOf`. That asymmetry is
    // deliberate and is recorded on paintGeometry above: an expression reading
    // the line color straight into `fill-color` paints every shape opaque, so
    // the fill's alpha has to travel inside the color it belongs to.
    instance.setPaintProperty('geometry-outline', 'line-width',
        ['coalesce', ['get', 'width'], GEOMETRY_LINE_WIDTH]);
    instance.setPaintProperty('geometry-outline', 'line-opacity',
        ['coalesce', ['get', 'lineOpacity'], 1]);
}

/**
 * The paint properties one shape carries, or nothing.
 *
 * The single gate on this path. `fillOf` returns null for anything that is not
 * a hex triple, and a shape that fails it carries no color at all, so the
 * `coalesce` above falls back to the theme. A value that reaches MapLibre is
 * therefore always one of this build's own strings.
 */
export function styleOf(shape: {
    color?: string;
    width?: string;
    lineOpacity?: string;
    fillOpacity?: string;
}): {color?: string; fill?: string; width?: number; lineOpacity?: number} {
    const style: {color?: string; fill?: string; width?: number; lineOpacity?: number} = {};

    const width = numberWithin(shape.width, 0, MAX_LINE_WIDTH);
    if (width !== null && width > 0) {
        style.width = width;
    }

    const lineOpacity = numberWithin(shape.lineOpacity, 0, 1);
    if (lineOpacity !== null) {
        style.lineOpacity = lineOpacity;
    }

    if (shape.color === undefined) {
        return style;
    }

    // A stated fill opacity replaces the theme's compositing alpha rather than
    // multiplying with it. A document asking for 0.25 means a quarter, not a
    // quarter of the 0.16 an unstyled shape gets.
    const alpha = numberWithin(shape.fillOpacity, 0, 1);
    const fill = fillOf(shape.color, alpha ?? GEOMETRY_FILL_ALPHA);
    if (fill === null) {
        return style;
    }

    style.color = shape.color;
    style.fill = fill;

    return style;
}
