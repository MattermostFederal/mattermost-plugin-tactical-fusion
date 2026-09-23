import type {Report} from './types';
import {headingOf, isPlaced} from './types';

import {positionPayload} from '../decorators/airport/map';
import type {MapEllipse} from '../decorators/location/map/overlay';
import type {MapShape} from '../decorators/location/map/paint';
import {isRenderable} from '../decorators/location/map/span';

export const METERS_PER_NAUTICAL_MILE = 1852;

export const REPORT_COLOR = '#2e7d9a';

export function placed(report: Report): {lat: number; lon: number} | null {
    if (!isPlaced(report)) {
        return null;
    }
    const coord = positionPayload({format: report.format, value: report.value})?.coord;
    if (!coord || !isRenderable(coord.lat.decimal)) {
        return null;
    }
    return {lat: coord.lat.decimal, lon: coord.lon.decimal};
}

export function radiusEllipse(report: Report): MapEllipse | undefined {
    if (report.radiusNm === '') {
        return undefined;
    }
    const radius = Number(report.radiusNm);
    if (!Number.isFinite(radius) || radius <= 0) {
        return undefined;
    }
    const meters = radius * METERS_PER_NAUTICAL_MILE;
    return {major: meters, minor: meters, angle: 0, color: REPORT_COLOR};
}

export function reportShapes(report: Report): MapShape[] {
    if (report.area.length < 3) {
        return [];
    }
    const ring: Array<{lat: number; lon: number}> = [];
    for (const vertex of report.area) {
        const coord = positionPayload({format: 'dd', value: vertex})?.coord;
        if (!coord || !isRenderable(coord.lat.decimal)) {
            return [];
        }
        ring.push({lat: coord.lat.decimal, lon: coord.lon.decimal});
    }
    return [{rings: [ring], closed: true, color: REPORT_COLOR}];
}

export function drawsNothing(report: Report): boolean {
    return placed(report) === null;
}

export function mapLabel(report: Report): string {
    const what = report.stationName === '' ? headingOf(report) : `${report.stationName} (${report.station})`;
    if (reportShapes(report).length > 0) {
        return `${what}, restricted area`;
    }
    return radiusEllipse(report) === undefined ? what : `${what}, ${report.radiusNm} NM radius`;
}
