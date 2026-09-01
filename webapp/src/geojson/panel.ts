import GeoJsonPanel, {GeoJsonTitle, PANEL_TITLE} from './GeoJsonPanel';
import type {GeoJsonPayload} from './types';
import {GEOJSON_PANEL_TYPE} from './types';

import {openRhs, setSelection} from '../decorators/selection';
import {getPanel, registerPanel} from '../panels';

/*
 * Deliberately NOT index.ts. tsconfig sets baseUrl to ./src, so a directory
 * named geojson with an index would make the bare specifier `geojson` resolve
 * here instead of to the npm package of the same name, which is where the
 * FeatureCollection and Feature types the map modules import come from.
 */

/**
 * Registers the GeoJSON sidebar panel.
 *
 * Idempotent, for the reason `registerCotPanel` is: the table lives in module
 * state that survives a plugin re-registration while `initialize()` runs again,
 * and throwing on the second pass would leave the sidebar dead until a page
 * reload.
 */
export function registerGeoJsonPanel(): void {
    if (getPanel(GEOJSON_PANEL_TYPE)) {
        return;
    }

    registerPanel(GEOJSON_PANEL_TYPE, {
        Panel: GeoJsonPanel,
        Title: GeoJsonTitle,
        summary: () => PANEL_TITLE,
    });
}

/** Opens the sidebar on one document. */
export function showGeoJsonDocument(payload: GeoJsonPayload): void {
    setSelection({type: GEOJSON_PANEL_TYPE, payload});
    openRhs();
}
