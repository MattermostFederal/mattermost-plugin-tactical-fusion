import React from 'react';

import {HONOLULU_METAR, NOTAM_WITH_RADIUS} from './report_fixtures';
import ReportPostBodyHarness from './ReportPostBodyHarness';

import {expect, test} from '../../playwright/ct-coverage';
import {serveMapAssets} from '../decorators/location/map/asset_fixtures';
import {stubFeaturesRoute} from '../features/stub_route';

test.beforeEach(async ({page}) => {
    await stubFeaturesRoute(page, {mapPanel: true, mapInline: false, mapPage: true});
});

test('renders the decoded rows and leaves the report as posted to the sidebar', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness/>);

    await expect(body.getByTestId('avreport-card')).toBeVisible();
    await expect(body.getByTestId('avreport-heading')).toHaveText('METAR PHNL');
    await expect(body.getByTestId('avreport-source')).toHaveCount(0);
    await expect(body.getByTestId('avreport-card')).not.toContainText(HONOLULU_METAR.src);
    await expect(body.getByTestId('avreport-summary')).toHaveCount(0);
    await expect(body.getByTestId('avreport-rows')).toContainText('Issued');
    await expect(body.getByTestId('avreport-rows')).toContainText('30.10 inHg');
    await expect(body.getByTestId('avreport-remarks')).toContainText('automated station');
    await expect(body.getByTestId('avreport-unknown')).toHaveText('Q9999');
    await expect(body.getByRole('button', {name: 'Daniel K. Inouye International Airport'})).toBeVisible();
});

test('the station name opens the airfield panel', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness/>);

    await body.getByRole('button', {name: 'Daniel K. Inouye International Airport'}).click();

    await expect(body.getByTestId('selection')).toHaveText('airport {"key":"icao","code":"PHNL"}');
});

test('Open details opens the report panel', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness/>);

    await body.getByRole('button', {name: 'Open details'}).click();

    await expect(body.getByTestId('selection')).toContainText('avreport-post');
});

test('a multi-line NOTAM shows its decode, not its raw text', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness payload={NOTAM_WITH_RADIUS}/>);

    await expect(body.getByTestId('avreport-heading')).toHaveText('NOTAM PHNL');
    await expect(body.getByTestId('avreport-source')).toHaveCount(0);
    await expect(body.getByTestId('avreport-card')).not.toContainText('Q) PHZH/QMRLC');
    await expect(body.getByTestId('avreport-rows')).toContainText('Effective');
    await expect(body.getByTestId('avreport-unknown')).toBeHidden();
});

test('says when the rows were dropped', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness payload={{...HONOLULU_METAR, rowsDropped: true, rows: [], remarks: [], unknown: []}}/>);

    await expect(body.getByTestId('avreport-degraded')).toBeVisible();
    await expect(body.getByTestId('avreport-source')).toBeVisible();
});

test('compact display keeps the report and drops the detail', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness compactDisplay={true}/>);

    await expect(body.getByTestId('avreport-source')).toBeVisible();
    await expect(body.getByTestId('avreport-rows')).toBeHidden();
});

test('falls back to the text without props', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness payload={null}/>);

    await expect(body.getByTestId('avreport-card')).toBeHidden();
    await expect(body).toContainText('```metar');
});

test('falls back to the text with a version it cannot read', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness version={99}/>);

    await expect(body.getByTestId('avreport-card')).toBeHidden();
});

test('stands down after an edit', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness editAt={5}/>);

    await expect(body.getByTestId('avreport-card')).toBeHidden();
    await expect(body).toContainText(HONOLULU_METAR.src);
});

test('draws no map when the inline map is off', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness/>);

    await expect(body.getByTestId('avreport-card')).toBeVisible();
    await expect(body.getByTestId('avreport-map')).toBeHidden();
});

test('draws the station under the post when the inline map is on', async ({mount, page}) => {
    await stubFeaturesRoute(page, {mapPanel: true, mapInline: true, mapPage: true});
    await serveMapAssets(page);

    const body = await mount(<ReportPostBodyHarness/>);

    await expect(body.getByTestId('avreport-map')).toBeVisible();
    await expect(body.getByRole('button', {name: 'Reset view'})).toBeVisible();
    await expect(body.getByRole('link', {name: 'Open larger'})).toHaveAttribute('href', /map\?post=post0000000000000000000000/);
});

test('draws no map for a station the build cannot place, even with the inline map on', async ({mount, page}) => {
    await stubFeaturesRoute(page, {mapPanel: true, mapInline: true, mapPage: true});

    const body = await mount(<ReportPostBodyHarness payload={{...HONOLULU_METAR, format: '', value: '', region: ''}}/>);

    await expect(body.getByTestId('avreport-card')).toBeVisible();
    await expect(body.getByTestId('avreport-map')).toBeHidden();
});

test('a report with an area on the map makes each decoded row a button that shows it', async ({mount, page}) => {
    await stubFeaturesRoute(page, {mapPanel: true, mapInline: true, mapPage: true});
    await serveMapAssets(page);

    const body = await mount(<ReportPostBodyHarness payload={NOTAM_WITH_RADIUS}/>);

    const rows = body.getByTestId('avreport-row-show');
    await expect(rows).toHaveCount(NOTAM_WITH_RADIUS.rows.length);
    await expect(rows.first()).toHaveAccessibleName(/^Show on the map: /);
    await rows.first().click();
    await expect(body.getByTestId('avreport-map')).toBeVisible();
});

test('rows are plain text for a report with no area, or with the inline map off', async ({mount, page}) => {
    await stubFeaturesRoute(page, {mapPanel: true, mapInline: true, mapPage: true});
    const station = await mount(<ReportPostBodyHarness/>);
    await expect(station.getByTestId('avreport-rows')).toBeVisible();
    await expect(station.getByTestId('avreport-row-show')).toHaveCount(0);
    await station.unmount();

    await stubFeaturesRoute(page, {mapPanel: true, mapInline: false, mapPage: true});
    const hidden = await mount(<ReportPostBodyHarness payload={NOTAM_WITH_RADIUS}/>);
    await expect(hidden.getByTestId('avreport-rows')).toBeVisible();
    await expect(hidden.getByTestId('avreport-row-show')).toHaveCount(0);
});
