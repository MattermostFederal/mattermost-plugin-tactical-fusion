import React, {useState} from 'react';

import {drawsNothing, mapLabel, placed, radiusEllipse, reportColor, reportShapes} from './map';
import type {Report} from './types';

import type {Camera} from '../decorators/location/map/camera';
import LocationMap, {INLINE_MAP_HEIGHT, MAP_HEIGHT} from '../decorators/location/map/LocationMap';
import {useNearViewport} from '../decorators/location/map/near_viewport';
import {overlayPageHref} from '../decorators/location/map/view';
import {INLINE_ID, isRowVisible} from '../decorators/location/rows';
import {withTheme} from '../decorators/theme';
import {useFeatures} from '../features/store';
import {pluginBaseUrl} from '../plugin_url';
import {usePreferences} from '../preferences/store';

export {REPORT_COLOR, drawsNothing, mapLabel} from './map';

export const REPORT_MAP_MAX_WIDTH_PX = 640;

const styles: Record<string, React.CSSProperties> = {
    panel: {marginTop: 16},
    frame: {maxWidth: REPORT_MAP_MAX_WIDTH_PX, padding: '0 12px 8px'},
    reserved: {height: MAP_HEIGHT},
    reservedInline: {height: INLINE_MAP_HEIGHT},
};

function coordinatePageHref(report: Report): string {
    const params = new URLSearchParams({f: report.format, v: report.value});
    return withTheme(`${pluginBaseUrl()}/map?${params.toString()}`);
}

function largerHref(pageEnabled: boolean, report: Report, postId: string | undefined): string | undefined {
    if (!pageEnabled) {
        return undefined;
    }
    return postId ? overlayPageHref(postId) : coordinatePageHref(report);
}

export const ReportMapCanvas: React.FC<{
    report: Report;
    pageEnabled: boolean;
    postId?: string;
    fill?: boolean;
    inline?: boolean;
    openAt?: Camera;
}> = ({report, pageEnabled, postId, fill, inline, openAt}) => {
    const point = placed(report);
    if (point === null) {
        return null;
    }

    return (
        <LocationMap
            lat={point.lat}
            lon={point.lon}
            cellDegLat={0}
            cellDegLon={0}
            region={report.region}
            pending={false}
            markers={[{...point, color: reportColor(report)}]}
            ellipse={radiusEllipse(report)}
            geometries={reportShapes(report)}
            markerLabel={mapLabel(report)}
            pageHref={largerHref(pageEnabled, report, postId)}
            fill={fill}
            inline={inline}
            openAt={openAt}
        />
    );
};

const ReportMap: React.FC<{
    report: Report;
    surface: 'card' | 'panel';
    postId?: string;
}> = ({report, surface, postId}) => {
    const {preferences} = usePreferences();
    const {features} = useFeatures();
    const [box, setBox] = useState<HTMLDivElement | null>(null);
    const near = useNearViewport(box);

    const enabled = surface === 'card' ? features.mapInline && isRowVisible(preferences.location.hiddenRows, INLINE_ID) : features.mapPanel;

    if (!enabled || drawsNothing(report)) {
        return null;
    }

    return (
        <div
            ref={setBox}
            style={surface === 'card' ? styles.frame : styles.panel}
            data-testid='avreport-map'
        >
            {near ? (
                <ReportMapCanvas
                    report={report}
                    pageEnabled={features.mapPage}
                    postId={postId}
                    inline={surface === 'card'}
                />
            ) : <div style={surface === 'card' ? styles.reservedInline : styles.reserved}/>}
        </div>
    );
};

export default ReportMap;
